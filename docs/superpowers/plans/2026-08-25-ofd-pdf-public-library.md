# OFD→PDF Public Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a same-repo public `pkg/ofd` library (GB/T 33190 → PDF, hybrid vector/raster) that the service calls only from `convert-worker`, plus a generic document-password path for every engine.

**Architecture:** Parent `ofdEngine` only `exec`s `convert-worker --engine ofd` (Windows Job Object; Unix process group). The worker imports `pkg/ofd` and calls `ofd.Convert(ctx, src, dst, ofd.Options{Password})` — never `converter.New` with engine `ofd` (that would recurse). Password lives on `queue.Task` plus an in-memory cache keyed by upload ID, and reaches COM/ofd workers only via env `MSOFFICE2PDF_DOC_PASSWORD`.

**Tech Stack:** Go 1.25, `archive/zip` + `encoding/xml`, existing `github.com/signintech/gopdf` / `github.com/pdfcpu/pdfcpu`, Gin upload, Vue upload form. No third-party OFD libraries. No Java.

**Spec:** `docs/superpowers/specs/2026-08-25-ofd-pdf-public-library-design.md`

## Global Constraints

- Public library path is exactly `pkg/ofd` (not `internal/ofd`, not a second Go module, not a DLL).
- `pkg/ofd` must not import `internal/converter` or `internal/queue`.
- Parent `internal/converter` ofdEngine must not import `pkg/ofd`.
- Engine name is exactly `ofd`. Legal `converter.engines`: `msoffice` | `wpsoffice` | `openoffice` | `ofd` (Go: `EngineMSOffice`, `EngineWPSOffice`, `EngineOpenOffice`, `EngineOFD`).
- Parent OFD conversion **always** uses `convert-worker`, independent of `converter.com_mode`.
- `convert-worker --engine ofd` must call `ofd.Convert` **directly**.
- Password never appears in argv, logs, pdflog, DB columns, or file/sandbox names.
- Env for subprocess password: `MSOFFICE2PDF_DOC_PASSWORD`.
- HTTP: Header `X-Doc-Password` wins over form field `password` when both are non-empty.
- Password errors: `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG` — **no retry**.
- Unencrypted file + extra password → ignore password, convert normally.
- `ext_engines` value `ofd` skips AppKind inference.
- New config table `upload.validate_ofd`; mutually exclusive with `validate_new` / `validate_ole` on the same extension.
- OfficeFamily `"ofd"` → ZIP/PK magic + ZIP member check.
- Fixtures live in `pkg/ofd/fixtures` (do **not** use a directory named `testdata/`).
- Sync any new YAML **keys** into `config/config.yaml`, `config/config.template_zh.yaml`, and `config/config.template_en.yaml`.
- `docs/` is gitignored; committing docs files requires `git add -f`.
- No COM E2E; no third-party OFD module; no GM auto-decrypt without the user password.

---

## File map

| File | Responsibility |
|------|----------------|
| `pkg/ofd/convert.go` | Public `Convert(ctx, src, dst, Options)` |
| `pkg/ofd/errors.go` | `ErrPasswordRequired`, `ErrPasswordWrong`, `ErrInvalidPackage`, `ErrNoPages` |
| `pkg/ofd/open.go` | ZIP package reader, `OFD.xml` location |
| `pkg/ofd/doc.go` | GB/T 33190 document/page/resource model (unexported) |
| `pkg/ofd/render.go` | Hybrid PDF write via gopdf |
| `pkg/ofd/raster.go` | Object-bbox raster fallback |
| `pkg/ofd/crypt.go` | Encryptions.xml + StdAES fixture decrypt |
| `pkg/ofd/logger.go` | slog WARN for skipped objects |
| `pkg/ofd/fixtures/` | Generated/committed OFD zip fixtures |
| `internal/converter/password.go` | Env name, app-layer sentinels, `IsPasswordError`, `ParseWorkerPasswordError` |
| `internal/converter/converter.go` | `Convert(ctx, src, dst, password string)` |
| `internal/converter/engine.go` | `EngineOFD = "ofd"` |
| `internal/converter/engine_iface.go` | Engine.Convert gains password |
| `internal/converter/ofd_engine.go` | Parent spawn-only engine (no `pkg/ofd` import) |
| `internal/converter/spawn_windows.go` / `spawn_unix.go` | Shared Job Object / process-group spawn |
| `internal/converter/com_windows.go` | Open with password |
| `internal/converter/subprocess_windows.go` | Password env; COM still uses `--app-kind` |
| `internal/converter/openoffice.go` | Pass password to soffice when non-empty |
| `internal/converter/stub_other.go` | Load `ofdEngine`; Convert signature |
| `internal/converter/env_windows.go` / `env_other.go` | `ofd` needs no COM/CLI probe |
| `cmd/msoffice2pdf/cli_convert_worker.go` | `--engine ofd` → `ofd.Convert` |
| `internal/config/config.go` | `ValidateOFD`, engine `ofd`, skip AppKind, family `ofd` |
| `internal/validate/validate.go` / `magic.go` | Family `ofd` ZIP magic + members |
| `internal/queue/task.go` | `DocPassword string` |
| `internal/queue/queue.go` | In-memory password cache by upload ID |
| `internal/queue/worker.go` | Pass password; no-retry archive on password errors |
| `internal/queue/requeue.go` | Restore password from cache |
| `internal/service/upload.go` | Accept password onto Task |
| `internal/handlers/upload.go` | Header then form |
| `ui/src/api/upload.ts` | Form field `password` |
| `ui/src/views/UploadView.vue` | Optional password input |
| `ui/src/i18n/locales/zh-CN.ts` / `en.ts` | Labels + error copy |
| Config YAML + usage/detailed design | Keys, examples, error codes |

