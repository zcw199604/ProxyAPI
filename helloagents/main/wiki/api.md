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

### Management 统计与价格

管理鉴权沿用现有 Management key。启用统计后可调用：

- `GET /v0/management/account-stats`：按 `auth_id`、`provider`、`model`、`from`、`to`、`group_by`（`day`/`model`/`day_model`）过滤并分页，返回请求数、Token、金额及 priced/unpriced 计数。
- `GET /v0/management/auth-files`：每个认证账号条目在启用统计后增加 `usage_stats`，可直接在认证文件/配额列表展示累计请求数、Token、金额和 priced/unpriced 状态。
- `GET|PUT /v0/management/account-pricing`、`PUT|DELETE /v0/management/account-pricing/:id`：读取、写入或软禁用账号级模型价格覆盖；价格使用 USD 字符串，最多 9 位小数，显式 `0` 表示免费。
- `GET /v0/management/pricing/status`、`GET /v0/management/pricing/models`、`POST /v0/management/pricing/sync`：查询价格目录同步状态、搜索模型或手动同步 sub2api 目录。
