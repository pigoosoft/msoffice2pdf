package converter

import (
	"runtime"
	"strings"
)

const (
	EngineMSOffice   = "msoffice"
	EngineWPSOffice  = "wpsoffice"
	EngineOpenOffice = "openoffice"
)

type AppKind string

const (
	AppWriter       AppKind = "writer"
	AppSpreadsheet  AppKind = "spreadsheet"
	AppPresentation AppKind = "presentation"
)

// ParseAppKind normalizes a config-derived app kind string.
func ParseAppKind(s string) (AppKind, bool) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case string(AppWriter):
		return AppWriter, true
	case string(AppSpreadsheet):
		return AppSpreadsheet, true
	case string(AppPresentation):
		return AppPresentation, true
	default:
		return "", false
	}
}

// AppSpec describes COM create + process image for one Office-like application.
type AppSpec struct {
	ProgIDs []string
	Image   string
}

// EngineProfile is a named conversion backend (unique engine name).
type EngineProfile struct {
	Name         string
	Writer       AppSpec
	Spreadsheet  AppSpec
	Presentation AppSpec
}

var profiles = map[string]EngineProfile{
	EngineMSOffice: {
		Name: EngineMSOffice,
		Writer: AppSpec{
			ProgIDs: []string{"Word.Application"},
			Image:   "WINWORD.EXE",
		},
		Spreadsheet: AppSpec{
			ProgIDs: []string{"Excel.Application"},
			Image:   "EXCEL.EXE",
		},
		Presentation: AppSpec{
			ProgIDs: []string{"PowerPoint.Application"},
			Image:   "POWERPNT.EXE",
		},
	},
	EngineWPSOffice: {
		Name: EngineWPSOffice,
		Writer: AppSpec{
			ProgIDs: []string{"KWPS.Application", "wps.Application", "WPS.Application"},
			Image:   "wps.exe",
		},
		Spreadsheet: AppSpec{
			ProgIDs: []string{"Ket.Application", "et.Application", "ET.Application"},
			Image:   "et.exe",
		},
		Presentation: AppSpec{
			ProgIDs: []string{"KWPP.Application", "wpp.Application", "WPP.Application"},
			Image:   "wpp.exe",
		},
	},
}

// Profile returns the built-in engine profile for name.
func Profile(name string) (EngineProfile, bool) {
	p, ok := profiles[strings.TrimSpace(strings.ToLower(name))]
	return p, ok
}

func openOfficeProcessImages() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"soffice.exe"}
	case "darwin":
		return []string{"soffice"}
	default:
		return []string{"soffice.bin", "soffice"}
	}
}

// COMEngines returns only COM-based engine names (msoffice / wpsoffice), excluding openoffice.
func COMEngines(engines []string) []string {
	var out []string
	for _, name := range engines {
		name = strings.TrimSpace(strings.ToLower(name))
		switch name {
		case EngineMSOffice, EngineWPSOffice:
			out = append(out, name)
		}
	}
	return out
}

// ImagesForEngines returns unique process image names for orphan cleanup.
func ImagesForEngines(names []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, name := range names {
		name = strings.TrimSpace(strings.ToLower(name))
		var imgs []string
		if name == EngineOpenOffice {
			imgs = openOfficeProcessImages()
		} else {
			p, ok := Profile(name)
			if !ok {
				continue
			}
			imgs = []string{p.Writer.Image, p.Spreadsheet.Image, p.Presentation.Image}
		}
		for _, img := range imgs {
			if img == "" {
				continue
			}
			key := strings.ToUpper(img)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, img)
		}
	}
	return out
}

func (p EngineProfile) Spec(kind AppKind) (AppSpec, bool) {
	switch kind {
	case AppWriter:
		return p.Writer, true
	case AppSpreadsheet:
		return p.Spreadsheet, true
	case AppPresentation:
		return p.Presentation, true
	default:
		return AppSpec{}, false
	}
}
