# MSOffice2Pdf Usage Guide

This document covers server installation, foreground operation, CLI user management, and HTTP API usage.

Chinese SSE client / server notes: [usage_zh.md](./usage_zh.md). Detailed SSE design: `docs/详细设计说明书.md` §4.4.

---

## 1. Requirements

| Component | Requirement |
|-----------|-------------|
| OS | Production COM conversion: **Windows Server 2016+ / Windows 10+**. **Linux / macOS** can enable `openoffice` only for real CLI conversion. With no usable engine configured, non-Windows builds use a stub minimal PDF for integration testing. |
| Go | 1.21+ (when building from source) |
| Database | MySQL 8.0+ or PostgreSQL 14+ |
| Office | `msoffice`: Microsoft Office 2016+ (licensed); `wpsoffice`: WPS Office; `openoffice`: LibreOffice / Apache OpenOffice (`soffice` CLI, cross-platform) |

### 1.1 Office / DCOM (required for production conversion)

Office COM is restricted in non-interactive sessions. If you later run as a Windows service, configure DCOM Identity on the host:

1. Run `dcomcnfg` (Component Services)
2. Go to **Component Services → Computers → My Computer → DCOM Config**
3. For **Microsoft Word Application / Excel Application / PowerPoint Application** → Properties
4. Set **Identity** to **This User**, and enter a Windows account with interactive rights (and password)

The current release primarily runs as a foreground process; Windows service install (`service install`) is not provided yet.

---

## 2. Installation

### 2.1 Prepare the database

**MySQL example (Docker):**

```bash
docker run -d --name msoffice2pdf-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=msoffice2pdf \
  -p 3306:3306 mysql:8
```

DSN example:

```text
root:root@tcp(127.0.0.1:3306)/msoffice2pdf?charset=utf8mb4&parseTime=True&loc=Local
```

For PostgreSQL, set `database.driver` to `postgres` and use the matching DSN.

On first start, GORM auto-migrates tables (`user` / `upload` / `upload_history` / `pdf` / `pdflog` / `expired_upload` (legacy, read-only), etc.); no manual schema script is required. Startup also migrates legacy `expired_upload` rows and terminal-state `upload` rows into `upload_history` once.

### 2.2 Configuration file

```bash
# Windows PowerShell
Copy-Item config\config.template_zh.yaml config\config.yaml
# or: Copy-Item config\config.template_en.yaml config\config.yaml

# or Linux / macOS / Git Bash
cp config/config.template_zh.yaml config/config.yaml
# or: cp config/config.template_en.yaml config/config.yaml
```

At minimum, set:

- `database.driver` / `database.dsn`
- `auth.jwt_secret` (use a long random string in production)

Optional environment variables (override the YAML counterparts):

| Environment variable | Config key |
|----------------------|------------|
| `MSOFFICE2PDF_DB_DSN` | `database.dsn` |
| `MSOFFICE2PDF_JWT_SECRET` | `auth.jwt_secret` |

Other common sections:

| Section | Purpose |
|---------|---------|
| `server` | Port, read/write timeouts |
| `storage` | `upload` / `output` / `trash` / `expired` directories |
| `converter` | Worker count, queue size, Office timeout, requeue interval, `com_mode`, `temp_sandbox`, `engines`, `ext_engines`, `openoffice` |
| `cleanup` | Upload/PDF TTL and cleanup interval (no trash TTL) |
| `upload` | Max size, allowed extensions, `validate_magic` / `validate_new` / `validate_ole` |

Common detail keys:

| Key | Description |
|-----|-------------|
| `upload.validate_magic` | Whether to check magic bytes (OLE/ZIP headers); default true. Structure checks are handled by `validate_new` / `validate_ole` |
| `upload.validate_new` | OOXML extension → required paths inside the ZIP (map); an extension listed here is treated as OOXML family |
| `upload.validate_ole` | OLE extension → required stream names in the compound document (map); mutually exclusive with `validate_new`. Covers MS binary and WPS native `.wps/.wpt/.et/.ett/.dps/.dpt` (stream names aligned to WordDocument / Workbook / PowerPoint Document). Application type for conversion (Writer/Spreadsheet/Presentation) is also derived from these maps — **not** hard-coded by extension in code |
| `converter.temp_sandbox` | Per-task isolated `TEMP`/`TMP`/`TMPDIR` (`msoffice2pdf-com-*`) to isolate Office temp / `~$` files; default true. With `inprocess`, requires `worker_count=1`; ignored for `openoffice` |
| `converter.engines` | Enabled converter engine names (unique); valid values `msoffice` \| `wpsoffice` \| `openoffice`; default `[msoffice]`. At startup: COM engines probe ProgIDs for Writer/Spreadsheet/Presentation; `openoffice` runs `command --version`; any failure logs ERROR and exits |
| `converter.openoffice.command` | LibreOffice / Apache OpenOffice executable path (or `soffice` on PATH); required when `engines` includes `openoffice` |
| `converter.openoffice.user_profile` | User profile root; each task uses `{user_profile}/{uuid}/` and deletes it after conversion; required when `engines` includes `openoffice` |
| `converter.ext_engines` | Extension → engine name (no auto); value must be ∈ `engines`. If an `allowed_exts` entry lacks a mapping, startup logs ERROR (does not exit) and uploads are rejected with `40004` |

