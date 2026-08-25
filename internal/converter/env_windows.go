//go:build windows

package converter

import (
	"fmt"
	"runtime"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// ValidateEnvironment probes each enabled engine (COM ProgIDs and/or OpenOffice CLI).
// On failure returns an error; caller should log and exit.
func ValidateEnvironment(opts Options) error {
	engines := opts.Engines
	if len(engines) == 0 {
		return nil
	}

	hasCOM := false
	for _, name := range engines {
		switch name {
		case EngineMSOffice, EngineWPSOffice:
			hasCOM = true
		case EngineOpenOffice:
		case EngineOFD:
		default:
			return fmt.Errorf("unknown engine %q", name)
		}
	}

	if hasCOM {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
				return fmt.Errorf("CoInitializeEx: %w", err)
			}
		}
		defer ole.CoUninitialize()
	}

	for _, name := range engines {
		switch name {
		case EngineMSOffice, EngineWPSOffice:
			p, ok := Profile(name)
			if !ok {
				return fmt.Errorf("unknown engine %q", name)
			}
			for _, item := range []struct {
				kind AppKind
				spec AppSpec
			}{
				{AppWriter, p.Writer},
				{AppSpreadsheet, p.Spreadsheet},
				{AppPresentation, p.Presentation},
			} {
				if err := probeApp(name, item.kind, item.spec); err != nil {
					return err
				}
			}
		case EngineOpenOffice:
			eng := &openOfficeEngine{
				command:     opts.OpenOfficeCommand,
				userProfile: opts.OpenOfficeUserProfile,
			}
			if err := eng.Validate(); err != nil {
				return err
			}
		case EngineOFD:
		}
	}
	return nil
}

func probeApp(engine string, kind AppKind, spec AppSpec) error {
	var last error
	for _, progID := range spec.ProgIDs {
		if err := probeProgID(progID); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("no ProgIDs configured")
	}
	return fmt.Errorf("engine %s app %s: %w", engine, kind, last)
}

func probeProgID(progID string) error {
	unknown, err := oleutil.CreateObject(progID)
	if err != nil {
		return fmt.Errorf("CreateObject(%s): %w", progID, err)
	}
	defer unknown.Release()

	app, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("QueryInterface(%s): %w", progID, err)
	}
	defer app.Release()

	_, _ = oleutil.PutProperty(app, "Visible", false)
	_, _ = oleutil.PutProperty(app, "DisplayAlerts", 0)
	_ = quitApp(app)
	return nil
}
