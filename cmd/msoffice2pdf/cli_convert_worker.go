package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"msoffice2pdf/internal/converter"
)

func runConvertWorker(args []string) error {
	var src, dst, fit, engine, appKind string
	fit = "fit_width"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--src="):
			src = strings.TrimPrefix(a, "--src=")
		case a == "--src" && i+1 < len(args):
			i++
			src = args[i]
		case strings.HasPrefix(a, "--dst="):
			dst = strings.TrimPrefix(a, "--dst=")
		case a == "--dst" && i+1 < len(args):
			i++
			dst = args[i]
		case strings.HasPrefix(a, "--excel-page-fit="):
			fit = strings.TrimPrefix(a, "--excel-page-fit=")
		case a == "--excel-page-fit" && i+1 < len(args):
			i++
			fit = args[i]
		case strings.HasPrefix(a, "--engine="):
			engine = strings.TrimPrefix(a, "--engine=")
		case a == "--engine" && i+1 < len(args):
			i++
			engine = args[i]
		case strings.HasPrefix(a, "--app-kind="):
			appKind = strings.TrimPrefix(a, "--app-kind=")
		case a == "--app-kind" && i+1 < len(args):
			i++
			appKind = args[i]
		default:
			return fmt.Errorf("convert-worker: unknown arg %q", a)
		}
	}
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(dst)
	if src == "" || dst == "" {
		return fmt.Errorf("convert-worker: --src and --dst are required")
	}
	fit = strings.TrimSpace(strings.ToLower(fit))
	switch fit {
	case "", "fit_width":
		fit = "fit_width"
	case "auto":
		fit = "auto"
	default:
		return fmt.Errorf("convert-worker: --excel-page-fit must be fit_width or auto")
	}
	engine = strings.TrimSpace(strings.ToLower(engine))
	switch engine {
	case converter.EngineMSOffice, converter.EngineWPSOffice:
	default:
		return fmt.Errorf("convert-worker: --engine must be msoffice or wpsoffice")
	}
	kind, ok := converter.ParseAppKind(appKind)
	if !ok {
		return fmt.Errorf("convert-worker: --app-kind must be writer, spreadsheet, or presentation")
	}

	bare := strings.ToLower(strings.TrimPrefix(filepath.Ext(src), "."))
	if bare == "" {
		return fmt.Errorf("convert-worker: src has no extension")
	}

	conv := converter.New(converter.Options{
		ExcelPageFit: fit,
		ComMode:      "inprocess",
		TempSandbox:  false,
		Engines:      []string{engine},
		ExtEngines:   map[string]string{bare: engine},
		ExtAppKinds:  map[string]converter.AppKind{bare: kind},
	})
	if err := conv.Convert(context.Background(), src, dst); err != nil {
		return errors.New(strings.TrimPrefix(err.Error(), "converter: "))
	}
	return nil
}
