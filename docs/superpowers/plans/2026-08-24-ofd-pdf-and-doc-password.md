# OFD→PDF and Document Password Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a self-hosted `ofd` conversion engine (GB/T 33190 → PDF, hybrid vector/raster) that always runs in `convert-worker`, plus a generic document-password path (Header + web form) for every engine.

**Architecture:** Parent process only `exec`s `convert-worker --engine ofd` (Windows Job Object; Unix process group). The worker calls `internal/ofd.Convert` in-process (never spawn another worker). Document password rides on `queue.Task` + an in-memory cache keyed by upload ID, is passed into `Convert(..., password)`, and reaches COM/ofd workers only via env `MSOFFICE2PDF_DOC_PASSWORD`. Password failures archive immediately and skip `retry_count`.

**Tech Stack:** Go 1.25, `archive/zip` + `encoding/xml`, existing `github.com/signintech/gopdf` / `github.com/pdfcpu/pdfcpu`, Gin upload, Vue upload form. No third-party OFD libraries.

**Spec:** `docs/superpowers/specs/2026-08-24-ofd-pdf-and-doc-password-design.md`

## Global Constraints

- Engine name is exactly `ofd`. Legal `converter.engines` values become `msoffice` | `wpsoffice` | `openoffice` | `ofd` (Go consts: `EngineMSOffice`, `EngineWPSOffice`, `EngineOpenOffice`, `EngineOFD`).
- Parent `ofd` conversion **always** uses `convert-worker`, independent of `converter.com_mode`.
- `convert-worker --engine ofd` must call `ofd.Convert` **directly**. `converter.New` with `ofd` in the worker would recurse.
- Password never appears in argv, logs, pdflog, DB columns, or file/sandbox names.
- Env for subprocess password: `MSOFFICE2PDF_DOC_PASSWORD`.
- HTTP: Header `X-Doc-Password` wins over form field `password` when both are non-empty.
- Password errors: `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG` — **no retry**.
- Unencrypted file + extra password → ignore password, convert normally.
- `ext_engines` value `ofd` skips AppKind inference; other engines still require writer/spreadsheet/presentation.
- New config table `upload.validate_ofd`; mutually exclusive with `validate_new` / `validate_ole` on the same extension.
- OfficeFamily `"ofd"` → ZIP/PK magic + ZIP member check (same helper as OOXML).
- Fixtures live in `internal/ofd/fixtures` (do **not** use a directory named `testdata/`).
- Sync any new YAML **keys** into `config/config.yaml`, `config/config.template_zh.yaml`, and `config/config.template_en.yaml`.
- `docs/` is gitignored; committing docs files requires `git add -f`.
- No COM E2E; no third-party OFD module; no GM auto-decrypt without the user password.

---

## File map

| File | Responsibility |
|------|----------------|
| `internal/domain/errors.go` | `ErrDocPasswordRequired`, `ErrDocPasswordWrong` |
| `internal/converter/password.go` | Env name, sentinel errors, `IsPasswordError` |
| `internal/converter/converter.go` | `Convert(ctx, src, dst, password string)` |
| `internal/converter/engine.go` | `EngineOFD = "ofd"` |
| `internal/converter/engine_iface.go` | Engine.Convert gains password; routing passes it |
| `internal/converter/ofd.go` | Parent `ofdEngine`: always spawn `convert-worker` |
| `internal/converter/spawn_windows.go` | Shared Job Object spawn (COM + ofd) |
| `internal/converter/spawn_unix.go` | `Setpgid` + kill process group |
| `internal/converter/com_windows.go` | Open with password; pass through Convert |
| `internal/converter/subprocess_windows.go` | Use spawn helper; set password env; ofd skips `--app-kind` |
| `internal/converter/openoffice.go` | Pass password into soffice when non-empty |
| `internal/converter/stub_other.go` | Load `ofdEngine` on non-Windows; Convert signature |
| `internal/converter/env_windows.go` / `env_other.go` | `ofd` needs no COM/CLI probe |
| `cmd/msoffice2pdf/cli_convert_worker.go` | Allow `--engine ofd`; in-process `ofd.Convert` |
| `internal/ofd/*.go` | Parse GB/T 33190 package + hybrid PDF render |
| `internal/ofd/fixtures/` | Generated/committed OFD zip fixtures for tests |
| `internal/config/config.go` | `ValidateOFD`, engine `ofd`, skip AppKind, family `ofd` |
| `internal/config/engines_test.go` | Config tests for `ofd` |
| `internal/validate/validate.go` / `magic.go` | Family `ofd` ZIP magic + members |
| `internal/queue/task.go` | `DocPassword string` |
| `internal/queue/queue.go` | In-memory password cache by upload ID |
| `internal/queue/worker.go` | Pass password; no-retry archive on password errors |
| `internal/queue/retry.go` / `requeue.go` | Restore password from cache |
| `internal/service/upload.go` | Accept password argument onto Task |
| `internal/handlers/upload.go` | Header then form |
| `ui/src/api/upload.ts` | Form field `password` |
| `ui/src/views/UploadView.vue` | Optional password input |
| `ui/src/i18n/locales/zh-CN.ts` / `en.ts` | Labels + error copy |
| Config YAML + `docs/usage.md` / detailed design | Keys, examples, error codes |