**Config key sync rule:** when you add/remove/rename any config **key** in `config.yaml`, update the same keys and comments in both `config.template_zh.yaml` and `config.template_en.yaml`.

### 2.3 Build a binary

From the project root. Create the output directory first if needed:

```bash
mkdir -p bin
```

**Windows (desktop shell needs CGO + matching `GOARCH`):**

If `go env GOARCH` is `arm64` but your CPU / MinGW toolchain is amd64 (common on Windows ARM64 hosts or mis-set env), build with:

```powershell
# PowerShell
$env:GOARCH='amd64'; $env:CGO_ENABLED='1'; go build -o bin/msoffice2pdf.exe ./cmd/msoffice2pdf
```

```bat
REM cmd.exe
set GOARCH=amd64&& set CGO_ENABLED=1&& go build -o bin/msoffice2pdf.exe ./cmd/msoffice2pdf
```

If `GOARCH` already matches the toolchain:

```powershell
go build -o bin/msoffice2pdf.exe ./cmd/msoffice2pdf
```

**Linux / macOS:**

```bash
CGO_ENABLED=1 go build -o bin/msoffice2pdf ./cmd/msoffice2pdf
```

Optional: embed a release version at link time (overrides the default in `internal/version.Version`):

```powershell
# Windows PowerShell (add GOARCH/CGO as above when needed)
go build -ldflags "-X msoffice2pdf/internal/version.Version=1.2.3" -o bin/msoffice2pdf.exe ./cmd/msoffice2pdf
```

```bash
# Linux / macOS
go build -ldflags "-X msoffice2pdf/internal/version.Version=1.2.3" -o bin/msoffice2pdf ./cmd/msoffice2pdf
```

After a successful build, sanity-check with:

```bash
# Windows
.\bin\msoffice2pdf.exe version

# Linux / macOS
./bin/msoffice2pdf version
```

Example output:

```text
MSOffice2Pdf
Version:     0.1.0
Description: HTTP service that converts Microsoft Office documents (Word / Excel / PowerPoint) to PDF via Office COM (Windows) or OpenOffice/LibreOffice, preserving layout as much as possible.
Copyright:   Copyright (c) 2026 pigoosoft (pigoosoft@gmail.com)
```

Show all CLI commands:

```bash
# Windows
.\bin\msoffice2pdf.exe help

# Linux / macOS
./bin/msoffice2pdf help
```

Aliases: `-h`, `--help`.

You can also skip the binary and run from source (see §3.0): `go run ./cmd/msoffice2pdf ...`.

Add `bin/` to `PATH`, or always invoke the binary via a relative path.

### 2.4 Directories and permissions

On startup the service creates storage directories from config if missing. The run account needs read/write access to:

```text
upload/   # original Office files
output/   # converted PDFs
trash/    # user-deleted archives
expired/  # TTL / failure archives
```

---

## 3. Running the service

Finish §2.1–2.2 (database and `config/config.yaml`) before starting. For production and day-to-day use, prefer the **binary from §2.3** in the foreground; for local development, §3.0 (`go run`) is fine.

### Desktop control shell (default)

By default on **Windows**, and on **Linux when `DISPLAY` is set**, starting `serve` (or no subcommand) opens a **desktop control shell** instead of starting HTTP immediately. Click **Start** to run HTTP + conversion workers + cleanup; **Stop** or close the window to shut down. The window shows filtered live logs from the service.

Only **one** `serve` / default-start process may run on the machine (**Windows / Linux / macOS**). A second launch exits immediately with an error. Startup also fails if `server.port` is already bound (by this app or any other process). `version`, `user *`, and `convert-worker` are not covered by this lock.

| Mode | How | Behavior |
|------|-----|----------|
| Desktop shell | default on Windows / Linux+X | Service **stopped** until you click Start |
| Console | `--noui` | Same as pre-shell behavior: service starts immediately |

