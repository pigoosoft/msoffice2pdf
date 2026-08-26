//go:build windows

package reslimit

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func diskFreeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	return free, nil
}

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

var procGlobalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func systemMemory() (total, avail uint64, err error) {
	var ms memoryStatusEx
	ms.length = uint32(unsafe.Sizeof(ms))
	r1, _, e := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		if e != nil {
			return 0, 0, e
		}
		return 0, 0, fmt.Errorf("GlobalMemoryStatusEx failed")
	}
	return ms.totalPhys, ms.availPhys, nil
}
