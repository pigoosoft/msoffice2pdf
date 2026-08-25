//go:build windows

package converter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// PowerPoint PpAlertLevel: ppAlertsNone = 1
const ppAlertsNone = 1

// Excel xlFixedFormatType: xlTypePDF = 0
const excelTypePDF = 0

func newConverter(opts Options) Converter {
	engines := map[string]Engine{}
	hasCOM := false
	for _, n := range opts.Engines {
		if n == EngineMSOffice || n == EngineWPSOffice {
			hasCOM = true
		}
	}
	var comConv Converter
	if hasCOM {
		comConv = newCOMOrSubprocess(opts)
	}
	var comBackend Engine
	if hasCOM {
		comBackend = &comBackendEngine{inner: comConv}
	}
	for _, n := range opts.Engines {
		switch n {
		case EngineMSOffice, EngineWPSOffice:
			engines[n] = comBackend
		case EngineOpenOffice:
			engines[n] = &openOfficeEngine{
				command:     opts.OpenOfficeCommand,
				userProfile: opts.OpenOfficeUserProfile,
			}
		case EngineOFD:
			engines[n] = &ofdEngine{}
		}
	}
	return &routingConverter{engines: engines, extEngines: opts.ExtEngines}
}

func newCOMOrSubprocess(opts Options) Converter {
	if opts.ComMode == "inprocess" {
		return &comConverter{
			excelPageFit: opts.ExcelPageFit,
			tempSandbox:  opts.TempSandbox,
			extEngines:   opts.ExtEngines,
			extAppKinds:  opts.ExtAppKinds,
		}
	}
	return newSubprocessConverter(opts)
}

type comConverter struct {
	excelPageFit string
	tempSandbox  bool
	extEngines   map[string]string
	extAppKinds  map[string]AppKind
}

// officeHandle holds the Application dispatch and PID for timeout cleanup.
// Timeout must kill by PID only — COM Quit from another OS thread is unsafe (STA).
type officeHandle struct {
	mu    sync.Mutex
	app   *ole.IDispatch
	pid   uint32
	image string // e.g. EXCEL.EXE — used if pid never resolved
}

func (h *officeHandle) set(app *ole.IDispatch, pid uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.app = app
	if pid != 0 {
		h.pid = pid
	}
}

func (h *officeHandle) setImage(image string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.image = image
}

func (h *officeHandle) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.app = nil
	h.pid = 0
}

func (h *officeHandle) forceStop() {
	// Do not call COM Quit here: Convert's select runs on another OS thread and
	// STA objects must not be used across threads. Kill by PID only.
	h.mu.Lock()
	pid := h.pid
	image := h.image
	h.mu.Unlock()

	if pid != 0 {
		if err := terminatePID(pid); err != nil {
			slog.Warn("converter: terminate office pid failed", "pid", pid, "err", err)
		}
		return
	}
	if image != "" {
		n := killAllByImage(image)
		slog.Warn("converter: timeout with no office pid; killed by image", "image", image, "count", n)
		return
	}
	slog.Warn("converter: timeout with no office pid yet; waiting for convert goroutine")
}

// resolveOfficePID prefers a newly appeared process image; falls back to HWND.
func resolveOfficePID(app *ole.IDispatch, image string, before map[uint32]struct{}) uint32 {
	if pid := findNewPID(before, image); pid != 0 {
		return pid
	}
	return captureAppPID(app)
}

func (c *comConverter) Convert(ctx context.Context, srcPath, dstPath, password string) error {
	if c.tempSandbox {
		dir, err := createTempSandboxDir()
		if err != nil {
			return err
		}
		defer removeTempSandbox(dir)
		oldTemp, oldTmp := os.Getenv("TEMP"), os.Getenv("TMP")
		_ = os.Setenv("TEMP", dir)
		_ = os.Setenv("TMP", dir)
		defer func() {
			_ = os.Setenv("TEMP", oldTemp)
			_ = os.Setenv("TMP", oldTmp)
		}()
	}

	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("converter: abs src: %w", err)
	}
	dstPath, err = filepath.Abs(dstPath)
	if err != nil {
		return fmt.Errorf("converter: abs dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("converter: mkdir dst: %w", err)
	}
	_ = os.Remove(dstPath)

	ext := strings.ToLower(filepath.Ext(srcPath))
	bare := strings.TrimPrefix(ext, ".")
	if _, ok := c.extAppKinds[bare]; !ok {
		return fmt.Errorf("converter: unsupported extension %s (no app kind from validate_*)", ext)
	}
	if eng, ok := c.extEngines[bare]; !ok || eng == "" {
		return fmt.Errorf("converter: no engine mapped for %s", ext)
	}

	handle := &officeHandle{}
	done := make(chan error, 1)
	go func() {
		done <- c.convertSync(ext, srcPath, dstPath, handle, password)
	}()

	select {
	case <-ctx.Done():
		handle.forceStop()
		// Wait for the converting goroutine so COM teardown finishes before reuse.
		<-done
		return fmt.Errorf("converter: %w", ctx.Err())
	case err := <-done:
		return err
	}
}