Use **`--noui`** for SSH, headless Linux (no X), Task Scheduler / nssm wrappers, or any host without a display:

```bash
./bin/msoffice2pdf --noui --config config/config.yaml
```

On Linux without `DISPLAY`, the process auto-falls back to console mode (one log line). `version`, `user *`, and `convert-worker` never open the shell.

### 3.0 Run from source (development)

No prior `go build` required. From the project root:

```bash
go run ./cmd/msoffice2pdf
# or
go run ./cmd/msoffice2pdf serve
go run ./cmd/msoffice2pdf serve --config=config/config.yaml
```

### 3.1 Foreground start with the binary (recommended)

Use the executable from §2.3. By default it loads `config/config.yaml` under the project root.

**Windows:**

```bash
# default config (same as serve)
.\bin\msoffice2pdf.exe

# explicit serve / config path
.\bin\msoffice2pdf.exe serve --config=config/config.yaml
.\bin\msoffice2pdf.exe --config config/config.yaml
```

**Linux / macOS:**

```bash
./bin/msoffice2pdf
./bin/msoffice2pdf serve --config=config/config.yaml
./bin/msoffice2pdf --config config/config.yaml
```

In **desktop shell** mode (default on Windows / Linux+X), the process only loads config and opens the window until you click **Start**. In **console** mode (`--noui`, or Linux without `DISPLAY`), on start the process connects to the database, ensures storage dirs, starts the conversion queue and Cleanup timer, and listens for HTTP. When listening begins, besides `addr` in JSON logs (e.g. `:8080`), it prints each local IPv4 URL to the console (shell log pane and/or stdout), for example:

```text
http://127.0.0.1:8080
http://192.168.1.10:8080
```

This helps local and LAN access.

#### Process file logging (optional)

By default only stdout JSON is used. Set `file_enabled: true` under the `log` section in `config.yaml` to also write `{file_dir}/yyyymmdd.log` (default dir `logs`), line format:

`{datetime} {uid|System} {LEVEL} {action} {message and fields}`

`flush_interval` controls buffered flush; the process forces Sync on exit. Background logs without request Context use actor `System`; missing `action` becomes `-`.

Stop: in the desktop shell use **Stop** or close the window; in console mode use `Ctrl+C` (SIGINT/SIGTERM). The process stops HTTP, Cleanup, then the queue.

### 3.2 Health check

```bash
curl -i http://127.0.0.1:8080/health
```

Returns success when the database is healthy; otherwise an unhealthy status (non-2xx HTTP; see implementation).

### 3.3 Create the first admin

You can start the service first, then create users via CLI (CLI talks to the DB only; HTTP is not required):

```bash
./bin/msoffice2pdf.exe user create-admin --uid=admin --pwd=secret --config=config/config.yaml
```

Example output (**`api_token` is printed only once — save it securely**):

```text
uid=admin
role=admin
api_token=<long random string>
```

### 3.4 Web UI

The frontend is a standalone Vite + Vue 3 app (`ui/`), calling the backend via same-origin `/api`. In development Vite proxies; in production Nginx (or similar) serves static assets and reverse-proxies the API.

**Development:**

```bash
# Terminal 1: backend (default :8080)
./bin/msoffice2pdf.exe serve --config=config/config.yaml

# Terminal 2: frontend
cd ui
npm install
npm run dev
```

Open the URL Vite prints (default `http://127.0.0.1:5173`). `ui/vite.config.ts` proxies `/api` and `/health` to `http://127.0.0.1:8080`. Cookie sessions require same-origin proxy; do not point the API at a cross-origin host.

**Production build:**

```bash
cd ui
npm run build
```

Output is in `ui/dist/`. Nginx example: set site `root` to that directory and reverse-proxy `/api` (and optionally `/health`) to the backend:

```nginx
server {
  listen 80;
  server_name example.com;
  root /path/to/msoffice2pdf/ui/dist;
  index index.html;

  location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  }

  location /health {
    proxy_pass http://127.0.0.1:8080;
  }

  location / {
    try_files $uri $uri/ /index.html;
  }
}
```

Web login uses an HttpOnly Cookie session; do not rely on `X-UID` / `X-Token` headers in the browser.

---

## 4. CLI usage

Global flags:

| Flag | Description |
|------|-------------|
| `--config=PATH` or `--config PATH` | Config file; default `config/config.yaml` |
| `--noui` | Skip desktop shell; start HTTP + workers immediately (console mode) |
| `-h` / `--help` / `help` | Print all CLI commands |
| `-v` / `--version` / `version` | Print app name, version, and copyright |

### 4.1 Command overview

