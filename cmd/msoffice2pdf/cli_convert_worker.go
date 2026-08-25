package main

import "msoffice2pdf/internal/convertworker"

func runConvertWorker(args []string) error {
	return convertworker.Run(args)
}
