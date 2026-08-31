package management

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	internalusage "github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestAccountUsageStatsSeparatesQuotaWindowsAndImportTotal(t *testing.T) {
	store, err := internalusage.OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	plugin := internalusage.NewStatsPlugin(store, pricing.Resolver{})
	h := &Handler{}
	h.SetStatsPlugin(plugin)

	now := time.Now().UTC()
	for index, requestedAt := range []time.Time{
		now.Add(-6 * time.Hour),
		now.Add(-3 * time.Hour),
		now.Add(-3 * 24 * time.Hour),
		now.Add(-8 * 24 * time.Hour),
	} {
		plugin.HandleUsage(nil, coreusage.Record{
			EventID:     "account-window-event-" + string(rune('a'+index)),
			RequestedAt: requestedAt,
			AuthID:      "auth-window",
			Provider:    "claude",
			Model:       "claude-sonnet",
			Detail: coreusage.Detail{
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
			},
		})
	}

	quota := coreauth.QuotaState{Signals: map[string]string{
		"Anthropic-Ratelimit-Unified-5h-Status": "allowed",
		"Anthropic-Ratelimit-Unified-7d-Status": "allowed",
	}}
	stats := h.accountUsageStats("auth-window", "claude", quota, now.Add(-30*24*time.Hour))
	if stats == nil {
		t.Fatal("accountUsageStats returned nil")
	}
	if _, err := json.Marshal(stats); err != nil {
		t.Fatalf("usage stats payload is not JSON serializable: %v", err)
	}
	windows, ok := stats["windows"].(gin.H)
	if !ok {
		t.Fatalf("windows payload = %#v", stats["windows"])
	}
	assertWindowRequestCount(t, windows, "total", 4)
	assertWindowRequestCount(t, windows, "5h", 1)
	assertWindowRequestCount(t, windows, "7d", 3)

	available, ok := stats["available_windows"].([]string)
	if !ok || len(available) != 2 || available[0] != "5h" || available[1] != "7d" {
		t.Fatalf("available windows = %#v", stats["available_windows"])
	}

	stats = h.accountUsageStats("auth-window", "claude", coreauth.QuotaState{Signals: map[string]string{
		"Anthropic-Ratelimit-Unified-7d-Status": "allowed",
	}}, now.Add(-30*24*time.Hour))
	windows, ok = stats["windows"].(gin.H)
	if !ok {
		t.Fatalf("windows payload without 5h = %#v", stats["windows"])
	}
	if _, exists := windows["5h"]; exists {
		t.Fatalf("5h window shown without a 5h account limit: %#v", windows)
	}
	assertWindowRequestCount(t, windows, "7d", 3)
}

func assertWindowRequestCount(t *testing.T, windows gin.H, name string, want int64) {
	t.Helper()
	payload, ok := windows[name].(gin.H)
	if !ok {
		t.Fatalf("window %s payload = %#v", name, windows[name])
	}
	got, ok := payload["request_count"].(int64)
	if !ok || got != want {
		t.Fatalf("window %s request_count = %#v, want %d", name, payload["request_count"], want)
	}
}
