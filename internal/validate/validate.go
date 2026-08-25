package validate

import (
	"errors"
	"fmt"
	"os"

	"msoffice2pdf/internal/config"
)

var (
	ErrMagic     = errors.New("ERR_FILE_MAGIC")
	ErrStructure = errors.New("ERR_FILE_STRUCTURE")
)

// File checks path on disk using original filename for extension/family.
func File(path, filename string, uploadCfg config.UploadConfig) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if uploadCfg.MagicEnabled() {
		if err := checkMagic(f, filename, uploadCfg); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
	}

	fam := uploadCfg.OfficeFamily(filename)
	switch fam {
	case "ooxml":
		req := config.LookupValidateEntries(uploadCfg.ValidateNew, filename)
		if len(req) == 0 {
			return fmt.Errorf("%w: no validate_new rules", ErrStructure)
		}
		return checkZIPMembers(path, req)
	case "ole":
		req := config.LookupValidateEntries(uploadCfg.ValidateOLE, filename)
		if len(req) == 0 {
			return fmt.Errorf("%w: no validate_ole rules", ErrStructure)
		}
		return checkOLEStreams(f, req)
	case "ofd":
		req := config.LookupValidateEntries(uploadCfg.ValidateOFD, filename)
		if len(req) == 0 {
			return fmt.Errorf("%w: no validate_ofd rules", ErrStructure)
		}
		return checkZIPMembers(path, req)
	default:
		// Allowed but unknown family: magic-only (should not happen if Validate() strict).
		return nil
	}
}
