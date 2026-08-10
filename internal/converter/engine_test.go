package converter_test

import (
	"testing"

	"msoffice2pdf/internal/converter"
)

func TestProfileKnown(t *testing.T) {
	p, ok := converter.Profile(converter.EngineMSOffice)
	if !ok || p.Writer.ProgIDs[0] != "Word.Application" {
		t.Fatalf("%+v %v", p, ok)
	}
	p, ok = converter.Profile(converter.EngineWPSOffice)
	if !ok || len(p.Writer.ProgIDs) < 1 {
		t.Fatal("wps profile")
	}
}

func TestParseAppKind(t *testing.T) {
	k, ok := converter.ParseAppKind("writer")
	if !ok || k != converter.AppWriter {
		t.Fatal(k, ok)
	}
	if _, ok := converter.ParseAppKind("rtf"); ok {
		t.Fatal("rtf")
	}
}

func TestExtAppKindsFromUpload(t *testing.T) {
	lookup := func(ext string) string {
		switch ext {
		case "docx", "wps":
			return "writer"
		case "et":
			return "spreadsheet"
		case "dps":
			return "presentation"
		default:
			return ""
		}
	}
	got := converter.ExtAppKindsFromUpload(lookup, []string{"docx", "wps", "et", "dps", "rtf"})
	if got["docx"] != converter.AppWriter || got["et"] != converter.AppSpreadsheet || got["dps"] != converter.AppPresentation {
		t.Fatalf("%v", got)
	}
	if _, ok := got["rtf"]; ok {
		t.Fatal("rtf should be absent")
	}
}

func TestCOMEngines(t *testing.T) {
	got := converter.COMEngines([]string{"msoffice", "openoffice", "wpsoffice", "unknown"})
	if len(got) != 2 || got[0] != converter.EngineMSOffice || got[1] != converter.EngineWPSOffice {
		t.Fatalf("%v", got)
	}
}

func TestImagesForEngines(t *testing.T) {
	imgs := converter.ImagesForEngines([]string{"msoffice"})
	if len(imgs) != 3 {
		t.Fatalf("%v", imgs)
	}
}

func TestImagesForEnginesOpenOffice(t *testing.T) {
	imgs := converter.ImagesForEngines([]string{"openoffice"})
	if len(imgs) == 0 {
		t.Fatal("expected non-empty openoffice process images")
	}
}
