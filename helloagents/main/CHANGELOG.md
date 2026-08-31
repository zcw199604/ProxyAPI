# Changelog

本文件记录项目所有重要变更。

## [Unreleased]

- Codex OAuth requests to the default ChatGPT Responses backend now use the upstream contract frozen from `earendil-works/pi@853a80d26c90a14c1886f0ebb8ffaae133ca2185`, including OAuth parameters, access-token account identity, payload defaults, Pi headers, SSE zstd compression, and WebSocket preference. Set `codex.disable-pi-upstream-parity: true` to restore the legacy profile.
