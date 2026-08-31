# 任务清单: Codex 上游请求严格模拟 Pi

目录: `helloagents/main/plan/202608301923_pi_codex_upstream_parity/`

---

## 0. 方案边界确认
- [√] 0.1 确认实现只覆盖 OAuth 默认 ChatGPT `/responses`，排除 API key/custom Base URL、Realtime、images 和 compact
  - 执行模式: AFK
  - 涉及文件: `why.md`, `how.md`, `task.md`
  - 完成标准: 三个文件的范围内/范围外一致，无未解释的扩展项
  - 验证方式: 只读核对方案包
- [√] 0.2 将 Pi 基线 commit `853a80d26c90a14c1886f0ebb8ffaae133ca2185` 写入契约测试注释/fixture 元数据
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/openai_auth_test.go`, `internal/runtime/executor/codex_pi_contract_test.go`
  - 完成标准: 测试失败信息可以定位参考 commit，且 fixture 不包含真实凭据
  - 验证方式: `rg -n "853a80d26c90a14c1886f0ebb8ffaae133ca2185" internal/auth/codex internal/runtime/executor`

## 1. OAuth 契约（TDD）
- [√] 1.1 RED: 添加浏览器授权 URL 测试，断言 Pi scope、`originator=pi`、PKCE 参数存在，`prompt=login` 不存在
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/openai_auth_test.go`
  - 完成标准: 测试因当前 URL 参数与 Pi 不一致而失败
  - 验证方式: `go test ./internal/auth/codex -run TestCodexAuthURLMatchesPiContract -count=1`
- [√] 1.2 RED: 添加登录/刷新从 access token 提取 account ID、缺失 claim 失败以及刷新字段对齐测试
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/openai_auth_test.go`, `internal/auth/codex/token_test.go`
  - 完成标准: 测试因当前从 ID token 取 account ID或刷新多发 scope 而失败
  - 验证方式: `go test ./internal/auth/codex -run 'Test(CodexTokenAccountID|CodexRefreshMatchesPiContract)' -count=1`
- [√] 1.3 GREEN: 最小修改 OAuth URL、token 校验、account ID 派生和刷新字段，使任务1.1-1.2通过
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/openai_auth.go`, `internal/auth/codex/jwt_parser.go`
  - 完成标准: 登录/刷新与 Pi 契约一致，旧 token schema仍可读取
  - 验证方式: `go test ./internal/auth/codex -count=1`
- [√] 1.4 REFACTOR: 提取 access-token claim helper并复用，保持测试通过
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/jwt_parser.go`, `internal/auth/codex/openai_auth.go`
  - 完成标准: 登录和刷新不再重复解析逻辑，错误带上下文且不含 token
  - 验证方式: `go test ./internal/auth/codex -count=1`

## 2. Pi profile 与配置隔离（TDD）
- [√] 2.1 RED: 添加 Pi parity 启用判定与回滚开关测试
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract_test.go`, `internal/config/config_types_test.go`
  - 完成标准: OAuth默认后端且禁用开关为 false 时判定=true；API key/custom endpoint/禁用开关为 true 时判定=false；测试在实现前失败
  - 验证方式: `go test ./internal/runtime/executor ./internal/config -run 'TestPiCodex|TestCodexPiUpstreamParity' -count=1`
- [√] 2.2 GREEN: 增加 `codex.disable-pi-upstream-parity` 默认值和集中启用判定
  - 执行模式: AFK
  - 涉及文件: `internal/config/config_types.go`, `internal/runtime/executor/codex_pi_contract.go`
  - 完成标准: 判定只命中 OAuth默认 `/responses`，配置关闭恢复 legacy path
  - 验证方式: `go test ./internal/runtime/executor ./internal/config -run 'TestPiCodex|TestCodexPiUpstreamParity' -count=1`

## 3. SSE 请求契约（TDD）
- [√] 3.1 RED: 添加 Pi base/SSE header 契约测试，覆盖动态 UA、Originator、Beta、account ID、session 和下游覆盖阻断
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract_test.go`
  - 完成标准: 当前 `codex-tui` 身份和缺失 SSE Beta 导致测试失败
  - 验证方式: `go test ./internal/runtime/executor -run TestPiCodexSSEHeaders -count=1`
- [√] 3.2 RED: 添加 payload normalizer 测试，覆盖 Pi 默认值、session cache key、保留 input/tools/reasoning 与删除非 Pi 字段
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract_test.go`
  - 完成标准: 当前空 instructions、随机 cache key或字段差异导致测试失败
  - 验证方式: `go test ./internal/runtime/executor -run TestNormalizePiCodexPayload -count=1`
- [√] 3.3 RED: 添加 zstd level 3 出站集成测试，捕获并解压 body，断言解压 JSON 与 header 一致
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract_test.go`
  - 完成标准: 当前未压缩请求导致测试失败
  - 验证方式: `go test ./internal/runtime/executor -run TestPiCodexSSEZstdRequest -count=1`
- [√] 3.4 GREEN: 实现 Pi UA、header、payload 和 zstd helper
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract.go`, `internal/runtime/executor/helps/pi_user_agent.go`
  - 完成标准: 任务3.1-3.3通过，不记录凭据或完整 JWT
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodex|TestNormalizePiCodex' -count=1`
- [√] 3.5 GREEN: 在 Codex普通 SSE Execute/Stream路径接入 Pi contract，legacy/custom path不变
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_executor_execute.go`, `internal/runtime/executor/codex_executor_stream.go`
  - 完成标准: fake upstream 捕获到 Pi URL/headers/zstd/payload；API key/custom endpoint回归测试通过
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodexSSE|TestCodex.*Custom|TestCodex.*APIKey' -count=1`

