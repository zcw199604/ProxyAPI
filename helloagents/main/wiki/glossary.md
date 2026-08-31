# 领域语言

| 术语 | 定义 | 同义词/禁用叫法 | 适用模块 | 状态 | 来源 |
|---|---|---|---|---|---|
| Provider | 上游模型服务或协议实现 | 禁用: 随意使用 vendor 表示内部 provider | 全局 | ✅已确认 | internal/runtime, internal/translator |
| Credential | 可用于访问 provider 的 API key 或 OAuth 凭据 | 凭据 | auth, runtime | ✅已确认 | internal/auth, internal/store |
| ThinkingConfig | 统一的思考/推理配置 | reasoning config | thinking, translator | ✅已确认 | internal/thinking |
| Executor | 执行 provider 请求并处理流式生命周期的运行时组件 | 执行器 | runtime | ✅已确认 | internal/runtime/executor |
| Model Registry | 模型元数据和可用模型索引 | 模型注册表 | registry, sdk | ✅已确认 | internal/registry |
| Cooldown | 凭据或路由目标在暂时性错误后的冷却状态 | 冷却 | config, runtime | ✅已确认 | internal/config, sdk/cliproxy |
| Provider attempt | 一次实际发往上游凭据的请求尝试；重试会产生新的事件 | client request（禁用作统计单位） | usage, runtime | ✅已确认 | sdk/cliproxy/usage, internal/usage |
| AuthID | 凭据跨重启稳定的内部标识，用于统计归属 | AuthIndex（仅展示标识） | auth, usage | ✅已确认 | sdk/cliproxy/auth, internal/usage |
| TokenBreakdown | 不重叠的输入/输出 token 规范，含 cache 与 reasoning 分桶 | 原始 provider usage | usage, runtime | ✅已确认 | sdk/cliproxy/usage |
| priced / unpriced | 事件是否能根据价格快照完整计算金额 | 未计价 | usage, management | ✅已确认 | internal/usage |
| nano-USD | 金额内部整数单位，1 USD = 1,000,000,000 nano-USD | 浮点金额（禁用作累计值） | usage, pricing | ✅已确认 | internal/usage/accounting.go |

## 维护规则
- 新领域术语先查本表；代码事实与本表冲突时以代码为准。
- 未确认术语标记为“待确认”，不得强制用于代码命名。
