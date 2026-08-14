# MSOffice2Pdf 使用说明（中文）

完整安装、CLI、API 英文版见 [usage_en.md](./usage_en.md)。本文补充 **转换状态 SSE** 的客户端用法与服务端运行原理（与英文 §5.6 对齐）。

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
