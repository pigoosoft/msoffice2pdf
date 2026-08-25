# 设计：OFD→PDF 公共类库与通用文档密码

**日期：** 2026-08-25  
**状态：** 已确认  
**依据：** 会话确认（类库形态 A：`pkg/ofd`；保真 A：混合矢量/光栅；密码进 Convert 并接到上传；密码对所有引擎共用；实现路径 1：`pkg/ofd` + `convert-worker` 子进程）  
**取代：** [2026-08-24-ofd-pdf-and-doc-password-design.md](./2026-08-24-ofd-pdf-and-doc-password-design.md)（该稿将 OFD 放在 `internal/ofd`；本稿改为同仓库公开包 `pkg/ofd`）  
**范围：** 公开类库 `pkg/ofd`、引擎 `ofd`、GB/T 33190 自研解析/混合渲染、`.ofd` 上传校验、通用文档密码、密码类失败不重试、配置/错误码/Web 上传框。  
**不含：** 第三方 OFD 库、独立 Go module、DLL、国密自动解密、验签/证书链、视频音频附件、3D、整页光栅作为主路径、`auto` 引擎、运行中热切换引擎、单独的 `ofd2pdf` 可执行文件。

**参考（只对照，不引入依赖）：** GB/T 33190-2016；ofdrw（`ofdrw-reader` 包结构、`ofdrw-converter` 导出流水线）。

**关联：** [转换引擎抽象](./2026-08-10-converter-engines-design.md)、[OpenOffice CLI 引擎](./2026-08-10-openoffice-engine-design.md)、[COM 子进程](./2026-08-05-com-subprocess-design.md)。

---

## 1. 目标

Microsoft Office 与 WPS 均不能打开 OFD（GB/T 33190 版式包）。需要：

- 把 `.ofd` 作为一等上传类型，转换成 PDF，进入现有队列、水印、归档
- OFD 处理单独放在 **`pkg/ofd`**，封装成同仓库**公开类库**；主程序只通过参数调用，不依赖类库内部类型
- **自研**解析与绘制（对照 ofdrw 与国标；不引入 Java 运行时、不引入第三方 OFD 专用库；PDF 复用已有 `gopdf` / `pdfcpu`）
- 保真策略：**混合**——文字、路径、图片走矢量；签章、渐变、无法稳定映射的效果按对象包围盒光栅化后贴回
- 转换在 **`convert-worker` 子进程**中执行（与 COM 相同的超时杀进程模型），不在主服务队列 Worker 里渲染
- 文档密码做成**所有引擎共用**的能力：先无密码打开；未加密则忽略密码；加密则用传入密码；错密码立即失败

对外：上传仍异步返回 `queued`；调用端用现有状态 API / SSE 读结果。

---

## 2. 已确认决策

