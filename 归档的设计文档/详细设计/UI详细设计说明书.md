# 设计：Web UI 详细设计

**日期：** 2026-08-07  
**状态：** 已批准  
**依据：** `归档的设计文档/架构设计说明书.md` §9；后端 `internal/server/server.go` + `docs/usage.md`（冲突以后端为准）

## 1. 目标与边界

### 1.1 目标

- 提供 MSOffice2Pdf 官方 Web 管理/使用界面：顶栏 + 左侧工具 + 主操作区。
- 同一套 UI 服务普通用户与管理员；管理员在侧栏「管理」下进入管理功能。
- 仅调用后端 REST API，不直连数据库或磁盘。
- 支持 PC / Pad / Phone 自适应。

### 1.2 非目标（本期）

- `go:embed` 嵌入二进制（UI 成熟后再做；本期独立 Nginx/静态托管）。
- SSE / WebSocket 推送（一期定时轮询；后期可替换）。
- 用户自助改密；原 Office 下载不作为主路径（API 保留，详情可选）。
- 前端自动化测试套件（手工验收）。

### 1.3 分期

| 期 | 范围 |
|----|------|
| 一期 | 登录、壳层、转换看板、上传、我的 PDF（含详情 pdflog）、历史 |
| 二期 | 「管理」：用户管理、系统总览（壳与路由可一期预留，非 admin 不显示） |

## 2. 技术选型（锁定）

| 项 | 选择 |
|----|------|
| 框架 | Vue 3 + TypeScript |
| UI | Element Plus |
| 状态 | Pinia |
| 构建 | Vite |
| HTTP | Axios（`withCredentials: true`） |
| 路由 | Vue Router |
| 实现风格 | 标准后台壳：`AppLayout` + 一页一路由（方案 1） |

## 3. 壳层与信息架构

### 3.1 布局

```
┌─────────────────────────────────────────────┐
│ TopNav：Logo / 产品名 │ 当前用户 │ 登出      │
├──────────┬──────────────────────────────────┤
│ SideNav  │  Main（操作区 / <router-view>）   │
│ 工作区   │                                  │
│  · 转换看板  ← 登录后默认                    │
│  · 上传                                     │
│  · 我的 PDF                                 │
│  · 历史                                     │
│ 管理（仅 admin，子菜单）                    │
│  · 用户管理                                 │
│  · 系统总览                                 │
└──────────┴──────────────────────────────────┘
```

- 工作区菜单扁平；管理区挂在「管理」父菜单下。
- 登录页全屏居中，不套壳。

### 3.2 路由

| 路径 | 页面 | 角色 |
|------|------|------|
| `/login` | 登录 | 公开 |
| `/board` | 转换看板（默认首页） | 用户 + 管理员 |
| `/upload` | 上传 | 用户 + 管理员 |
| `/pdfs` | 我的 PDF | 用户 + 管理员 |
| `/history` | 历史（上传归档 / pdflog Tab） | 用户 + 管理员 |
| `/admin/users` | 用户管理 | 管理员 |
| `/admin/overview` | 系统总览 | 管理员 |

- 未登录 → `/login`。
- 非管理员访问 `/admin/*` → 前端提示并回 `/board`；后端仍是最终权限裁决。

### 3.3 响应式

| 断点 | 行为 |
|------|------|
| ≥992px（PC） | 侧栏常驻展开（约 220px） |
| 768–991（Pad） | 侧栏默认展开，可手动折叠为图标宽 |
| &lt;768（Phone） | 侧栏隐藏；顶栏汉堡打开抽屉；点菜单后关闭 |

## 4. 页面职责与 API 映射

| 页面 | 用户操作 | API |
|------|----------|-----|
| 登录 | uid + pwd | `POST /api/auth/login`；进壳前 `GET /api/auth/verify` |
| 顶栏登出 | 清会话 | `POST /api/auth/logout` |
| 转换看板 | 待转队列；状态；删任务；完成后下 PDF | `GET /api/uploads`；活跃项 `GET /api/pdf/:fileid/status`；`DELETE /api/upload/:fileid`；`GET /api/pdf/:fileid/download` |
| 上传 | 拖拽/选择；进度；可选 watermark | `POST /api/upload`（multipart `file` + 可选 `watermark`） |
| 我的 PDF | 列表、下载、打开详情 | `GET /api/pdfs`；`GET /api/pdf/:fileid/download`；详情见 §5 |
| 历史 | Tab：上传归档 / 全局 pdflog | `GET /api/history/uploads`；`GET /api/history/pdflogs` |
| 用户管理 | CRUD、冻结、重置 Token | `/api/admin/users/*` |
| 系统总览 | 全站 uploads + pdfs；可开 PDF 详情 | `GET /api/admin/uploads`；`GET /api/admin/pdfs`；详情 pdflog 用 admin history |

