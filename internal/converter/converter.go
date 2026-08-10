package converter

import "context"

type Converter interface {
	Convert(ctx context.Context, srcPath, dstPath string) error
}

// Options configures a Converter instance.
type Options struct {
	ExcelPageFit          string
	ComMode               string // subprocess | inprocess (already normalized)
	TempSandbox           bool
	Engines               []string
	ExtEngines            map[string]string  // bare ext → engine name
	ExtAppKinds           map[string]AppKind // bare ext → writer|spreadsheet|presentation (from validate_*)
	OpenOfficeCommand     string
	OpenOfficeUserProfile string
}

// New returns a Converter. comMode must already be normalized: "subprocess" or "inprocess".
func New(opts Options) Converter {
	return newConverter(opts)
}

// ExtAppKindsFromUpload builds bare-ext → AppKind from upload.validate_* markers.
func ExtAppKindsFromUpload(appKindForExt func(ext string) string, bareExts []string) map[string]AppKind {
	out := make(map[string]AppKind, len(bareExts))
	for _, ext := range bareExts {
		k, ok := ParseAppKind(appKindForExt(ext))
		if ok {
			out[ext] = k
		}
	}
	return out
}
