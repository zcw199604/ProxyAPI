package executor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestCodexTerminalEventWithoutBlankLine(t *testing.T) {
	for _, ending := range []string{"", "\n", "\r\n"} {
		for _, mode := range []string{"execute", "stream", "buffered"} {
			t.Run(mode+"/"+strings.ReplaceAll(ending, "\n", "LF"), func(t *testing.T) {
				data := `data: {"type":"response.completed","response":{"id":"resp_eof","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}` + ending
				ctx := context.WithValue(context.Background(), "cliproxy.roundtripper", roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(data)), Request: req}, nil
				}))
				cfg := &config.Config{}
				cfg.Codex.StreamBootstrapBuffering = mode == "buffered"
				e := NewCodexExecutor(cfg)
				auth := &cliproxyauth.Auth{Attributes: map[string]string{"base_url": "http://codex.test", "api_key": piCodexTestAccessToken()}}
				req := cliproxyexecutor.Request{Model: "gpt-5.5", Payload: []byte(`{"input":"hello"}`)}
				opts := cliproxyexecutor.Options{SourceFormat: sdktranslator.FormatOpenAIResponse, Stream: mode != "execute"}
				if mode == "execute" {
					resp, err := e.Execute(ctx, auth, req, opts)
					if err != nil {
						t.Fatal(err)
					}
					if gjson.GetBytes(resp.Payload, "id").String() != "resp_eof" {
						t.Fatalf("lost terminal response: %s", resp.Payload)
					}
					return
				}
				result, err := e.ExecuteStream(ctx, auth, req, opts)
				if err != nil {
					t.Fatal(err)
				}
				var output strings.Builder
				for chunk := range result.Chunks {
					if chunk.Err != nil {
						t.Fatal(chunk.Err)
					}
					output.Write(chunk.Payload)
				}
				if !strings.Contains(output.String(), "response.completed") || !strings.Contains(output.String(), "resp_eof") {
					t.Fatalf("lost terminal event: %s", output.String())
				}
			})
		}
	}
}