```text
msoffice2pdf [--config=PATH] [--noui]
msoffice2pdf serve [--config=PATH] [--noui]
msoffice2pdf help
msoffice2pdf version
msoffice2pdf user create-admin --uid=UID --pwd=PWD [--config=PATH]
msoffice2pdf user create --uid=UID --pwd=PWD [--config=PATH]
msoffice2pdf user update --uid=UID [--pwd=PWD] [--role=admin|user] [--config=PATH]
msoffice2pdf user reset-token --uid=UID [--config=PATH]
msoffice2pdf user deactivate --uid=UID [--config=PATH]
msoffice2pdf user activate --uid=UID [--config=PATH]
```

| Command | Description |
|---------|-------------|
| (no args) / `serve` | Start HTTP + queue + Cleanup |
| `help` (or `-h` / `--help`) | Print full CLI help |
| `version` (or `-v` / `--version`) | Print app name, version, and copyright |
| `user create-admin` | Create admin; print `api_token` |
| `user create` | Create normal user; print `api_token` |
| `user update` | Change password and/or role (`--pwd` / `--role` at least one); print `uid`, `role` |
| `user reset-token` | Reset API Token; print new `api_token` |
| `user deactivate` | Freeze user (`status=1`; cannot login / call API) |
| `user activate` | Unfreeze user (`status=0`) |

### 4.2 Examples

```bash
# normal user
./bin/msoffice2pdf.exe user create --uid=u1 --pwd=secret --config=config/config.yaml

# change password / role (can pass both)
./bin/msoffice2pdf.exe user update --uid=u1 --pwd=newsecret --config=config/config.yaml
./bin/msoffice2pdf.exe user update --uid=u1 --role=admin --config=config/config.yaml

# reset Token (old Token invalid immediately)
./bin/msoffice2pdf.exe user reset-token --uid=u1 --config=config/config.yaml

# freeze / unfreeze
./bin/msoffice2pdf.exe user deactivate --uid=u1 --config=config/config.yaml
./bin/msoffice2pdf.exe user activate --uid=u1 --config=config/config.yaml
```

Notes:

- Passwords are stored as **MD5 (lowercase hex)** in `pwd_hash` (project decision).
- The `api_token` printed by CLI user creation is for headers `X-UID` + `X-Token`, distinct from login JWT.
- `user update` does not reset `api_token` or invalidate issued JWTs; use `activate` / `deactivate` for status.
- Frozen users cannot operate via any auth method.

---

## 5. API usage

### 5.1 Conventions

**Base URL:** `http://127.0.0.1:8080` (port from `server.port`)

**Unified JSON envelope:**

```json
{
  "code": 0,
  "message": "optional error description",
  "data": {}
}
```

Success: `code` is `0`. Common business codes:

| code | HTTP | Meaning |
|------|------|---------|
| 40001 | 400 | Bad request / invalid parameters |
| 40002 | 400 | File magic mismatch (`ERR_FILE_MAGIC`) |
| 40003 | 400 | File structure mismatch (`ERR_FILE_STRUCTURE`) |
| 40004 | 400 | Extension not mapped to a converter engine (`ERR_EXT_ENGINE_UNMAPPED`; see `converter.ext_engines`) |
| 40101 | 401 | Unauthenticated |
| 40301 | 403 | Forbidden (including frozen users) |
| 40401 | 404 | Not found |
| 40901 | 409 | Conflict (e.g. PDF not ready) |
| 50001 | 500 | Internal error |

### 5.2 Authentication

| Scenario | Method |
|----------|--------|
| External systems / scripts | Headers: `X-UID` + `X-Token` (long-lived API Token) |
| Web / temporary session | `POST /api/auth/login` for JWT, then `Authorization: Bearer <jwt>` or Cookie `access_token` |

Except `/health` and `POST /api/auth/login`, business APIs require authentication. Admin APIs also require `role=admin`.

Examples below use **API Token** by default; for JWT, replace headers with `Authorization: Bearer <jwt>`.

---

### 5.3 Auth API

#### Login

```bash
curl -s -X POST http://127.0.0.1:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"uid\":\"admin\",\"pwd\":\"secret\"}"
```

Success `data` includes `uid`, `token` (JWT), `role`; also sets Cookie `access_token`.  
Wrong password → 401; frozen → 403.

#### Verify credentials

```bash
# JWT
curl -s http://127.0.0.1:8080/api/auth/verify \
  -H "Authorization: Bearer <jwt>"

# API Token
curl -s http://127.0.0.1:8080/api/auth/verify \
  -H "X-UID: admin" \
  -H "X-Token: <api_token>"
```

#### Logout (clear Cookie)

