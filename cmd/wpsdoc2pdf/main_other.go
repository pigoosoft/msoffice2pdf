//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "wpsdoc2pdf: Windows + WPS COM only")
	os.Exit(1)
}
