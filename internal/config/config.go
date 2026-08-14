package config

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Storage   StorageConfig   `yaml:"storage"`
	Converter ConverterConfig `yaml:"converter"`
	Cleanup   CleanupConfig   `yaml:"cleanup"`
	Auth      AuthConfig      `yaml:"auth"`
	Log       LogConfig       `yaml:"log"`
	Upload    UploadConfig    `yaml:"upload"`
	Watermark WatermarkConfig `yaml:"watermark"`
	Desktop   DesktopConfig   `yaml:"desktop"`
}

// DesktopConfig is the Fyne control-shell preference. Ignored in --noui / console mode.
type DesktopConfig struct {
	// Language is en | zh. Empty/omitted: follow OS (non-Chinese → en). Unsupported → en.
	Language string `yaml:"language"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	SSE          SSEConfig     `yaml:"sse"`
}

type SSEConfig struct {
	MaxDuration       time.Duration `yaml:"max_duration"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	MaxFileIDs        int           `yaml:"max_fileids"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type StorageConfig struct {
	UploadDir  string `yaml:"upload_dir"`
	OutputDir  string `yaml:"output_dir"`
	TrashDir   string `yaml:"trash_dir"`
	ExpiredDir string `yaml:"expired_dir"`
}

type OpenOfficeConfig struct {
	Command     string `yaml:"command"`
	UserProfile string `yaml:"user_profile"`
}

type ConverterConfig struct {
	WorkerCount     int               `yaml:"worker_count"`
	QueueSize       int               `yaml:"queue_size"`
	OfficeTimeout   time.Duration     `yaml:"office_timeout"`
	RequeueInterval time.Duration     `yaml:"requeue_interval"`
	RetryCount      int               `yaml:"retry_count"`
	RetryInterval   time.Duration     `yaml:"retry_interval"`
	ExcelPageFit    string            `yaml:"excel_page_fit"`
	ComMode         string            `yaml:"com_mode"`
	TempSandbox     *bool             `yaml:"temp_sandbox"` // nil → true
	Engines         []string          `yaml:"engines"`
	ExtEngines      map[string]string `yaml:"ext_engines"` // after Validate: bare ext → engine name
	OpenOffice      OpenOfficeConfig  `yaml:"openoffice"`
}

// EngineForFilename returns the configured converter engine for a filename's extension.
func (c ConverterConfig) EngineForFilename(filename string) (string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" || c.ExtEngines == nil {
		return "", false
	}
	eng, ok := c.ExtEngines[ext]
	return eng, ok && eng != ""
}

type CleanupConfig struct {
	// UploadTTL is deprecated: terminal uploads archive immediately via ArchiveUpload.
	// Still readable for one compatibility round; ignored by cleanup logic.
	UploadTTL time.Duration `yaml:"upload_ttl"`

	HistoryTTLEnabled   bool          `yaml:"history_ttl_enabled"`
	HistoryTTL          time.Duration `yaml:"history_ttl"`
	HistoryTTLDeleteRow bool          `yaml:"history_ttl_delete_row"`

	PdfTTL   time.Duration `yaml:"pdf_ttl"`
	Interval time.Duration `yaml:"interval"`
}

type AuthConfig struct {
	JWTSecret   string        `yaml:"jwt_secret"`
	TokenExpire time.Duration `yaml:"token_expire"`
}

type LogConfig struct {
	Level         string        `yaml:"level"`
	Output        string        `yaml:"output"`
	FileEnabled   *bool         `yaml:"file_enabled"` // nil → false
	FileDir       string        `yaml:"file_dir"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

func (l LogConfig) FileLoggingEnabled() bool {
	return boolOrDefault(l.FileEnabled, false)
}

type UploadConfig struct {
	MaxSize       string              `yaml:"max_size"`
	AllowedExts   []string            `yaml:"allowed_exts"`
	MaxSizeBytes  int64               `yaml:"-"`
	ValidateMagic *bool               `yaml:"validate_magic"` // nil → true
	ValidateNew   map[string][]string `yaml:"validate_new"`
	ValidateOLE   map[string][]string `yaml:"validate_ole"`
}

func boolOrDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func (u UploadConfig) MagicEnabled() bool {
	return boolOrDefault(u.ValidateMagic, true)
}

func (c ConverterConfig) TempSandboxEnabled() bool {
	return boolOrDefault(c.TempSandbox, true)
}

// WatermarkConfig is the global primary watermark and visual style for PDF post-process.
type WatermarkConfig struct {
	Text         string  `yaml:"text"`
	Angle        float64 `yaml:"angle"`
	Density      string  `yaml:"density"`
	DensityCount int     `yaml:"density_count"`
	Opacity      float64 `yaml:"opacity"`
	Color        string  `yaml:"color"`
	FontSize     float64 `yaml:"font_size"`
	FontPath     string  `yaml:"font_path"`
}

func parseMaxSizeBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	upper := strings.ToUpper(s)
	for _, spec := range []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
	} {
		if strings.HasSuffix(upper, spec.suffix) {
			numPart := strings.TrimSpace(s[:len(s)-len(spec.suffix)])
			n, err := strconv.ParseInt(numPart, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size %q: %w", s, err)
			}
			if n <= 0 {
				return 0, fmt.Errorf("size must be > 0")
			}
			return n * spec.mult, nil
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("size must be > 0")
	}
	return n, nil
}