```bash
curl -s -X POST http://127.0.0.1:8080/api/auth/logout \
  -H "Authorization: Bearer <jwt>"
```

#### Profile (current user)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/profile` | Account info, plaintext API token, upload count, successful conversion count |
| PUT | `/api/profile/password` | Change own password (`old_pwd` + `new_pwd`) |
| POST | `/api/profile/reset-token` | Regenerate own API token (response includes new plaintext `token`) |

```bash
curl -s http://127.0.0.1:8080/api/profile \
  -H "Authorization: Bearer <jwt>"

curl -s -X PUT http://127.0.0.1:8080/api/profile/password \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d "{\"old_pwd\":\"secret\",\"new_pwd\":\"newsecret\"}"

curl -s -X POST http://127.0.0.1:8080/api/profile/reset-token \
  -H "Authorization: Bearer <jwt>"
```

`upload_count` = live uploads in queue + non-soft-deleted `upload_history`; `convert_success_count` = rows among those with status / `final_status` = `completed`.

---

### 5.4 User management (admin)

All require admin credentials.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/admin/users` | Create user |
| GET | `/api/admin/users` | List (`page` / `page_size`) |
| GET | `/api/admin/users/:uid` | Detail |
| PUT | `/api/admin/users/:uid` | Update password/role |
| DELETE | `/api/admin/users/:uid` | Delete user |
| POST | `/api/admin/users/:uid/freeze` | Freeze / unfreeze |
| POST | `/api/admin/users/:uid/reset-token` | Reset API Token |

**Create user:**

```bash
curl -s -X POST http://127.0.0.1:8080/api/admin/users \
  -H "X-UID: admin" -H "X-Token: <admin_api_token>" \
  -H "Content-Type: application/json" \
  -d "{\"uid\":\"u1\",\"pwd\":\"secret\",\"role\":\"user\"}"
```

`role`: `user` (default) or `admin`. Response includes plaintext `token` (only on create/reset).

**Freeze / unfreeze:**

```bash
curl -s -X POST http://127.0.0.1:8080/api/admin/users/u1/freeze \
  -H "X-UID: admin" -H "X-Token: <admin_api_token>" \
  -H "Content-Type: application/json" \
  -d "{\"frozen\":true}"
```

**Reset Token:**

```bash
curl -s -X POST http://127.0.0.1:8080/api/admin/users/u1/reset-token \
  -H "X-UID: admin" -H "X-Token: <admin_api_token>"
```

---

### 5.5 Upload

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/upload` | multipart: `file` (required); optional `watermark` (secondary watermark, max 255); optional Header `X-Request-ID` (max 128) |
| GET | `/api/uploads` | Current user's **pending conversion queue** (`upload` table only) |
| GET | `/api/upload/limits` | Current upload limits: `allowed_exts` (`.ext` array) and `max_size` (bytes); auth required. Web UI fetches after login / session restore |
| GET | `/api/upload/:fileid` | Detail: `upload` first, then fall back to `upload_history` terminal snapshot after archive |
| GET | `/api/upload/:fileid/download` | Download original (live queue row only; 404 after archive) |
| DELETE | `/api/upload/:fileid` | Archive delete (`ArchiveUpload` → `trash/` + `upload_history`) |
| GET | `/api/admin/uploads` | Admin: all uploads |

After a successful upload, status is usually `queued`; if the queue is full it stays `pending` until `converter.requeue_interval` scans and requeues. On conversion failure, retries follow `converter.retry_count` / `retry_interval`; after exhaustion, `ArchiveUpload` archives to `expired/` (`upload_history.final_status=failed`, `error_code=ERR_RETRY_LIMIT_EXCEEDED`). Successful conversion is archived immediately as well; `/api/uploads` excludes completed items — see `/api/history/uploads` or `GET /api/pdf/:fileid/status` (checks `upload`, then `upload_history`). Excel defaults to `excel_page_fit: fit_width` (one page wide horizontally, may paginate vertically); set `auto` to keep the document's own page setup.

PDF watermark: config `watermark.text` is the primary watermark; form field `watermark` is secondary (smaller font). If both are empty, no watermark is applied. Watermarking is post-processed after COM export; on failure the PDF remains downloadable with `status=completed` and `warn_code=WARN_WATERMARK`.

Clients may send `X-Request-ID`: the server stores it in `upload.request_id`; the upload response JSON includes `fileid` + `request_id` (and echoes response header `X-Request-ID`). Detail/list and `GET /api/pdf/:fileid/status` also return `request_id`, so async clients can map a client ID to `fileid`.