### 4.1 状态语义

`pending` → `queued` → `converting` → 终态后离开 `/api/uploads`（成功/失败进 history）。  
看板以队列为主；轮询 status 收尾后提示「可下载 / 已失败」，并引导「我的 PDF」或「历史」。

## 5. PDF 详情与 pdflog

在「我的 PDF」或「系统总览」打开某条 PDF 时，使用 `PdfDetailDrawer`：

- 展示 PDF 元信息与下载入口。
- 展示该文件关联的 **pdflog** 时间线（`generate` / `download` / `delete` / `expire`）。
- 普通用户与管理员均可使用（各自权限范围内）。

### 5.1 后端依赖（UI 同期或前置）

现有 `GET /api/history/pdflogs` 仅分页（admin 可加 `uid`），无按文件过滤。需增补：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/history/pdflogs?fileid=<fileid>` | 当前用户、按 fileid 过滤 |
| GET | `/api/admin/history/pdflogs?fileid=<fileid>` | 管理员；可与现有 `uid` 并用 |

权限仍由后端裁决。历史页全局 pdflog Tab 保留；详情抽屉是按文件入口。

## 6. 认证、请求层、错误与轮询

### 6.1 认证

- 仅依赖 HttpOnly Cookie `access_token`；**不把 JWT 写入 localStorage / sessionStorage**。
- Axios `withCredentials: true`。
- 部署：Nginx 同站托管 UI，`/api` 反代后端；开发用 Vite proxy。
- 启动/刷新：`GET /api/auth/verify` 填充 Pinia（`uid`、`role`）；失败清状态并去 `/login`。
- 路由守卫按 role 隐藏「管理」；401/403 以后端为准。

### 6.2 统一响应与错误

- 解析 `{ code, message, data }`。
- 业务失败：`ElMessage`。
- 401 → 登出并跳登录；403（含冻结）→ 提示并禁止操作。

### 6.3 上传

- `el-upload` 拖拽；进度条；可选 watermark。
- 扩展名/大小前端预检 + 后端最终校验。

### 6.4 轮询

- 看板存在非终态任务时每 **3s** 刷新（`GET /api/uploads` 和/或活跃 `status`）。
- 全部终态或离开页面则停止。
- 后期可换 SSE；本期不实现。

### 6.5 下载

- 带 Cookie 凭证下载（blob 或同源导航）；触发后端 download 写 pdflog。

## 7. 目录与组件

```text
ui/
├── index.html
├── package.json
├── vite.config.ts
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── api/
│   ├── stores/          # auth；可选 app（侧栏）
│   ├── router/
│   ├── layouts/         # AppLayout
│   ├── views/           # Login, Board, Upload, Pdfs, History, admin/*
│   ├── components/      # StatusTag, UploadDropzone, PdfDetailDrawer, PagedTable
│   └── styles/
└── dist/
```

| 组件 | 职责 |
|------|------|
| `AppLayout` | 顶栏 + 侧栏/抽屉 + 主区；按 role 渲染「管理」 |
| `StatusTag` | 转换/PDF 状态徽章 |
| `UploadDropzone` | 拖拽、进度、watermark |
| `PdfDetailDrawer` | 元信息 + 按 fileid 的 pdflog + 下载 |
| `PagedTable` | 统一分页表格 |

## 8. 质量属性与验收

### 8.1 质量属性

- 首屏 &lt; 2s（生产构建 + 路由懒加载）。
- PC / Pad / Phone（&lt;768 抽屉侧栏）。
- 上传拖拽 + 进度反馈。
- 转换状态 3s 轮询。

### 8.2 手工验收清单

1. 登录成功写 Cookie；刷新后 `verify` 仍有效；登出清 Cookie。
2. 普通用户无「管理」；直接访问 `/admin/*` 被拦。
3. 上传 → 看板状态变化 → 完成后可下载 PDF。
4. 「我的 PDF」打开详情可见对应 pdflog；下载后再现 download 记录。
5. 历史页两 Tab 分页正常。
6. 管理员：用户管理 CRUD/冻结/重置 Token；系统总览列表与 PDF 详情 pdflog。
7. Phone 宽度下侧栏为抽屉，点菜单后关闭。

## 9. 与架构文档的关系

- 对齐 §9.2 技术选型、§9.3 模块、§9.4 独立部署优先、§9.5 Cookie 会话、§9.6 质量属性。
- §9.6「桌面/平板」扩展为明确含 Phone（抽屉方案）。
- 实现前若改配置键或公开 API，须同步 `usage.md` / 模板配置说明。