func (c *comConverter) convertSync(ext, src, dst string, handle *officeHandle, password string) error {
	// COM STA requires a fixed OS thread for CoInitialize + all subsequent calls.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE (1): COM already initialized on this thread — OK.
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
			return fmt.Errorf("converter: CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	bare := strings.TrimPrefix(strings.ToLower(ext), ".")
	engine, ok := c.extEngines[bare]
	if !ok || engine == "" {
		return fmt.Errorf("converter: no engine mapped for %s", ext)
	}
	kind, ok := c.extAppKinds[bare]
	if !ok {
		return fmt.Errorf("converter: unsupported extension %s (no app kind from validate_*)", ext)
	}
	profile, ok := Profile(engine)
	if !ok {
		return fmt.Errorf("converter: unknown engine %q", engine)
	}
	spec, ok := profile.Spec(kind)
	if !ok {
		return fmt.Errorf("converter: engine %s missing app %s", engine, kind)
	}

	switch kind {
	case AppWriter:
		return convertWord(src, dst, handle, spec, password)
	case AppSpreadsheet:
		return c.convertExcel(src, dst, handle, spec, password)
	case AppPresentation:
		return convertPowerPoint(src, dst, handle, spec, password)
	default:
		return fmt.Errorf("converter: unsupported app kind %s", kind)
	}
}

func convertWord(src, dst string, handle *officeHandle, spec AppSpec, password string) error {
	handle.setImage(spec.Image)
	var lastCreate error
	for _, progID := range spec.ProgIDs {
		err := convertWordWithProgID(src, dst, handle, spec.Image, progID, password)
		if err == nil {
			return nil
		}
		if isCreateObjectErr(err) {
			lastCreate = err
			continue
		}
		return err
	}
	if lastCreate == nil {
		lastCreate = fmt.Errorf("no ProgIDs")
	}
	return fmt.Errorf("converter: create writer app: %w", lastCreate)
}

func isCreateObjectErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "CreateObject")
}

func convertWordWithProgID(src, dst string, handle *officeHandle, image, progID, password string) error {
	before := snapshotPIDs(image)

	unknown, err := oleutil.CreateObject(progID)
	if err != nil {
		return fmt.Errorf("converter: CreateObject(%s): %w", progID, err)
	}
	defer unknown.Release()

	word, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("converter: word QueryInterface: %w", err)
	}
	defer word.Release()

	handle.set(word, resolveOfficePID(word, image, before))
	defer handle.clear()

	if err := putAppHeadless(word, 0); err != nil {
		_ = quitApp(word)
		return err
	}
	handle.set(word, resolveOfficePID(word, image, before))
	if opts, e := oleutil.GetProperty(word, "Options"); e == nil {
		if optDisp := opts.ToIDispatch(); optDisp != nil {
			_, _ = oleutil.PutProperty(optDisp, "ConfirmConversions", false)
		}
		_ = opts.Clear()
	}

	docsVar, err := oleutil.GetProperty(word, "Documents")
	if err != nil {
		_ = quitApp(word)
		return fmt.Errorf("converter: word Documents: %w", err)
	}
	defer docsVar.Clear()
	docs := docsVar.ToIDispatch()
	if docs == nil {
		_ = quitApp(word)
		return fmt.Errorf("converter: word Documents: not dispatch")
	}

	docVar, err := oleutil.CallMethod(docs, "Open", src, false, true, false, "", "")
	if err != nil {
		docVar, err = oleutil.CallMethod(docs, "Open", src, false, true, false)
		if err != nil {
			docVar, err = oleutil.CallMethod(docs, "Open", src)
		}
	}
	if err != nil && password != "" {
		docVar, err = oleutil.CallMethod(docs, "Open", src, false, true, false, password, "")
		if err != nil {
			docVar, err = oleutil.CallMethod(docs, "Open", src, false, true, false, password)
		}
	}
	if err != nil {
		_ = quitApp(word)
		return mapOfficeOpenError(err, password, officeOpenLooksLikePassword(err))
	}
	defer docVar.Clear()
	doc := docVar.ToIDispatch()
	if doc == nil {
		_ = quitApp(word)
		return fmt.Errorf("converter: word Open: not dispatch")
	}
	handle.set(word, resolveOfficePID(word, image, before))

	if _, err = oleutil.CallMethod(doc, "SaveAs2", dst, WordFormatPDF); err != nil {
		if _, err2 := oleutil.CallMethod(doc, "ExportAsFixedFormat", dst, WordFormatPDF); err2 != nil {
			if _, err3 := oleutil.CallMethod(doc, "SaveAs", dst, WordFormatPDF); err3 != nil {
				_, _ = oleutil.CallMethod(doc, "Close", false)
				_ = quitApp(word)
				return fmt.Errorf("converter: word export PDF: %w", err)
			}
		}
	}

	if _, err = oleutil.CallMethod(doc, "Close", false); err != nil {
		_ = quitApp(word)
		return fmt.Errorf("converter: word Close: %w", err)
	}
	if err := quitApp(word); err != nil {
		return fmt.Errorf("converter: word Quit: %w", err)
	}
	return nil
}

