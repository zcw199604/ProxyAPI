package config

import "testing"

func TestParseConfigAppliesUsagePricingDefaults(t *testing.T) {
	cfg, err := ParseConfigBytes([]byte("host: 127.0.0.1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UsageStatsDBPath != DefaultUsageStatsDBPath || cfg.PricingDataDir != DefaultPricingDataDir || cfg.PricingSyncURL != DefaultPricingSyncURL || cfg.PricingSyncInterval != DefaultPricingSyncInterval {
		t.Fatalf("defaults = %+v", cfg)
	}
	if !cfg.PricingSyncEnabled {
		t.Fatal("PricingSyncEnabled default = false, want true")
	}
}

func TestParseConfigRejectsInvalidPricingURL(t *testing.T) {
	if _, err := ParseConfigBytes([]byte("pricing-sync-url: localhost\n")); err == nil {
		t.Fatal("invalid pricing URL was accepted")
	}
}
