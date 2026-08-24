# 设计：OFD→PDF 自研引擎与通用文档密码

**日期：** 2026-08-24  
**状态：** 已确认  
**依据：** 会话确认（方案 2：自研 OFD 渲染 + `convert-worker` 子进程隔离；文档密码 API Header + 网页表单）  
**范围：** 新引擎 `ofd`、GB/T 33190 自研解析/混合渲染、`.ofd` 上传校验、通用文档密码、密码类失败不重试、配置/错误码/Web 上传框。  
**不含：** 第三方 OFD 库、国密自动解密、验签/证书链、视频音频附件、3D、整页光栅作为主路径、`auto` 引擎、运行中热切换引擎。

**关联：** [转换引擎抽象](./2026-08-10-converter-engines-design.md)、[OpenOffice CLI 引擎](./2026-08-10-openoffice-engine-design.md)、[COM 子进程](./2026-08-05-com-subprocess-design.md)。

---

## 1. 目标

Microsoft Office 与 WPS 均不能打开 OFD（GB/T 33190 版式包）。需要：

- 把 `.ofd` 作为一等上传类型，转换成 PDF，进入现有队列、水印、归档
- **自研**解析与绘制（不引入 OFD 专用第三方库；绘制复用已有 `gopdf` / `pdfcpu`）
- 保真策略：**混合**——文字、路径、图片走矢量；签章、渐变、无法稳定映射的效果按对象包围盒光栅化后贴回
- 转换在 **`convert-worker` 子进程**中执行（与 COM 相同的超时杀进程模型），不在主服务进程内渲染
- 文档密码做成**所有引擎共用**的能力：先无密码打开，失败再带密码；错密码立即失败并告知调用端

对外：上传仍异步返回 `queued`；调用端用现有状态 API / SSE 读结果。

---

## 2. 已确认决策

| 项 | 选择 |
|----|------|
| 文档范围 | 通用 OFD（不限发票）；细节尽量保真 |
| 保真方式 | 混合：文字/路径/图片矢量；签章与画不稳对象光栅化贴回 |
| 引擎名 | `ofd`（唯一；∈ `converter.engines`） |
| 扩展绑定 | `ext_engines`：`*.ofd` → `ofd`；无 `auto` |
| 渲染位置 | **始终**子进程 `convert-worker --engine ofd`；不跟 `com_mode` |
| OFD 库 | 不依赖第三方 OFD 包；自研 `internal/ofd` |
| PDF 写出 | 现有 `gopdf` / `pdfcpu`；光栅用标准库 / `golang.org/x/image` |
| 密码入口 | Header `X-Doc-Password` + 表单字段 `password`；网页可选输入框 |
| 冲突 | Header 与表单都有且均非空 → **Header 优先** |
| 密码存储 | 只在内存 `queue.Task`；子进程用环境变量；不入库、不打日志、不进命令行 |
| 打开顺序 | 先无密码 → 判定加密后再用密码 → 仍失败则密码类错误 |
| 密码错误 | **不重试**（不消耗/不等待 `retry_count` 路径） |
| 未加密文件 | 忽略传入密码，正常转 |
| AppKind | `ofd` 映射的扩展 **不要求** writer/spreadsheet/presentation |
| 启动探测 | `ofd` 无需 COM/CLI；列入 `engines` 即可加载；不因本机无 Office 而失败 |
| 超时 | 沿用 `converter.office_timeout`；到期杀掉 worker 进程（Windows Job Object；Unix 进程组） |

---

## 3. 架构

```text
POST /api/upload
  file + (X-Doc-Password | form password) + optional watermark
        │
        ▼
  validate magic/structure（ofd：ZIP + OFD.xml）
  落盘 upload/{uid}/…    写 upload 行 pending
  Task{Src, Dst, DocPassword} → 内存队列   ← 密码仅此处
        │
        ▼
  Queue Worker（主进程）
        │  Convert(ctx, src, dst, password)
        ▼
  routingConverter
        │
        ├─ engine=ofd ──► ofdEngine.exec(self, convert-worker --engine ofd)
        │                      env MSOFFICE2PDF_DOC_PASSWORD
        │                      Job Object / process group + office_timeout
        │                      子进程：internal/ofd 解析+混合渲染 → dst.pdf
        │
        └─ msoffice / wpsoffice / openoffice ──► 现有路径，同样接收 password
        │
        ▼
  成功：水印后处理（不变）→ pdf completed → ArchiveUpload
  失败：删不完整 dst → 状态 failed + error_code（密码类不重试）
```

**不变：** Gin 分层、upload/pdf 表结构（不增加密码列）、水印、TTL 清理、`ext_engines` 路由模型。

**包划分：**

| 包 | 职责 |
|----|------|
| `internal/ofd` | 解包、解密（用户密码）、对象模型、混合渲染、写出 PDF |
| `internal/converter` | `EngineNameOFD = "ofd"`；`ofdEngine` 实现 `Engine`；拉起 `convert-worker` |
| `cmd/msoffice2pdf` | `convert-worker` 增加 `--engine ofd`；从环境变量读密码 |
| `internal/queue` | `Task.DocPassword`；Convert 传入；密码错误短路失败 |
| `internal/handlers` + `ui` | Header/表单/上传页密码框 |

