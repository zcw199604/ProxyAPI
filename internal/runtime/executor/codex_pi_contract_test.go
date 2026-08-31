package executor

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
)

const piCodexReferenceCommit = "853a80d26c90a14c1886f0ebb8ffaae133ca2185"

func piCodexTestAccessToken() string {
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"acct-pi"}}`)) + ".signature"
}

func piOAuthTestAuth() *cliproxyauth.Auth {
	accessToken := piCodexTestAccessToken()
	return &cliproxyauth.Auth{
		Provider:   "codex",
		Attributes: map[string]string{cliproxyauth.AttributeAuthKind: cliproxyauth.AuthKindOAuth},
		Metadata:   map[string]any{"access_token": accessToken, "account_id": "acct-pi"},
	}
}

func TestPiCodexUpstreamSelection(t *testing.T) {
	auth := piOAuthTestAuth()
	executor := NewCodexAutoExecutor(&config.Config{})
	for _, tc := range []struct {
		name string
		cfg  *config.Config
		auth *cliproxyauth.Auth
		base string
		alt  string
	}{
		{name: "default oauth", cfg: nil, auth: auth, base: "", alt: ""},
		{name: "custom endpoint still pi", cfg: &config.Config{}, auth: auth, base: "https://codex.example.invalid", alt: ""},
		{name: "compact still pi profile", cfg: &config.Config{}, auth: auth, base: "", alt: "responses/compact"},
		{name: "api key cannot select legacy profile", cfg: &config.Config{}, auth: &cliproxyauth.Auth{Provider: "codex", Attributes: map[string]string{"api_key": piCodexTestAccessToken()}}, base: "https://legacy.invalid", alt: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !executor.piWebsocketPreferred(tc.auth, cliproxyexecutor.Options{Alt: tc.alt}) {
				t.Fatal("all Codex requests must use the Pi profile")
			}
		})
	}
}

func TestPiCodexCredentialsPreferOAuthTokenWithoutChangingTargetOverride(t *testing.T) {
	auth := piOAuthTestAuth()
	auth.Attributes["api_key"] = "legacy-api-key"
	auth.Attributes["base_url"] = "https://legacy.invalid"
	token, baseURL := codexCreds(auth)
	if token != auth.Metadata["access_token"] {
		t.Fatal("Codex must use the Pi OAuth access token")
	}
	if baseURL != "https://legacy.invalid" {
		t.Fatalf("target override = %q", baseURL)
	}
}

func TestPiCodexWebSocketContinuationUsesIncrementalInput(t *testing.T) {
	session := &codexWebsocketSession{}
	first := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":"one"}]}`)
	completed := []byte(`{"type":"response.completed","response":{"id":"resp-1","output":[{"type":"message","role":"assistant","content":"answer"}]}}`)
	session.storePiContinuation(first, completed)
	second := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":"one"},{"type":"message","role":"assistant","content":"answer"},{"type":"message","role":"user","content":"two"}]}`)
	got := session.buildPiContinuationBody(second)
	assertJSONValue(t, got, "previous_response_id", "resp-1")
	if count := len(gjson.GetBytes(got, "input").Array()); count != 1 {
		t.Fatalf("incremental input count = %d, want 1; body=%s", count, got)
	}
	assertJSONValue(t, got, "input.0.content", "two")
}

func TestPiCodexWebSocketFailurePinsSessionToSSE(t *testing.T) {
	store := &codexWebsocketSessionStore{sessions: make(map[string]*codexWebsocketSession)}
	exec := NewCodexWebsocketsExecutor(&config.Config{})
	exec.store = store
	if exec.piSSEFallbackActive("session-1") {
		t.Fatal("new session unexpectedly pinned to SSE")
	}
	exec.activatePiSSEFallback("session-1")
	if !exec.piSSEFallbackActive("session-1") {
		t.Fatal("failed Pi websocket session must remain pinned to SSE")
	}
}