**Interfaces (locked for later tasks):**

```go
// converter.Converter / Engine
Convert(ctx context.Context, srcPath, dstPath, password string) error

const DocPasswordEnv = "MSOFFICE2PDF_DOC_PASSWORD"
var (
    ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
    ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
)
func IsPasswordError(err error) bool

// ofd
func Convert(ctx context.Context, srcPath, dstPath, password string) error

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
- Modify: every `Convert(...)` implementation and caller so the tree compiles (`converter.go`, `engine_iface.go`, `openoffice.go`, `stub_other.go`, `com_windows.go`, `subprocess_windows.go`, `internal/queue/worker.go`, `cmd/msoffice2pdf/cli_convert_worker.go`)

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
```

- [ ] **Step 2: Run test — expect FAIL** (undefined names)

Run: `go test ./internal/converter/ -run "TestIsPasswordError|TestPasswordEnvRoundTrip" -count=1`

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
	return strings.TrimSpace(os.Getenv(DocPasswordEnv))
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
	if strings.TrimSpace(password) != "" {
		out = append(out, prefix+password)
	}
	return out
}
```

`internal/domain/errors.go` add:

```go
ErrDocPasswordRequired = "ERR_DOC_PASSWORD_REQUIRED"
ErrDocPasswordWrong    = "ERR_DOC_PASSWORD_WRONG"
```

Change `Converter` / `Engine` / all impls / `routingConverter` / `comBackendEngine` / worker:

```go
Convert(ctx context.Context, srcPath, dstPath, password string) error
```

Worker: `q.Converter.Convert(ctx, t.SrcPath, t.DstPath, t.DocPassword)` (add the field in Task in this same compile-fix; cache comes in Task 4).

`cli_convert_worker.go`: `conv.Convert(ctx, src, dst, converter.PasswordFromEnv())`.

Stub/openoffice/com: accept the argument; COM/OO may ignore it until Task 8. Do **not** log `password`.

- [ ] **Step 4: Tests pass + `go test ./internal/converter/ ./internal/queue/ ./internal/domain/ -count=1`**

- [ ] **Step 5: Commit**

```bash
git add internal/converter internal/domain internal/queue/worker.go internal/queue/task.go cmd/msoffice2pdf/cli_convert_worker.go
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

