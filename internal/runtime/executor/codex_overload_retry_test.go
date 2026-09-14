package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
)

func TestCodexOverloadRetry(t *testing.T) {
	for _, transport := range []string{"sse", "http-error", "websocket"} {
		for _, streaming := range []bool{false, true} {
			for _, recover := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/recover=%t", transport, streaming, recover), func(t *testing.T) {
					t.Parallel()
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if transport == "websocket" {
							upgrader := websocket.Upgrader{}
							conn, errUpgrade := upgrader.Upgrade(w, r, nil)
							if errUpgrade != nil {
								t.Errorf("websocket upgrade: %v", errUpgrade)
								return
							}
							defer func() { _ = conn.Close() }()
							if _, _, errRead := conn.ReadMessage(); errRead != nil {
								t.Errorf("websocket request: %v", errRead)
								return
							}
							attempt := calls.Add(1)
							_ = conn.WriteMessage(websocket.TextMessage, []byte(codexCreatedEvent))
							terminal := codexOverloadEvent
							if recover && attempt == 4 {
								terminal = codexCompletedEventBody
							}
							_ = conn.WriteMessage(websocket.TextMessage, []byte(terminal))
							// Keep the connection alive until the executor tears it down.
							_, _, _ = conn.ReadMessage()
							return
						}
						if r.Header.Get("Upgrade") != "" {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						attempt := calls.Add(1)
						if transport == "http-error" && !(recover && attempt == 4) {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusServiceUnavailable)
							_, _ = fmt.Fprintf(w, `{"error":%s}`, gjson.Get(codexOverloadEvent, "error").Raw)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = fmt.Fprintf(w, "data: %s\n\n", codexCreatedEvent)
						if recover && attempt == 4 {
							_, _ = fmt.Fprintf(w, "data: %s\n\n", codexCompletedEventBody)
						} else {
							_, _ = fmt.Fprintf(w, "data: %s\n\n", codexOverloadEvent)
						}
					}))
					defer server.Close()
					manager := cliproxyauth.NewManager(nil, nil, nil)
					manager.SetRetryConfig(5, 0, 6)
					executor := NewCodexAutoExecutor(&config.Config{})
					defer executor.CloseExecutionSession(t.Name())
					manager.RegisterExecutor(executor)
					auth := codexTestAuth(server.URL)
					auth.ID = t.Name()
					auth.Provider = "codex"
					auth.Status = cliproxyauth.StatusActive
					auth.Attributes["priority"] = "100"
					reg := registry.GetGlobalRegistry()
					reg.RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: "gpt-5.6-terra"}})
					t.Cleanup(func() { reg.UnregisterClient(auth.ID) })
					if _, err := manager.Register(context.Background(), auth); err != nil {
						t.Fatal(err)
					}
					backup := auth.Clone()
					backup.ID += "-backup"
					backup.Attributes["priority"] = "0"
					reg.RegisterClient(backup.ID, "codex", []*registry.ModelInfo{{ID: "gpt-5.6-terra"}})
					t.Cleanup(func() { reg.UnregisterClient(backup.ID) })
					if _, err := manager.Register(context.Background(), backup); err != nil {
						t.Fatal(err)
					}
					req, opts := codexTestRequest()
					lifecycle := newTerminalFailureLifecycle()
					if transport == "websocket" {
						opts.Metadata = map[string]any{cliproxyexecutor.ExecutionSessionMetadataKey: t.Name()}
						opts.ExecutionLifecycle = lifecycle
					}
					start := time.Now()
					var err error
					var payload string
					if streaming {
						var result *cliproxyexecutor.StreamResult
						result, err = manager.ExecuteStream(context.Background(), []string{"codex"}, req, opts)
						if err == nil {
							payload, err = drainChunks(result)
						}
					} else {
						opts.Stream = false
						var response cliproxyexecutor.Response
						response, err = manager.Execute(context.Background(), []string{"codex"}, req, opts)
						payload = string(response.Payload)
					}
					if calls.Load() != 4 {
						t.Fatalf("upstream calls = %d, want initial + 3 retries; err=%v", calls.Load(), err)
					}
					if transport == "websocket" && lifecycle.ends.Load() != 0 {
						t.Errorf("overload retry prematurely ended execution lifecycle %d times", lifecycle.ends.Load())
					}
					if elapsed := time.Since(start); elapsed < 15*time.Second {
						t.Errorf("retry waits = %s, want at least 15s", elapsed)
					}
					if (err == nil) != recover {
						t.Fatalf("error = %v, recover=%t", err, recover)
					}
					if recover && !strings.Contains(payload, "hello") {
						t.Errorf("missing successful response: %s", payload)
					}
					if !recover {
						var status cliproxyexecutor.StatusError
						if !errors.As(err, &status) || (status.StatusCode() != 502 && status.StatusCode() != 503) || gjson.Get(err.Error(), "error.code").String() != "server_is_overloaded" {
							t.Errorf("upstream error body/status lost: %v", err)
						}
					}
					updated, ok := manager.GetByID(auth.ID)
					if !ok || updated.Unavailable || updated.Disabled || !updated.NextRetryAfter.IsZero() {
						t.Fatalf("account was made unavailable: %+v", updated)
					}
					for model, state := range updated.ModelStates {
						if state.Unavailable || !state.NextRetryAfter.IsZero() {
							t.Errorf("model %s was cooled: %+v", model, state)
						}
					}
				})
			}
		}
	}
}
