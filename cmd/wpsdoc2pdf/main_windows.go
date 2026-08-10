//go:build windows

// wpsdoc2pdf is a one-off probe: open a .doc/.docx via WPS COM and export PDF.
// Usage: go run ./cmd/wpsdoc2pdf -- path\to\in.doc path\to\out.pdf
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Word-compatible PDF format code (also used by WPS Writer).
const formatPDF = 17

var wpsProgIDs = []string{
	"KWPS.Application",
	"wps.Application",
	"WPS.Application",
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <src.doc|docx> <dst.pdf>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}
	src, dst := os.Args[1], os.Args[2]
	if err := convert(src, dst); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK")
}

func convert(src, dst string) error {
	src, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("abs src: %w", err)
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("abs dst: %w", err)
	}
	if st, err := os.Stat(src); err != nil {
		return fmt.Errorf("stat src: %w", err)
	} else if st.IsDir() {
		return fmt.Errorf("src is a directory")
	}
	ext := strings.ToLower(filepath.Ext(src))
	if ext != ".doc" && ext != ".docx" {
		return fmt.Errorf("unsupported extension %q (want .doc/.docx)", ext)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir dst: %w", err)
	}
	_ = os.Remove(dst)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	start := time.Now()
	var lastCreateErr error
	for _, progID := range wpsProgIDs {
		fmt.Printf("try ProgID=%s ...\n", progID)
		err := convertWithProgID(progID, src, dst)
		if err == nil {
			fi, err := os.Stat(dst)
			if err != nil {
				return fmt.Errorf("PDF missing after success: %w", err)
			}
			if fi.Size() == 0 {
				return fmt.Errorf("PDF is empty: %s", dst)
			}
			fmt.Printf("success ProgID=%s method logged above size=%d elapsed=%s\n",
				progID, fi.Size(), time.Since(start).Round(time.Millisecond))
			return nil
		}
		fmt.Printf("  create/convert failed: %v\n", err)
		lastCreateErr = err
		_ = os.Remove(dst)
	}
	return fmt.Errorf("all ProgIDs failed; last: %w", lastCreateErr)
}

func convertWithProgID(progID, src, dst string) error {
	unknown, err := oleutil.CreateObject(progID)
	if err != nil {
		return fmt.Errorf("CreateObject(%s): %w", progID, err)
	}
	defer unknown.Release()

	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("QueryInterface: %w", err)
	}
	defer app.Release()

	_, _ = oleutil.PutProperty(app, "Visible", false)
	_, _ = oleutil.PutProperty(app, "DisplayAlerts", 0)

	docsVar, err := oleutil.GetProperty(app, "Documents")
	if err != nil {
		_ = callQuit(app)
		return fmt.Errorf("Documents: %w", err)
	}
	defer docsVar.Clear()
	docs := docsVar.ToIDispatch()
	if docs == nil {
		_ = callQuit(app)
		return fmt.Errorf("Documents: not dispatch")
	}

	// Open(FileName, ConfirmConversions, ReadOnly, AddToRecentFiles, ...)
	docVar, err := oleutil.CallMethod(docs, "Open", src, false, true, false)
	if err != nil {
		// Fallback: Open with filename only (some WPS builds are picky about args).
		docVar, err = oleutil.CallMethod(docs, "Open", src)
		if err != nil {
			_ = callQuit(app)
			return fmt.Errorf("Open: %w", err)
		}
	}
	defer docVar.Clear()
	doc := docVar.ToIDispatch()
	if doc == nil {
		_ = callQuit(app)
		return fmt.Errorf("Open: not dispatch")
	}

	method, err := exportPDF(doc, dst)
	if err != nil {
		_, _ = oleutil.CallMethod(doc, "Close", false)
		_ = callQuit(app)
		return err
	}
	fmt.Printf("  export via %s\n", method)

	if _, err = oleutil.CallMethod(doc, "Close", false); err != nil {
		_ = callQuit(app)
		return fmt.Errorf("Close: %w", err)
	}
	if err := callQuit(app); err != nil {
		return fmt.Errorf("Quit: %w", err)
	}
	return nil
}

func exportPDF(doc *ole.IDispatch, dst string) (string, error) {
	// Prefer ExportAsFixedFormat (WPS / Word-compatible).
	// ExportAsFixedFormat(OutputFileName, ExportFormat, ...) — ExportFormat 17 = PDF.
	if _, err := oleutil.CallMethod(doc, "ExportAsFixedFormat", dst, formatPDF); err == nil {
		return "ExportAsFixedFormat", nil
	} else {
		fmt.Printf("  ExportAsFixedFormat failed: %v\n", err)
	}

	if _, err := oleutil.CallMethod(doc, "SaveAs2", dst, formatPDF); err == nil {
		return "SaveAs2", nil
	} else {
		fmt.Printf("  SaveAs2 failed: %v\n", err)
	}

	if _, err := oleutil.CallMethod(doc, "SaveAs", dst, formatPDF); err == nil {
		return "SaveAs", nil
	} else {
		return "", fmt.Errorf("SaveAs: %w", err)
	}
}

func callQuit(app *ole.IDispatch) error {
	_, err := oleutil.CallMethod(app, "Quit")
	return err
}
