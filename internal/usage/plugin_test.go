package usage

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestStatsPluginCountsFailedAndZeroTokenAttempts(t *testing.T) {
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	input := int64(100)
	output := int64(200)
	plugin := NewStatsPlugin(store, pricing.Resolver{Catalog: pricing.Catalog{Models: map[string]pricing.Price{"gpt-test": {ModelKey: "gpt-test", InputNanoUSDPerToken: &input, OutputNanoUSDPerToken: &output}}}}, func(string) (string, bool) { return "account-a", true })
	requested := time.Now().UTC()
	plugin.HandleUsage(nil, coreusage.Record{EventID: "attempt-1", AuthID: "auth-a", AuthIndex: "idx", Provider: "openai", Model: "gpt-test", RequestedAt: requested, Failed: true})
	plugin.HandleUsage(nil, coreusage.Record{EventID: "attempt-2", AuthID: "auth-a", Provider: "openai", Model: "gpt-test", RequestedAt: requested})
	items, _, err := plugin.Query(Query{AuthID: "auth-a", GroupBy: "model"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RequestCount != 2 || items[0].FailedCount != 1 || items[0].PricedRequestCount != 2 {
		t.Fatalf("items = %+v", items)
	}
}