func (c *comConverter) convertExcel(src, dst string, handle *officeHandle, spec AppSpec, password string) error {
	handle.setImage(spec.Image)
	var lastCreate error
	for _, progID := range spec.ProgIDs {
		err := c.convertExcelWithProgID(src, dst, handle, spec.Image, progID, password)
		if err == nil {
			return nil
		}
		if isCreateObjectErr(err) {
			lastCreate = err
			continue
		}
		return err
	}
	if lastCreate == nil {
		lastCreate = fmt.Errorf("no ProgIDs")
	}
	return fmt.Errorf("converter: create spreadsheet app: %w", lastCreate)
}

func (c *comConverter) convertExcelWithProgID(src, dst string, handle *officeHandle, image, progID, password string) error {
	before := snapshotPIDs(image)

	unknown, err := oleutil.CreateObject(progID)
	if err != nil {
		return fmt.Errorf("converter: CreateObject(%s): %w", progID, err)
	}
	defer unknown.Release()

	excel, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("converter: excel QueryInterface: %w", err)
	}
	defer excel.Release()

	handle.set(excel, resolveOfficePID(excel, image, before))
	defer handle.clear()

	if err := putAppHeadless(excel, false); err != nil {
		_ = quitApp(excel)
		return err
	}
	handle.set(excel, resolveOfficePID(excel, image, before))
	_, _ = oleutil.PutProperty(excel, "AskToUpdateLinks", false)

	booksVar, err := oleutil.GetProperty(excel, "Workbooks")
	if err != nil {
		_ = quitApp(excel)
		return fmt.Errorf("converter: excel Workbooks: %w", err)
	}
	defer booksVar.Clear()
	books := booksVar.ToIDispatch()
	if books == nil {
		_ = quitApp(excel)
		return fmt.Errorf("converter: excel Workbooks: not dispatch")
	}

	wbVar, err := oleutil.CallMethod(books, "Open", src, 0, true)
	if err != nil {
		wbVar, err = oleutil.CallMethod(books, "Open", src)
	}
	if err != nil && password != "" {
		wbVar, err = oleutil.CallMethod(books, "Open", src, 0, true, 0, password)
	}
	if err != nil {
		_ = quitApp(excel)
		return mapOfficeOpenError(err, password, officeOpenLooksLikePassword(err))
	}
	defer wbVar.Clear()
	wb := wbVar.ToIDispatch()
	if wb == nil {
		_ = quitApp(excel)
		return fmt.Errorf("converter: excel Open: not dispatch")
	}
	handle.set(excel, resolveOfficePID(excel, image, before))

	if c.excelPageFit == "fit_width" {
		applyExcelFitWidth(wb)
	}

	if err := clearPut(oleutil.CallMethod(wb, "ExportAsFixedFormat", excelTypePDF, dst)); err != nil {
		_ = clearPut(oleutil.CallMethod(wb, "Close", false))
		_ = quitApp(excel)
		return fmt.Errorf("converter: excel ExportAsFixedFormat: %w", err)
	}

	if err := clearPut(oleutil.CallMethod(wb, "Close", false)); err != nil {
		_ = quitApp(excel)
		return fmt.Errorf("converter: excel Close: %w", err)
	}
	if err := quitApp(excel); err != nil {
		return fmt.Errorf("converter: excel Quit: %w", err)
	}
	return nil
}

