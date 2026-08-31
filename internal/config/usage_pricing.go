package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const DefaultPricingSyncURL = "https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/backend/resources/model-pricing/model_prices_and_context_window.json"

func normalizeUsagePricingConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.UsageStatsDBPath) == "" {
		cfg.UsageStatsDBPath = DefaultUsageStatsDBPath
	}
	if cfg.UsageStatsRetentionDays < 0 {
		cfg.UsageStatsRetentionDays = 0
	}
	if strings.TrimSpace(cfg.PricingDataDir) == "" {
		cfg.PricingDataDir = DefaultPricingDataDir
	}
	if strings.TrimSpace(cfg.PricingSyncURL) == "" {
		cfg.PricingSyncURL = DefaultPricingSyncURL
	}
	if strings.TrimSpace(cfg.PricingSyncInterval) == "" {
		cfg.PricingSyncInterval = DefaultPricingSyncInterval
	}
	parsedURL, err := url.ParseRequestURI(cfg.PricingSyncURL)
	if err != nil {
		return fmt.Errorf("invalid pricing-sync-url: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return fmt.Errorf("invalid pricing-sync-url: expected http(s) URL")
	}
	if parsed, err := time.ParseDuration(cfg.PricingSyncInterval); err != nil || parsed <= 0 {
		return fmt.Errorf("pricing-sync-interval must be a positive duration")
	}
	return nil
}
