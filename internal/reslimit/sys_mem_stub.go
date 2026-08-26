//go:build !windows && !linux

package reslimit

import "fmt"

func systemMemory() (total, avail uint64, err error) {
	return 0, 0, fmt.Errorf("system memory not available on this OS")
}