| 项 | 选择 |
|----|------|
| 类库形态 | 同仓库公开包 `pkg/ofd`（非 `internal/`、非独立 module、非 DLL） |
| 文档范围 | 通用 OFD（不限发票）；细节尽量保真 |
| 保真方式 | 混合：文字/路径/图片矢量；签章与画不稳对象光栅化贴回 |
| 引擎名 | `ofd`（唯一；∈ `converter.engines`） |
| 扩展绑定 | `ext_engines`：`*.ofd` → `ofd`；无 `auto` |
| 渲染位置 | **始终**子进程 `convert-worker --engine ofd`；不跟 `com_mode` |
| OFD 库 | 不依赖第三方 OFD 包；自研 `pkg/ofd` |
| PDF 写出 | 现有 `gopdf` / `pdfcpu`；光栅用标准库 / `golang.org/x/image` |
| 密码入口 | Header `X-Doc-Password` + 表单字段 `password`；网页可选输入框 |
| 冲突 | Header 与表单都有且均非空 → **Header 优先** |
| 密码范围 | 所有引擎共用（OFD 与 Office） |
| 密码存储 | 只在内存 `queue.Task`；子进程用环境变量；不入库、不打日志、不进命令行 |
| 打开顺序 | 先无密码 → 未加密则忽略密码；已加密再用密码 → 仍失败则密码类错误 |
| 密码错误 | **不重试**（不走 `retry_count`） |
| AppKind | `ofd` 映射的扩展 **不要求** writer/spreadsheet/presentation |
| 启动探测 | `ofd` 无需 COM/CLI；列入 `engines` 即可加载 |
| 超时 | 沿用 `converter.office_timeout`；到期杀掉 worker（Windows Job Object；Unix 进程组） |

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
        │                      子进程：pkg/ofd.Convert(ctx, src, dst, Options{Password})
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
| `pkg/ofd` | 公共类库：解包、解密、对象模型、混合渲染、写出 PDF；稳定 `Convert` API |
| `internal/converter` | `EngineOFD = "ofd"`；`ofdEngine` 只拉起 `convert-worker`，**不** import `pkg/ofd` |
| `cmd/msoffice2pdf` | `convert-worker --engine ofd` 时 import `pkg/ofd` 并调用 `Convert` |
| `internal/queue` | `Task.DocPassword`；传入 Convert；密码错误短路失败 |
| `internal/handlers` + `ui` | Header/表单/上传页密码框 |

主进程与 worker 是**同一个可执行文件**的不同子命令。隔离靠 `exec`：主进程队列 Worker **不得**调用 `ofd.Convert`；只有 `convert-worker` 进程调用类库。禁止 `converter.New` 在 worker 内再选 `ofd` 引擎（会递归 exec）。

---

## 4. `pkg/ofd` 公开 API

调用方只依赖下列符号；类库内部类型（ZIP 成员、页面对象、渲染器）不导出。

```go
package ofd

type Options struct {
    Password string // 可空；未加密包必须忽略
}

func Convert(ctx context.Context, srcPath, dstPath string, opts Options) error
```

导出错误（`errors.Is` 可判定；`Error()` 不得包含密码明文）：

| 错误 | 含义 |
|------|------|
| `ErrPasswordRequired` | 包已加密且 `Options.Password` 为空 |
| `ErrPasswordWrong` | 已提供密码仍无法解密 |
| `ErrInvalidPackage` | 不是合法 OFD（非 ZIP、无 `OFD.xml`、结构损坏） |
| `ErrNoPages` | 解析后没有可输出页 |

`ctx` 取消时停止转换并删除不完整输出。成功写出：先写 `dstPath + ".partial"`，再 rename 为 `dstPath`。

内部可按文件拆分（如 `open.go` / `doc.go` / `render.go` / `crypt.go`），不拆成多个对外 module。对象模型与流水线对照 ofdrw-reader → ofdrw-converter，实现为 Go 自研。

### 4.1 转换流水线

1. 打开 `srcPath` 为 ZIP，定位 `OFD.xml`
2. 若存在加密描述（如 `Encryptions.xml` / `Encryption.xml`）：无密码 → `ErrPasswordRequired`；有密码则解密；失败 → `ErrPasswordWrong`
3. 按 GB/T 33190 加载 DocBody → Document → 公共/文档资源 → 各页 Content
4. 多文档包按 `OFD.xml` 声明顺序，全部页写入**同一个** PDF
5. 混合绘制（见第 6 节）
6. 原子落盘；失败删除 `.partial`

不支持的加密算法（含国密）：有密码 → `ErrPasswordWrong`；无密码 → `ErrPasswordRequired`。不实现无用户口令的自动破解。

`pkg/ofd` **不得** import `internal/converter` 或 `internal/queue`（避免类库依赖本服务）。密码错误码字符串与应用层对齐为 `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG`，以便 worker 原样返回、队列 `errors.Is` 判定。

---

## 5. 配置

同步更新：`config/config.yaml`、`config/config.template_zh.yaml`、`config/config.template_en.yaml`（**键与说明必须同步**）。

