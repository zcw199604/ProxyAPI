package executor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	codexauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/codex"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor/helps"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	piCodexDefaultInstructions = "You are a helpful assistant."
)

func normalizePiCodexPayload(body []byte, model string, sessionID string) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		body = []byte(`{}`)
	}
	body = helps.SetStringIfDifferent(body, "model", model)
	body = helps.SetBoolIfDifferent(body, "store", false)
	body = helps.SetBoolIfDifferent(body, "stream", true)
	if strings.TrimSpace(gjson.GetBytes(body, "instructions").String()) == "" {
		body = helps.SetStringIfDifferent(body, "instructions", piCodexDefaultInstructions)
	}
	if !gjson.GetBytes(body, "input").IsArray() {
		body, _ = sjson.SetRawBytes(body, "input", []byte(`[]`))
	}
	// Pi forwards Responses input items with the same 64-rune identifier
	// constraint as the Codex backend. Keep this normalization in the shared
	// Pi payload path so HTTP and WebSocket transports behave identically.
	body = helps.SanitizeCodexInputItemIDs(body)
	if strings.TrimSpace(gjson.GetBytes(body, "text.verbosity").String()) == "" {
		body, _ = sjson.SetBytes(body, "text.verbosity", "low")
	}
	body, _ = sjson.SetRawBytes(body, "include", []byte(`["reasoning.encrypted_content"]`))
	if !gjson.GetBytes(body, "tool_choice").Exists() {
		body = helps.SetStringIfDifferent(body, "tool_choice", "auto")
	}
	body = helps.SetBoolIfDifferent(body, "parallel_tool_calls", true)
	for _, field := range []string{"generate", "prompt_cache_retention", "safety_identifier", "stream_options"} {
		body, _ = sjson.DeleteBytes(body, field)
	}
	if sessionID = clampPiCodexSessionID(sessionID); sessionID != "" {
		body = helps.SetStringIfDifferent(body, "prompt_cache_key", sessionID)
	} else {
		body, _ = sjson.DeleteBytes(body, "prompt_cache_key")
	}
	return body
}

func clampPiCodexSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if utf8.RuneCountInString(sessionID) <= 64 {
		return sessionID
	}
	runes := []rune(sessionID)
	return string(runes[:64])
}

func piCodexSessionID(req cliproxyexecutor.Request) string {
	if promptCacheKey := gjson.GetBytes(req.Payload, "prompt_cache_key"); promptCacheKey.Exists() {
		return clampPiCodexSessionID(promptCacheKey.String())
	}
	if req.Metadata != nil {
		if executionID, ok := req.Metadata[cliproxyexecutor.ExecutionSessionMetadataKey].(string); ok {
			if executionID = clampPiCodexSessionID(executionID); executionID != "" {
				return executionID
			}
		}
	}
	return clampPiCodexSessionID(helps.DerivedSessionID(req.Metadata))
}

func piCodexWebsocketRequestID(req cliproxyexecutor.Request) string {
	if sessionID := piCodexSessionID(req); sessionID != "" {
		return sessionID
	}
	requestID, err := uuid.NewV7()
	if err == nil {
		return requestID.String()
	}
	return uuid.NewString()
}

func newPiCodexWebSocketHeaders(auth *cliproxyauth.Auth, accessToken string, model string, req cliproxyexecutor.Request, additionalHeaders http.Header) (http.Header, error) {
	headers := additionalHeaders.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(&http.Request{Header: headers}, attrs, additionalHeaders)
	applyModelHeaderOverrides(headers, model)
	if err := applyPiCodexWebSocketHeaders(headers, auth, accessToken, piCodexWebsocketRequestID(req), ""); err != nil {
		return nil, err
	}
	return headers, nil
}

