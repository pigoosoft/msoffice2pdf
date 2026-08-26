# MSOffice2Pdf 使用说明（中文）

完整安装、CLI、API 英文版见 [usage_en.md](./usage_en.md)。中文安装与接口全文见 [docs/usage.md](./docs/usage.md)。

本文包含：**新机器部署所需支持**（与英文 §1.2 对齐），**转换并发与容量规划**（与英文 §6.1 对齐），以及 **转换状态 SSE**（与英文 §5.6 对齐）。

---

## 新系统部署：需要哪些支持

目标机器 **不必安装 Go**。Go 只在**编译** exe 的那台电脑上需要。把编好的程序和配置拷到新系统即可。

`convert-worker` 不是单独软件，而是**同一个 exe** 再拉起自己（`msoffice2pdf convert-worker …`）。不要再拷一份 worker。

### 需要拷贝的文件

| 项 | 是否必须 | 说明 |
|----|----------|------|
| `msoffice2pdf.exe`（Windows）或 `msoffice2pdf`（Linux/macOS） | 是 | 必须与目标机 **操作系统 + CPU 架构** 一致（例如 Windows amd64 的 exe 跑在 64 位 Windows 上）。ARM64 Windows 上不能直接跑 amd64 包，除非系统提供模拟。 |
| `config/config.yaml` | 是 | 从模板复制后修改；至少配置 `database.dsn`、`auth.jwt_secret`。详见英文 §2.2 / [docs/usage.md](./docs/usage.md) §2.2。 |
| `upload` / `output` / `trash` / `expired` | 否 | 首次启动会按 `storage.*` 自动建目录。 |
| 水印字体 | 若配置了路径 | `watermark.font_path` 指向的文件必须在新机器上存在（如 `C:\Windows\Fonts\simhei.ttf`），否则改配置。 |
| Web 静态资源 `ui/dist` | 仅当自行托管前端 | HTTP API 在 exe 内。浏览器界面是独立的 Vite 产物；生产上常用 Nginx 托管 `ui/dist`，并把 `/api` 反代到本服务。 |

### 新机器上要安装的软件

只装 **`converter.engines` 里真正启用的引擎**。未安装 Microsoft Office / WPS 时 **不要** 把 `msoffice` / `wpsoffice` 写进 `engines`：启动会做 ProgID 探测，失败则 **直接退出**。