**Interfaces (locked):**

```go
// pkg/ofd
type Options struct{ Password string }
func Convert(ctx context.Context, srcPath, dstPath string, opts Options) error
var (
    ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
    ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
    ErrInvalidPackage   = errors.New("ERR_OFD_INVALID_PACKAGE")
    ErrNoPages          = errors.New("ERR_OFD_NO_PAGES")
)

// converter.Converter / Engine
Convert(ctx context.Context, srcPath, dstPath, password string) error
const DocPasswordEnv = "MSOFFICE2PDF_DOC_PASSWORD"
var (
    ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
    ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
)
func IsPasswordError(err error) bool
func ParseWorkerPasswordError(msg string) error // maps stderr to sentinels or nil

// domain
ErrDocPasswordRequired = "ERR_DOC_PASSWORD_REQUIRED"
ErrDocPasswordWrong    = "ERR_DOC_PASSWORD_WRONG"

// queue.Task
DocPassword string // memory only
```

---

### Task 1: Password sentinels + Convert signature

**Files:**
- Create: `internal/converter/password.go`, `internal/converter/password_test.go`
- Modify: `internal/domain/errors.go`
- Modify: every `Convert(...)` impl and caller so the tree compiles (`converter.go`, `engine_iface.go`, `openoffice.go`, `stub_other.go`, `com_windows.go`, `subprocess_windows.go`, `internal/queue/worker.go`, `internal/queue/task.go`, `cmd/msoffice2pdf/cli_convert_worker.go`)

**Interfaces:**
- Consumes: existing `Convert(ctx, src, dst)`
- Produces: `Convert(ctx, src, dst, password string)`; empty password = none

- [ ] **Step 1: Write failing tests**

```go
package converter

import (
	"errors"
	"testing"
)

func TestIsPasswordError(t *testing.T) {
	if IsPasswordError(errors.New("boom")) {
		t.Fatal("plain error")
	}
	if !IsPasswordError(ErrPasswordRequired) || !IsPasswordError(ErrPasswordWrong) {
		t.Fatal("sentinels")
	}
	if !IsPasswordError(errors.Join(ErrPasswordWrong, errors.New("ole"))) {
		t.Fatal("wrapped")
	}
}

func TestPasswordEnvRoundTrip(t *testing.T) {
	t.Setenv(DocPasswordEnv, "s3cret")
	if got := PasswordFromEnv(); got != "s3cret" {
		t.Fatalf("got %q", got)
	}
}

func TestParseWorkerPasswordError(t *testing.T) {
	if !errors.Is(ParseWorkerPasswordError("converter: ERR_DOC_PASSWORD_REQUIRED"), ErrPasswordRequired) {
		t.Fatal("required")
	}
	if !errors.Is(ParseWorkerPasswordError("ERR_DOC_PASSWORD_WRONG"), ErrPasswordWrong) {
		t.Fatal("wrong")
	}
	if ParseWorkerPasswordError("timeout") != nil {
		t.Fatal("non-password")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL** (undefined names)

Run: `go test ./internal/converter/ -run "TestIsPasswordError|TestPasswordEnvRoundTrip|TestParseWorkerPasswordError" -count=1`

- [ ] **Step 3: Implement**

`internal/converter/password.go`:

```go
package converter

import (
	"errors"
	"os"
	"strings"
)

const DocPasswordEnv = "MSOFFICE2PDF_DOC_PASSWORD"

var (
	ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
	ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
)

func IsPasswordError(err error) bool {
	return errors.Is(err, ErrPasswordRequired) || errors.Is(err, ErrPasswordWrong)
}

func PasswordFromEnv() string {
	return os.Getenv(DocPasswordEnv)
}

