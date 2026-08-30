# Thinking 管线

## 目的
将请求中的 reasoning/thinking 后缀和 body 配置规范化为 canonical `ThinkingConfig`，再转换为 provider 参数。

## 模块概述
- **职责:** suffix 解析、配置合并、集中校验和 provider applier 输出。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 规范
- `ApplyThinking()` 负责统一入口；suffix 覆盖 body 配置。
- 保持“规范表示 -> provider 翻译”的边界，不在各 provider 重复定义规则。

## 依赖
- `internal/translator`
- `internal/runtime/executor`