Adjust `baseCfg()` if it does not compile (`ValidateOFD` field missing → that's the failing compile).

- [ ] **Step 2: Run — FAIL**

`go test ./internal/config/ -run "TestValidateOFD|TestExtEnginesOFD" -count=1`

- [ ] **Step 3: Implement**

Add `ValidateOFD map[string][]string \`yaml:"validate_ofd"\`` to `UploadConfig`.

Nil-init like `ValidateNew`.

`OfficeFamily`: if only `validate_ofd` owns the ext → `"ofd"`.

Validate engines switch add `"ofd"`.

Ext-engines AppKind loop:

```go
for ext, eng := range c.Converter.ExtEngines {
    if eng == "ofd" {
        continue
    }
    if c.Upload.AppKind(ext) == "" {
        return fmt.Errorf("converter.ext_engines[%q]: cannot infer app kind ...", ext)
    }
}
```

Allowed-ext mutex: an ext must not appear in more than one of `validate_new` / `validate_ole` / `validate_ofd`.

- [ ] **Step 4: Tests pass**

- [ ] **Step 5: Commit** `feat: allow ofd engine and validate_ofd config`

---

### Task 3: Upload validation for OFD packages

**Files:**
- Modify: `internal/validate/magic.go`, `internal/validate/validate.go`
- Create: `internal/validate/ofd_test.go`

- [ ] **Step 1: Failing test**

Build a tiny ZIP with `OFD.xml` in a temp file (use `archive/zip`). Also a ZIP missing `OFD.xml`, and a JPEG renamed to `.ofd`.

```go
func TestOFDMagicAndStructure(t *testing.T) {
    cfg := config.UploadConfig{
        ValidateMagic: boolPtr(true),
        ValidateOFD:   map[string][]string{"*.ofd": {"OFD.xml"}},
    }
    // helper writeZip(t, entries)
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

- [ ] **Step 2: Run — FAIL** (`OfficeFamily` ofd not handled in `File`/`checkMagic`)

- [ ] **Step 3: Implement**

`checkMagic`: `case "ofd":` same PK check as `"ooxml"`.

`File`: `case "ofd":` `LookupValidateEntries(ValidateOFD)` then existing ZIP member checker (`checkZIPMembers` / `checkZIPMembers` — use the function already in the package).

- [ ] **Step 4: Pass** `go test ./internal/validate/ -count=1`

- [ ] **Step 5: Commit** `feat: validate OFD zip magic and OFD.xml`

---

### Task 4: Queue password cache and no-retry failures

**Files:**
- Modify: `internal/queue/queue.go`, `task.go`, `worker.go`, `retry.go`, `requeue.go`
- Create: `internal/queue/password_test.go`

**Produces:**
- `Queue.setPassword` / `passwordFor` / `clearPassword`
- `failConvertNoRetry(uploadID, pdf, code, msg)` → mark pdf failed, `ArchiveUpload(..., code, msg, expired)` **without** `RecordFailure` retry bump
- Worker: if `converter.IsPasswordError(err)` → map to `domain.ErrDocPasswordRequired` or `Wrong`, no retry
- `TryEnqueue` stores `t.DocPassword` in cache; retry/requeue copies `passwordFor(u.ID)` onto Task
- `clearPassword` on success archive and on no-retry fail

- [ ] **Step 1: Unit-test the cache** (table-driven set/get/clear; missing key → `""`)

- [ ] **Step 2: FAIL, then implement on `Queue`** (`pwdMu sync.Mutex`, `passwords map[int64]string`)

- [ ] **Step 3: Worker branch**

```go
err = q.Converter.Convert(ctx, t.SrcPath, t.DstPath, t.DocPassword)
if err != nil {
    _ = storage.RemoveIfExists(t.DstPath)
    if errors.Is(err, converter.ErrPasswordRequired) {
        q.failConvertNoRetry(t.UploadID, pdf, domain.ErrDocPasswordRequired, "document is encrypted; password required")
        return
    }
    if errors.Is(err, converter.ErrPasswordWrong) {
        q.failConvertNoRetry(t.UploadID, pdf, domain.ErrDocPasswordWrong, "document password is incorrect")
        return
    }
    // existing timeout / failConvert path
}
```

Never include `t.DocPassword` in slog attrs.

- [ ] **Step 4: `go test ./internal/queue/ -count=1`**

- [ ] **Step 5: Commit** `feat: keep doc password in memory and skip retry on password errors`

---

### Task 5: HTTP + upload service password

**Files:**
- Modify: `internal/handlers/upload.go`, `internal/service/upload.go`
- Create: `internal/handlers/password_test.go` only if a small helper is extracted; otherwise test helper `pickDocPassword(header, form string) string` in `internal/handlers/docpassword.go`

- [ ] **Step 1: Test pickDocPassword**

```go
func TestPickDocPassword(t *testing.T) {
    if pickDocPassword("H", "F") != "H" { t.Fatal("header wins") }
    if pickDocPassword("", "F") != "F" { t.Fatal("form") }
    if pickDocPassword("  ", "") != "" { t.Fatal("blank header ignored") }
}
```

- [ ] **Step 2–4: Implement**

```go
func pickDocPassword(header, form string) string {
    h := strings.TrimSpace(header)
    if h != "" {
        return h
    }
    return strings.TrimSpace(form)
}
```

Handler: `pwd := pickDocPassword(c.GetHeader("X-Doc-Password"), c.PostForm("password"))` then `h.Svc.Upload(..., wm, reqID, pwd)`.

`UploadService.Upload` last arg `docPassword string`; put on `queue.Task{DocPassword: docPassword}`.

Do not store on `domain.Upload`.

- [ ] **Step 5: Commit** `feat: accept X-Doc-Password and form password on upload`

---

### Task 6: `internal/ofd` parse + vector render (fixtures)

**Files:**
- Create: `internal/ofd/convert.go`, `open.go`, `doc.go`, `render.go`, `xmltypes.go`, `errors.go`
- Create: `internal/ofd/convert_test.go`
- Create: helper `internal/ofd/fixture_test.go` writing zips under `t.TempDir()` (and optionally copy stable zips into `internal/ofd/fixtures/`)

**Produces:** `func Convert(ctx context.Context, srcPath, dstPath, password string) error`

Minimal fixture (unencrypted):

```
OFD.xml          → DocBody/DocRoot = Doc_0/Document.xml
Doc_0/Document.xml → CommonData/PageArea/PhysicalBox, Pages/Page BaseLoc
Doc_0/Pages/Page_0/Content.xml → Layer with TextObject + PathObject + ImageObject (PNG bytes)
```

Units: millimetres → PDF points (`mm * 72 / 25.4`). Apply page/object CTM. Draw in layer order.

- [ ] **Step 1: Failing tests**

1. Minimal text page → PDF exists, starts with `%PDF`, `pdfcpu` or `gopdf` can open, page count 1, width/height within 1pt of source box.
2. Corrupt zip / missing `OFD.xml` / zero pages → error (not password sentinels).
3. Unencrypted + password `"x"` → still success (ignore password).

- [ ] **Step 2: Run — FAIL** (package missing)

- [ ] **Step 3: Implement parser + gopdf vector path**

- Open ZIP; find `OFD.xml` (case-insensitive path).
- Parse resources for embedded fonts (TTF in Res) and images; embed in gopdf; missing font → fallback to a bundled or Windows `msyh.ttc`/`arial.ttf` search, `slog.Warn` without failing.
- TextObject → `Cell`/`Text` at Boundary.
- PathObject → lines/beziers, fill/stroke, even-odd vs nonzero if present.
- ImageObject → embed JPEG/PNG/BMP; no extra recompress if already JPEG/PNG.
- CompositeObject → recurse.
- Empty output or 0 pages → `fmt.Errorf("ofd: no pages")`.

Do **not** import this package from queue workers in the parent except via `convert-worker`.

- [ ] **Step 4:** `go test ./internal/ofd/ -count=1`

- [ ] **Step 5: Commit** `feat: parse OFD packages and render vector PDF`

---

### Task 7: Raster fallback + multi-doc + skip list

**Files:** `internal/ofd/raster.go`, `render.go`, tests

- [ ] Stamp / gradient / unsupported colorspace / exotic blend → rasterize **object bbox only** (PNG) and `Image` onto the page. Never full-page screenshot as the main path.
- [ ] Multiple `DocBody` → concatenate pages in `OFD.xml` order.
- [ ] Video/audio/attachments/3D/bookmarks → skip + `slog.Warn`.
- [ ] Tests: fixture with a synthetic “stamp” object marked unsupported → PDF still builds; page not a single full-page bitmap (assert via pdfcpu that the page has mixed content, or that image XObject bbox is smaller than page).

Commit: `feat: hybrid raster fallback and multi-document OFD`

---

### Task 8: OFD encryption with user password

**Files:** `internal/ofd/crypt.go`, tests

- [ ] If package contains `Doc_0/Encryptions.xml` (or documented GB path) **or** zip members fail to decrypt:
  - empty password → `converter.ErrPasswordRequired` (`errors.Is` from ofd wrapping that sentinel; ofd may import converter sentinels **or** define local errors that convert maps — prefer wrapping `converter.ErrPasswordRequired` to keep one type).
  - non-empty password, decrypt fails → `converter.ErrPasswordWrong`.
- [ ] Supported decrypt for tests: AES-256-GCM (or XOR) fixture keyed by SHA-256(password), algorithm named in Encryptions.xml (`EncryptMethod="StdAES"`). Real GM packages: unsupported algorithm + password provided → `ErrPasswordWrong`; no password → `ErrPasswordRequired`.
- [ ] Tests: required / wrong / correct for the StdAES fixture.

Commit: `feat: decrypt OFD with user password`

To avoid import cycles (`ofd` → `converter` → `ofd`), put sentinels in `internal/ofd` **or** `internal/docpass` tiny package. **Choose `internal/docpass`:**

```go
package docpass
const Env = "MSOFFICE2PDF_DOC_PASSWORD"
var ErrRequired, ErrWrong error
```

If Task 1 already put sentinels in `converter`, **move** them to `internal/docpass` in this task and update imports (one extra compile pass). Do not create an import cycle.

---

### Task 9: `ofdEngine` subprocess + convert-worker in-process render

**Files:**
- Create: `internal/converter/ofd_engine.go`
- Create/modify spawn helpers
- Modify: `com_windows.go` `newConverter`, `stub_other.go`, `env_*.go`, `cli_convert_worker.go`, `subprocess_windows.go`

**Critical anti-recursion:**

```go
// convert-worker
if engine == converter.EngineOFD {
    return ofd.Convert(context.Background(), src, dst, converter.PasswordFromEnv())
}
```

Parent:

```go
case EngineOFD:
    engines[n] = &ofdEngine{}
```

`ofdEngine.Convert` calls shared `runConvertWorker(ctx, args, password)` with `--engine ofd --src --dst` and **no** `--app-kind`. Env via `PasswordEnv`.

Windows: reuse Job Object pattern from current `subprocess_windows.go` (extract `runConvertWorker` so COM subprocess and ofd share it). Unix: `SysProcAttr.Setpgid = true`; on `ctx.Done()` `kill(-pid)`.

`ValidateEnvironment`: `case EngineOFD: // no-op`.

`ProcessImages` for ofd: `nil` (no Office images).

- [ ] **Tests:** `TestOFDWorkerArgs` if you extract argv builder; optional fake that `ofdEngine` does not require AppKind.

- [ ] Commit `feat: run OFD conversion in convert-worker subprocess`

---

### Task 10: Office COM / OpenOffice passwords

**Files:** `internal/converter/com_windows.go`, `openoffice.go`, `subprocess_windows.go`

Word already: `Open(src, false, true, false, "", "")` — 5th arg `PasswordDocument`. Pass `password` there. Try no-password first if you can detect encryption; otherwise:

1. Open with empty password.
2. On failure, if password empty → `ErrPasswordRequired`; else retry Open with password; still fail → `ErrPasswordWrong` if the original error looks like encryption, else wrap original.

Excel: `Workbooks.Open` 5th parameter Password — extend the current 3-arg Open.

PowerPoint: pass password if the Open signature allows it; else same map-to-sentinel heuristic.

OpenOffice: if `password != ""`, add soffice `--password=<pwd>` **only on the soffice process**, never on `convert-worker` argv. Strip from our logs.

Subprocess COM worker already reads env after Task 1.

Commit: `feat: open encrypted Office files with document password`

---

### Task 11: Web UI

**Files:** `ui/src/api/upload.ts`, `UploadView.vue`, `zh-CN.ts`, `en.ts`, history/board copy if they show `error_code`

- [ ] `uploadFile(file, watermark, password, onProgress)` append `password` to FormData when non-empty. Do **not** put it in the URL.
- [ ] Upload form: `el-input type="password"` next to watermark; clear after success like watermark.
- [ ] i18n:

```
upload.docPassword: 文档密码（可选） / Document password (optional)
upload.docPasswordPlaceholder: 加密文档才需要 / Required only for encrypted files
history/board: map ERR_DOC_PASSWORD_REQUIRED / WRONG to short sentences
```

If `npm` is available: `npm run build` in `ui/` (or skip if the repo usually ships `ui/dist` separately — still change `ui/src`).

Commit: `feat: optional document password on web upload`

---

### Task 12: Config templates + user docs

**Files:** `config/config.yaml`, `config/config.template_zh.yaml`, `config/config.template_en.yaml`, `docs/usage.md`, detailed design under `docs/` (force-add if committing)

Add `ofd` to engines comment list; `*.ofd: ofd` in `ext_engines`; `*.ofd` in `allowed_exts`; `validate_ofd` with `OFD.xml`.

Document:

```bash
curl -H "X-Doc-Password: secret" -F "file=@a.ofd" ...
```

Error codes `ERR_DOC_PASSWORD_*` are **async** (status/SSE/history), not upload HTTP 400.

Commit: `docs: OFD engine and document password`

---

## Spec coverage

| Spec section | Task |
|--------------|------|
| Engine `ofd`, always subprocess | 9 |
| Hybrid vector/raster | 6–7 |
| No third-party OFD lib | 6 |
| `validate_ofd` / family / skip AppKind | 2–3 |
| Password header/form, header wins | 5, 11 |
| Memory-only password + env | 1, 4, 9 |
| Open order + no retry | 4, 8, 10 |
| Async error codes | 4, 12 |
| Tests/fixtures | 6–8 |
| Docs/templates | 12 |
| Anti-recursion convert-worker | 9 |

## Placeholder / naming check

- Convert password arg name is `password` everywhere.
- Sentinels end as `docpass` or `converter.ErrPasswordRequired` after Task 8 (prefer `internal/docpass` to avoid cycles).
- Config key `validate_ofd`, family `"ofd"`, engine `"ofd"`.
- Env `MSOFFICE2PDF_DOC_PASSWORD`.
- Header `X-Doc-Password`, form `password`.
