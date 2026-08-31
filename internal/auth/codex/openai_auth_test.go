package codex

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"golang.org/x/sync/singleflight"
)

const piReferenceCommit = "853a80d26c90a14c1886f0ebb8ffaae133ca2185"

func fakeCodexAccessToken(accountID string) string {
	payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":%q},"email":"pi@example.invalid"}`, accountID)
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature"
}

func TestCodexAuthURLMatchesPiContract(t *testing.T) {
	// Contract frozen from earendil-works/pi at piReferenceCommit.
	authURL, err := NewCodexAuth(nil).GenerateAuthURL("state", &PKCECodes{CodeChallenge: "challenge"})
	if err != nil {
		t.Fatalf("GenerateAuthURL: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	query := parsed.Query()
	want := map[string]string{
		"client_id":                  ClientID,
		"response_type":              "code",
		"redirect_uri":               RedirectURI,
		"scope":                      "openid profile email offline_access",
		"state":                      "state",
		"code_challenge":             "challenge",
		"code_challenge_method":      "S256",
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"originator":                 "pi",
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("%s = %q, want %q (Pi %s)", key, got, value, piReferenceCommit)
		}
	}
	if query.Has("prompt") {
		t.Errorf("prompt must be absent for Pi %s", piReferenceCommit)
	}
}

func TestCodexRefreshMatchesPiContract(t *testing.T) {
	resetCodexRefreshGroupForTest()
	t.Cleanup(resetCodexRefreshGroupForTest)
	accessToken := fakeCodexAccessToken("acct-pi")
	var form url.Values
	auth := &CodexAuth{httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		form = make(url.Values, len(req.PostForm))
		for key, values := range req.PostForm {
			form[key] = append([]string(nil), values...)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"access_token":%q,"refresh_token":"new-refresh","expires_in":3600}`, accessToken))),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}

	tokenData, err := auth.RefreshTokens(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("RefreshTokens: %v", err)
	}
	if tokenData.AccountID != "acct-pi" {
		t.Fatalf("AccountID = %q, want acct-pi", tokenData.AccountID)
	}
	want := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"old-refresh"}, "client_id": {ClientID}}
	if form.Encode() != want.Encode() {
		t.Fatalf("refresh form = %q, want %q (Pi %s)", form.Encode(), want.Encode(), piReferenceCommit)
	}
}

func TestCodexRefreshRejectsMissingAccessTokenAccountID(t *testing.T) {
	resetCodexRefreshGroupForTest()
	t.Cleanup(resetCodexRefreshGroupForTest)
	auth := &CodexAuth{httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"eyJhbGciOiJub25lIn0.e30.signature","refresh_token":"new-refresh","expires_in":3600}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}

	_, err := auth.RefreshTokens(context.Background(), "old-refresh")
	if err == nil || !strings.Contains(err.Error(), "account ID") {
		t.Fatalf("error = %v, want account ID extraction failure", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewCodexAuthDoesNotSetRequestTimeout(t *testing.T) {
	if got := NewCodexAuth(nil).httpClient.Timeout; got != 0 {
		t.Fatalf("HTTP client timeout = %s, want zero", got)
	}
}

func TestRefreshTokens_UsesIndependentTimeout(t *testing.T) {
	resetCodexRefreshGroupForTest()
	defer resetCodexRefreshGroupForTest()

	callerCtx, cancelCaller := context.WithCancel(context.Background())
	cancelCaller()
	var requestDeadline time.Time
	auth := &CodexAuth{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var ok bool
				requestDeadline, ok = req.Context().Deadline()
				if !ok {
					t.Fatal("refresh request has no deadline")
				}
				if errContext := req.Context().Err(); errContext != nil {
					t.Fatalf("refresh request context is already done: %v", errContext)
				}
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"probe"}`)),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		},
	}

	_, err := auth.RefreshTokens(callerCtx, "independent-timeout-token")
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if requestDeadline.IsZero() || !requestDeadline.After(time.Now()) {
		t.Fatalf("refresh deadline = %v, want a future deadline", requestDeadline)
	}
}

func resetCodexRefreshGroupForTest() {
	codexRefreshGroup = singleflight.Group{}
}

func TestRefreshTokensWithRetry_NonRetryableOnlyAttemptsOnce(t *testing.T) {
	var calls int32
	auth := &CodexAuth{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&calls, 1)
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","code":"refresh_token_reused"}`)),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		},
	}

	_, err := auth.RefreshTokensWithRetry(context.Background(), "dummy_refresh_token", 3)
	if err == nil {
		t.Fatalf("expected error for non-retryable refresh failure")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "refresh_token_reused") {
		t.Fatalf("expected refresh_token_reused in error, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 refresh attempt, got %d", got)
	}
}

func TestRefreshTokens_DeduplicatesConcurrentRefreshAcrossInstances(t *testing.T) {
	resetCodexRefreshGroupForTest()
	t.Cleanup(resetCodexRefreshGroupForTest)

	var calls int32
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		once.Do(func() { close(started) })
		<-release
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{
					"access_token":%q,
					"refresh_token":"new-refresh",
					"token_type":"Bearer",
					"expires_in":3600
				}`, fakeCodexAccessToken("shared-account")))),
			Header:  make(http.Header),
			Request: req,
		}, nil
	})
	authA := &CodexAuth{httpClient: &http.Client{Transport: transport}}
	authB := &CodexAuth{httpClient: &http.Client{Transport: transport}}

	results := make(chan *CodexTokenData, 2)
	errs := make(chan error, 2)
	runRefresh := func(auth *CodexAuth, launched chan<- struct{}) {
		if launched != nil {
			close(launched)
		}
		tokenData, errRefresh := auth.RefreshTokens(context.Background(), "shared-refresh-token")
		results <- tokenData
		errs <- errRefresh
	}

	go runRefresh(authA, nil)
	<-started

	secondLaunched := make(chan struct{})
	go runRefresh(authB, secondLaunched)
	<-secondLaunched
	time.Sleep(20 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected concurrent refresh to share a single upstream call, got %d", got)
	}
	close(release)

	for i := 0; i < 2; i++ {
		if errRefresh := <-errs; errRefresh != nil {
			t.Fatalf("expected refresh to succeed, got %v", errRefresh)
		}
		tokenData := <-results
		if tokenData == nil || tokenData.AccountID != "shared-account" || tokenData.RefreshToken != "new-refresh" {
			t.Fatalf("unexpected token data: %#v", tokenData)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected both refresh callers to share a single upstream call, got %d", got)
	}
}

func TestNewCodexAuthWithProxyURL_OverrideDirectDisablesProxy(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{ProxyURL: "http://proxy.example.com:8080"}}
	auth := NewCodexAuthWithProxyURL(cfg, "direct")

	transport, ok := auth.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http.Transport, got %T", auth.httpClient.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}

func TestNewCodexAuthWithProxyURL_OverrideProxyTakesPrecedence(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{ProxyURL: "http://global.example.com:8080"}}
	auth := NewCodexAuthWithProxyURL(cfg, "http://override.example.com:8081")

	transport, ok := auth.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http.Transport, got %T", auth.httpClient.Transport)
	}
	req, errReq := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errReq != nil {
		t.Fatalf("new request: %v", errReq)
	}
	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("proxy func: %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://override.example.com:8081" {
		t.Fatalf("proxy URL = %v, want http://override.example.com:8081", proxyURL)
	}
}
