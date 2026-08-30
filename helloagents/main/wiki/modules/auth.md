# Auth 与凭据

## 目的
管理 API key、OAuth 登录、令牌刷新和 provider 凭据选择。

## 模块概述
- **职责:** provider-specific OAuth、凭据持久化、刷新注册表、并发限制和安全校验。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 依赖
- `internal/store`
- `internal/config`
- `sdk/auth`

