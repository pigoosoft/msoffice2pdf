//go:build windows

package consoleattach

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS (-1)

var (
	modKernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole    = modKernel32.NewProc("AttachConsole")
	procAllocConsole     = modKernel32.NewProc("AllocConsole")
	procGetConsoleWindow = modKernel32.NewProc("GetConsoleWindow")
)

func ensureCLI() {
	if getConsoleWindow() == 0 {
		r1, _, _ := procAttachConsole.Call(uintptr(attachParentProcess))
		if r1 == 0 &&
			!stdHandleUsable(windows.STD_OUTPUT_HANDLE) &&
			!stdHandleUsable(windows.STD_ERROR_HANDLE) {
			// Explorer / double-click fatal path: allocate a visible console.
			_, _, _ = procAllocConsole.Call()
		}
	}

	if getConsoleWindow() != 0 {
		rebindConsoleFiles()
		return
	}

	// Inherited pipes/files (CI, editor shells): reconnect Go os.Std* to them.
	rebindInheritedHandles()
}

func detachForUI() {
	// GUI subsystem builds have no console; nothing to detach.
}

func getConsoleWindow() windows.HWND {
	hwnd, _, _ := procGetConsoleWindow.Call()
	return windows.HWND(hwnd)
}

func stdHandleUsable(n uint32) bool {
	h, err := windows.GetStdHandle(n)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return false
	}
	t, err := windows.GetFileType(h)
	if err != nil || t == windows.FILE_TYPE_UNKNOWN {
		return false
	}
	return true
}

func rebindConsoleFiles() {
	in, errIn := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	out, errOut := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if errIn == nil {
		os.Stdin = in
		_ = windows.SetStdHandle(windows.STD_INPUT_HANDLE, windows.Handle(in.Fd()))
		syscall.Stdin = syscall.Handle(in.Fd())
	}
	if errOut == nil {
		os.Stdout = out
		os.Stderr = out
		h := windows.Handle(out.Fd())
		_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, h)
		_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, h)
		syscall.Stdout = syscall.Handle(out.Fd())
		syscall.Stderr = syscall.Handle(out.Fd())
	}
}

func rebindInheritedHandles() {
	rebindOne(windows.STD_INPUT_HANDLE, &os.Stdin, &syscall.Stdin, "stdin")
	rebindOne(windows.STD_OUTPUT_HANDLE, &os.Stdout, &syscall.Stdout, "stdout")
	rebindOne(windows.STD_ERROR_HANDLE, &os.Stderr, &syscall.Stderr, "stderr")
}

func rebindOne(n uint32, destFile **os.File, destSys *syscall.Handle, name string) {
	h, err := windows.GetStdHandle(n)
	if err != nil || h == 0 || h == windows.InvalidHandle {
		return
	}
	t, err := windows.GetFileType(h)
	if err != nil || t == windows.FILE_TYPE_UNKNOWN {
		return
	}
	f := os.NewFile(uintptr(h), name)
	*destFile = f
	*destSys = syscall.Handle(h)
}
