//go:build windows

package singleinstance

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

const mutexName = `Global\msoffice2pdf-serve`

type windowsLock struct {
	handle windows.Handle
}

// Acquire takes a machine-wide named mutex. The caller must Release (or exit)
// so the OS drops the handle; a second serve process then fails with ErrAlreadyRunning.
func Acquire() (Lock, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, fmt.Errorf("singleinstance mutex name: %w", err)
	}
	h, err := windows.CreateMutex(nil, false, name)
	if h == 0 {
		if err != nil {
			return nil, fmt.Errorf("singleinstance create mutex: %w", err)
		}
		return nil, fmt.Errorf("singleinstance create mutex: nil handle")
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(h)
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		_ = windows.CloseHandle(h)
		return nil, fmt.Errorf("singleinstance create mutex: %w", err)
	}
	return &windowsLock{handle: h}, nil
}

func (l *windowsLock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}