主进程 **不得** `import` 渲染热路径以外的方式在 Worker 里直接画 OFD；主进程只 exec。`internal/ofd` 允许被 `convert-worker` 调用。

---

## 4. 配置

同步更新：`config/config.yaml`、`config/config.template_zh.yaml`、`config/config.template_en.yaml`（**键与说明必须同步**）。

合法 `converter.engines` 值扩展为：`msoffice` | `wpsoffice` | `openoffice` | `ofd`。

```yaml
converter:
  engines:
    - msoffice
    # - wpsoffice
    # - openoffice
    - ofd
  ext_engines:
    # …现有 Office 映射…
    "*.ofd": ofd

upload:
  allowed_exts:
    # …现有…
    - "*.ofd"
  # OFD 包内必须存在的 ZIP 成员。与 validate_new / validate_ole 互斥。
  validate_ofd:
    "*.ofd":
      - "OFD.xml"
```

**校验规则：**

- `ofd` ∈ `engines` 时无需 `converter.openoffice.*`，无需 COM 探测
- `ext_engines` 值为 `ofd` 的扩展：**跳过** AppKind 推断；其它引擎仍必须能从 `validate_new` / `validate_ole` 推断
- 同一扩展不得同时出现在 `validate_new`、`validate_ole`、`validate_ofd` 中的任意两者
- `OfficeFamily` 增加 `"ofd"`：由 `validate_ofd` 拥有该扩展时返回；魔数为 ZIP/PK（与 OOXML 相同文件头，靠表归属区分家族）
- `allowed_exts` 含 `*.ofd` 但 `ext_engines` 无映射：现有行为（启动 ERROR 日志，上传 `40004` / `ERR_EXT_ENGINE_UNMAPPED`）

`com_mode` / `temp_sandbox` / `excel_page_fit` 对 `ofd` 无效。子进程仍建议在隔离环境跑；是否套 COM 沙箱目录由实现沿用现有 worker 启动逻辑，**不得**把文档密码写入沙箱路径或文件名。

---

## 5. OFD 渲染范围

按 GB/T 33190 解析 ZIP 包：`OFD.xml` → 文档 → 页内容 → 公共/文档资源。

多文档包：按 `OFD.xml` 声明顺序将全部页写入**同一个** PDF。

### 5.1 矢量（必须做实）

- 页面尺寸与坐标系（mm → PDF 点；页级/对象级 CTM；裁剪）
- `TextObject`：位置、字号、字距、方向；优先嵌入包内字体，缺失则系统回退并 WARN
- `PathObject`：描边/填充、线宽、线型、非零/奇偶规则
- `ImageObject`：JPEG/PNG/BMP 等常见栅格按原图嵌入，避免无故再压缩
- `CompositeObject`：递归展开
- 图层按文档顺序叠放

### 5.2 光栅化后贴回

- 签章 / Stamp 类注释（含骑缝：按页裁切贴边）
- 渐变、图案填充、不支持的颜色空间
- 无法映射的路径效果（复杂透明混合等）
- 按**该对象包围盒**光栅化，禁止把整页先画成一张图再当主输出

### 5.3 第一版跳过（WARN，页面仍输出）

- 视频/音频、附件、书签大纲
- 验签与证书链（签章只当图形）
- 3D、交互注释

解析失败、无页面、或写出的 PDF 无法打开 / 页数为 0 → 任务 `failed`（可按普通转换失败重试）。

---

## 6. 文档密码（通用）

### 6.1 入口

| 来源 | 字段 | 用途 |
|------|------|------|
| HTTP Header | `X-Doc-Password` | API / 脚本 |
| multipart 表单 | `password` | 网页与也可用于 curl `-F` |
| Web UI | 上传页可选密码框 | 只写入表单，不进 URL |

两边都有非空值时 Header 优先。空字符串视为未提供。

### 6.2 传递

1. Handler 读出密码，交给 Upload 入队
2. `queue.Task.DocPassword` 仅内存；**upload / pdf / pdflog / upload_history 均无密码列**
3. `Converter.Convert(ctx, src, dst, password string)`（`password` 可空）
4. `ofd` 与 COM 子进程：环境变量 `MSOFFICE2PDF_DOC_PASSWORD`；**禁止** `--password` 命令行参数
5. 子进程用完即弃；stderr 只回错误码语义，不得回显口令
6. 结构化日志字段禁止包含密码；错误 `Error()` 若需包装底层信息必须剥离口令

进程重启后内存队列丢失。从 DB 扫回的 queued/converting 任务**没有密码**；若文件仍加密，按「需要密码」失败，而不是使用任何持久化口令。

内存内 `requeue` / 队列重投必须拷贝 `DocPassword`（同一进程未重启时重试普通错误仍带原密码）。**密码类错误不得进入该重试。**

### 6.3 打开顺序（所有引擎）