| 需求 | 安装 / 配置 |
|------|-------------|
| 操作系统 | COM 转换：**Windows 10 / Windows Server 2016+**。Linux / macOS：只能启用 `openoffice` 和/或 `ofd`（没有 Microsoft Office COM）。 |
| 数据库 | **MySQL 8.0+** 或 **PostgreSQL 14+**，本机或远程均可。空库即可：首次启动 GORM 自动建表。运行 exe 的机器必须能连上 `database.dsn`。 |
| 引擎 `msoffice` | 已授权的 **Microsoft Office 2016+**（含 Word、Excel、PowerPoint）。 |
| 引擎 `wpsoffice` | **WPS Office**（文字 / 表格 / 演示对应 ProgID）。 |
| 引擎 `openoffice` | **LibreOffice 或 Apache OpenOffice**；配置 `converter.openoffice.command`（如 `soffice.exe`）和 `user_profile`。 |
| 引擎 `ofd` | **不用另装。** OFD→PDF 已编进二进制（`internal/ofd` + [zc310/ofd](https://github.com/zc310/ofd)）。 |
| 桌面控制窗（Windows 默认） | 普通带桌面的会话（OpenGL）。无界面 / Server Core：用 **`--noui`**。 |
| CGO 编出的 Windows exe 无法启动 | 安装对应的 **Visual C++ 可再发行组件**（amd64 包装 x64）。多数桌面系统已有。 |

### 新机器上不必装

- **Go**、Git、C 编译器（除非还要在那台机器上从源码编译）
- **Node.js** / npm（除非还要在那台机器上重新构建 Web UI）
- 未启用的引擎对应的 Office / WPS / LibreOffice
- 第二个名为 convert-worker 的程序

### 新机器上第一次启动

服务器 / 无界面推荐控制台模式（立刻起 HTTP 与 Worker）：

```powershell
.\msoffice2pdf.exe serve --noui --config config\config.yaml
```

```bash
./msoffice2pdf serve --noui --config config/config.yaml
```

同一 exe 创建管理员（仍不需要 Go）：

```powershell
.\msoffice2pdf.exe user create-admin --uid=admin --pwd=secret --config config\config.yaml
```

健康检查：`curl -i http://127.0.0.1:8080/health`（端口见 `server.port`）。

Windows 上不加 `--noui` 会打开 **桌面控制窗口**，要点 **Start** 才会监听 HTTP；日志在窗口里，不一定出现在 PowerShell。

每台机器只允许 **一个** serve 进程；`server.port` 已被占用时启动失败。

### DCOM（仅当 COM 跑在无交互会话时）

已登录用户前台或 `--noui` 通常不必再配 DCOM。若以后用 Windows 服务（Session 0）跑 COM，需按 [usage_en.md](./usage_en.md) §1.1 / [docs/usage.md](./docs/usage.md) §1.1 用 `dcomcnfg` 为 Word / Excel / PowerPoint 设置 Identity（This User）。

---

## 转换并发、内存与磁盘

完整键说明与英文对照见 [docs/usage.md](./docs/usage.md) §6.1 / [usage_en.md](./usage_en.md) §6.1；配置注释见 `config/config.template_zh.yaml` 的 `converter` 节。

按**峰值并发**预留内存和磁盘，不要只按 Go 进程估算。每路 COM 任务大约 **0.5–1.5GB** 子进程内存，磁盘上同时存在源文件与 PDF（约 `2 × upload.max_size`）。

| 键 | 口径 |
|----|------|
| `converter.worker_count` | 同时转换上限。建议 `≤ min(CPU 核数, 物理内存 GB / 2)`；8GB+ Office 机常见 **4** |
| `converter.min_workers` | 资源不足时的下限，缺省 **1** |
| `converter.mem_limit_mb` | Go 堆告警线。`0` = 物理内存 50%、至少 512MiB。不含 Office 子进程 |
| `converter.disk_min_free_mb` | 监视 `upload`/`output`/`trash`/`expired` 与 `log.file_dir`。缺省 **1024**。建议 `≥ worker_count × max_size × 2` 再加日志余量 |
| `converter.log_backlog_max_mb` | 未落盘日志内存上限。缺省 **32** |

内存或磁盘不够时，服务把并发降到 `min_workers`（已入队任务不丢）；恢复后每次 +1 直到 `worker_count`。日志写入文件/控制台后立刻从内存剔除，不阻塞转换、也不丢行。不要用同步写盘来“省内存”，那会卡住 Worker。

```yaml
converter:
  worker_count: 4
  min_workers: 1
  mem_limit_mb: 0
  disk_min_free_mb: 1024
  log_backlog_max_mb: 32
```

改配置后需重启服务。键须同步到两份配置模板。

管理员可在 Web **管理 → 性能概览** 查看当前积压与历史曲线（`GET /api/admin/metrics`、`/api/admin/metrics/history`）。采样间隔 `cleanup.metrics_interval`（默认 10s），保留 `cleanup.metrics_ttl`（默认 7 天）。

---

## 转换状态 SSE

不想轮询 `GET /api/pdf/:fileid/status` 时，可用 SSE 订阅一个或多个文件的状态推送。轮询接口仍然保留。

### 接口

| 项 | 说明 |
|----|------|
| 方法/路径 | `GET /api/pdf/events` |
| 鉴权 | 与其它 API 相同：`X-UID` + `X-Token`，或 Web Session Cookie |
| Query `fileid` | **必填**；支持两种写法（可混用），去重后数量为 `1 … server.sse.max_fileids`（默认 50） |

| 写法 | 示例 |
|------|------|
| 重复参数（原写法） | `?fileid=aaa&fileid=bbb` |
| 逗号分隔 | `?fileid=aaa,bbb,ccc` |

任一 `fileid` 不存在或无权访问 → **开流前**整单失败（JSON `404`/`403`），不会半开订阅。

### 事件

响应：`Content-Type: text/event-stream`。

| event | 时机 | data |
|-------|------|------|
| `status` | 连接后每个 `fileid` 先发一条快照；之后仅状态指纹变化时再发 | 与 `GET /api/pdf/:fileid/status` 同形 JSON（含 `fileid` / `request_id` / `status` / `final_status` / `error_*` / `retry_count` / `warn_code` 等） |
| `ping` | 每隔 `server.sse.heartbeat_interval`（默认 15s） | `{}`（可忽略） |
| `done` | 流结束 | `reason=all_terminal` 或 `reason=max_duration`（后者带 `pending`） |

客户端用 `request_id` / `fileid` 更新本地 UI。收到 `done` 后若仍有未完成文件，可再次请求同一接口重连。

### 服务端运行原理（摘要）

1. **建流前预检**：对每个 `fileid` 走与 status 相同的查询与权限校验。  
2. **写超时豁免**：对本连接 `SetWriteDeadline(零值)`，避免全局 `server.write_timeout`（如 60s）掐断长连接。  
3. **快照**：推送初始 `status`；若已全部终态则立刻 `done(all_terminal)` 并关闭。  
4. **循环**：按 `poll_interval`（默认 1s）查库；指纹  
   `(status, retry_count, error_msg, final_status, error_code, warn_code)`  
   变化才再推 `status`；按 `heartbeat_interval` 发 `ping`。  
5. **结束**：全部到达**终态**（`upload_history.final_status` 为 `completed` / `failed` / `deleted`），或达到 `max_duration`（默认 **5 分钟**）。活表中待重试的 `failed` **不算**终态。

与 `converter.office_timeout`（单次转换）无关；SSE 业务超时只看 `server.sse.max_duration`。

### 配置

```yaml
server:
  sse:
    max_duration: 5m
    heartbeat_interval: 15s
    poll_interval: 1s
    max_fileids: 50
```

键须同步到 `config/config.template_zh.yaml` / `config.template_en.yaml`。

### curl 示例

```bash
# 重复参数
curl -N "http://127.0.0.1:8080/api/pdf/events?fileid=<id1>&fileid=<id2>" \
  -H "X-UID: u1" -H "X-Token: <api_token>"

# 逗号分隔
curl -N "http://127.0.0.1:8080/api/pdf/events?fileid=<id1>,<id2>" \
  -H "X-UID: u1" -H "X-Token: <api_token>"
```

### 本地冒烟客户端

目录 `testdata/sse_smoke`（`testdata/` 默认 gitignore）：上传 `1.*`…`8.*`，`X-Request-ID` 为 `1`…`8`，再订阅 SSE 并在控制台连续打印事件。

```bash
# 仓库根目录
go run ./testdata/sse_smoke

# 或在 testdata/ 下
go run ./sse_smoke

# 可选：-base -uid -token -dir -from -to
go run ./sse_smoke -comma   # 使用 ?fileid=a,b,c
```

技术细节（指纹、组件、时序）见 [docs/详细设计说明书.md](./docs/详细设计说明书.md) §4.4。