// applyExcelFitWidth sets each worksheet to one page wide (unlimited pages tall).
// Failures are logged; the sheet/workbook keeps prior PageSetup (auto semantics).
//
// VARIANT ownership (Word/Excel/PowerPoint): ToIDispatch does not AddRef;
// VariantClear releases the contained IDispatch. Do not Release() the dispatch
// and Clear() the same VARIANT — that double-frees and native-crashes
// (Exception 0xc00000aa on Windows/ARM64, seen on Presentations/Documents cleanup).
func applyExcelFitWidth(wb *ole.IDispatch) {
	sheetsVar, err := oleutil.GetProperty(wb, "Worksheets")
	if err != nil {
		slog.Warn("converter: excel Worksheets for page fit failed; using auto page setup", "err", err)
		return
	}
	defer sheetsVar.Clear()
	sheets := sheetsVar.ToIDispatch()
	if sheets == nil {
		slog.Warn("converter: excel Worksheets not dispatch; using auto page setup")
		return
	}

	countVar, err := oleutil.GetProperty(sheets, "Count")
	if err != nil {
		slog.Warn("converter: excel Worksheets.Count failed; using auto page setup", "err", err)
		return
	}
	count := int(countVar.Val)
	_ = countVar.Clear()

	for i := 1; i <= count; i++ {
		itemVar, err := oleutil.GetProperty(sheets, "Item", i)
		if err != nil {
			slog.Warn("converter: excel Worksheet item failed; skipping sheet", "index", i, "err", err)
			continue
		}
		ws := itemVar.ToIDispatch()
		if ws == nil {
			_ = itemVar.Clear()
			slog.Warn("converter: excel Worksheet item not dispatch; skipping sheet", "index", i)
			continue
		}

		name := ""
		if nameVar, e := oleutil.GetProperty(ws, "Name"); e == nil {
			name = nameVar.ToString()
			_ = nameVar.Clear()
		}

		psVar, err := oleutil.GetProperty(ws, "PageSetup")
		if err != nil {
			slog.Warn("converter: excel PageSetup failed; keeping sheet setup", "sheet", name, "index", i, "err", err)
			_ = itemVar.Clear()
			continue
		}
		ps := psVar.ToIDispatch()
		if ps == nil {
			_ = psVar.Clear()
			_ = itemVar.Clear()
			slog.Warn("converter: excel PageSetup not dispatch; keeping sheet setup", "sheet", name, "index", i)
			continue
		}

		if err := clearPut(oleutil.PutProperty(ps, "Zoom", false)); err != nil {
			slog.Warn("converter: excel PageSetup.Zoom failed; keeping sheet setup", "sheet", name, "index", i, "err", err)
			_ = psVar.Clear()
			_ = itemVar.Clear()
			continue
		}
		if err := clearPut(oleutil.PutProperty(ps, "FitToPagesWide", 1)); err != nil {
			slog.Warn("converter: excel PageSetup.FitToPagesWide failed; keeping sheet setup", "sheet", name, "index", i, "err", err)
			_ = psVar.Clear()
			_ = itemVar.Clear()
			continue
		}
		if err := clearPut(oleutil.PutProperty(ps, "FitToPagesTall", false)); err != nil {
			slog.Warn("converter: excel PageSetup.FitToPagesTall failed; keeping sheet setup", "sheet", name, "index", i, "err", err)
		}

		_ = psVar.Clear()
		_ = itemVar.Clear()
	}
}

// clearPut clears a PutProperty/CallMethod result VARIANT and returns the call error.
func clearPut(v *ole.VARIANT, err error) error {
	if v != nil {
		_ = v.Clear()
	}
	return err
}

