package helps

import (
	"context"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
)

type codexOverloadRetryKey struct{}

// WithCodexOverloadRetry keeps execution ownership with the caller while it retries overloads.
func WithCodexOverloadRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, codexOverloadRetryKey{}, true)
}

// CodexOverloadRetryEnabled reports whether the caller owns the bounded overload retry loop.
func CodexOverloadRetryEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(codexOverloadRetryKey{}).(bool)
	return enabled
}

// IsCodexOverload matches the upstream capacity rejection, not generic 503s or quota errors.
func IsCodexOverload(err error) bool {
	if err == nil {
		return false
	}
	body := err.Error()
	return strings.EqualFold(gjson.Get(body, "error.code").String(), "server_is_overloaded") ||
		strings.EqualFold(gjson.Get(body, "error.type").String(), "service_unavailable_error")
}

type codexOverloadError struct{ error }

func (e codexOverloadError) Unwrap() error       { return e.error }
func (codexOverloadError) IsRequestScoped() bool { return true }

// CodexOverloadRequestError preserves the upstream body/status while preventing cooldown
// and additional credential retries after the bounded overload retry policy has finished.
func CodexOverloadRequestError(err error) error {
	if IsCodexOverload(err) {
		return codexOverloadError{err}
	}
	return err
}

// RetryCodexOverload retries the same execution at most three times, five seconds apart.
// Waiting is cancellable and does not impose a deadline on upstream network activity.
func RetryCodexOverload[T any](ctx context.Context, attempt func() (T, error)) (T, error) {
	for retries := 0; ; retries++ {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		result, err := attempt()
		if !IsCodexOverload(err) {
			return result, err
		}
		if retries == 3 {
			return result, CodexOverloadRequestError(err)
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			var zero T
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}

// CodexOverloadStream preserves availability for late overloads without replaying output
// that has already been delivered to the caller.
func CodexOverloadStream(ctx context.Context, result *cliproxyexecutor.StreamResult) *cliproxyexecutor.StreamResult {
	if result == nil || result.Chunks == nil {
		return result
	}
	out := make(chan cliproxyexecutor.StreamChunk)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-result.Chunks:
				if !ok {
					return
				}
				chunk.Err = CodexOverloadRequestError(chunk.Err)
				select {
				case <-ctx.Done():
					return
				case out <- chunk:
				}
			}
		}
	}()
	return &cliproxyexecutor.StreamResult{Headers: result.Headers, Chunks: out}
}
