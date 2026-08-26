//go:build linux

package reslimit

import "golang.org/x/sys/unix"

func systemMemory() (total, avail uint64, err error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, 0, err
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	total = uint64(info.Totalram) * unit
	avail = uint64(info.Freeram) * unit
	return total, avail, nil
}