```bash
# upload (optional secondary watermark + X-Request-ID)
curl -s -X POST http://127.0.0.1:8080/api/upload \
  -H "X-UID: u1" -H "X-Token: <api_token>" \
  -H "X-Request-ID: client-req-001" \
  -F "file=@C:/path/to/report.docx" \
  -F "watermark=secondary watermark text"

# upload limits (Web / client preflight)
curl -s http://127.0.0.1:8080/api/upload/limits \
  -H "Authorization: Bearer <jwt>"

# list
curl -s "http://127.0.0.1:8080/api/uploads?page=1&page_size=20" \
  -H "X-UID: u1" -H "X-Token: <api_token>"

# download original (-o sets save-as name; or -OJ to use Content-Disposition)
curl -s -o report.docx http://127.0.0.1:8080/api/upload/<fileid>/download \
  -H "X-UID: u1" -H "X-Token: <api_token>"

curl -s -X DELETE http://127.0.0.1:8080/api/upload/<fileid> \
  -H "X-UID: u1" -H "X-Token: <api_token>"
```

Allowed types and size come from `upload.allowed_exts` and `upload.max_size`. Uploads also run magic and configurable ZIP/OLE structure checks (`validate_magic` / `validate_new` / `validate_ole`): fake Office files (e.g. renaming `.jpg` to `.docx`, or missing required package paths) are rejected before writing under `upload/` or the DB — HTTP 400 with `code` `40002` (`ERR_FILE_MAGIC`) or `40003` (`ERR_FILE_STRUCTURE`). Unmapped extensions in `converter.ext_engines` return `40004` (`ERR_EXT_ENGINE_UNMAPPED`).

---

### 5.6 PDF conversion and download

`:fileid` in paths is the system-wide unique business key (generated at upload; shared by `upload` / `pdf` / `pdflog` / disk paths).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/pdf/:fileid/status` | Conversion status (`upload` first, then `upload_history`; includes `final_status` / `error_code`, etc.) |
| GET | `/api/pdf/events` | SSE subscribe via `fileid` (comma-separated and/or repeated); events `status` / `ping` / `done`; see `server.sse.*` |
| GET | `/api/pdf/:fileid/download` | Download PDF (after hard-delete of upload, linked via `upload_history.upload_id`; not ready → 409) |
| GET | `/api/pdfs` | Current user's PDF list |
| GET | `/api/admin/pdfs` | Admin: all PDFs |

Queue status flow: `pending` → `queued` → `converting` → `failed` (awaiting retry) or immediate archive to `upload_history` on success/exhaustion (`upload` row hard-deleted). Polling `status` still works after archive and returns terminal fields such as `final_status`.

Status / list JSON includes `warn_code` (empty or `WARN_WATERMARK`). Polling `status` also returns `warn_code` when a pdf row exists.

#### Conversion status SSE (client + server)

Use SSE when you want push updates for one or many uploads instead of polling `GET /api/pdf/:fileid/status`. Polling remains supported.

**Endpoint:** `GET /api/pdf/events`  
**Auth:** same as other API calls (`X-UID` + `X-Token`, or Web session cookie).  
**Query `fileid` (required):** both forms are accepted and may be mixed; values are trimmed and deduped; count must be `1 … server.sse.max_fileids` (default 50).

| Form | Example |
|------|---------|
| Repeated | `?fileid=aaa&fileid=bbb` |
| Comma-separated | `?fileid=aaa,bbb,ccc` |

Any unknown / forbidden `fileid` fails the whole request **before** the stream opens (JSON `404` / `403`); there is no half-open subscription.

**Events (`text/event-stream`):**

| `event` | When | `data` |
|---------|------|--------|
| `status` | Immediately once per subscribed `fileid` (snapshot), then again when status fingerprint changes | Same JSON shape as `GET /api/pdf/:fileid/status` (`fileid`, `request_id`, `status`, `final_status`, `error_*`, `retry_count`, `warn_code`, …) |
| `ping` | Every `server.sse.heartbeat_interval` (default `15s`) | `{}` (ignore) |
| `done` | Stream ends | `{"reason":"all_terminal","fileids":[…]}` or `{"reason":"max_duration","fileids":[…],"pending":[…]}` |

Map UI rows by `request_id` / `fileid`. After `done`, reconnect with the same (or remaining) `fileid`s if you still need updates.

**Server operating model (summary):**

1. Preflight: resolve + ACL every `fileid` via the same logic as status API.  
2. Clear this connection’s write deadline so global `server.write_timeout` (e.g. 60s) does **not** kill the stream.  
3. Emit snapshot `status` events; if all are already terminal, emit `done(all_terminal)` and close.  
4. Loop: every `poll_interval` (default `1s`) re-query DB; emit `status` only when fingerprint `(status, retry_count, error_msg, final_status, error_code, warn_code)` changes; send `ping` on heartbeat.  
5. Close with `done` when every file is **terminal** (archived `final_status` = `completed` / `failed` / `deleted`) or when `max_duration` (default **5m**) elapses. Live queue `failed` awaiting retry is **not** terminal.

Config (`config.yaml` / templates):

```yaml
server:
  sse:
    max_duration: 5m        # SSE business timeout (not office_timeout)
    heartbeat_interval: 15s
    poll_interval: 1s
    max_fileids: 50
