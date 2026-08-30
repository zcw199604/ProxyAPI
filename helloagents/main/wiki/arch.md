# 架构设计

## 总体架构
```mermaid
flowchart TD
    Client[OpenAI/Gemini/Claude/Codex Client] --> API[Gin API Gateway]
    API --> Auth[Auth and Credential Manager]
    API --> Translate[Protocol Translators]
    Translate --> Think[Canonical Thinking Pipeline]
    Think --> Exec[Provider Runtime Executors]
    Exec --> Upstream[Provider APIs/WebSockets]
    Config[Config and Watchers] --> Auth
    Config --> Registry[Model Registry and Routing]
    Registry --> Exec
    Store[File/PG/Git/Object Store] --> Auth
```

## 技术栈
- **后端:** Go 1.26, Gin
- **传输:** HTTP(S), WebSocket, WebRTC
- **数据:** YAML 配置、文件凭据；可选 PostgreSQL、Git、对象存储和 Redis

## 核心流程
```mermaid
sequenceDiagram
    Client->>API: 兼容协议请求
    API->>Auth: 选择并校验凭据
    API->>Translate: 解析为内部请求
    Translate->>Think: 规范化 thinking 配置
    Think->>Exec: provider-specific 请求
    Exec->>Upstream: 上游调用
    Upstream-->>Exec: 流或响应
    Exec-->>API: 翻译为客户端协议
    API-->>Client: 响应/事件流
```

## 重大架构决策
| adr_id | title | date | status | affected_modules | details |
|---|---|---|---|---|---|
| - | 当前初始化未记录独立 ADR | 2026-08-30 | - | - | - |

