package pricing

import "testing"

func TestParseSub2APICatalogConvertsTokenPricesToNanoUSD(t *testing.T) {
	raw := []byte(`{"claude-test":{"litellm_provider":"anthropic","input_cost_per_token":0.000003,"output_cost_per_token":0.000015,"cache_read_input_token_cost":0.0000003}}`)
	catalog, err := ParseCatalog(raw, "v1")
	if err != nil {
		t.Fatalf("ParseCatalog() error = %v", err)
	}
	price, ok := catalog.Lookup("claude-test")
	if !ok {
		t.Fatal("Lookup() did not find claude-test")
	}
	if price.InputNanoUSDPerToken == nil || *price.InputNanoUSDPerToken != 3000 {
		t.Fatalf("input nano price = %v, want 3000", price.InputNanoUSDPerToken)
	}
	if price.OutputNanoUSDPerToken == nil || *price.OutputNanoUSDPerToken != 15000 {
		t.Fatalf("output nano price = %v, want 15000", price.OutputNanoUSDPerToken)
	}
	if price.CacheReadNanoUSDPerToken == nil || *price.CacheReadNanoUSDPerToken != 300 {
		t.Fatalf("cache read nano price = %v, want 300", price.CacheReadNanoUSDPerToken)
	}
}

func TestParseSub2APICatalogRejectsNegativePrice(t *testing.T) {
	raw := []byte(`{"bad":{"input_cost_per_token":-0.000001}}`)
	if _, err := ParseCatalog(raw, "v1"); err == nil {
		t.Fatal("ParseCatalog() error = nil, want negative price rejection")
	}
}

func TestResolverPrefersAccountExactOverride(t *testing.T) {
	baseInput := int64(100)
	baseOutput := int64(200)
	overrideOutput := int64(900)
	resolver := Resolver{
		Catalog: Catalog{Models: map[string]Price{"gpt-test": {
			ModelKey:              "gpt-test",
			InputNanoUSDPerToken:  &baseInput,
			OutputNanoUSDPerToken: &baseOutput,
		}}},
		Overrides: []Override{{
			ID:                    "override-1",
			AuthID:                "auth-a",
			ModelPattern:          "gpt-test",
			OutputNanoUSDPerToken: &overrideOutput,
			Enabled:               true,
		}},
	}
	resolved, ok := resolver.Resolve("auth-a", "openai", "gpt-test")
	if !ok {
		t.Fatal("Resolve() did not find price")
	}
	if resolved.OutputNanoUSDPerToken == nil || *resolved.OutputNanoUSDPerToken != overrideOutput {
		t.Fatalf("resolved output price = %v, want %d", resolved.OutputNanoUSDPerToken, overrideOutput)
	}
	if resolved.InputNanoUSDPerToken == nil || *resolved.InputNanoUSDPerToken != baseInput {
		t.Fatalf("resolved input price = %v, want inherited %d", resolved.InputNanoUSDPerToken, baseInput)
	}
}
