# API 手册

## 概述
服务通过 Gin 暴露 OpenAI-compatible、Claude-compatible、Gemini-compatible、Codex 和实时 API。

## 认证方式
标准 API 使用配置的 API key/Bearer 认证；实时端点按路由使用 realtime 或 standard auth。OAuth 凭据由服务端管理，不应写入日志。

## 接口列表
### 健康检查
#### GET `/healthz`
返回 `{ "status": "ok" }`。

### OpenAI 兼容
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/images/generations`
- `POST /v1/responses`
- `POST /v1/responses/compact`

### Claude 兼容
- `POST /v1/messages`
- `POST /v1/messages/count_tokens`

### Gemini 兼容
- `GET /v1beta/models`
- `GET|POST /v1beta/models/*action`
- `POST /v1beta/interactions`

### 实时与 Codex
- `GET|POST /v1/realtime`
- `POST /v1/realtime/calls`
- `GET /v1/realtime/calls/:call_id`
- `POST /codex/responses`
- `GET /codex/responses`（WebSocket）

