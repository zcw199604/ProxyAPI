# SDK

## 目的
提供可嵌入的服务入口、配置 builder、插件、访问控制、执行注册表和使用量统计。

## 模块概述
- **职责:** `sdk/cliproxy` 服务生命周期和 provider 注册，`sdk/translator` 插件式翻译，`sdk/auth` 与 `sdk/access` 公共接口。
- **状态:** ✅稳定
- **最后更新:** 2026-08-31

## 依赖
- `internal/*` 对应实现模块

## 账号使用量统计

`sdk/cliproxy/usage` 的 `Manager` 为每个 provider attempt 补齐 `EventID`，并把记录分发给 `internal/usage.StatsPlugin`。服务启用 `usage-statistics-enabled` 后，StatsPlugin 将事件写入 SQLite，按 AuthID、provider、model 和 UTC 日聚合请求数、成功/失败数、Token 及 nano-USD 金额。统计写入不依赖请求 context，因此客户端断开不会丢失已观测的上游尝试。

价格目录由 `internal/usage/pricing.SyncService` 读取 sub2api/LiteLLM JSON，可缓存、ETag 增量同步并保留最后有效版本；账号覆盖按“精确模型 > 前缀通配 > 账号通配 > 全局目录”解析。

设计与执行记录：`helloagents/main/history/2026-08/202608311643_account-usage-billing/`。
