# 项目技术约定

## 技术栈
- **核心:** Go 1.26.0
- **HTTP:** Gin 1.10.1
- **协议与网络:** WebSocket、WebRTC、OAuth2
- **存储:** 文件存储；可选 PostgreSQL、Git、S3/对象存储、Redis

## 开发约定
- 使用 `gofmt`，保持 KISS，错误使用 `%w` 包装上下文。
- 日志使用 logrus，禁止在日志中泄露密钥、令牌和敏感信息。
- HTTP handler 避免 panic，返回有意义的状态码。
- 超时仅用于凭据获取及代码中明确列出的 WebSocket/relay/管理 API 例外。

## 错误与日志
- 通过错误返回传播失败，由边界层映射为 HTTP 状态码。
- 使用结构化日志；不使用 `log.Fatal`/`log.Fatalf`。

## 测试与流程
- 单元测试与集成测试使用 Go testing；变更后运行 `go test ./...`。
- 编译门禁：`go build -o test-output ./cmd/server`。