## 4. WebSocket 契约与自动回退（TDD）
- [√] 4.1 RED: 添加 WebSocket header/session契约测试，覆盖 Pi Beta、无 SSE headers、request/session ID相同、账号隔离
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_websockets_executor_test.go`, `internal/runtime/executor/codex_pi_contract_test.go`
  - 完成标准: 当前 header/session语义与 Pi 不一致时测试失败
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodexWebSocket' -count=1`
- [√] 4.2 RED: 添加建连前失败回退 SSE、首事件后不回退、连接限制和 missing continuation 单次重试测试
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_websockets_executor_test.go`
  - 完成标准: 每个测试在目标策略未实现前以预期原因失败
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodexAutoTransport' -count=1`
- [√] 4.3 GREEN: 接入 Pi WebSocket header、UUID/session和连接生命周期策略
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_websockets_request.go`, `internal/runtime/executor/codex_websockets_session.go`
  - 完成标准: header/session/TTL测试通过且账号间不复用连接
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodexWebSocket' -count=1`
- [√] 4.4 GREEN: 实现 Pi auto transport 的安全回退与单次重试规则
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_websockets_execute.go`, `internal/runtime/executor/codex_websockets_errors.go`
  - 完成标准: 建连前可回退，首事件后不重放，重试次数受限
  - 验证方式: `go test ./internal/runtime/executor -run 'TestPiCodexAutoTransport' -count=1`
- [√] 4.5 REFACTOR: 合并 HTTP/WS共用 Pi身份和session helper，保持所有目标测试通过
  - 执行模式: AFK
  - 涉及文件: `internal/runtime/executor/codex_pi_contract.go`, `internal/runtime/executor/codex_websockets_request.go`
  - 完成标准: Pi-owned字段只有一个权威构造入口，无重复常量
  - 验证方式: `go test ./internal/runtime/executor -count=1`

## 5. 安全与兼容检查
- [√] 5.1 检查 token、JWT、account ID和 Authorization不会新增日志泄漏
  - 执行模式: AFK
  - 涉及文件: 本次所有生产与测试文件
  - 完成标准: 错误和日志不包含完整 token/JWT，测试 fixture均为伪造值
  - 验证方式: `rg -n "access_token|refresh_token|Authorization|JWT" internal/auth/codex internal/runtime/executor`
- [√] 5.2 验证 API key、自定义 Base URL、禁用开关和旧 token文件兼容
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex/*_test.go`, `internal/runtime/executor/codex*_test.go`
  - 完成标准: 四类兼容测试全部通过，旧配置无需迁移
  - 验证方式: `go test ./internal/auth/codex ./internal/runtime/executor ./internal/config -count=1`

## 6. 文档与知识库
- [√] 6.1 更新配置示例，说明 Pi parity启用范围、冻结 commit和回滚开关
  - 执行模式: AFK
  - 涉及文件: `config.example.yaml`, `docs/pi-codex-auth-replication.md`
  - 完成标准: 用户可判断何时启用、如何回滚、哪些 provider不受影响
  - 验证方式: 只读核对配置键、默认值和文档一致
- [√] 6.2 同步 Auth/Runtime知识库和 CHANGELOG
  - 执行模式: AFK
  - 涉及文件: `helloagents/main/wiki/modules/auth.md`, `helloagents/main/wiki/modules/runtime.md`, `helloagents/main/CHANGELOG.md`
  - 完成标准: 稳定架构事实、Pi基线和行为变化已记录
  - 验证方式: `rg -n "Pi|disable-pi-upstream-parity|853a80d" helloagents/main`

## 7. 最终验证
- [√] 7.1 VERIFY: 格式化所有本次修改的 Go文件
  - 执行模式: AFK
  - 涉及文件: 本次修改的 `.go` 文件
  - 完成标准: `gofmt` 后无格式差异
  - 验证方式: `gofmt -w <changed-go-files>` 后 `git diff --check`
- [X] 7.2 VERIFY: 运行受影响包测试和全量测试
  - 执行模式: AFK
  - 涉及文件: `internal/auth/codex`, `internal/runtime/executor`, `internal/config`
  - 完成标准: 目标包与 `go test ./...` 均通过
  - 验证方式: `go test ./internal/auth/codex ./internal/runtime/executor ./internal/config -count=1`; `go test ./...`
  > 备注: 受影响的 Auth/Config 与 Codex 定向 executor 测试通过；`go test ./...` 已执行，但被未触达的 `internal/home` 120 秒取消超时、Windows `internal/store` 临时 `.git` Access denied，以及 xAI TTFT 纳秒级时序断言阻断。详见 qa-review.json 的 P2 环境/既有测试发现。
- [√] 7.3 VERIFY: 执行强制编译门禁
  - 执行模式: AFK
  - 涉及文件: `cmd/server`及依赖
  - 完成标准: server成功编译，测试产物已删除
  - 验证方式: `go build -o test-output ./cmd/server`，成功后删除 `test-output`
- [√] 7.4 记录 TDD RED/GREEN/REFACTOR/VERIFY 和 QA 证据
  - 执行模式: AFK
  - 涉及文件: `helloagents/main/plan/202608301923_pi_codex_upstream_parity/qa-review.json`, `task.md`
  - 完成标准: schema v3记录目标行为、RED失败原因、GREEN/REFACTOR/VERIFY结果和剩余风险
  - 验证方式: 只读核对 QA 证据与实际命令输出一致

---

## 执行顺序与依赖
- 任务1 → 任务2 → 任务3 → 任务4 → 任务5-7。
- OAuth、SSE和WebSocket都涉及共享 Codex executor/契约文件，不标注并行子代理，避免同文件冲突和契约漂移。
- 所有行为变更采用 TDD；纯文档任务6为 TDD-EXEMPT，以只读一致性核对替代。
