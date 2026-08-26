package reslimit

import "fmt"

// AutoMemLimitBytes is 50% of physical RAM, at least 512MiB.
func AutoMemLimitBytes() uint64 {
	total, _, err := systemMemory()
	if err != nil || total == 0 {
		return 512 * 1024 * 1024
	}
	half := total / 2
	const floor = 512 * 1024 * 1024
	if half < floor {
		return floor
	}
	return half
}

func DiskFreeBytes(path string) (uint64, error) {
	if path == "" {
		return 0, fmt.Errorf("empty path")
	}
	return diskFreeBytes(path)
}

func SystemMemory() (total, avail uint64, err error) {
	return systemMemory()
}
