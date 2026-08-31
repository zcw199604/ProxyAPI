# 任务清单: 账号请求、Token 与金额统计

执行状态：已完成（目标包验证通过；全量测试存在未触达模块的既有 Windows/cache timing 失败，详见 `qa-review.json`）。

目录: `helloagents/main/plan/202608311643_account-usage-billing/`

---

## 0. 方案边界确认

- [x] 0.1 确认本次实现只覆盖 `why.md` 的持久化统计、价格同步、账号覆盖和 Management API；不实现前端、用户扣费和共享数据库
  - 执行模式: AFK
  - 涉及文件: `why.md`, `how.md`, `task.md`
  - 完成标准: 三份方案文档对范围内/范围外描述一致
  - 验证方式: 只读核对方案包
- [x] 0.2 确认使用 SQLite 默认后端和 `StatsStore` 抽象，不把统计逻辑写入 executor/translator
  - 执行模式: AFK
  - 涉及文件: `how.md`
  - 完成标准: 存储、usage sink、价格服务、API 职责边界明确
  - 验证方式: 只读核对 how.md 设计边界和 ADR

---

## 1. Usage 事件契约与统计入口

- [x] 1.1 为 `sdk/cliproxy/usage.Record` 增加可选 `EventID`，在 `Manager.Publish` 为空时生成 UUID，保持既有插件兼容
  - 执行模式: AFK
  - 涉及文件: `sdk/cliproxy/usage/manager.go`, `sdk/cliproxy/usage/manager_test.go`
  - 完成标准: 所有进入 manager 的 record 都有稳定的单次 event_id；显式 EventID 不被覆盖；旧调用编译不变
  - 验证方式: `go test ./sdk/cliproxy/usage/...`
- [x] 1.2 创建 `internal/usage` 的 StatsPlugin、事件规范化和 `StatsStore` 接口，接收 success/failure/zero-token 记录并保证幂等
  - 执行模式: AFK
  - 涉及文件: `internal/usage/plugin.go`, `internal/usage/event.go`, `internal/usage/store.go`, 对应测试文件
  - 完成标准: 重复 event_id 不重复计数；缺少 Token 仍计 provider-attempt 请求；通过注入的只读 auth lookup 保存账号展示快照；缺少账号 ID 的记录按明确策略标记并可查询
  - 验证方式: `go test ./internal/usage/...`
- [x] 1.3 在 server 生命周期注册/启动/停止 StatsPlugin，接入现有 `usage-statistics-enabled` 配置并确保关闭时不写入
  - 执行模式: AFK
  - 涉及文件: `cmd/server/main.go`, `internal/api/server_reload.go`, `internal/config/config.go`, `config.example.yaml`
  - 完成标准: 配置关闭时无统计副作用；启动/停止能 drain 或报告失败；热重载不会重复注册 sink
  - 验证方式: `go test ./cmd/server ./internal/api/... ./internal/config/...`

## 2. 持久化事件与日聚合

- [x] 2.0 引入并锁定 `modernc.org/sqlite@v1.57.0`，核对纯 Go 构建、许可证和 Go 1.26 编译兼容性
  - 执行模式: AFK
  - 涉及文件: `go.mod`, `go.sum`
  - 完成标准: 依赖版本可重复下载，Windows/Linux 构建无需 CGO，许可证与项目分发要求相容
  - 验证方式: `go mod verify`; `go build -o cli-proxy-api ./cmd/server`
- [x] 2.1 实现 SQLite schema migration、事件表、日聚合表和单写事务，包含目录权限、busy timeout 和 schema version
  - 执行模式: AFK
  - 涉及文件: `internal/usage/sqlite_store.go`, `internal/usage/migrations.go`, 对应测试文件
  - 完成标准: 首次启动自动建表；重复迁移幂等；事件插入和日聚合递增同事务提交；唯一 event_id 可重放
  - 验证方式: `go test ./internal/usage/... -run 'Test.*(Migration|Append|Idempot|Aggregate)'`
