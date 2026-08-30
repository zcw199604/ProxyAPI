# Config 与 Watcher

## 目的
加载 YAML 配置、解析环境变量，并在运行时应用配置变化。

## 模块概述
- **职责:** 配置模型、默认值、热加载、差异计算、凭据/模型变更事件。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 依赖
- `internal/watcher`
- `internal/registry`
- `internal/auth`

