# Registry 与路由

## 目的
维护模型元数据、远程模型更新和可用目标选择。

Codex Plus、Pro、Team 的内置目录包含 `gpt-6-astra`，按 Pi v0.85.1 定义使用 272000 上下文、128000 最大输出，以及 low/medium/high/xhigh/max 推理等级。远程模型更新仍可覆盖内置元数据。

## 模块概述
- **职责:** 模型注册、别名/排除规则、权重和 round-robin 路由。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 依赖
- `internal/config`
- `internal/runtime/executor`
- `sdk/cliproxy`
