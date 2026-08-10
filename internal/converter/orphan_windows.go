//go:build windows

package converter

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CleanupOrphanOfficeProcesses terminates leftover office/WPS processes for enabled engines.
func CleanupOrphanOfficeProcesses(engines []string) {
	for _, image := range ImagesForEngines(engines) {
		n := killAllByImage(image)
		if n > 0 {
			slog.Warn("converter: terminated orphan office processes", "image", image, "count", n)
		}
	}
}

// CleanupOrphanConvertWorkers kills every matching convert-worker except self (startup).
func CleanupOrphanConvertWorkers() {
	_, killed := SweepConvertWorkers(0)
	if killed > 0 {
		slog.Warn("converter: terminated orphan convert-worker processes", "count", killed)
	}
}

// CleanupOrphansAtStart clears leftover convert-worker processes, then Office images.
func CleanupOrphansAtStart(engines []string) {
	CleanupOrphanConvertWorkers()
	CleanupOrphanOfficeProcesses(engines)
	_ = SweepTempSandboxes(0)
}

// SweepTempSandboxes removes msoffice2pdf-com-* dirs under TempDir older than maxAge.
// maxAge <= 0 removes all matching directories (startup).
func SweepTempSandboxes(maxAge time.Duration) (removed int) {
	base := os.TempDir()
	entries, err := os.ReadDir(base)
	if err != nil {
		slog.Warn("converter: SweepTempSandboxes read", "err", err)
		return 0
	}
	now := time.Now()
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), tempSandboxPrefix) {
			continue
		}
		p := filepath.Join(base, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if maxAge > 0 && now.Sub(info.ModTime()) < maxAge {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			slog.Warn("converter: SweepTempSandboxes remove", "path", p, "err", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		slog.Info("converter: swept temp sandboxes", "removed", removed)
	}
	return removed
}

// SweepConvertWorkers kills convert-worker processes older than maxAge (maxAge<=0 → all matching).
// Returns remaining matching count after sweep, and how many were killed.
//
// Match rules: same image base name as this executable, cmdline contains "convert-worker",
// and pid != self. Never kills when cmdline cannot be read, or when maxAge>0 and age is unknown.
func SweepConvertWorkers(maxAge time.Duration) (alive, killed int) {
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("converter: SweepConvertWorkers: executable path", "err", err)
		return 0, 0
	}
	image := filepath.Base(exe)
	self := uint32(os.Getpid())
	reason := "orphan"
	if maxAge > 0 {
		reason = "stale"
	}

	for _, pid := range listPIDsByImage(image) {
		if pid == 0 || pid == self {
			continue
		}
		cmdline, err := readProcessCommandLine(pid)
		if err != nil {
			continue
		}
		if !strings.Contains(cmdline, "convert-worker") {
			continue
		}

		if maxAge > 0 {
			age, ok := processAge(pid)
			if !ok || age < maxAge {
				alive++
				continue
			}
		}

		if err := terminatePID(pid); err != nil {
			slog.Warn("converter: terminate convert-worker failed", "pid", pid, "err", err)
			alive++
			continue
		}
		slog.Warn("converter: terminated convert-worker process", "pid", pid, "reason", reason)
		killed++
	}
	return alive, killed
}

func processAge(pid uint32) (time.Duration, bool) {
	h, err := openProcessQuery(pid)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	ns := creation.Nanoseconds()
	if ns <= 0 {
		return 0, false
	}
	return time.Since(time.Unix(0, ns)), true
}

func openProcessQuery(pid uint32) (windows.Handle, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err == nil {
		return h, nil
	}
	return windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
}

func openProcessQueryVMRead(pid uint32) (windows.Handle, error) {
	access := uint32(windows.PROCESS_QUERY_INFORMATION | windows.PROCESS_VM_READ)
	h, err := windows.OpenProcess(access, false, pid)
	if err == nil {
		return h, nil
	}
	access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.PROCESS_VM_READ
	return windows.OpenProcess(access, false, pid)
}

// readProcessCommandLine reads the remote PEB → ProcessParameters → CommandLine.
func readProcessCommandLine(pid uint32) (string, error) {
	h, err := openProcessQueryVMRead(pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	var pbi windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation,
		unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), nil); err != nil {
		return "", err
	}
	if pbi.PebBaseAddress == nil {
		return "", errors.New("nil peb")
	}

	pebAddr := uintptr(unsafe.Pointer(pbi.PebBaseAddress))
	var peb windows.PEB
	var paramsPtr uintptr
	if err := readProcessMemory(h, pebAddr+unsafe.Offsetof(peb.ProcessParameters),
		unsafe.Pointer(&paramsPtr), unsafe.Sizeof(paramsPtr)); err != nil {
		return "", err
	}
	if paramsPtr == 0 {
		return "", errors.New("nil process parameters")
	}

	var pp windows.RTL_USER_PROCESS_PARAMETERS
	var us windows.NTUnicodeString
	if err := readProcessMemory(h, paramsPtr+unsafe.Offsetof(pp.CommandLine),
		unsafe.Pointer(&us), unsafe.Sizeof(us)); err != nil {
		return "", err
	}
	if us.Length == 0 || us.Buffer == nil {
		return "", errors.New("empty command line")
	}

	byteLen := uintptr(us.Length)
	raw := make([]byte, byteLen)
	bufAddr := uintptr(unsafe.Pointer(us.Buffer))
	if err := readProcessMemory(h, bufAddr, unsafe.Pointer(&raw[0]), byteLen); err != nil {
		return "", err
	}
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&raw[0])), int(byteLen)/2)
	return windows.UTF16ToString(u16), nil
}

func readProcessMemory(h windows.Handle, addr uintptr, buf unsafe.Pointer, size uintptr) error {
	var n uintptr
	if err := windows.ReadProcessMemory(h, addr, (*byte)(buf), size, &n); err != nil {
		return err
	}
	if n != size {
		return errors.New("short ReadProcessMemory")
	}
	return nil
}

func snapshotPIDs(image string) map[uint32]struct{} {
	out := make(map[uint32]struct{})
	for _, p := range listPIDsByImage(image) {
		out[p] = struct{}{}
	}
	return out
}

func findNewPID(before map[uint32]struct{}, image string) uint32 {
	for _, p := range listPIDsByImage(image) {
		if _, ok := before[p]; !ok {
			return p
		}
	}
	return 0
}

func killAllByImage(image string) int {
	n := 0
	for _, pid := range listPIDsByImage(image) {
		if err := terminatePID(pid); err != nil {
			slog.Warn("converter: terminate orphan pid failed", "image", image, "pid", pid, "err", err)
			continue
		}
		n++
	}
	return n
}

func listPIDsByImage(image string) []uint32 {
	want := strings.ToUpper(image)
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		slog.Warn("converter: CreateToolhelp32Snapshot failed", "err", err)
		return nil
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return nil
	}

	var pids []uint32
	for {
		name := windows.UTF16ToString(pe.ExeFile[:])
		if strings.EqualFold(name, want) {
			pids = append(pids, pe.ProcessID)
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return pids
}
