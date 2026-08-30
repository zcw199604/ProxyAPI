# Runtime 执行器

## 目的
为各 provider 执行 HTTP、流式和 WebSocket 请求并维护生命周期。

## 模块概述
- **职责:** 凭据绑定、请求执行、流事件、重试/冷却和 Codex WebSocket。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 依赖
- `internal/auth`
- `internal/registry`
- `internal/translator`
- `internal/wsrelay`