func convertPowerPoint(src, dst string, handle *officeHandle, spec AppSpec, password string) error {
	handle.setImage(spec.Image)
	var lastCreate error
	for _, progID := range spec.ProgIDs {
		err := convertPowerPointWithProgID(src, dst, handle, spec.Image, progID, password)
		if err == nil {
			return nil
		}
		if isCreateObjectErr(err) {
			lastCreate = err
			continue
		}
		return err
	}
	if lastCreate == nil {
		lastCreate = fmt.Errorf("no ProgIDs")
	}
	return fmt.Errorf("converter: create presentation app: %w", lastCreate)
}

func convertPowerPointWithProgID(src, dst string, handle *officeHandle, image, progID, password string) error {
	before := snapshotPIDs(image)

	unknown, err := oleutil.CreateObject(progID)
	if err != nil {
		return fmt.Errorf("converter: CreateObject(%s): %w", progID, err)
	}
	defer unknown.Release()

	ppt, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("converter: powerpoint QueryInterface: %w", err)
	}
	defer ppt.Release()

	handle.set(ppt, resolveOfficePID(ppt, image, before))
	defer handle.clear()

	if _, err := oleutil.PutProperty(ppt, "Visible", false); err != nil {
		slog.Debug("converter: powerpoint Visible=false rejected", "err", err)
	}
	if _, err := oleutil.PutProperty(ppt, "DisplayAlerts", ppAlertsNone); err != nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint DisplayAlerts: %w", err)
	}
	if _, err := oleutil.PutProperty(ppt, "AutomationSecurity", AutomationSecurityForceDisable); err != nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint AutomationSecurity: %w", err)
	}

	presCollVar, err := oleutil.GetProperty(ppt, "Presentations")
	if err != nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint Presentations: %w", err)
	}
	defer presCollVar.Clear()
	presColl := presCollVar.ToIDispatch()
	if presColl == nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint Presentations: not dispatch")
	}

	presVar, err := oleutil.CallMethod(presColl, "Open", src, true, false, false)
	if err != nil {
		presVar, err = oleutil.CallMethod(presColl, "Open", src)
	}
	if err != nil && password != "" {
		presVar, err = oleutil.CallMethod(presColl, "Open", src, true, false, false, password)
	}
	if err != nil {
		_ = quitApp(ppt)
		return mapOfficeOpenError(err, password, officeOpenLooksLikePassword(err))
	}
	defer presVar.Clear()
	pres := presVar.ToIDispatch()
	if pres == nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint Open: not dispatch")
	}

	if _, err = oleutil.CallMethod(pres, "SaveAs", dst, PowerPointFormatPDF); err != nil {
		_, _ = oleutil.CallMethod(pres, "Close")
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint SaveAs: %w", err)
	}

	if _, err = oleutil.CallMethod(pres, "Close"); err != nil {
		_ = quitApp(ppt)
		return fmt.Errorf("converter: powerpoint Close: %w", err)
	}
	if err := quitApp(ppt); err != nil {
		return fmt.Errorf("converter: powerpoint Quit: %w", err)
	}
	return nil
}

func putAppHeadless(app *ole.IDispatch, displayAlerts any) error {
	if _, err := oleutil.PutProperty(app, "Visible", false); err != nil {
		return fmt.Errorf("converter: Visible: %w", err)
	}
	if _, err := oleutil.PutProperty(app, "DisplayAlerts", displayAlerts); err != nil {
		return fmt.Errorf("converter: DisplayAlerts: %w", err)
	}
	if _, err := oleutil.PutProperty(app, "AutomationSecurity", AutomationSecurityForceDisable); err != nil {
		return fmt.Errorf("converter: AutomationSecurity: %w", err)
	}
	return nil
}

func quitApp(app *ole.IDispatch) error {
	_, err := oleutil.CallMethod(app, "Quit")
	return err
}

// captureAppPID reads Application.Hwnd (or HWND) and resolves the process id.
func captureAppPID(app *ole.IDispatch) uint32 {
	for _, prop := range []string{"Hwnd", "HWND", "hwnd"} {
		v, err := oleutil.GetProperty(app, prop)
		if err != nil {
			continue
		}
		hwnd := uintptr(v.Val)
		v.Clear()
		if hwnd == 0 {
			continue
		}
		var pid uint32
		_, _ = windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid)
		if pid != 0 {
			return pid
		}
	}
	slog.Warn("converter: could not resolve office pid from application hwnd")
	return 0
}

func terminatePID(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
