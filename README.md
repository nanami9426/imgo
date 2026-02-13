# LLMXpress

**Overview**
LLMXpress 是一个基于 Go + Gin 的后端服务，集成了用户系统、JWT 鉴权、WebSocket 私聊、OpenAI 兼容的 vLLM 代理，以及 API 用量统计。服务默认监听端口 `:5000`。

**Features**
- 用户管理：注册、登录、更新、删除、token 校验。
- JWT 鉴权：支持 `Authorization: Bearer <token>` 或 `token` 查询参数。
- vLLM 代理：提供 OpenAI 兼容的 `/v1` 接口，并将请求转发到上游。
- 用量统计：记录 API 调用、Token 数、响应延迟与状态码。
- WebSocket 私聊：基于 Redis Pub/Sub 的实时消息。
- Swagger 文档：内置 Swagger UI。

**Architecture**
- HTTP 框架：Gin
- 数据库：MySQL（GORM）
- 缓存/消息：Redis（Pub/Sub + Token 版本控制）
- 上游推理服务：vLLM（默认 `http://127.0.0.1:8000`，在 `internal/service/vllm_service.go` 中可调整）

**Quick Start**
1. 准备依赖：Go（见 `go.mod`）、MySQL、Redis、可选 vLLM。
2. 配置文件：编辑 `config/app.yaml`（可参考 `config/example.yaml`），确保包含 `login_device_max.n`。
3. 初始化数据库表（可选但建议，服务本身不自动迁移）：

```bash
go run ./test/test_gorm.go
```

4. 启动服务：

```bash
go run ./main.go
```

5. 健康检查：

```bash
curl http://localhost:5000/healthz
```

**Configuration**
`config/app.yaml` 必须存在，否则启动会失败。字段说明（`example.yaml` 中缺失的字段需自行补齐）：
- `mysql.host` / `mysql.port` / `mysql.user` / `mysql.password` / `mysql.db`
- `redis.host` / `redis.port` / `redis.password` / `redis.db`
- `jwt.secret` / `jwt.ttl_h`
- `ws.public_channel`
- `token_version_max.n`
- `login_device_max.n`

限流配置（`config/app.yaml`，针对 `POST /v1/chat/completions`）：
- `rate_limit.request_per_min`：请求级配额（默认 `0`，`<=0` 表示关闭）
- `rate_limit.token_per_min`：token 级配额（默认 `0`，`<=0` 表示关闭）
- `rate_limit.token_k`：token 成本缩放系数 `K`（默认 `100`，`K>=1`）
- `rate_limit.default_max_tokens`：未传 `max_tokens` 时的默认值
- `rate_limit.window_seconds`：固定窗口秒数
- `rate_limit.redis_prefix`：Redis key 前缀（默认 `rl:chat`）

计费规则：
- 请求级：每次请求成本固定为 `1`
- token 级：`cost = ceil((prompt_tokens_est + max_tokens) / K)`，其中 `prompt_tokens_est` 按本次请求 `messages` 文本字节估算（`ceil(bytes/4)`，不包含会话历史拼接）

request 级计算示例：
- 假设配置：`request_per_min=2`
- 每次 `POST /v1/chat/completions` 固定消耗 `1`
- 结果：同一用户在该 60 秒窗口内前 `2` 次可通过，第 `3` 次会触发 `429` 且 `dimension=request`。

token 级计算示例：
- 假设配置：`token_per_min=12`、`token_k=100`
- 某次请求：`prompt_tokens_est=180`、`max_tokens=220`
- 则：`cost = ceil((180+220)/100) = ceil(4.0) = 4`
- 结果：该请求会扣 `4` 个 token 配额单位；同一用户在该 60 秒窗口内最多可通过 `3` 次同等请求（第 `4` 次会触发 `429` 且 `dimension=token`）。
- 非整除示例：若 `prompt_tokens_est=181` 且 `max_tokens=220`，则 `cost = ceil(401/100) = 5`。

注意事项：
- `config/app.yaml` 中应使用自己的实际配置与密钥，不要提交真实凭据。
- CORS 白名单在 `internal/utils/sys_init.go` 的 `AllowedOrigins` 中维护。

**API**
基础路由：
- `GET /healthz`
- `GET /swagger/index.html`

用户模块：
- `POST /user/user_list`
- `POST /user/create_user`
- `POST /user/del_user`
- `POST /user/update_user`
- `POST /user/user_login`
- `POST /user/check_token`

用量统计：
- `POST /usage/stats`
- `POST /usage/total`

WebSocket 私聊：
- `GET /chat/send_message`（会升级为 WebSocket）

vLLM 代理（需要鉴权）：
- `POST /v1/chat/completions`
- `GET /v1/conversations`
- `GET /v1/conversations/:conversation_id/messages`
- `DELETE /v1/conversations/:conversation_id`
- `ANY /v1/:path`
- `ANY /v1/:path/*any`

会话续聊扩展（网关自定义字段）：
- 在 `POST /v1/chat/completions` 的 JSON body 中可选传：
  - `conversation_id`：指定历史会话续聊
  - `new_chat`：`true` 时强制新建会话
- 响应头会返回 `X-Conversation-ID`（前端可用于后续续聊）

**WebSocket**
- 连接方式（优先级）：`Sec-WebSocket-Protocol: authorization.bearer.<JWT>` 或 `authorization.bearer.b64.<base64url(JWT)>`，其次 `GET /chat/send_message?token=<JWT>`，最后 `Authorization: Bearer <JWT>`。
- 使用 `Sec-WebSocket-Protocol` 传 token 时，服务端会在握手响应中回写选中的子协议。
- 客户端发送：

```json
{"to_id":1,"content":"hi"}
```

- 服务端响应：

```json
{"message_id":123,"from_id":1,"to_id":2,"content":"hi","timestamp":"2024-01-01T00:00:00Z"}
```

**Usage Logging**
- `/v1` 相关请求会通过 `APILoggingMiddleware` 写入 `api_usage` 表。
- 支持从 OpenAI 风格 JSON 或 SSE 流中解析 Token 使用情况。
- `/v1/chat/completions` 请求会自动写入会话历史（`llm_conversation`、`llm_conversation_message`）。

**Swagger**
默认已集成 Swagger UI：`/swagger/index.html`。如果需要重新生成文档：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g main.go -o docs
```

**Development Notes**
- 生成表结构脚本依赖 `.env` 中的 MySQL 配置（参见 `test/test_gorm.go`）。
- vLLM 代理上游地址目前为硬编码，如需配置化可考虑引入环境变量或配置项。
