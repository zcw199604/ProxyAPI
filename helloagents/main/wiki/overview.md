# CLIProxyAPI

## 1. 项目概述

### 目标与背景
提供 OpenAI、Gemini、Claude、Codex 兼容的代理 API，将多种上游认证、模型和协议统一到一个可部署服务。

### 范围
- **范围内:** API 兼容层、OAuth/密钥认证、模型注册与路由、负载均衡、流式和实时连接、配置与凭据管理。
- **范围外:** 上游模型本身的推理和第三方账号生命周期管理。

## 2. 模块索引

| 模块名称 | 职责 | 状态 | 文档 |
|---|---|---|---|
| API 网关 | Gin 路由、认证、中间件和管理端点 | 稳定 | [api](api.md) |
| 思考管线 | 统一 reasoning/thinking 配置并转换到 provider | 稳定 | [thinking](modules/thinking.md) |
| 运行时执行器 | 按 provider 建立请求、流和 WebSocket 会话 | 稳定 | [runtime](modules/runtime.md) |
| 翻译器 | OpenAI/Gemini/Claude/Codex 协议互转 | 稳定 | [translator](modules/translator.md) |
| 认证与凭据 | OAuth、API key、刷新和凭据并发控制 | 稳定 | [auth](modules/auth.md) |
| 配置与热加载 | YAML 配置、watcher 和差异合并 | 稳定 | [config](modules/config.md) |
| 注册表与路由 | 模型注册、远程更新和轮询选择 | 稳定 | [registry](modules/registry.md) |
| 存储 | 文件及可选外部后端 | 稳定 | [store](modules/store.md) |
| SDK | 可嵌入服务、插件、访问控制和使用量 | 稳定 | [sdk](modules/sdk.md) |

## 3. 快速链接
- [技术约定](../project.md)
- [架构设计](arch.md)
- [API 手册](api.md)
- [数据模型](data.md)
- [领域语言](glossary.md)
- [变更历史](../history/index.md)

