# Runtime 执行器

## 目的
为各 provider 执行 HTTP、流式和 WebSocket 请求并维护生命周期。

## 模块概述
- **职责:** 凭据绑定、请求执行、流事件、重试/冷却和 Codex WebSocket。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## Codex Pi 上游 Profile
- OAuth + 默认 `https://chatgpt.com/backend-api/codex/responses` 默认启用严格 Pi profile；API key、自定义 Base URL、compact、images 与 realtime 保持 legacy。
- Pi profile 在 translator/thinking 后统一请求体，并最终覆盖 Pi-owned Header；SSE 使用 zstd level 3，WebSocket 使用 Pi Beta、同值 request/session ID、5 分钟 idle 和 55 分钟 max age。
- `codex.disable-pi-upstream-parity=true` 可将符合条件的请求回滚到 legacy profile。

## 依赖
- `internal/auth`
- `internal/registry`
- `internal/translator`
- `internal/wsrelay`
