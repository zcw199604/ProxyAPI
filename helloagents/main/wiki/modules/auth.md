# Auth 与凭据

## 目的
管理 API key、OAuth 登录、令牌刷新和 provider 凭据选择。

## 模块概述
- **职责:** provider-specific OAuth、凭据持久化、刷新注册表、并发限制和安全校验。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## Codex OAuth Pi 契约
- 默认 ChatGPT Codex OAuth 浏览器登录冻结对齐 `earendil-works/pi@853a80d26c90a14c1886f0ebb8ffaae133ca2185`。
- 授权 URL 使用 `originator=pi`、Pi scope 与 PKCE S256，不发送 `prompt=login`。
- 登录和刷新均只从 access token 的 `https://api.openai.com/auth.chatgpt_account_id` 派生账号 ID；缺失时失败，不回退 ID token。
- refresh form 仅发送 `grant_type`、`refresh_token` 和 `client_id`。

## 依赖
- `internal/store`
- `internal/config`
- `sdk/auth`