func PasswordEnv(base []string, password string) []string {
	out := make([]string, 0, len(base)+1)
	prefix := DocPasswordEnv + "="
	for _, e := range base {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	if password != "" {
		out = append(out, prefix+password)
	}
	return out
}

func ParseWorkerPasswordError(msg string) error {
	switch {
	case strings.Contains(msg, ErrPasswordRequired.Error()):
		return ErrPasswordRequired
	case strings.Contains(msg, ErrPasswordWrong.Error()):
		return ErrPasswordWrong
	default:
		return nil
	}
}
```

`internal/domain/errors.go` add:

```go
ErrDocPasswordRequired = "ERR_DOC_PASSWORD_REQUIRED"
ErrDocPasswordWrong    = "ERR_DOC_PASSWORD_WRONG"
```

Change `Converter` / `Engine` / all impls / `routingConverter` / `comBackendEngine`:

```go
Convert(ctx context.Context, srcPath, dstPath, password string) error
```

`internal/queue/task.go` add `DocPassword string`.

Worker: `q.Converter.Convert(ctx, t.SrcPath, t.DstPath, t.DocPassword)`.

`cli_convert_worker.go`: `conv.Convert(ctx, src, dst, converter.PasswordFromEnv())`.

COM/OpenOffice/stub: accept `password`; ignore until Task 10 except do not log it.

`subprocess_windows.go`: after building `cmd`, `cmd.Env = PasswordEnv(cmd.Env, password)` if `cmd.Env` is nil use `PasswordEnv(os.Environ(), password)` **after** sandbox env is applied so sandbox still wins for TEMP but password is appended. If `ParseWorkerPasswordError(msg) != nil`, return `fmt.Errorf("converter: %w", that)`.

- [ ] **Step 4: Tests pass + compile**

Run: `go test ./internal/converter/ ./internal/queue/ ./internal/domain/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/converter internal/domain/errors.go internal/queue/worker.go internal/queue/task.go cmd/msoffice2pdf/cli_convert_worker.go
git commit -m "feat: thread document password through Convert"
```

---

### Task 2: Config — engine `ofd` + `validate_ofd`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/engines_test.go`

**Interfaces:**
- Consumes: `normalizeExtPattern`, `lookupValidateEntries`, `AppKind`
- Produces: `UploadConfig.ValidateOFD map[string][]string`; `OfficeFamily` → `"ofd"`; Validate allows `ofd`; skip AppKind when engine is `ofd`

- [ ] **Step 1: Write failing tests** (append to `engines_test.go`)

```go
func TestValidateOFDEngineAndFamily(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"ofd"}
	c.Converter.ExtEngines = map[string]string{"*.ofd": "ofd"}
	c.Upload.AllowedExts = []string{"*.ofd"}
	c.Upload.ValidateNew = nil
	c.Upload.ValidateOLE = nil
	c.Upload.ValidateOFD = map[string][]string{"*.ofd": {"OFD.xml"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("ofd mapping should pass: %v", err)
	}
	if c.Upload.OfficeFamily("x.ofd") != "ofd" {
		t.Fatalf("family: %q", c.Upload.OfficeFamily("x.ofd"))
	}
	if c.Upload.AppKind("x.ofd") != "" {
		t.Fatal("ofd must not infer office app kind")
	}
}

func TestValidateOFDMutexWithNew(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"msoffice", "ofd"}
	c.Converter.ExtEngines = map[string]string{"*.docx": "msoffice", "*.ofd": "ofd"}
	c.Upload.AllowedExts = []string{"*.docx", "*.ofd"}
	c.Upload.ValidateOFD = map[string][]string{"*.ofd": {"OFD.xml"}}
	c.Upload.ValidateNew["*.ofd"] = []string{"word/document.xml"}
	if err := c.Validate(); err == nil {
		t.Fatal("want mutex error")
	}
}

func TestExtEnginesOFDSkipsAppKind(t *testing.T) {
	c := baseCfg()
	c.Converter.Engines = []string{"ofd"}
	c.Converter.ExtEngines = map[string]string{"*.ofd": "ofd"}
	c.Upload.AllowedExts = []string{"*.ofd"}
	c.Upload.ValidateNew = nil
	c.Upload.ValidateOLE = nil
	c.Upload.ValidateOFD = map[string][]string{"*.ofd": {"OFD.xml"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("skip app kind: %v", err)
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/config/ -run "TestValidateOFD|TestExtEnginesOFD" -count=1`

- [ ] **Step 3: Implement**

Add to `UploadConfig`:

```go
ValidateOFD map[string][]string `yaml:"validate_ofd"`
```

Nil-init like `ValidateNew` in `Validate()`.

`OfficeFamily`: if only `validate_ofd` owns the ext → `"ofd"`. If ofd overlaps new/ole, still return `""` here; mutex in Validate will reject.

Engine switch add `"ofd"`. Error text: `want msoffice, wpsoffice, openoffice, or ofd`.

Ext-engines AppKind loop skip `eng == "ofd"`.

Allowed-ext mutex: an ext must not appear in more than one of `validate_new` / `validate_ole` / `validate_ofd`.

- [ ] **Step 4: Tests pass**

Run: `go test ./internal/config/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/engines_test.go
git commit -m "feat: allow ofd engine and validate_ofd config"
```

---

### Task 3: Upload validation for OFD packages

**Files:**
- Modify: `internal/validate/magic.go`, `internal/validate/validate.go`
- Create: `internal/validate/ofd_test.go`

**Interfaces:**
- Consumes: `OfficeFamily` `"ofd"`, `ValidateOFD`
- Produces: `File` accepts OFD ZIP with required members

- [ ] **Step 1: Failing test**

```go
package validate_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/validate"
)

func boolPtr(v bool) *bool { return &v }

func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.ofd")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOFDMagicAndStructure(t *testing.T) {
	cfg := config.UploadConfig{
		ValidateMagic: boolPtr(true),
		ValidateOFD:   map[string][]string{"*.ofd": {"OFD.xml"}},
	}
	pathOK := writeZip(t, map[string]string{"OFD.xml": "<ofd/>"})
	if err := validate.File(pathOK, "a.ofd", cfg); err != nil {
		t.Fatal(err)
	}
	pathBad := writeZip(t, map[string]string{"readme.txt": "x"})
	if err := validate.File(pathBad, "a.ofd", cfg); err == nil || !errors.Is(err, validate.ErrStructure) {
		t.Fatalf("want structure, got %v", err)
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/validate/ -run TestOFDMagicAndStructure -count=1`

- [ ] **Step 3: Implement**

`checkMagic`: `case "ofd":` same PK check as `"ooxml"`.

`File`: `case "ofd":` `LookupValidateEntries(ValidateOFD)` then existing ZIP member checker.

- [ ] **Step 4: Pass**

Run: `go test ./internal/validate/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/validate
git commit -m "feat: validate OFD zip magic and OFD.xml"
```

---

### Task 4: Queue password cache and no-retry failures

**Files:**
- Modify: `internal/queue/queue.go`, `internal/queue/worker.go`, `internal/queue/requeue.go`
- Create: `internal/queue/password_test.go` (test cache helpers; if they are unexported, put tests in package `queue`)

**Interfaces:**
- Consumes: `Task.DocPassword`, `converter.IsPasswordError`
- Produces: cache by upload ID; password errors archive immediately with domain codes

- [ ] **Step 1: Failing tests** (`package queue`)

```go
func TestPasswordCacheRoundTrip(t *testing.T) {
	q := &Queue{passwords: map[int64]string{}}
	q.setPassword(7, "secret")
	if q.passwordFor(7) != "secret" {
		t.Fatal("cache")
	}
	q.clearPassword(7)
	if q.passwordFor(7) != "" {
		t.Fatal("cleared")
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/queue/ -run TestPasswordCacheRoundTrip -count=1`

- [ ] **Step 3: Implement**

On `Queue` add `passwords map[int64]string` (init in `New`).

```go
func (q *Queue) setPassword(id int64, pw string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.passwords == nil {
		q.passwords = map[int64]string{}
	}
	if pw == "" {
		delete(q.passwords, id)
		return
	}
	q.passwords[id] = pw
}

func (q *Queue) passwordFor(id int64) string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.passwords[id]
}

func (q *Queue) clearPassword(id int64) {
	q.mu.Lock()
	delete(q.passwords, id)
	q.mu.Unlock()
}
```

`TryEnqueue`: after accepting, `q.setPassword(t.UploadID, t.DocPassword)` (also when already inflight, refresh password if non-empty).

`requeueOnce`: `task.DocPassword = q.passwordFor(u.ID)`.

`doneInflight`: do **not** clear password until terminal archive (retry still needs it). Clear in `failConvert` after archive and in success `ArchiveUpload` path via `clearPassword`.

In `worker.go` after Convert error:

```go
if converter.IsPasswordError(err) {
    code := domain.ErrDocPasswordRequired
    if errors.Is(err, converter.ErrPasswordWrong) {
        code = domain.ErrDocPasswordWrong
    }
    q.failPassword(t.UploadID, pdf, code, code)
    return
}
```

`failPassword` marks pdf failed, `RecordFailure`, then **always** `ArchiveUpload(..., code, code, expired)` — do not wait for `retry_count`. Then `clearPassword`.

Do not log the password; `err.Error()` is the code string only.

- [ ] **Step 4: Pass**

Run: `go test ./internal/queue/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/queue
git commit -m "feat: cache doc password and skip retry on password errors"
```

---

### Task 5: HTTP Header / form password into Task

**Files:**
- Modify: `internal/handlers/upload.go`
- Create: `internal/handlers/docpass_test.go`
- Modify: `internal/service/upload.go` — `Upload(..., docPassword string)` and set `Task.DocPassword`

**Interfaces:**
- Consumes: Header `X-Doc-Password`, form `password`
- Produces: `pickDocPassword`; service passes through to Task

- [ ] **Step 1: Failing test**

```go
package handlers

import "testing"

func TestPickDocPassword(t *testing.T) {
	if pickDocPassword("H", "F") != "H" {
		t.Fatal("header wins")
	}
	if pickDocPassword("", "F") != "F" {
		t.Fatal("form")
	}
	if pickDocPassword("  ", "F") != "F" {
		t.Fatal("blank header")
	}
	if pickDocPassword("", "  ") != "" {
		t.Fatal("blank form")
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/handlers/ -run TestPickDocPassword -count=1`

- [ ] **Step 3: Implement**

```go
func pickDocPassword(header, form string) string {
	h := strings.TrimSpace(header)
	if h != "" {
		return h
	}
	return strings.TrimSpace(form)
}
```

Handler: `docPass := pickDocPassword(c.GetHeader("X-Doc-Password"), c.PostForm("password"))` then `h.Svc.Upload(..., wm, reqID, docPass)`.

Service signature add last arg `docPassword string`; put it on `queue.Task{..., DocPassword: docPassword}`. Update every `Upload(` call site (tests/helpers) so it compiles.

- [ ] **Step 4: Pass**

Run: `go test ./internal/handlers/ ./internal/service/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/handlers internal/service/upload.go
git commit -m "feat: accept document password from header or form"
```

---

### Task 6: `pkg/ofd` parse + vector PDF (public API)

**Files:**
- Create: `pkg/ofd/errors.go`, `pkg/ofd/convert.go`, `pkg/ofd/open.go`, `pkg/ofd/doc.go`, `pkg/ofd/xmltypes.go`, `pkg/ofd/render.go`, `pkg/ofd/logger.go`
- Create: `pkg/ofd/convert_test.go`, `pkg/ofd/fixture_test.go`
- Create: `pkg/ofd/fixtures/` only if you commit binary zips; prefer generating zips in tests via `fixture_test.go`

**Interfaces:**
- Consumes: none
- Produces: `ofd.Convert`, exported errors, vector PDF for Text/Path/Image

- [ ] **Step 1: Write failing tests** that generate a minimal OFD zip in `t.TempDir()`:

Required zip members (GB/T 33190-shaped):

- `OFD.xml` with one DocBody `DocRoot="Doc_0/Document.xml"`
- `Doc_0/Document.xml` with `PhysicalBox="0 0 210 297"`, `PublicRes="PublicRes.xml"`, one Page `BaseLoc="Pages/Page_0/Content.xml"`
- `Doc_0/PublicRes.xml` with a Font ID `1` FontName `Dummy` (no FontFile)
- `Doc_0/Pages/Page_0/Content.xml` with Layer containing:
  - `TextObject` Boundary + TextCode `Hello`
  - `PathObject` with `AbbreviatedData` `M 10 10 L 20 10`
  - `ImageObject` ResourceID pointing at a tiny PNG in PublicRes MultiMedia

Assert: `Convert` succeeds; destination exists; file starts with `%PDF`; `pdfcpu` or a byte search shows the page is not a single full-page XObject only (text/path present). Minimum bar: PDF opens (`%PDF` header + `%%EOF`) and page count is 1. Add a second DocBody fixture test that yields 2 pages.

Also: empty zip → `ErrInvalidPackage`; OFD.xml with no pages → `ErrNoPages`; cancelled ctx → error and no final dst.

- [ ] **Step 2: Run — FAIL**

Run: `go test ./pkg/ofd/ -count=1`

- [ ] **Step 3: Implement public API + pipeline**

`pkg/ofd/errors.go`:

```go
package ofd

import "errors"

var (
	ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
	ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
	ErrInvalidPackage   = errors.New("ERR_OFD_INVALID_PACKAGE")
	ErrNoPages          = errors.New("ERR_OFD_NO_PAGES")
)
```

`pkg/ofd/convert.go`:

```go
func Convert(ctx context.Context, srcPath, dstPath string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pkg, err := openPackage(srcPath)
	if err != nil {
		return err
	}
	defer pkg.Close()
	if err := decryptIfNeeded(pkg, opts.Password); err != nil {
		return err
	}
	doc, err := loadDocument(pkg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	tmp := dstPath + ".partial"
	if err := renderPDF(ctx, doc, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

type Options struct {
	Password string
}
```

In this task `decryptIfNeeded` is a no-op returning nil (Task 8 fills it).

`openPackage`: `zip.OpenReader`; find `OFD.xml` (case-insensitive, also `./OFD.xml`); missing → `ErrInvalidPackage`. Wrap zip open errors as `fmt.Errorf("%w: %v", ErrInvalidPackage, err)`.

Unexported model: pages with WidthMM/HeightMM, layers of text/path/image objects; fonts map; images map. Namespace resource IDs by DocBody index (`%d:%s`).

Parse Content.xml with `encoding/xml` token walk (OFD namespace: ignore `Name.Space`, match `Local`). Coordinate: OFD origin top-left mm; PDF origin bottom-left points (`pt = mm * 72 / 25.4`). Apply Boundary and CTM.

Render with `gopdf`: one PDF page per OFD page; `AddTTFFontByReader` when font bytes exist; else `AddTTFFont` from Windows `C:\Windows\Fonts\arial.ttf` or `simhei.ttf` if present, else skip text with slog WARN. Path: parse AbbreviatedData commands M/L/C/Q/A/S/B (implement M/L/C at minimum; unknown commands WARN skip). Images: PNG/JPEG via gopdf ImageFromBytes / register.

Skip VideoObject/AudioObject/Attachment/Bookmark with slog WARN.

Do not import `internal/*`.

- [ ] **Step 4: Pass**

Run: `go test ./pkg/ofd/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add pkg/ofd
git commit -m "feat: add pkg/ofd Convert with vector PDF render"
```

---

### Task 7: Object-bbox raster fallback

**Files:**
- Modify: `pkg/ofd/doc.go`, `pkg/ofd/render.go`
- Create: `pkg/ofd/raster.go`, `pkg/ofd/raster_test.go`

**Interfaces:**
- Consumes: Task 6 `pageObject` set
- Produces: stamp/gradient/colorspace/blend objects drawn as PNG in the object Boundary, not full-page raster

- [ ] **Step 1: Failing test**

Build a fixture whose Content has `PathObject` with child `AxialShd`, and a `StampAnnot` (or `StampObject`). Convert. Assert PDF exists and page count 1. Assert the test records that those objects were classified as raster (export a test-only helper `classifyForTest(raw xml) []string` returning `gradient` / `stamp`, **or** check logs via a hook). Simplest: unexported `decodePathObject` test in package `ofd` that AxialShd yields `rasterObject{Reason:"gradient"}`.

- [ ] **Step 2: Run — FAIL**

Run: `go test ./pkg/ofd/ -run Raster -count=1`

- [ ] **Step 3: Implement**

When decoding: AxialShd/RadialShd/Pattern → `rasterObject{Reason: "gradient"}`. StampObject/StampAnnot/Seal → `"stamp"`. Unsupported ColorSpace → `"colorspace"`. BlendMode not normal/compatible → `"blend"`.

`drawRaster`: fill Boundary with a placeholder PNG (solid color or `image.NRGBA` of the bbox size at 96 dpi) and embed as image. Apply CTM if present. Never rasterize the whole page as the only content. Riding stamp: if Boundary crosses page edge, clip to page box (intersect rectangles) then embed.

CompositeObject: look up vector resource by ResourceID if present in Res; else WARN skip. Recurse children when the composite XML embeds them; if only ResourceID, skip with WARN until a CompositeGraph unit is loaded from Res (load CompositeGraphicUnit from PublicRes if the XML has it; otherwise WARN).

- [ ] **Step 4: Pass**

Run: `go test ./pkg/ofd/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add pkg/ofd
git commit -m "feat: rasterize OFD stamps and unsupported paint"
```

---

### Task 8: OFD encryption with user password

**Files:**
- Create: `pkg/ofd/crypt.go`, `pkg/ofd/crypt_test.go`
- Modify: `decryptIfNeeded` in convert/open path

**Interfaces:**
- Consumes: `Options.Password`, `ErrPasswordRequired`, `ErrPasswordWrong`
- Produces: StdAES fixture decrypt; GM/unknown algo maps to password sentinels

- [ ] **Step 1: Failing tests**

Generate three zips:

1. Unencrypted (Task 6 fixture) + password `"x"` → success
2. Encrypted members: include `Doc_0/Encryptions.xml` with `EncryptMethod="StdAES"`; ciphertext = AES-256-GCM of UTF-8 XML using key `sha256(password)` (document IV in the XML as hex). Empty password → `ErrPasswordRequired`. Wrong password → `ErrPasswordWrong`. Correct password → Convert success.
3. `EncryptMethod="SM4"` (unsupported): empty password → `ErrPasswordRequired`; any password → `ErrPasswordWrong`

- [ ] **Step 2: Run — FAIL**

Run: `go test ./pkg/ofd/ -run Crypt -count=1`

- [ ] **Step 3: Implement**

`decryptIfNeeded`: if no Encryptions.xml / Encryption.xml (search zip, case-insensitive) → return nil (ignore password).

If present and password empty → `ErrPasswordRequired`.

Parse EncryptMethod. `StdAES`: decrypt listed entries in place into an overlay map on the package reader (`read` checks overlay first). Fail → `ErrPasswordWrong`.

Any other method: `ErrPasswordWrong` if password non-empty else `ErrPasswordRequired`.

Do not log password. `Error()` stays the sentinel string.

- [ ] **Step 4: Pass**

Run: `go test ./pkg/ofd/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add pkg/ofd
git commit -m "feat: decrypt OFD with user password in pkg/ofd"
```

---

### Task 9: `ofdEngine` subprocess + convert-worker in-process `pkg/ofd`

**Files:**
- Create: `internal/converter/ofd_engine.go`, `internal/converter/ofd_engine_test.go`
- Create: `internal/converter/spawn_windows.go` (extract from `subprocess_windows.go`)
- Create: `internal/converter/spawn_unix.go` (`//go:build !windows`)
- Modify: `engine.go` add `EngineOFD`
- Modify: `com_windows.go` `newConverter`, `stub_other.go`, `env_windows.go`, `env_other.go`
- Modify: `cmd/msoffice2pdf/cli_convert_worker.go`

**Interfaces:**
- Consumes: `ofd.Convert`, `PasswordFromEnv`, `runConvertWorker`
- Produces: parent never imports `pkg/ofd`; worker never constructs `ofdEngine`

- [ ] **Step 1: Failing test**

```go
func TestOFDWorkerArgs(t *testing.T) {
	got := ofdWorkerArgs("C:\\a.ofd", "C:\\a.pdf")
	want := []string{"convert-worker", "--src", "C:\\a.ofd", "--dst", "C:\\a.pdf", "--engine", EngineOFD}
	if len(got) != len(want) {
		t.Fatalf("%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q", got)
		}
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/converter/ -run TestOFDWorkerArgs -count=1`

- [ ] **Step 3: Implement**

`engine.go`: `EngineOFD = "ofd"`

`ofd_engine.go` (no build tag; no `pkg/ofd` import):

```go
type ofdEngine struct{}

func (e *ofdEngine) Name() string { return EngineOFD }
func (e *ofdEngine) Validate() error { return nil }
func (e *ofdEngine) ProcessImages() []string { return nil }

func (e *ofdEngine) Convert(ctx context.Context, srcPath, dstPath, password string) error {
	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("converter: abs src: %w", err)
	}
	dstPath, err = filepath.Abs(dstPath)
	if err != nil {
		return fmt.Errorf("converter: abs dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("converter: mkdir dst: %w", err)
	}
	_ = os.Remove(dstPath)
	return runConvertWorker(ctx, ofdWorkerArgs(srcPath, dstPath), password, os.Environ())
}

func ofdWorkerArgs(src, dst string) []string {
	return []string{"convert-worker", "--src", src, "--dst", dst, "--engine", EngineOFD}
}
```

Extract `runConvertWorker(ctx, args []string, password string, baseEnv []string) error` from current subprocess start/wait/job-object logic. COM subprocess builds its argv then calls the same helper. Unix: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`; on `ctx.Done()` `syscall.Kill(-pid, SIGKILL)`.

`newConverter` (windows + stub_other): `case EngineOFD: engines[n] = &ofdEngine{}`.

`ValidateEnvironment`: `case EngineOFD:` no-op (do not hit `unknown engine`).

`cli_convert_worker.go` **before** `converter.New`:

```go
if engine == converter.EngineOFD {
    return mapOFDErr(ofd.Convert(context.Background(), src, dst, ofd.Options{Password: converter.PasswordFromEnv()}))
}
func mapOFDErr(err error) error {
    if err == nil {
        return nil
    }
    if errors.Is(err, ofd.ErrPasswordRequired) {
        return converter.ErrPasswordRequired
    }
    if errors.Is(err, ofd.ErrPasswordWrong) {
        return converter.ErrPasswordWrong
    }
    return err
}
```

Keep msoffice/wpsoffice requiring `--app-kind`. ofd must not require it.

- [ ] **Step 4: Pass**

Run: `go test ./internal/converter/ ./pkg/ofd/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/converter cmd/msoffice2pdf/cli_convert_worker.go
git commit -m "feat: run OFD conversion in convert-worker via pkg/ofd"
```

---

### Task 10: Office COM / OpenOffice passwords

**Files:**
- Modify: `internal/converter/com_windows.go`, `openoffice.go`

**Interfaces:**
- Consumes: `Convert(..., password)`, `ErrPasswordRequired`, `ErrPasswordWrong`
- Produces: Word/Excel/PowerPoint/soffice open with password; map failures to sentinels

- [ ] **Step 1: Unit-test the mapper only** (no COM)

```go
func TestMapOfficeOpenError(t *testing.T) {
	if !errors.Is(mapOfficeOpenError(errors.New("Password"), "", true), ErrPasswordRequired) {
		t.Fatal("required")
	}
	if !errors.Is(mapOfficeOpenError(errors.New("Password"), "x", true), ErrPasswordWrong) {
		t.Fatal("wrong")
	}
}
```

`looksLikePassword` is true when the OLE error string contains `password` (case-insensitive) or known HRESULT; if Open failed and `looksLikePassword` cannot be decided: empty password → Required, non-empty → Wrong.

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/converter/ -run TestMapOfficeOpenError -count=1`

- [ ] **Step 3: Implement**

Word: first `Open(src, false, true, false, "", "")`; on failure if password empty → `mapOfficeOpenError`; else retry `Open(src, false, true, false, password, "")`.

Excel: `Workbooks.Open` 5th parameter Password — extend the current 3-arg Open to pass password when non-empty; same try-empty-then-password order.

PowerPoint: `Presentations.Open` with password if the signature allows a password argument; else use `mapOfficeOpenError` heuristic.

OpenOffice: if `password != ""`, add `--password=`+password **only** on the soffice argv, never on convert-worker. Strip from logs (do not slog the argv slice).

- [ ] **Step 4: Pass**

Run: `go test ./internal/converter/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/converter/com_windows.go internal/converter/openoffice.go internal/converter/password.go internal/converter/password_test.go
git commit -m "feat: open encrypted Office files with document password"
```

---

### Task 11: Web UI password field

**Files:**
- Modify: `ui/src/api/upload.ts`, `ui/src/views/UploadView.vue`, `ui/src/i18n/locales/zh-CN.ts`, `ui/src/i18n/locales/en.ts`
- Modify: history/board display of `error_code` if they show raw codes (map the two new codes to i18n)

**Interfaces:**
- Consumes: `uploadFile(file, watermark, password, onProgress)`
- Produces: FormData field `password` when non-empty; never in URL

- [ ] **Step 1: Change API**

```ts
export function uploadFile(
  file: File,
  watermark?: string,
  password?: string,
  onProgress?: (percent: number) => void,
) {
  const form = new FormData()
  form.append('file', file)
  const wm = watermark?.trim()
  if (wm) form.append('watermark', wm)
  const pw = password?.trim()
  if (pw) form.append('password', pw)
  // existing axios post
}
```

Update `UploadView.vue` call site: `uploadFile(selected.value, watermark.value, docPassword.value, (p) => ...)`.

Add `el-input type="password"` next to watermark; clear `docPassword` after success like watermark.

i18n:

```
upload.docPassword: 文档密码（可选） / Document password (optional)
upload.docPasswordPlaceholder: 加密文档才需要 / Required only for encrypted files
history.errDocPasswordRequired: 需要文档密码 / Document password required
history.errDocPasswordWrong: 文档密码错误 / Wrong document password
```

Where `error_code` is shown, if value is `ERR_DOC_PASSWORD_REQUIRED` / `WRONG`, show the i18n sentence.

- [ ] **Step 2: `npm run build` in `ui/`** if npm is available so `ui/dist` stays in sync (this repo ships dist).

- [ ] **Step 3: Commit**

```bash
git add ui/src ui/dist
git commit -m "feat: optional document password on web upload"
```

---

### Task 12: Config templates + user docs

**Files:**
- Modify: `config/config.yaml`, `config/config.template_zh.yaml`, `config/config.template_en.yaml`
- Modify: `docs/usage.md` and `docs/详细设计说明书.md` (create/update the OFD engine, `pkg/ofd` API, `validate_ofd`, Header, error codes). `git add -f` those docs.

**Interfaces:** none new

- [ ] **Step 1: YAML keys**

```yaml
converter:
  engines:
    - msoffice
    - ofd
  ext_engines:
    "*.ofd": ofd

upload:
  allowed_exts:
    - "*.ofd"
  validate_ofd:
    "*.ofd":
      - "OFD.xml"
```

Comments: legal engine names include `ofd`; ofd needs no COM probe; `*.ofd` must map to `ofd`; `validate_ofd` mutex with new/ole.

Keep existing Office mappings. Do not drop `wpsoffice` from a machine that uses it; add `ofd` alongside.

- [ ] **Step 2: usage example**

```bash
curl -H "X-Doc-Password: secret" -F "file=@a.ofd" http://127.0.0.1:8080/api/upload
```

State that `ERR_DOC_PASSWORD_*` are async (status/SSE/history), not upload HTTP 400.

- [ ] **Step 3: Commit**

```bash
git add config/config.yaml config/config.template_zh.yaml config/config.template_en.yaml
git add -f docs/usage.md docs/详细设计说明书.md
git commit -m "docs: OFD pkg/ofd engine and document password"
```

---

## Spec coverage

| Spec section | Task |
|--------------|------|
| `pkg/ofd` public Convert/Options | 6 |
| Hybrid vector/raster | 6–7 |
| No third-party OFD lib; no internal import | 6–8 |
| Engine `ofd`, always subprocess | 9 |
| `validate_ofd` / family / skip AppKind | 2–3 |
| Password header/form, header wins | 5, 11 |
| Memory-only password + env | 1, 4, 9 |
| Open order + no retry | 4, 8, 10 |
| Async error codes | 4, 12 |
| Tests/fixtures in `pkg/ofd/fixtures` or generated | 6–8 |
| Docs/templates | 12 |
| Anti-recursion convert-worker | 9 |

## Placeholder / naming check

- Library: `ofd.Convert(ctx, src, dst, ofd.Options{Password: password})`
- App converter: `Convert(ctx, src, dst, password string)`
- Sentinels share strings `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG`
- Config key `validate_ofd`, family `"ofd"`, engine `"ofd"`
- Env `MSOFFICE2PDF_DOC_PASSWORD`
- Header `X-Doc-Password`, form `password`