```

**curl examples:**

```bash
# poll status
curl -s http://127.0.0.1:8080/api/pdf/<fileid>/status \
  -H "X-UID: u1" -H "X-Token: <api_token>"

# SSE — repeated fileid=
curl -N "http://127.0.0.1:8080/api/pdf/events?fileid=<id1>&fileid=<id2>" \
  -H "X-UID: u1" -H "X-Token: <api_token>"

# SSE — comma-separated fileid=
curl -N "http://127.0.0.1:8080/api/pdf/events?fileid=<id1>,<id2>" \
  -H "X-UID: u1" -H "X-Token: <api_token>"
```

**Local smoke client** (`testdata/sse_smoke`, gitignored sample Office files + tool): uploads `1.*`…`8.*` with `X-Request-ID` `1`…`8`, then prints SSE events to the console.

```bash
# repo root
go run ./testdata/sse_smoke

# or from testdata/
go run ./sse_smoke

# optional: -base -uid -token -dir -from -to -comma
go run ./sse_smoke -comma   # use ?fileid=a,b,c instead of repeated fileid=
```

```bash
# download PDF (-o sets save-as name; or -OJ for Content-Disposition name)
curl -s -o report.pdf http://127.0.0.1:8080/api/pdf/<fileid>/download \
  -H "X-UID: u1" -H "X-Token: <api_token>"

# list
curl -s "http://127.0.0.1:8080/api/pdfs?page=1&page_size=20" \
  -H "X-UID: u1" -H "X-Token: <api_token>"
```

On Windows with COM engines (`msoffice` / `wpsoffice`), or any platform with `openoffice`, a real PDF is produced. On non-Windows without `openoffice`, a stub minimal PDF is written so the queue and API can be exercised.

---

### 5.7 History

Data comes from `upload_history` (rows with non-null `deleted_at` excluded by default). Responses include `final_status` (`completed` / `failed` / `deleted`), `error_code`, `error_msg`, `retry_count`, `archive_dir`, `upload_time`, `finished_at`, etc.; `status` equals `final_status` (legacy client compatibility). Filter with query param `final_status`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/history/uploads` | Current user's archived upload history |
| GET | `/api/history/pdflogs` | Current user's PDF operation logs (optional `?fileid=`) |
| GET | `/api/admin/history/uploads` | Admin: all archived upload history |
| GET | `/api/admin/history/pdflogs` | Admin: all pdflog (optional `uid`, `fileid`) |

```bash
curl -s "http://127.0.0.1:8080/api/history/uploads?page=1&page_size=20" \
  -H "X-UID: u1" -H "X-Token: <api_token>"

curl -s "http://127.0.0.1:8080/api/history/pdflogs?page=1&page_size=20" \
  -H "X-UID: u1" -H "X-Token: <api_token>"
```

---

### 5.8 Typical end-to-end flow

```bash
BASE=http://127.0.0.1:8080
UID=u1
TOKEN=<api_token>
REQID=client-req-001

# 1) upload (with X-Request-ID; response data has fileid + request_id)
RESP=$(curl -s -X POST "$BASE/api/upload" \
  -H "X-UID: $UID" -H "X-Token: $TOKEN" \
  -H "X-Request-ID: $REQID" \
  -F "file=@./sample.docx")
echo "$RESP"
# take FILEID from JSON data.fileid; map data.request_id to your local business id if needed

# 2) poll until completed or failed (status response includes request_id)
curl -s "$BASE/api/pdf/$FILEID/status" \
  -H "X-UID: $UID" -H "X-Token: $TOKEN"

# 3) download PDF (save as a chosen filename)
curl -s -o "./out-$FILEID.pdf" "$BASE/api/pdf/$FILEID/download" \
  -H "X-UID: $UID" -H "X-Token: $TOKEN"
```

---

## 6. Operations notes (brief)