合法 `converter.engines` 值：`msoffice` | `wpsoffice` | `openoffice` | `ofd`。

```yaml
converter:
  engines:
    - msoffice
    - ofd
  ext_engines:
    # …现有 Office 映射…
    "*.ofd": ofd

upload:
  allowed_exts:
    # …现有…
    - "*.ofd"
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

`com_mode` / `temp_sandbox` / `excel_page_fit` 对 `ofd` 无效。**不得**把文档密码写入沙箱路径或文件名。

---

## 6. OFD 渲染范围

按 GB/T 33190 解析 ZIP 包：`OFD.xml` → 文档 → 页内容 → 公共/文档资源。

### 6.1 矢量（必须做实）

- 页面尺寸与坐标系（mm → PDF 点；页级/对象级 CTM；裁剪）
- `TextObject`：位置、字号、字距、方向；优先嵌入包内字体，缺失则系统回退并 WARN
- `PathObject`：描边/填充、线宽、线型、非零/奇偶规则
- `ImageObject`：JPEG/PNG/BMP 等常见栅格按原图嵌入，避免无故再压缩
- `CompositeObject`：递归展开
- 图层按文档顺序叠放

### 6.2 光栅化后贴回

- 签章 / Stamp 类注释（含骑缝：按页裁切贴边）
- 渐变、图案填充、不支持的颜色空间
- 无法映射的路径效果（复杂透明混合等）
- 按**该对象包围盒**光栅化，禁止把整页先画成一张图再当主输出

### 6.3 第一版跳过（WARN，页面仍输出）

- 视频/音频、附件、书签大纲
- 验签与证书链（签章只当图形）
- 3D、交互注释

解析失败、无页面、或写出的 PDF 无法打开 / 页数为 0 → 返回错误，任务 `failed`（可按普通转换失败重试）。

---

## 7. 文档密码（通用）

### 7.1 入口

| 来源 | 字段 | 用途 |
|------|------|------|
| HTTP Header | `X-Doc-Password` | API / 脚本 |
| multipart 表单 | `password` | 网页与 curl `-F` |
| Web UI | 上传页可选密码框 | 只写入表单，不进 URL |

两边都有非空值时 Header 优先。空字符串视为未提供。

### 7.2 传递

1. Handler 读出密码，交给 Upload 入队
2. `queue.Task.DocPassword` 仅内存；**upload / pdf / pdflog / upload_history 均无密码列**
3. `Converter.Convert(ctx, src, dst, password string)`（`password` 可空）；所有 `Engine` 签名对齐
4. `ofd` 与 COM 子进程：环境变量 `MSOFFICE2PDF_DOC_PASSWORD`；**禁止** `--password` 命令行参数
5. `convert-worker --engine ofd` 读取环境变量后调用 `ofd.Convert(..., Options{Password: ...})`
6. 子进程用完即弃；stderr 只回错误码语义，不得回显口令
7. 结构化日志字段禁止包含密码；`Error()` 包装底层信息必须剥离口令

进程重启后内存队列丢失。从 DB 扫回的 queued/converting 任务**没有密码**；若文件仍加密，按「需要密码」失败。

内存内 `requeue` / 队列重投必须拷贝 `DocPassword`。**密码类错误不得进入该重试。**

### 7.3 打开顺序（所有引擎）

1. 无密码打开
2. 若文件未加密：成功则忽略调用方给的密码
3. 若加密或打开失败且判定为密码保护：无密码 → `ERR_DOC_PASSWORD_REQUIRED`；有密码则再试
4. 带密码仍失败 → `ERR_DOC_PASSWORD_WRONG`

OFD：以包内加密描述及解包结果为准。Office COM / OpenOffice：打开 API 的 Password 参数；无法区分「密码错」与「文件损坏」时：已提供密码则 `ERR_DOC_PASSWORD_WRONG`，未提供则 `ERR_DOC_PASSWORD_REQUIRED`。

应用层 `internal/converter` 定义同名语义的 sentinel（供 COM/OpenOffice 使用），**不** import `pkg/ofd`。`convert-worker` 将 `pkg/ofd` 错误映射为相同 `ERR_DOC_PASSWORD_*` 字符串，供队列判定。

### 7.4 错误码（异步任务，非上传同步 400）

上传成功仍返回 200 + `queued`。密码问题出现在转换阶段，经状态查询 / SSE / 看板展示。

| 码 | 含义 | 重试 |
|----|------|------|
| `ERR_DOC_PASSWORD_REQUIRED` | 文档已加密且未提供密码 | 否 |
| `ERR_DOC_PASSWORD_WRONG` | 已提供密码仍无法打开 | 否 |

写入 `upload` 失败信息及归档 `upload_history.error_code`。Web 文案：需要密码 / 文档密码错误（中英 i18n）。不新增上传同步 HTTP 数字码（如 40005）。

---

## 8. 转换接口与子进程

现有 `Convert(ctx, src, dst)` 扩展为带密码；所有 `Engine` 实现签名对齐。

`convert-worker`：

- `--engine` 允许 `ofd`（以及现有 `msoffice` / `wpsoffice`）
- `--engine ofd` 时 **不要求** `--app-kind`
- 从 `MSOFFICE2PDF_DOC_PASSWORD` 读取密码（可空）
- 子进程内直接调用 `pkg/ofd.Convert`（禁止再走 `ofdEngine`）

Windows：沿用 Job Object，超时杀进程树；孤儿 `convert-worker` 扫描规则不变。

非 Windows：`Setpgid` + 超时杀进程组。

---

## 9. 失败处理

| 情况 | 行为 |
|------|------|
| `ERR_DOC_PASSWORD_REQUIRED` / `ERR_DOC_PASSWORD_WRONG` | 立即 failed，不按 `retry_count` 重试 |
| OFD 损坏、无页面、空 PDF | failed，走普通重试 |
| 超时 / worker 被杀 / worker 崩溃 | failed，文案 timeout 或崩溃信息，可重试；主服务继续跑 |
| 单对象无法绘制 | WARN + 跳过该对象，任务仍可成功 |
| 水印失败 | 保持现有：超时则整任务失败；其它水印错误 WARN |

---

## 10. Web UI

上传页在水印旁增加可选「文档密码」输入框（`type=password`，可清空）。提交走现有 `FormData`，字段名 `password`。看板/详情对上述两个 error_code 显示本地化短句。不在浏览器持久化口令。

---

## 11. 测试与验收

不依赖本机 Office/WPS。夹具放 `pkg/ofd/fixtures`（根目录 `testdata/` 在 `.gitignore` 中，勿用该名）。

**解析/渲染：**

- 最小合法包 → 可打开 PDF，页面尺寸与源一致
- 文字+路径+图 → PDF 中对应内容为矢量/嵌入图，而非整页位图
- 签章/渐变 → 该区域为贴图，其余仍矢量
- 多 Doc → 页序与 `OFD.xml` 一致
- 缺字体 → 回退 + WARN，任务成功
- 坏 ZIP / 无 `OFD.xml` / 无页面 → 失败且可区分（`ErrInvalidPackage` / `ErrNoPages`）

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

## 12. 文档与错误码表

实现时同步：

- `docs/详细设计说明书.md`：引擎表、`validate_ofd`、`pkg/ofd` API、密码 Header、错误码
- `docs/usage.md` / `usage.md`：上传示例带 `-H "X-Doc-Password: …"`
- Web i18n（中/英）

---

## 13. 非目标

- 不实现无密码的国密自动破解
- 不验证电子签章真伪
- 不把 OFD 交给 LibreOffice / WPS / Microsoft Office
- 不把密码写入磁盘或数据库
- 不发布独立 module、不编译 DLL、不新增 `ofd2pdf` 二进制
- 第一版不追求 GB/T 33190 全条款覆盖；未列对象跳过并 WARN
