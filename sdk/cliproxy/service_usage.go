package cliproxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	internalusage "github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	sdkusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func (s *Service) initUsageServices(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg == nil {
		return nil
	}
	var catalog pricing.Catalog
	if cfg.PricingSyncEnabled {
		interval, err := time.ParseDuration(strings.TrimSpace(cfg.PricingSyncInterval))
		if err != nil || interval <= 0 {
			interval = 24 * time.Hour
		}
		s.pricingService = pricing.NewSyncService(cfg.PricingSyncURL, cfg.PricingDataDir, interval)
		if err := s.pricingService.Load(); err != nil {
			// A corrupt/newly missing cache should not prevent the proxy from serving;
			// the last known catalog remains empty and events are marked unpriced.
			catalog = pricing.Catalog{}
		} else {
			catalog = s.pricingService.Catalog()
		}
		if err := s.pricingService.Sync(ctx); err != nil {
			// Network failures fall back to the loaded last-known-good cache.
			if catalog.Models == nil {
				catalog = s.pricingService.Catalog()
			}
		}
	}
	if !cfg.UsageStatisticsEnabled {
		sdkusage.UnregisterNamedPlugin("account-stats")
		if s.pricingService != nil {
			s.pricingService.Start(context.Background())
		}
		return nil
	}
	dbPath := strings.TrimSpace(cfg.UsageStatsDBPath)
	if dbPath == "" {
		dbPath = "data/usage.db"
	}
	store, err := internalusage.OpenSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("open usage statistics store: %w", err)
	}
	if cfg.UsageStatsRetentionDays > 0 {
		if purgeStore, ok := any(store).(interface{ PurgeBefore(time.Time) error }); ok {
			if err := purgeStore.PurgeBefore(time.Now().UTC().AddDate(0, 0, -cfg.UsageStatsRetentionDays)); err != nil {
				_ = store.Close()
				return fmt.Errorf("purge usage statistics: %w", err)
			}
		}
	}
	lookup := func(authID string) (string, bool) {
		if s.coreManager == nil {
			return "", false
		}
		auth, ok := s.coreManager.GetByID(strings.TrimSpace(authID))
		if !ok || auth == nil {
			return "", false
		}
		if label := strings.TrimSpace(auth.Label); label != "" {
			return label, true
		}
		if kind, value := auth.AccountInfo(); kind == coreauth.AuthKindOAuth && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
		return auth.EnsureIndex(), true
	}
	s.statsPlugin = internalusage.NewStatsPlugin(store, pricing.Resolver{Catalog: catalog}, lookup)
	if s.pricingService != nil {
		s.pricingService.SetOnUpdate(func(updated pricing.Catalog) {
			if s.statsPlugin != nil {
				s.statsPlugin.SetCatalog(updated)
			}
		})
		s.pricingService.Start(context.Background())
	}
	sdkusage.RegisterNamedPlugin("account-stats", s.statsPlugin)
	return nil
}
