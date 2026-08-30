# 数据模型

## 概述
运行时配置来自 YAML，认证材料位于 `auths/`（可通过配置更改），请求使用内部结构在翻译器和执行器之间传递。

## 主要模型
### Config
包含 host/port、TLS、日志、重试、冷却、路由、插件、provider keys、OAuth alias/excluded models、payload rules 和存储相关设置。

### Credential/Auth bundle
表示 provider 的 API key 或 OAuth token、刷新信息、过期时间、账号标识及运行时状态。

### ThinkingConfig
思考级别、预算和开关的规范化表示，由 `internal/thinking` 校验后转换为 provider 参数。

### Usage record
记录请求、输入/输出 token、provider/model 和使用量统计，可进入文件或 Redis 队列。

