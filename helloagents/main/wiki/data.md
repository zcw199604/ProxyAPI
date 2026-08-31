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
记录一次 provider attempt 的 EventID、账号稳定 ID、provider/model、成功/失败状态和 canonical TokenBreakdown；可进入 SDK 插件、文件或 Redis 队列。

### Account usage event
`internal/usage.Event` 是持久化统计事件。事件不保存 API key、OAuth token、prompt 或完整响应，只保存账号展示快照、价格快照、priced/unpriced 状态及 nano-USD 成本。SQLite `usage_events` 为事实表，`usage_daily` 为可重建的 UTC 日聚合表，`usage_price_overrides` 保存账号级价格规则。管理接口基于事实表的精确时间戳计算滚动窗口：最近 5 小时、最近 7 天和账号导入后累计；5h/7d 是否输出由当前账号额度信号中的明确窗口决定。累计窗口查询账号全部保留事件，不受运行时 `CreatedAt` 重建或 7d 周期切换影响；`usage-stats-retention-days: 0` 时永久保留。

### TokenBreakdown
输入 uncached/cache-read/cache-write 与输出（含 reasoning）互斥归一化，reasoning 已包含在 output 中，不重复计费。无法确认协议语义时保留为 unclassified。
