package usage

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestSQLiteStoreAppendIsIdempotentAndAggregates(t *testing.T) {
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	inPrice, outPrice := int64(100), int64(200)
	p := pricing.ResolvedPrice{InputNanoUSDPerToken: &inPrice, OutputNanoUSDPerToken: &outPrice}
	event := Event{EventID: "event-1", RequestedAt: time.Date(2026, 8, 31, 1, 0, 0, 0, time.UTC), AuthID: "auth-a", Provider: "openai", Model: "gpt-test", Success: true, Detail: coreusage.Detail{InputTokens: 10, OutputTokens: 5, TotalTokens: 15, TokenBreakdown: coreusage.NewSubsetTokenBreakdown(10, 0, 0, 5, 0, 15)}, Price: p, CostNanoUSD: 2000, PricingStatus: pricingStatusPriced}
	if err := store.AppendEvent(event); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(event); err != nil {
		t.Fatal(err)
	}
	items, total, err := store.QuerySummary(Query{AuthID: "auth-a", GroupBy: "model"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("summary count = %d/%d, want 1/1", total, len(items))
	}
	if items[0].RequestCount != 1 || items[0].TotalTokens != 15 || items[0].TotalCostNanoUSD != 2000 {
		t.Fatalf("summary = %+v", items[0])
	}
}

func TestSQLiteStoreOverridesSoftDisable(t *testing.T) {
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	o, err := store.UpsertOverride(pricing.Override{AuthID: "auth-a", ModelPattern: "gpt-*", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ListOverrides("auth-a")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListOverrides = %v, %+v", err, items)
	}
	if err := store.DisableOverride(o.ID); err != nil {
		t.Fatal(err)
	}
	items, err = store.ListOverrides("auth-a")
	if err != nil || len(items) != 1 || items[0].Enabled {
		t.Fatalf("disabled override = %v, %+v", err, items)
	}
}