- [x] 2.2 实现 token breakdown 归一化、nano-USD 整数金额计算和价格快照存储
  - 执行模式: AFK
  - 涉及文件: `internal/usage/accounting.go`, `internal/usage/accounting_test.go`
  - 完成标准: cache/reasoning 不重复计数；显式 0 价格为 priced；缺价为 unpriced；大 Token 数不发生 int64 溢出或静默负值
  - 验证方式: `go test ./internal/usage/... -run 'Test.*(Token|Cost|Price)'`
- [x] 2.3 增加日聚合查询和按事件重建能力，支持账号/provider/model/time 过滤和分页上限
  - 执行模式: AFK
  - 涉及文件: `internal/usage/query.go`, `internal/usage/rebuild.go`, 对应测试文件
  - 完成标准: 聚合查询结果与逐事件重算一致；时间边界使用 UTC 且不越界；超限请求返回明确错误
  - 验证方式: `go test ./internal/usage/... -run 'Test.*(Query|Rebuild|Range)'`

## 3. sub2api 价格目录同步

- [x] 3.1 实现 LiteLLM/sub2api JSON 解析、字段校验、模型规范化和内存只读目录
  - 执行模式: AFK
  - 涉及文件: `internal/usage/pricing/catalog.go`, `internal/usage/pricing/parser.go`, 对应测试文件
  - 完成标准: 解析目标价格字段并归一化为 nano-USD；负值/NaN/Inf 被拒绝；未知模型不会猜价；精确 alias 行为有测试
  - 验证方式: `go test ./internal/usage/pricing/...`
- [x] 3.2 实现启动加载、ETag/Last-Modified、SHA-256、原子替换、旧版本回退和定时同步
  - 执行模式: AFK
  - 涉及文件: `internal/usage/pricing/sync.go`, `internal/usage/pricing/persistence.go`, 对应测试文件
  - 完成标准: 网络失败或格式错误不会覆盖最后有效目录；同步状态包含 hash、时间、模型数和错误；停止时 goroutine 退出
  - 验证方式: `go test ./internal/usage/pricing/... -run 'Test.*(Sync|Fallback|ETag|Hash)'`
- [x] 3.3 增加价格相关配置和示例，默认 URL 指向 sub2api 文件，限制响应大小与请求超时
  - 执行模式: AFK
  - 涉及文件: `internal/config/config.go`, `internal/config/config_load.go`, `config.example.yaml`, `.env.example`（如需要）
  - 完成标准: 配置可关闭自动同步、覆盖 URL/间隔/数据目录；非法 URL/间隔在校验阶段被拒绝
  - 验证方式: `go test ./internal/config/...`

## 4. 账号价格覆盖

- [x] 4.1 实现 override schema、精确/通配/默认匹配和优先级解析，支持 nullable 价格与显式免费价格
  - 执行模式: AFK
  - 涉及文件: `internal/usage/pricing/overrides.go`, `internal/usage/pricing/overrides_test.go`, `internal/usage/sqlite_store.go`
  - 完成标准: 匹配顺序固定且有冲突测试；账号 A 的覆盖不影响账号 B；禁用规则立即不再命中新事件
  - 验证方式: `go test ./internal/usage/pricing/... -run 'Test.*Override'`
- [x] 4.2 实现 Management API 的账号价格 GET/PUT/DELETE，复用现有鉴权并隐藏凭证
  - 执行模式: AFK
  - 涉及文件: `internal/api/handlers/management/account_pricing.go`, `internal/api/server_management.go`, 对应测试文件
  - 完成标准: 支持 auth_id/auth_index；价格字符串校验、非负和精度限制生效；删除为软禁用；错误状态码稳定
  - 验证方式: `go test ./internal/api/handlers/management/... -run 'Test.*AccountPricing'`

## 5. 统计与价格 Management API