func newPiCodexSSERequest(ctx context.Context, url string, auth *cliproxyauth.Auth, accessToken string, model string, body []byte, req cliproxyexecutor.Request, additionalHeaders http.Header) (*http.Request, []byte, error) {
	sessionID := piCodexSessionID(req)
	body = normalizePiCodexPayload(body, model, sessionID)
	upstreamBody := body
	contentEncoding := ""
	if compressed, errCompress := compressPiCodexBody(body); errCompress == nil && !isLoopbackCodexTarget(url) {
		upstreamBody = compressed
		contentEncoding = "zstd"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, nil, err
	}
	for key, values := range additionalHeaders {
		httpReq.Header[key] = append([]string(nil), values...)
	}
	var attrs map[string]string
	if auth != nil {
		attrs = auth.Attributes
	}
	util.ApplyCustomHeadersFromAttrs(httpReq, attrs, additionalHeaders)
	applyModelHeaderOverrides(httpReq.Header, model)
	if errHeaders := applyPiCodexSSEHeaders(httpReq, auth, accessToken, sessionID, ""); errHeaders != nil {
		return nil, nil, errHeaders
	}
	if contentEncoding != "" {
		httpReq.Header.Set("Content-Encoding", contentEncoding)
	} else {
		httpReq.Header.Del("Content-Encoding")
	}
	return httpReq, upstreamBody, nil
}

func isLoopbackCodexTarget(rawURL string) bool {
	parsed, errParse := neturl.Parse(rawURL)
	if errParse != nil {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func piCodexAccountID(accessToken string) (string, error) {
	claims, err := codexauth.ParseJWTToken(accessToken)
	if err != nil {
		return "", fmt.Errorf("parse Pi Codex access token: %w", err)
	}
	accountID := strings.TrimSpace(claims.GetAccountID())
	if accountID == "" {
		return "", fmt.Errorf("Pi Codex access token is missing chatgpt_account_id")
	}
	return accountID, nil
}

func applyPiCodexBaseHeaders(headers http.Header, accessToken string, userAgent string) error {
	accountID, err := piCodexAccountID(accessToken)
	if err != nil {
		return err
	}
	if userAgent == "" {
		userAgent = helps.PiUserAgent()
	}
	headers.Set("Authorization", "Bearer "+accessToken)
	headers.Set("Chatgpt-Account-Id", accountID)
	headers.Set("Originator", "pi")
	headers.Set("User-Agent", userAgent)
	return nil
}

func applyPiCodexSSEHeaders(req *http.Request, auth *cliproxyauth.Auth, accessToken string, sessionID string, userAgent string) error {
	_ = auth
	if req == nil {
		return fmt.Errorf("Pi Codex SSE request is nil")
	}
	if err := applyPiCodexBaseHeaders(req.Header, accessToken, userAgent); err != nil {
		return err
	}
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	deleteHeaderCaseInsensitive(req.Header, "Session-Id")
	deleteHeaderCaseInsensitive(req.Header, "X-Client-Request-Id")
	if sessionID = clampPiCodexSessionID(sessionID); sessionID != "" {
		req.Header.Set("Session-Id", sessionID)
		req.Header.Set("X-Client-Request-Id", sessionID)
	}
	return nil
}

func applyPiCodexWebSocketHeaders(headers http.Header, auth *cliproxyauth.Auth, accessToken string, requestID string, userAgent string) error {
	_ = auth
	if headers == nil {
		return fmt.Errorf("Pi Codex websocket headers are nil")
	}
	if err := applyPiCodexBaseHeaders(headers, accessToken, userAgent); err != nil {
		return err
	}
	deleteHeaderCaseInsensitive(headers, "Accept")
	deleteHeaderCaseInsensitive(headers, "Content-Type")
	deleteHeaderCaseInsensitive(headers, "OpenAI-Beta")
	headers.Set("OpenAI-Beta", codexResponsesWebsocketBetaHeaderValue)
	headers.Set("X-Client-Request-Id", requestID)
	headers.Set("Session-Id", requestID)
	return nil
}

func compressPiCodexBody(body []byte) ([]byte, error) {
	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(3)))
	if err != nil {
		return nil, fmt.Errorf("create Pi Codex zstd encoder: %w", err)
	}
	if _, errWrite := encoder.Write(body); errWrite != nil {
		encoder.Close()
		return nil, fmt.Errorf("compress Pi Codex request body: %w", errWrite)
	}
	if errClose := encoder.Close(); errClose != nil {
		return nil, fmt.Errorf("finish Pi Codex request compression: %w", errClose)
	}
	return compressed.Bytes(), nil
}
