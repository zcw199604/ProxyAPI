# 变更提案: Codex 上游请求严格模拟 Pi

## 需求背景
当前仓库与 `earendil-works/pi` 都通过 OpenAI OAuth 访问 ChatGPT Codex Responses 后端，但对 OpenAI 可观察到的客户端身份、OAuth 参数、账号标识派生、SSE 请求头、请求体默认值、压缩方式和 WebSocket 会话语义并不完全一致。用户要求本地仓库发往 OpenAI 的上游请求完全模拟 Pi。

本方案以 Pi commit `853a80d26c90a14c1886f0ebb8ffaae133ca2185` 为冻结参考版本。所谓“完全模拟”是指：对于使用 ChatGPT OAuth 凭据并访问标准 `https://chatgpt.com/backend-api/codex` 后端的请求，OpenAI 可观察到的 OAuth 授权参数、URL、请求头、JSON 负载、SSE zstd 压缩、WebSocket 握手与会话标识、自动传输回退语义与该版本 Pi 一致。下游 API 兼容层、账号池、调度、响应翻译和本地日志能力不要求与 Pi 相同。

## 变更内容
1. 引入受冻结 Pi commit 约束的 Codex 上游契约，统一 OAuth、身份头、负载默认值、SSE 和 WebSocket 行为。
2. ChatGPT OAuth 登录和刷新统一从 access token 提取 `chatgpt_account_id`，同时兼容既有 token 文件。
3. OAuth 标准后端请求固定使用 Pi 身份，不再让下游身份头覆盖 Pi 契约；自定义 Codex API key/Base URL 维持现状。
4. 使用契约快照和行为测试覆盖 OAuth、SSE、WebSocket、压缩、会话和回退路径。

## 范围边界
- **范围内:** `codex` OAuth 登录/刷新；标准 ChatGPT Codex `/responses` 的 HTTP/SSE 和 WebSocket 出站请求；请求体 Pi 默认值；相关配置、测试与知识库同步。
- **范围外:** OpenAI API key provider、`openai-compatibility` provider、Codex Realtime/Live、images、`responses/compact`、非标准 Codex Base URL、账号轮询算法、下游 API 协议和响应翻译语义。
- **拆分说明:** 本次只交付普通 Codex Responses 的 Pi parity。Realtime、images 和 compact 使用 Pi 主路径没有等价契约，继续保持本仓库现状。

## 影响范围
- **模块:** `internal/auth/codex`、`internal/runtime/executor`、`internal/config`、HelloAGENTS 知识库。
- **文件:** OAuth 实现、Codex HTTP/WebSocket 请求构造、配置类型与示例、对应测试。
- **API:** 下游 `/v1/chat/completions`、`/v1/responses` 等路径保持不变；仅改变满足 Pi parity 条件的上游请求。
- **数据:** 既有 Codex token 文件结构保持兼容；刷新后 `account_id` 改为以 access token claim 为权威来源。

## 核心场景

### 需求: OAuth 登录与凭据派生对齐
**模块:** `internal/auth/codex`

#### 场景: 浏览器 OAuth 登录
使用 ChatGPT OAuth 登录时：
- 授权 URL 的 scope、PKCE、redirect URI、组织参数、simplified flow 和 `originator=pi` 与冻结 Pi 版本一致。
- 不再发送 Pi 未发送的 `prompt=login`。
- account ID 从 access token 的 `https://api.openai.com/auth.chatgpt_account_id` 提取；缺失时登录失败，不持久化不完整凭据。

#### 场景: Token 刷新
access token 过期并刷新时：
- token 请求字段与 Pi 一致。
- 新 access/refresh/expiry/account ID 原子更新。
- 并发刷新仍由本仓库现有 singleflight/存储机制保护。

### 需求: SSE 上游请求对齐
**模块:** `internal/runtime/executor`

#### 场景: OAuth 标准后端请求
使用 OAuth 凭据访问默认 ChatGPT Codex 后端时：
- URL 为 `https://chatgpt.com/backend-api/codex/responses`。
- 强制发送 Pi 的 Authorization、account ID、`originator: pi`、动态 `pi (<goos> <release>; <arch>)` User-Agent、`OpenAI-Beta: responses=experimental`、Accept 和 Content-Type。
- 有 session ID 时，`session-id` 与 `x-client-request-id` 相同；无 session ID 时两者均不注入。
- JSON 请求体满足 Pi 默认契约，并优先使用 zstd level 3 压缩；压缩不可用时发送未压缩 JSON。

### 需求: WebSocket 上游请求对齐
**模块:** `internal/runtime/executor`

#### 场景: 自动传输与会话复用
Pi parity 请求使用 WebSocket 时：
- 使用 `wss://chatgpt.com/backend-api/codex/responses` 和 `OpenAI-Beta: responses_websockets=2026-02-06`。
- 不发送 SSE 专属 Accept/Content-Type/Beta 值。
- request ID、session ID、连接复用、空闲 5 分钟及最大 55 分钟生命周期与 Pi 一致。
- 建连前失败回退 SSE；已开始输出后失败不重放请求。

### 需求: 兼容性隔离
**模块:** `internal/config`、`internal/runtime/executor`

#### 场景: 自定义 Codex 上游
使用 `codex-api-key` 或自定义 Base URL 时：
- 不强制应用 Pi OAuth 身份和 ChatGPT 专用头。
- 保持现有 header override、代理、轮询和兼容行为。

## 风险评估
- **风险:** 修改 OAuth 字段可能使旧环境登录行为变化。**缓解:** 固定 Pi commit，使用 URL 参数契约测试并保留 device-code 现有入口。
- **风险:** 强制 Pi 身份会破坏依赖下游自定义 Originator/User-Agent 的用户。**缓解:** 仅对标准 ChatGPT OAuth 路径启用严格契约，自定义 API key/Base URL 不启用。
- **风险:** zstd 和 WebSocket 自动回退可能造成重复请求。**缓解:** 只允许建连且未产生事件时回退；首事件后禁止自动重放。
- **风险:** access token claim 缺失导致已有凭据不可用。**缓解:** 加载旧凭据时可暂用已有 `account_id`，但登录/刷新必须从新 access token校验并更新。
- **风险:** Pi 后续版本漂移。**缓解:** 契约中记录 commit；升级必须单独审查并更新快照。
