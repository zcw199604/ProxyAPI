# Pi-Compatible ChatGPT Codex OAuth and Upstream Requests

This document describes the reproducible upstream contract implemented by this repository for ChatGPT Codex OAuth credentials. The reference is `earendil-works/pi` commit `853a80d26c90a14c1886f0ebb8ffaae133ca2185`.

## Scope

Pi parity is enabled by default only when all of the following are true:

- The selected provider is `codex`.
- The credential is explicitly classified as OAuth.
- The upstream base URL is empty or exactly `https://chatgpt.com/backend-api/codex`, ignoring a trailing slash.
- The operation is the ordinary `/responses` operation.

API-key credentials, custom base URLs, `/responses/compact`, images, and realtime/live operations retain the legacy CLIProxyAPI behavior. Set the following emergency rollback option to restore the legacy request profile for otherwise eligible OAuth requests:

```yaml
codex:
  disable-pi-upstream-parity: true
```

## OAuth Login Contract

Pi and this implementation use these fixed endpoints and identifiers:

```text
client_id: app_EMoamEEZ73f0CkXaXp7hrann
authorize endpoint: https://auth.openai.com/oauth/authorize
token endpoint: https://auth.openai.com/oauth/token
redirect URI: http://localhost:1455/auth/callback
scope: openid profile email offline_access
```

The browser authorization request uses PKCE S256 and includes:

```text
response_type=code
client_id=<client ID>
redirect_uri=http://localhost:1455/auth/callback
scope=openid profile email offline_access
code_challenge=<PKCE challenge>
code_challenge_method=S256
state=<random state>
id_token_add_organizations=true
codex_cli_simplified_flow=true
originator=pi
```

It does not send `prompt=login`.

The authorization-code token request is form encoded and contains only the expected OAuth exchange fields: `grant_type`, `client_id`, `code`, `redirect_uri`, and `code_verifier`. A refresh request contains exactly:

```text
grant_type=refresh_token
refresh_token=<refresh token>
client_id=app_EMoamEEZ73f0CkXaXp7hrann
```

Both exchange and refresh responses must contain a non-empty `access_token`, a non-empty `refresh_token`, and a positive numeric `expires_in`. The account identifier is derived only from this access-token claim:

```json
{
  "https://api.openai.com/auth": {
    "chatgpt_account_id": "account identifier"
  }
}
```

An invalid access-token JWT or a missing/empty `chatgpt_account_id` fails authentication. The ID token is never used as an account-ID fallback; it may still supply the email stored in the repository's existing credential schema.

## Common Request Payload

Existing downstream translators first produce a Codex Responses payload. Immediately before sending an eligible request, the Pi contract normalizes Pi-owned fields:

```json
{
  "model": "<resolved upstream model>",
  "store": false,
  "stream": true,
  "instructions": "You are a helpful assistant.",
  "input": [],
  "text": { "verbosity": "low" },
  "include": ["reasoning.encrypted_content"],
  "tool_choice": "auto",
  "parallel_tool_calls": true
}
```

An explicit non-empty instruction, text verbosity, or tool choice is retained because Pi exposes those values as request options. The translated `input`, `tools`, `reasoning`, `temperature`, and `service_tier` fields are retained. The following non-Pi fields are removed from an ordinary request:

- `previous_response_id`
- `generate`
- `prompt_cache_retention`
- `safety_identifier`
- `stream_options`

When a session ID is present, it is Unicode-clamped to 64 code points and sent as `prompt_cache_key`. A caller-supplied `prompt_cache_key` has priority, followed by the execution or derived session identity. No random prompt-cache key is generated when there is no Pi session.

## SSE Transport

The endpoint is:

```text
https://chatgpt.com/backend-api/codex/responses
```

The final Pi-owned headers are applied after additional credential, downstream, and model headers, so they cannot be overridden:

```http
Authorization: Bearer <access token>
chatgpt-account-id: <account ID from access token>
originator: pi
User-Agent: pi (<Node-style platform> <OS release>; <Node-style architecture>)
OpenAI-Beta: responses=experimental
Accept: text/event-stream
Content-Type: application/json
```

For example, Go `windows/amd64` is mapped to Node's `win32/x64`. Windows release values come from `RtlGetVersion`; Unix release values come from `uname`, matching Node's `os.release()` source closely without launching an external process.

When a session exists, both headers contain the same clamped value:

```http
session-id: <session ID>
x-client-request-id: <session ID>
```

The UTF-8 JSON body is zstd-compressed at level 3 and sent with `Content-Encoding: zstd`. If encoder creation or compression fails, the implementation safely sends the original JSON without `Content-Encoding`. No token or full request payload is written to a new diagnostic log by this fallback.

## WebSocket Transport

Pi-parity requests prefer WebSocket even when the downstream connection is HTTP/SSE. The endpoint is:

```text
wss://chatgpt.com/backend-api/codex/responses
```

The common identity headers are the same as the SSE path, while transport-specific headers are:

```http
OpenAI-Beta: responses_websockets=2026-02-06
x-client-request-id: <session ID or UUIDv7>
session-id: <same value>
```

`Accept` and `Content-Type` are removed from the WebSocket handshake. The JSON frame is uncompressed and adds `"type":"response.create"`, matching the Responses WebSocket protocol.

Reusable WebSocket connections are isolated by execution session and selected credential target. They have a five-minute liveness/idle deadline and a maximum age of 55 minutes. A failed WebSocket handshake before any response event can fall back to the SSE implementation; failures after streaming starts are not replayed, preventing duplicate tool execution or billing.

## Reproduction Checks

No live OpenAI credential is required. Run the local contract and compatibility tests:

```bash
go test ./internal/auth/codex ./internal/runtime/executor ./internal/config -count=1
```

The tests use synthetic JWTs and local HTTP/WebSocket servers. They verify OAuth parameters, access-token claim extraction, profile isolation, Pi-owned header precedence, payload normalization, zstd round-trip behavior, WebSocket header differences, and legacy API-key/custom-endpoint behavior.

To verify the build after changes:

```bash
gofmt -w <changed Go files>
go test ./...
go build -o test-output ./cmd/server
```

Remove `test-output` after the compile check.