1. 无密码打开
2. 若文件未加密：成功则忽略调用方给的密码
3. 若加密或打开失败且判定为密码保护：无密码 → `ERR_DOC_PASSWORD_REQUIRED`；有密码则再试
4. 带密码仍失败 → `ERR_DOC_PASSWORD_WRONG`

OFD：以包内加密描述（如 `Encryption.xml`）及解包结果为准。Office COM / OpenOffice：打开 API 的 Password 参数；实现须把「密码错」与「文件损坏」区分到上述两个码，无法区分时：已提供密码则 `ERR_DOC_PASSWORD_WRONG`，未提供则 `ERR_DOC_PASSWORD_REQUIRED`。

### 6.4 错误码（异步任务，非上传同步 400）

上传成功仍返回 200 + `queued`。密码问题出现在转换阶段，经状态查询 / SSE / 看板展示。

| 码 | 含义 | 重试 |
|----|------|------|
| `ERR_DOC_PASSWORD_REQUIRED` | 文档已加密且未提供密码 | 否 |
| `ERR_DOC_PASSWORD_WRONG` | 已提供密码仍无法打开 | 否 |

写入 `upload` 失败信息及归档 `upload_history.error_code`（与 `ERR_RETRY_LIMIT_EXCEEDED` 同机制）。Web 文案：需要密码 / 文档密码错误（中英 i18n）。

不新增上传同步 HTTP 数字码（40005 等），避免与「先入队再转换」语义冲突。

---

## 7. 转换接口与子进程

现有 `Convert(ctx, src, dst)` 扩展为带密码；所有 `Engine` 实现签名对齐。

`convert-worker`：

- `--engine` 允许 `ofd`（以及现有 `msoffice` / `wpsoffice`）
- `--engine ofd` 时 **不要求** `--app-kind`
- 从 `MSOFFICE2PDF_DOC_PASSWORD` 读取密码（可空）
- 子进程内直接调用 `internal/ofd`（`inprocess` 仅对 COM 有意义；ofd worker 内渲染即本进程，与主服务隔离）

Windows：沿用 Job Object，超时 `CloseHandle` 杀树；孤儿 `convert-worker` 扫描规则不变（命令行含 `convert-worker`）。

非 Windows：为 `ofd`（及需要时的 COM 以外引擎）实现等价 subprocess：`Setpgid` + 超时杀进程组，避免只杀父进程留下渲染子进程。

---

## 8. 失败处理（与现有队列对齐）

| 情况 | 行为 |
|------|------|
| `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG` | 立即 failed，不按 `retry_count` 重试 |
| OFD 损坏、无页面、空 PDF | failed，走普通重试 |
| 超时 / worker 被杀 / worker 崩溃 | failed，文案 `timeout` 或崩溃信息，可重试 |
| 单对象无法绘制 | WARN + 跳过该对象，任务仍可成功 |
| 水印失败 | 保持现有：超时则整任务失败；其它水印错误 WARN 码 |

---

## 9. Web UI

上传页在水印旁增加可选「文档密码」输入框（`type=password`，可清空）。提交走现有 `FormData`，字段名 `password`。看板/详情对上述两个 error_code 显示本地化短句。不在浏览器持久化口令。

---

## 10. 测试与验收

不依赖本机 Office/WPS。夹具放 `internal/ofd/fixtures`（根目录 `testdata/` 在 `.gitignore` 中，勿用该名）。

**解析/渲染：**

- 最小合法包 → 可打开 PDF，页面尺寸与源一致
- 文字+路径+图 → PDF 中对应内容为矢量/嵌入图，而非整页位图
- 签章/渐变 → 该区域为贴图，其余仍矢量
- 多 Doc → 页序与 `OFD.xml` 一致
- 缺字体 → 回退 + WARN，任务成功
- 坏 ZIP / 无 `OFD.xml` / 无页面 → 失败且可区分

**密码：**

- 未加密 + 带密码 → 成功
- 加密 + 无密码 → `ERR_DOC_PASSWORD_REQUIRED`，不重试
- 加密 + 错密码 → `ERR_DOC_PASSWORD_WRONG`，不重试
- 加密 + 对密码 → 成功
- 日志、DB、pdflog、进程命令行均无口令明文

**接入：**

- 映射完整时可上传 `.ofd`；未映射 `40004`
- Header 优先于表单
- `ofd` 走 `convert-worker`；超时杀子进程后主服务仍存活

**人工验收：** 至少 1 个无加密 OFD、1 个带签章、1 个加密 OFD。文字可选、印章位置肉眼可接受；不要求与专有阅读器像素级一致。

---

## 11. 文档与错误码表

实现时同步：

- `docs/详细设计说明书.md`（或现行 `docs/` 下详细设计）：引擎表、`validate_ofd`、密码 Header、错误码
- `docs/usage.md` / `usage.md`：上传示例带 `-H "X-Doc-Password: …"`
- Web i18n（中/英）

---

## 12. 非目标（再次收口）

- 不实现无密码的国密自动破解
- 不验证电子签章真伪
- 不把 OFD 交给 LibreOffice/WPS/MS Office
- 不把密码写入磁盘或数据库（含加密列也不做）
- 第一版不追求 GB/T 33190 全条款覆盖；未列对象跳过并 WARN
