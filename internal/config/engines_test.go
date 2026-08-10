package config_test

import (
	"strings"
	"testing"

	"msoffice2pdf/internal/config"
)

func baseCfg() config.Config {
	t := true
	return config.Config{
		Server:   config.ServerConfig{Port: 8080},
		Database: config.DatabaseConfig{Driver: "mysql", DSN: "x"},
		Storage:  config.StorageConfig{UploadDir: "u", OutputDir: "o", TrashDir: "t", ExpiredDir: "e"},
		Auth:     config.AuthConfig{JWTSecret: "secret"},
		Upload: config.UploadConfig{
			MaxSizeBytes: 1024,
			AllowedExts:  []string{"*.docx", "*.wps"},
			ValidateNew:  map[string][]string{"*.docx": {"word/document.xml"}},
			ValidateOLE:  map[string][]string{"*.wps": {"WordDocument"}},
		},
		Converter: config.ConverterConfig{
			WorkerCount: 1, QueueSize: 1, OfficeTimeout: 1, RequeueInterval: 1,
			RetryCount: 0, RetryInterval: 1, ExcelPageFit: "fit_width", ComMode: "subprocess",
			TempSandbox: &t,
			Engines:     []string{"msoffice", "wpsoffice"},
			ExtEngines:  map[string]string{"*.docx": "msoffice", "*.wps": "wpsoffice"},
		},
		Cleanup:   config.CleanupConfig{PdfTTL: 1, Interval: 1},
		Watermark: config.WatermarkConfig{Density: "medium", Opacity: 0.2, Color: "#808080"},
	}
}

func TestValidateEnginesDefaultAndRejectEmpty(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = nil
	c.Upload.AllowedExts = []string{"*.docx"}
	c.Upload.ValidateNew = map[string][]string{"*.docx": {"word/document.xml"}}
	c.Upload.ValidateOLE = nil
	c.Converter.ExtEngines = map[string]string{"*.docx": "msoffice"}
	if err := c.Validate(); err != nil {
		t.Fatalf("nil engines should default: %v", err)
	}
	if len(c.Converter.Engines) != 1 || c.Converter.Engines[0] != "msoffice" {
		t.Fatalf("got engines %#v", c.Converter.Engines)
	}

	c = baseCfg()
	c.Converter.Engines = []string{}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "engines") {
		t.Fatalf("explicit empty engines want error, got %v", err)
	}
}

func TestValidateEnginesUniqueAndKnown(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"msoffice", "msoffice"}
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate engines")
	}
	c = baseCfg()
	c.Converter.Engines = []string{"libreoffice"}
	if err := c.Validate(); err == nil {
		t.Fatal("unknown engine")
	}
}

func TestValidateExtEnginesMustBeEnabled(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"msoffice"}
	c.Converter.ExtEngines = map[string]string{"*.docx": "wpsoffice"}
	c.Upload.AllowedExts = []string{"*.docx"}
	c.Upload.ValidateNew = map[string][]string{"*.docx": {"word/document.xml"}}
	c.Upload.ValidateOLE = nil
	if err := c.Validate(); err == nil {
		t.Fatal("ext_engines value not in engines")
	}
}

func TestEngineForFilename(t *testing.T) {
	c := baseCfg()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	eng, ok := c.Converter.EngineForFilename("x.DOCX")
	if !ok || eng != "msoffice" {
		t.Fatalf("docx: %v %v", eng, ok)
	}
	eng, ok = c.Converter.EngineForFilename("a.wps")
	if !ok || eng != "wpsoffice" {
		t.Fatalf("wps: %v %v", eng, ok)
	}
	_, ok = c.Converter.EngineForFilename("a.rtf")
	if ok {
		t.Fatal("rtf should be unmapped")
	}
}

func TestAppKindFromValidate(t *testing.T) {
	c := baseCfg()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := c.Upload.AppKind("a.docx"); got != "writer" {
		t.Fatalf("docx app kind: %q", got)
	}
	if got := c.Upload.AppKind("a.wps"); got != "writer" {
		t.Fatalf("wps app kind: %q", got)
	}
	c.Upload.ValidateOLE["*.et"] = []string{"Workbook"}
	if got := c.Upload.AppKind(".et"); got != "spreadsheet" {
		t.Fatalf("et app kind: %q", got)
	}
	c.Upload.ValidateNew["*.pptx"] = []string{"ppt/presentation.xml"}
	if got := c.Upload.AppKind("x.pptx"); got != "presentation" {
		t.Fatalf("pptx app kind: %q", got)
	}
}

func TestExtEnginesRequiresInferableAppKind(t *testing.T) {
	c := baseCfg()
	c.Converter.ExtEngines = map[string]string{"*.docx": "msoffice", "*.wps": "wpsoffice"}
	c.Upload.ValidateOLE = nil // wps mapped but no validate markers
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "app kind") {
		t.Fatalf("want app kind error, got %v", err)
	}
}

func TestAllowedExtWithoutValidateMapsOK(t *testing.T) {
	c := baseCfg()
	c.Upload.AllowedExts = []string{"*.docx", "*.xyz"}
	c.Converter.ExtEngines = map[string]string{"*.docx": "msoffice"}
	c.Upload.ValidateOLE = nil
	// *.xyz allowed but not in ext_engines and no validate_* — OK at Validate
	if err := c.Validate(); err != nil {
		t.Fatalf("allowed without validate/ext_engines should be OK: %v", err)
	}
}

func TestValidateOpenOfficeRequiredWhenEnabled(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"openoffice"}
	c.Converter.ExtEngines = map[string]string{"*.docx": "openoffice"}
	c.Upload.AllowedExts = []string{"*.docx"}
	c.Upload.ValidateNew = map[string][]string{"*.docx": {"word/document.xml"}}
	c.Upload.ValidateOLE = nil
	c.Converter.OpenOffice = config.OpenOfficeConfig{}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "openoffice") {
		t.Fatalf("want openoffice required error, got %v", err)
	}

	c.Converter.OpenOffice = config.OpenOfficeConfig{
		Command:     "soffice",
		UserProfile: "C:/data/lo-profile",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("openoffice with fields should pass: %v", err)
	}
}

func TestValidateOpenOfficeOptionalWhenDisabled(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"msoffice"}
	c.Converter.ExtEngines = map[string]string{"*.docx": "msoffice"}
	c.Upload.AllowedExts = []string{"*.docx"}
	c.Upload.ValidateNew = map[string][]string{"*.docx": {"word/document.xml"}}
	c.Upload.ValidateOLE = nil
	c.Converter.OpenOffice = config.OpenOfficeConfig{}
	if err := c.Validate(); err != nil {
		t.Fatalf("openoffice section optional when disabled: %v", err)
	}
}
