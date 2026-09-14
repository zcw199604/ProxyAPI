package helps

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

func TestRetryCodexOverloadPolicy(t *testing.T) {
	overload := errors.New(`{"error":{"type":"service_unavailable_error","code":"server_is_overloaded"}}`)
	for _, successAt := range []int{1, 2, 4, 5} {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			calls := 0
			value, err := RetryCodexOverload(context.Background(), func() (string, error) {
				if elapsed := time.Since(start); elapsed != time.Duration(calls)*5*time.Second {
					t.Fatalf("attempt %d at %s", calls+1, elapsed)
				}
				calls++
				if calls == successAt {
					return "completed", nil
				}
				return "", overload
			})
			if calls != min(successAt, 4) {
				t.Fatalf("calls=%d, successAt=%d", calls, successAt)
			}
			if successAt <= 4 {
				if value != "completed" || err != nil {
					t.Fatalf("value=%q err=%v", value, err)
				}
			} else {
				var scoped cliproxyexecutor.RequestScopedError
				if !errors.Is(err, overload) || err.Error() != overload.Error() || !errors.As(err, &scoped) || !scoped.IsRequestScoped() {
					t.Fatalf("exhausted overload must preserve body and skip credential penalties: %v", err)
				}
			}
		})
	}
}

func TestRetryCodexOverloadCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		calls := 0
		start := time.Now()
		_, err := RetryCodexOverload(ctx, func() (string, error) {
			calls++
			return "", errors.New(`{"error":{"code":"server_is_overloaded"}}`)
		})
		if !errors.Is(err, context.Canceled) || calls != 1 || time.Since(start) != time.Second {
			t.Fatalf("cancellation: calls=%d elapsed=%s err=%v", calls, time.Since(start), err)
		}
	})
}

func TestRetryCodexOverloadDoesNotRetryOtherErrors(t *testing.T) {
	for _, body := range []string{
		`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`,
		`{"error":{"type":"authentication_error","message":"unauthorized"}}`,
		`{"error":{"message":"server_is_overloaded"}}`,
		"service unavailable",
	} {
		original := errors.New(body)
		calls := 0
		_, err := RetryCodexOverload(context.Background(), func() (string, error) {
			calls++
			return "", original
		})
		if err != original || calls != 1 {
			t.Fatalf("non-overload changed: calls=%d err=%v", calls, err)
		}
	}
}

func TestCodexOverloadStreamPreservesPayloadAndClassifiesLateError(t *testing.T) {
	chunks := make(chan cliproxyexecutor.StreamChunk, 2)
	chunks <- cliproxyexecutor.StreamChunk{Payload: []byte("already generated")}
	chunks <- cliproxyexecutor.StreamChunk{Err: errors.New(`{"error":{"code":"server_is_overloaded"}}`)}
	close(chunks)
	result := CodexOverloadStream(context.Background(), &cliproxyexecutor.StreamResult{Chunks: chunks})
	if chunk := <-result.Chunks; string(chunk.Payload) != "already generated" || chunk.Err != nil {
		t.Fatalf("payload changed: %+v", chunk)
	}
	var scoped cliproxyexecutor.RequestScopedError
	if chunk := <-result.Chunks; !errors.As(chunk.Err, &scoped) || !scoped.IsRequestScoped() {
		t.Fatalf("late overload must not cool credentials: %v", chunk.Err)
	}
	if _, ok := <-result.Chunks; ok {
		t.Fatal("expected stream closure")
	}
}