| Topic | Notes |
|-------|-------|
| Conversion queue | `converter.worker_count` / `queue_size` / `office_timeout` / `requeue_interval` / `retry_count` / `retry_interval` / `excel_page_fit` / `com_mode` / `temp_sandbox` / `engines` / `ext_engines` / `openoffice` |
| Status SSE | `server.sse.max_duration` / `heartbeat_interval` / `poll_interval` / `max_fileids`; endpoint `GET /api/pdf/events` (see §5.6) |
| PDF watermark | `watermark.text` / `angle` / `density` / `opacity` / `color` / `font_size` / `font_path`; upload form secondary field `watermark` |
| TTL cleanup | `cleanup.history_ttl_enabled` / `history_ttl` / `history_ttl_delete_row` / `pdf_ttl` / `interval`; one run at startup; terminal uploads use immediate `ArchiveUpload` (**not** `upload_ttl`); user deletes go to `trash/`, **no** trash TTL |
| Account freeze | `user.status`: `0` active, `1` frozen |
| Windows service | Not built-in yet; for production, host the foreground process with Task Scheduler / nssm / etc., and configure DCOM Identity |

**Converter engines and process modes:**

- `converter.engines`: enabled engine list (`msoffice` / `wpsoffice` / `openoffice`, unique names). Startup probes: COM engines (`msoffice` / `wpsoffice`) `CreateObject` (ProgID) for Writer/Spreadsheet/Presentation; `openoffice` validates `command` / `user_profile` and runs `command --version`; any failure logs and exits. Non-Windows may only enable `openoffice`; configuring a COM engine causes exit.
- `converter.openoffice`: LibreOffice / Apache OpenOffice CLI backend (cross-platform). `command` is the `soffice` path; `user_profile` is the profile root. Each conversion creates an isolated profile under `{user_profile}/{uuid}/` and deletes that subdirectory afterward (safe with multiple Workers). Workers `exec` directly — no `convert-worker` subprocess; `com_mode` / `temp_sandbox` / `excel_page_fit` do not apply to `openoffice`.
- `converter.ext_engines`: explicit extension → engine binding (e.g. `"*.docx": msoffice`, `"*.wps": wpsoffice`). No `auto`. WPS-native `.wps` / `.et` / `.dps` should only be mapped when `wpsoffice` is enabled. Pure `openoffice` deploy example:

  ```yaml
  converter:
    engines:
      - openoffice
    ext_engines:
      "*.docx": openoffice
      "*.xlsx": openoffice
      "*.pptx": openoffice
    openoffice:
      command: "/usr/bin/soffice"   # Windows example: C:/Program Files/LibreOffice/program/soffice.exe
      user_profile: "/var/lib/msoffice2pdf/lo-profile"
  ```

- `converter.com_mode`: `subprocess` (default; COM in short-lived `convert-worker` child with `--engine`) or `inprocess` (go-ole inside the Worker process; debug only — native COM crashes can take down the whole service); COM engines only
- `converter.retry_count` / `retry_interval`: extra retries after conversion failure and minimum wait before requeue; on exhaustion archive with `ERR_RETRY_LIMIT_EXCEEDED` (written to `upload_history`)
- `converter.temp_sandbox`: per-task isolated `TEMP`/`TMP` (prefix `msoffice2pdf-com-`) to isolate Office temp / `~$` files; `subprocess` injects via `cmd.Env`, inherited by `convert-worker`. With `inprocess`, requires `worker_count=1`; COM engines only
- Orphan sweep: on service start, clean leftover `convert-worker` processes, process images for **enabled** engines (including `openoffice` `soffice.exe` / `soffice.bin` / `soffice`), and all `msoffice2pdf-com-*` TEMP dirs; while running, on each `requeue_interval`, kill `convert-worker` processes older than `office_timeout + 2m` (matched by command line containing `convert-worker`, not the main service), and remove stale TEMP sandboxes under the same grace; if any worker was killed this round or there is currently no `convert-worker`, also clear Office/WPS/soffice images for enabled engines
- **Concurrency cap:** concurrent `convert-worker` count is capped by `converter.worker_count` (each Worker runs one Convert at a time; HTTP load does not unboundedly spawn children); `openoffice` runs inside the Worker and is not limited by `convert-worker` count

---

## 7. Related documents

- [Docs index](./docs/README.md)
- [Architecture design (Chinese)](./docs/架构设计说明书.md)
- [Detailed design (Chinese)](./docs/详细设计说明书.md)
- [Config template (Chinese)](./config/config.template_zh.yaml)
- [Config template (English)](./config/config.template_en.yaml)
- [Project README](./README.md)
- [Chinese usage](./docs/usage.md)
- Design evolution: `docs/superpowers/specs/`