func normalizeExtPattern(pattern string) string {
	p := strings.TrimSpace(pattern)
	p = strings.TrimPrefix(p, "*.")
	p = strings.TrimPrefix(p, ".")
	return strings.ToLower(strings.TrimSpace(p))
}

func lookupValidateEntries(m map[string][]string, ext string) []string {
	ext = strings.ToLower(ext)
	for k, v := range m {
		if normalizeExtPattern(k) == ext {
			return v
		}
	}
	return nil
}

func normalizeFilenameOrExt(filenameOrExt string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filenameOrExt), "."))
	if ext == "" {
		ext = normalizeExtPattern(filenameOrExt)
	}
	return ext
}

// LookupValidateEntries returns configured validation paths for a filename or extension.
func LookupValidateEntries(m map[string][]string, filenameOrExt string) []string {
	return lookupValidateEntries(m, normalizeFilenameOrExt(filenameOrExt))
}

// OfficeFamily returns "ooxml", "ole", or "" based on validate_new / validate_ole ownership.
func (u UploadConfig) OfficeFamily(filenameOrExt string) string {
	ext := normalizeFilenameOrExt(filenameOrExt)
	inNew := len(lookupValidateEntries(u.ValidateNew, ext)) > 0
	inOLE := len(lookupValidateEntries(u.ValidateOLE, ext)) > 0
	switch {
	case inNew && !inOLE:
		return "ooxml"
	case inOLE && !inNew:
		return "ole"
	default:
		return ""
	}
}

// AppKind returns "writer", "spreadsheet", "presentation", or "" from validate_* entry markers.
// OOXML paths (word/…, xl/…, ppt/…) and OLE streams (WordDocument, Workbook, PowerPoint Document)
// are the sole source of truth — no hardcoded extension lists.
func (u UploadConfig) AppKind(filenameOrExt string) string {
	ext := normalizeFilenameOrExt(filenameOrExt)
	if k := inferAppKindFromEntries(lookupValidateEntries(u.ValidateNew, ext)); k != "" {
		return k
	}
	return inferAppKindFromEntries(lookupValidateEntries(u.ValidateOLE, ext))
}

func inferAppKindFromEntries(entries []string) string {
	for _, e := range entries {
		el := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(e), "\\", "/"))
		switch {
		case strings.HasPrefix(el, "word/"), el == "worddocument":
			return "writer"
		case strings.HasPrefix(el, "xl/"), el == "workbook":
			return "spreadsheet"
		case strings.HasPrefix(el, "ppt/"), strings.Contains(el, "powerpoint"):
			return "presentation"
		}
	}
	return ""
}

func (u UploadConfig) ExtAllowed(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	base := filepath.Base(filename)

	for _, pattern := range u.AllowedExts {
		raw := strings.TrimSpace(pattern)
		norm := normalizeExtPattern(raw)
		if norm == "" || norm == "*" || strings.EqualFold(raw, "*.*") {
			return true
		}
		if ext == norm {
			return true
		}
		for _, matchPat := range []string{raw, "*." + norm} {
			matched, err := path.Match(strings.ToLower(matchPat), strings.ToLower(base))
			if err == nil && matched {
				return true
			}
		}
	}
	return false
}