- [x] 5.1 实现 `/v0/management/account-stats` 汇总、按日/模型分组、分页和 priced/unpriced 字段
  - 执行模式: AFK
  - 涉及文件: `internal/api/handlers/management/account_stats.go`, `internal/api/server_management.go`, 对应测试文件
  - 完成标准: 单账号请求数、Token、金额与数据库聚合一致；时间范围和账号过滤正确；不返回 token/key/prompt
  - 验证方式: `go test ./internal/api/handlers/management/... -run 'Test.*AccountStats'`
- [x] 5.2 实现 `/v0/management/pricing/status`、`/models` 和 `POST /sync`
  - 执行模式: AFK
  - 涉及文件: `internal/api/handlers/management/pricing.go`, `internal/api/server_management.go`, 对应测试文件
  - 完成标准: 状态可查询；模型列表分页/搜索；手动同步不阻塞长时间重试并返回可诊断结果
  - 验证方式: `go test ./internal/api/handlers/management/... -run 'Test.*Pricing'`

## 6. 安全检查

- [x] 6.1 执行输入、权限、敏感信息、URL 和资源上限检查
  - 执行模式: AFK
  - 涉及文件: 本次所有 Go/config 文件
  - 完成标准: 未记录 API key/OAuth token/prompt；Management API 仍要求管理密钥；价格源响应和查询范围有上限；无危险命令或未确认 EHRB
  - 验证方式: `rg -n "api_key|access_token|refresh_token|prompt|Authorization" internal/usage internal/api/handlers/management` 并人工核对日志与权限

## 7. 知识库同步

- [x] 7.1 更新 SDK、数据模型、Management API 和 glossary 文档，链接本方案包
  - 执行模式: AFK
  - 涉及文件: `helloagents/main/wiki/modules/sdk.md`, `helloagents/main/wiki/data.md`, `helloagents/main/wiki/api.md`, `helloagents/main/wiki/glossary.md`
  - 完成标准: 文档描述与实现、API 字段、价格优先级和单实例边界一致
  - 验证方式: 只读核对文档与代码；执行项目文档审计（如有）

## 8. 测试与验收

### 8A. TDD 路径

- [x] 8A.1 RED: 为 event 幂等、金额公式、价格优先级、同步回退和 API 汇总添加失败测试
  - 执行模式: AFK
  - 涉及文件: `sdk/cliproxy/usage/*_test.go`, `internal/usage/**/*_test.go`, `internal/api/handlers/management/*_test.go`
  - 完成标准: 测试先失败且失败原因对应目标行为
  - 验证方式: 分模块执行 `go test` 并保留失败摘要
- [x] 8A.2 GREEN: 以最小实现让 RED 测试通过
  - 执行模式: AFK
  - 涉及文件: 对应生产文件
  - 完成标准: 核心测试通过，未扩大到前端/扣费/共享数据库
  - 验证方式: 同 8A.1 测试命令
- [x] 8A.3 REFACTOR: 整理 store、pricing 和 handler 边界，保持测试通过
  - 执行模式: AFK
  - 涉及文件: `internal/usage/**`, `internal/api/handlers/management/**`
  - 完成标准: 行为不变、无重复注册/竞态、错误有上下文
  - 验证方式: `go test ./internal/usage/... ./internal/api/handlers/management/...`
- [x] 8A.4 VERIFY: 执行全量测试与构建
  - 执行模式: AFK
  - 涉及文件: 本次所有改动
  - 完成标准: 全量测试和构建成功，结果摘要可复核
  - 验证方式: `gofmt -w .`; `go test ./...`; `go build -o cli-proxy-api ./cmd/server`

### 8B. TDD-EXEMPT 路径

- [x] 8B.1 TDD-EXEMPT: 仅在价格源文档或 schema 注释等无运行时行为的文字调整上使用；原因: 纯文档变更；替代验证: 文档审计和链接核对
  - 执行模式: AFK
  - 涉及文件: `helloagents/main/wiki/**`, `config.example.yaml` 注释
  - 完成标准: 例外原因和替代验证均记录，运行时代码仍走 8A 验证
  - 验证方式: 只读核对文档与配置
