# Translator 翻译器

## 目的
在 OpenAI、Gemini、Claude、Codex 等协议与内部请求模型之间转换。

## 模块概述
- **职责:** 请求/响应格式转换、工具调用、流式事件和 thinking 参数映射。
- **状态:** ✅稳定
- **最后更新:** 2026-08-30

## 规范
- 翻译器消费 canonical thinking 表示，不绕过 `internal/thinking` 自行解析 suffix。

