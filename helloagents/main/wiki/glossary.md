# 领域语言

| 术语 | 定义 | 同义词/禁用叫法 | 适用模块 | 状态 | 来源 |
|---|---|---|---|---|---|
| Provider | 上游模型服务或协议实现 | 禁用: 随意使用 vendor 表示内部 provider | 全局 | ✅已确认 | internal/runtime, internal/translator |
| Credential | 可用于访问 provider 的 API key 或 OAuth 凭据 | 凭据 | auth, runtime | ✅已确认 | internal/auth, internal/store |
| ThinkingConfig | 统一的思考/推理配置 | reasoning config | thinking, translator | ✅已确认 | internal/thinking |
| Executor | 执行 provider 请求并处理流式生命周期的运行时组件 | 执行器 | runtime | ✅已确认 | internal/runtime/executor |
| Model Registry | 模型元数据和可用模型索引 | 模型注册表 | registry, sdk | ✅已确认 | internal/registry |
| Cooldown | 凭据或路由目标在暂时性错误后的冷却状态 | 冷却 | config, runtime | ✅已确认 | internal/config, sdk/cliproxy |

## 维护规则
- 新领域术语先查本表；代码事实与本表冲突时以代码为准。
- 未确认术语标记为“待确认”，不得强制用于代码命名。

