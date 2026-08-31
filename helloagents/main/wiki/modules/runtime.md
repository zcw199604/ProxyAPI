# Runtime 执行器

## 目的
为各 provider 执行 HTTP、流式和 WebSocket 请求并维护生命周期。

## 模块概述
- **职责:** 凭据绑定、请求执行、流事件、重试/冷却和 Codex WebSocket。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## Codex Pi 上游 Profile
- 所有 Codex 请求只使用严格 Pi profile；默认目标为 `https://chatgpt.com/backend-api/codex/responses`，自定义 Base URL 仅改变目标而不再选择另一套 profile。compact 与 direct images 的 legacy 出站路径已移除，无包含 ChatGPT account claim 的有效 access-token JWT 时请求在出站前失败。
- Pi profile 在 translator/thinking 后统一请求体，并最终覆盖 Pi-owned Header；SSE 使用 zstd level 3，WebSocket 使用 Pi Beta、同值 request/session ID、5 分钟 idle 和 55 分钟 max age。
- Pi WebSocket 会话按 execution session + access-token account ID 隔离，支持增量 continuation；建连前失败回退 SSE 后，该 session 保持 SSE 模式。

## 依赖
- `internal/auth`
- `internal/registry`
- `internal/translator`
- `internal/wsrelay`