// AllowedExtsForClient returns deduplicated extensions as ".ext" for Web UI accept lists.
// Skips empty patterns and wildcards "*" / "*.*".
func (u UploadConfig) AllowedExtsForClient() []string {
	seen := make(map[string]struct{}, len(u.AllowedExts))
	out := make([]string, 0, len(u.AllowedExts))
	for _, pattern := range u.AllowedExts {
		raw := strings.TrimSpace(pattern)
		if raw == "" || strings.EqualFold(raw, "*.*") {
			continue
		}
		norm := normalizeExtPattern(raw)
		if norm == "" || norm == "*" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, "."+norm)
	}
	return out
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if v := os.Getenv("MSOFFICE2PDF_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("MSOFFICE2PDF_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	maxBytes, err := parseMaxSizeBytes(cfg.Upload.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("upload.max_size: %w", err)
	}
	cfg.Upload.MaxSizeBytes = maxBytes
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	switch c.Database.Driver {
	case "mysql", "postgres":
	default:
		return fmt.Errorf("database.driver must be mysql or postgres, got %q", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be > 0")
	}
	if c.Server.SSE.MaxDuration <= 0 {
		c.Server.SSE.MaxDuration = 5 * time.Minute
	}
	if c.Server.SSE.HeartbeatInterval <= 0 {
		c.Server.SSE.HeartbeatInterval = 15 * time.Second
	}
	if c.Server.SSE.PollInterval <= 0 {
		c.Server.SSE.PollInterval = time.Second
	}
	if c.Server.SSE.MaxFileIDs <= 0 {
		c.Server.SSE.MaxFileIDs = 50
	}
	if c.Server.SSE.PollInterval >= c.Server.SSE.MaxDuration {
		return fmt.Errorf("server.sse.poll_interval must be < server.sse.max_duration")
	}
	if c.Server.SSE.HeartbeatInterval >= c.Server.SSE.MaxDuration {
		return fmt.Errorf("server.sse.heartbeat_interval must be < server.sse.max_duration")
	}
	if c.Storage.UploadDir == "" || c.Storage.OutputDir == "" ||
		c.Storage.TrashDir == "" || c.Storage.ExpiredDir == "" {
		return fmt.Errorf("storage directories must all be set")
	}
	if strings.TrimSpace(c.Auth.JWTSecret) == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if c.Log.FileLoggingEnabled() {
		if strings.TrimSpace(c.Log.FileDir) == "" {
			return fmt.Errorf("log.file_dir is required when log.file_enabled is true")
		}
	}
	if c.Upload.MaxSizeBytes <= 0 {
		return fmt.Errorf("upload.max_size must be > 0")
	}
	if len(c.Upload.AllowedExts) == 0 {
		return fmt.Errorf("upload.allowed_exts must not be empty")
	}
	if c.Converter.WorkerCount <= 0 {
		return fmt.Errorf("converter.worker_count must be > 0")
	}
	if c.Converter.QueueSize <= 0 {
		return fmt.Errorf("converter.queue_size must be > 0")
	}
	if c.Converter.OfficeTimeout <= 0 {
		return fmt.Errorf("converter.office_timeout must be > 0")
	}
	if c.Converter.RequeueInterval <= 0 {
		return fmt.Errorf("converter.requeue_interval must be > 0")
	}
	if c.Converter.RetryCount < 0 {
		return fmt.Errorf("converter.retry_count must be >= 0")
	}
	if c.Converter.RetryInterval <= 0 {
		return fmt.Errorf("converter.retry_interval must be > 0")
	}
	fit := strings.TrimSpace(strings.ToLower(c.Converter.ExcelPageFit))
	switch fit {
	case "", "fit_width":
		c.Converter.ExcelPageFit = "fit_width"
	case "auto":
		c.Converter.ExcelPageFit = "auto"
	default:
		return fmt.Errorf("converter.excel_page_fit must be fit_width or auto, got %q", c.Converter.ExcelPageFit)
	}
	mode := strings.TrimSpace(strings.ToLower(c.Converter.ComMode))
	switch mode {
	case "", "subprocess":
		c.Converter.ComMode = "subprocess"
	case "inprocess":
		c.Converter.ComMode = "inprocess"
	default:
		return fmt.Errorf("converter.com_mode must be subprocess or inprocess, got %q", c.Converter.ComMode)
	}
	if c.Converter.TempSandboxEnabled() &&
		c.Converter.ComMode == "inprocess" &&
		c.Converter.WorkerCount > 1 {
		return fmt.Errorf("converter.temp_sandbox with com_mode=inprocess requires worker_count=1")
	}

	if c.Converter.Engines == nil {
		c.Converter.Engines = []string{"msoffice"}
	}
	if len(c.Converter.Engines) == 0 {
		return fmt.Errorf("converter.engines must not be empty")
	}
	seenEngines := map[string]struct{}{}
	for i, raw := range c.Converter.Engines {
		name := strings.TrimSpace(strings.ToLower(raw))
		switch name {
		case "msoffice", "wpsoffice", "openoffice":
		default:
			return fmt.Errorf("converter.engines[%d] unknown engine %q (want msoffice, wpsoffice, or openoffice)", i, raw)
		}
		if _, ok := seenEngines[name]; ok {
			return fmt.Errorf("converter.engines duplicate name %q", name)
		}
		seenEngines[name] = struct{}{}
		c.Converter.Engines[i] = name
	}
	if _, ok := seenEngines["openoffice"]; ok {
		ooCmd := strings.TrimSpace(c.Converter.OpenOffice.Command)
		ooProf := strings.TrimSpace(c.Converter.OpenOffice.UserProfile)
		if ooCmd == "" {
			return fmt.Errorf("converter.openoffice.command is required when engines includes openoffice")
		}
		if ooProf == "" {
			return fmt.Errorf("converter.openoffice.user_profile is required when engines includes openoffice")
		}
		c.Converter.OpenOffice.Command = ooCmd
		c.Converter.OpenOffice.UserProfile = ooProf
	}
	if c.Converter.ExtEngines == nil {
		c.Converter.ExtEngines = map[string]string{}
	}
	normalizedExtEngines := make(map[string]string, len(c.Converter.ExtEngines))
	for k, v := range c.Converter.ExtEngines {
		ext := normalizeExtPattern(k)
		if ext == "" || ext == "*" {
			return fmt.Errorf("converter.ext_engines key %q is invalid", k)
		}
		eng := strings.TrimSpace(strings.ToLower(v))
		if _, ok := seenEngines[eng]; !ok {
			return fmt.Errorf("converter.ext_engines[%q] engine %q is not in converter.engines", k, v)
		}
		if prev, ok := normalizedExtEngines[ext]; ok && prev != eng {
			return fmt.Errorf("converter.ext_engines conflicting mappings for %q", ext)
		}
		normalizedExtEngines[ext] = eng
	}
	c.Converter.ExtEngines = normalizedExtEngines

	if c.Upload.ValidateNew == nil {
		c.Upload.ValidateNew = map[string][]string{}
	}
	if c.Upload.ValidateOLE == nil {
		c.Upload.ValidateOLE = map[string][]string{}
	}

	for ext := range c.Converter.ExtEngines {
		if c.Upload.AppKind(ext) == "" {
			return fmt.Errorf("converter.ext_engines[%q]: cannot infer app kind from upload.validate_new/validate_ole (need word/|xl/|ppt/ or WordDocument|Workbook|PowerPoint Document)", ext)
		}
	}

	for _, pattern := range c.Upload.AllowedExts {
		ext := normalizeExtPattern(pattern)
		if ext == "" || ext == "*" {
			continue
		}
		inNew := len(lookupValidateEntries(c.Upload.ValidateNew, ext)) > 0
		inOLE := len(lookupValidateEntries(c.Upload.ValidateOLE, ext)) > 0
		if inNew && inOLE {
			return fmt.Errorf("upload.allowed_exts %q cannot appear in both validate_new and validate_ole", ext)
		}
		// neither: allowed (structure validation skipped)
	}
	if c.Cleanup.UploadTTL > 0 {
		slog.Warn("cleanup.upload_ttl is deprecated and ignored; terminal uploads archive immediately")
	}
	if c.Cleanup.HistoryTTLEnabled && c.Cleanup.HistoryTTL <= 0 {
		return fmt.Errorf("cleanup.history_ttl must be > 0 when history_ttl_enabled is true")
	}
	if c.Cleanup.PdfTTL <= 0 {
		return fmt.Errorf("cleanup.pdf_ttl must be > 0")
	}
	if c.Cleanup.Interval <= 0 {
		return fmt.Errorf("cleanup.interval must be > 0")
	}
	if err := c.Watermark.validateAndNormalize(); err != nil {
		return err
	}
	return nil
}

func (w *WatermarkConfig) validateAndNormalize() error {
	d := strings.TrimSpace(strings.ToLower(w.Density))
	switch d {
	case "":
		w.Density = "medium"
	case "light", "medium", "heavy":
		w.Density = d
	default:
		return fmt.Errorf("watermark.density must be light, medium, or heavy, got %q", w.Density)
	}
	if w.DensityCount < 0 || w.DensityCount >= 20 {
		return fmt.Errorf("watermark.density_count must be 0 (use density) or 1..19, got %d", w.DensityCount)
	}
	if w.Opacity < 0 || w.Opacity > 1 {
		return fmt.Errorf("watermark.opacity must be in [0,1], got %v", w.Opacity)
	}
	if w.FontSize < 0 {
		return fmt.Errorf("watermark.font_size must be >= 0, got %v", w.FontSize)
	}
	c := strings.TrimSpace(w.Color)
	if c == "" {
		w.Color = "#808080"
	} else {
		if len(c) != 7 || c[0] != '#' {
			return fmt.Errorf("watermark.color must be #RRGGBB, got %q", w.Color)
		}
		for _, ch := range c[1:] {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return fmt.Errorf("watermark.color must be #RRGGBB, got %q", w.Color)
			}
		}
		w.Color = c
	}
	return nil
}