func TestNormalizePiCodexPayload(t *testing.T) {
	body := []byte(`{"model":"old","instructions":"","input":[{"role":"user"}],"tools":[{"type":"function","name":"x"}],"reasoning":{"effort":"high"},"previous_response_id":"prev","generate":true,"prompt_cache_retention":"24h","safety_identifier":"s","stream_options":{"include_usage":true}}`)
	got := normalizePiCodexPayload(body, "gpt-5.4", "session-1")
	assertJSONValue(t, got, "model", "gpt-5.4")
	assertJSONValue(t, got, "instructions", "You are a helpful assistant.")
	assertJSONValue(t, got, "text.verbosity", "low")
	assertJSONValue(t, got, "include.0", "reasoning.encrypted_content")
	assertJSONValue(t, got, "tool_choice", "auto")
	assertJSONValue(t, got, "prompt_cache_key", "session-1")
	if !gjson.GetBytes(got, "store").Exists() || gjson.GetBytes(got, "store").Bool() {
		t.Fatal("store must be false")
	}
	if !gjson.GetBytes(got, "stream").Bool() || !gjson.GetBytes(got, "parallel_tool_calls").Bool() {
		t.Fatal("stream and parallel_tool_calls must be true")
	}
	for _, path := range []string{"generate", "prompt_cache_retention", "safety_identifier", "stream_options"} {
		if gjson.GetBytes(got, path).Exists() {
			t.Errorf("%s must be absent", path)
		}
	}
	assertJSONValue(t, got, "previous_response_id", "prev")
	if !gjson.GetBytes(got, "input.0").Exists() || !gjson.GetBytes(got, "tools.0").Exists() || !gjson.GetBytes(got, "reasoning").Exists() {
		t.Fatal("translated input/tools/reasoning must be preserved")
	}
	withoutSession := normalizePiCodexPayload([]byte(`{"prompt_cache_key":"proxy-generated"}`), "gpt-5.4", "")
	if gjson.GetBytes(withoutSession, "prompt_cache_key").Exists() {
		t.Fatal("prompt_cache_key must be absent without a Pi session")
	}
	if !gjson.GetBytes(withoutSession, "input").IsArray() {
		t.Fatal("Pi payload input must always be an array")
	}
}

func TestPiCodexSSEHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Originator", "downstream")
	req.Header.Set("User-Agent", "downstream")
	req.Header.Set("OpenAI-Beta", "downstream")
	auth := piOAuthTestAuth()
	accessToken := auth.Metadata["access_token"].(string)
	if err := applyPiCodexSSEHeaders(req, auth, accessToken, "session-1", "pi (win32 10.0.26100; x64)"); err != nil {
		t.Fatalf("applyPiCodexSSEHeaders: %v", err)
	}
	want := map[string]string{
		"Authorization":       "Bearer " + accessToken,
		"Chatgpt-Account-Id":  "acct-pi",
		"Originator":          "pi",
		"User-Agent":          "pi (win32 10.0.26100; x64)",
		"OpenAI-Beta":         "responses=experimental",
		"Accept":              "text/event-stream",
		"Content-Type":        "application/json",
		"Session-Id":          "session-1",
		"X-Client-Request-Id": "session-1",
	}
	for key, value := range want {
		if got := req.Header.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
}

func TestPiCodexWebSocketHeaders(t *testing.T) {
	headers := http.Header{"Accept": {"text/event-stream"}, "Content-Type": {"application/json"}, "Openai-Beta": {"responses=experimental"}}
	auth := piOAuthTestAuth()
	if err := applyPiCodexWebSocketHeaders(headers, auth, auth.Metadata["access_token"].(string), "request-1", "pi (linux 6.8.0; x64)"); err != nil {
		t.Fatalf("applyPiCodexWebSocketHeaders: %v", err)
	}
	if got := headers.Get("OpenAI-Beta"); got != "responses_websockets=2026-02-06" {
		t.Fatalf("OpenAI-Beta = %q", got)
	}
	if headers.Get("Accept") != "" || headers.Get("Content-Type") != "" {
		t.Fatal("websocket headers must not contain SSE content headers")
	}
	if headers.Get("Session-Id") != "request-1" || headers.Get("X-Client-Request-Id") != "request-1" {
		t.Fatal("websocket request and session IDs must match")
	}
}

func TestCompressPiCodexBody(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.4","stream":true}`)
	compressed, err := compressPiCodexBody(raw)
	if err != nil {
		t.Fatalf("compressPiCodexBody: %v", err)
	}
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zstd.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(decoder)
	decoder.Close()
	if err != nil {
		t.Fatalf("decode compressed body: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("decoded body = %s, want %s", decoded, raw)
	}
}

func TestPiCodexSSEZstdRequest(t *testing.T) {
	auth := piOAuthTestAuth()
	accessToken := auth.Metadata["access_token"].(string)
	req := cliproxyexecutor.Request{
		Model:   "gpt-5.4",
		Payload: []byte(`{"prompt_cache_key":"session-1"}`),
	}
	httpReq, compressed, err := newPiCodexSSERequest(
		context.Background(),
		"https://chatgpt.com/backend-api/codex/responses",
		auth,
		accessToken,
		"gpt-5.4",
		[]byte(`{"input":[]}`),
		req,
		http.Header{"Originator": {"downstream"}, "Authorization": {"Bearer downstream"}},
	)
	if err != nil {
		t.Fatalf("newPiCodexSSERequest: %v", err)
	}
	if httpReq.Header.Get("Content-Encoding") != "zstd" {
		t.Fatalf("Content-Encoding = %q, want zstd", httpReq.Header.Get("Content-Encoding"))
	}
	if httpReq.Header.Get("Originator") != "pi" || httpReq.Header.Get("Authorization") != "Bearer "+accessToken {
		t.Fatal("Pi-owned headers were overridden")
	}
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zstd.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(decoder)
	decoder.Close()
	if err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	assertJSONValue(t, decoded, "prompt_cache_key", "session-1")
	assertJSONValue(t, decoded, "instructions", piCodexDefaultInstructions)
}

func TestPiCodexAutoPrefersWebSocket(t *testing.T) {
	executor := NewCodexAutoExecutor(&config.Config{})
	if !executor.piWebsocketPreferred(piOAuthTestAuth(), cliproxyexecutor.Options{}) {
		t.Fatal("strict Pi OAuth path must prefer websocket transport")
	}
	if !executor.piWebsocketPreferred(piOAuthTestAuth(), cliproxyexecutor.Options{Alt: "responses/compact"}) {
		t.Fatal("all Codex operations must use Pi websocket transport")
	}
}

func TestPiCodexWebSocketRetryClassification(t *testing.T) {
	for _, raw := range []string{
		`{"error":{"code":"websocket_connection_limit_reached"}}`,
		`{"error":{"code":"previous_response_not_found"}}`,
	} {
		if !isPiCodexWebsocketRetryError(codexWebsocketPreStartError{err: newCodexStatusErr(http.StatusBadRequest, []byte(raw))}) {
			t.Fatalf("error %s must receive one Pi websocket retry", raw)
		}
	}
	if isPiCodexWebsocketRetryError(newCodexStatusErr(http.StatusUnauthorized, []byte(`{"error":{"code":"auth_unavailable"}}`))) {
		t.Fatal("authentication failures must not receive a Pi websocket retry")
	}
}

func TestPiCodexWebSocketMaxAgeExpiresConnection(t *testing.T) {
	conn := &websocket.Conn{}
	session := &codexWebsocketSession{
		conn:          conn,
		connCloser:    newWebsocketConnectionCloser(conn),
		connCreatedAt: time.Now().Add(-codexResponsesWebsocketMaxAge - time.Second),
	}
	detached, closer := detachExpiredWebsocketSessionConn(session)
	if detached != conn || closer == nil {
		t.Fatal("55-minute-old Pi websocket connection was not detached")
	}
	if session.conn != nil || !session.connCreatedAt.IsZero() {
		t.Fatal("expired Pi websocket state was not cleared")
	}
}

func assertJSONValue(t *testing.T, body []byte, path string, want string) {
	t.Helper()
	if got := gjson.GetBytes(body, path).String(); got != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}
