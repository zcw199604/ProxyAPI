package usage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

// AuthLookup resolves a stable auth ID to a non-secret display label.
type AuthLookup func(authID string) (label string, ok bool)

// StatsPlugin converts SDK records into durable account statistics.
// It intentionally ignores the request context when writing so client
// disconnects cannot discard an already-observed provider attempt.
type StatsPlugin struct {
	store  StatsStore
	lookup AuthLookup

	mu       sync.RWMutex
	resolver pricing.Resolver
	enabled  atomic.Bool
	closed   atomic.Bool
}

// NewStatsPlugin creates a statistics sink. The catalog may be empty when
// price synchronization is disabled; events are still retained as unpriced.
func NewStatsPlugin(store StatsStore, resolver pricing.Resolver, lookups ...AuthLookup) *StatsPlugin {
	var lookup AuthLookup
	if len(lookups) > 0 {
		lookup = lookups[0]
	}
	p := &StatsPlugin{store: store, lookup: lookup, resolver: resolver}
	p.enabled.Store(true)
	return p
}

// SetEnabled toggles writes without unregistering the sink.
func (p *StatsPlugin) SetEnabled(enabled bool) {
	if p != nil {
		p.enabled.Store(enabled)
	}
}

// SetCatalog replaces the global pricing snapshot for future events.
func (p *StatsPlugin) SetCatalog(catalog pricing.Catalog) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.resolver.Catalog = catalog
	p.mu.Unlock()
}

// SetOverrides replaces account-specific pricing rules for future events.
func (p *StatsPlugin) SetOverrides(overrides []pricing.Override) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.resolver.Overrides = append([]pricing.Override(nil), overrides...)
	p.mu.Unlock()
}

// HandleUsage implements sdk/cliproxy/usage.Plugin.
func (p *StatsPlugin) HandleUsage(_ context.Context, record coreusage.Record) {
	if p == nil || p.store == nil || !p.enabled.Load() || p.closed.Load() {
		return
	}
	event := p.normalize(record)
	if err := p.store.AppendEvent(event); err != nil {
		log.WithError(err).WithField("event_id", event.EventID).Warn("usage statistics event was not persisted")
	}
}

func (p *StatsPlugin) normalize(record coreusage.Record) Event {
	eventID := strings.TrimSpace(record.EventID)
	if eventID == "" {
		eventID = uuid.NewString()
	}
	requestedAt := record.RequestedAt.UTC()
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	detail := coreusage.EnsureTokenBreakdownForProvider(record.Detail, record.Provider, record.ExecutorType)
	authID := strings.TrimSpace(record.AuthID)
	label := strings.TrimSpace(record.AuthIndex)
	if p.lookup != nil && authID != "" {
		if resolved, ok := p.lookup(authID); ok && strings.TrimSpace(resolved) != "" {
			label = strings.TrimSpace(resolved)
		}
	}
	p.mu.RLock()
	resolver := p.resolver
	p.mu.RUnlock()
	// Persisted overrides are authoritative even when a plugin was created
	// before the management API changed a rule.
	if overrides, err := p.store.ListOverrides(authID); err == nil && len(overrides) > 0 {
		resolver.Overrides = overrides
	}
	resolved, found := resolver.Resolve(authID, record.Provider, record.Model)
	status := pricingStatusUnpriced
	var cost int64
	if found {
		cost, status = calculateEventCost(detail, resolved, 1, 1)
	}
	return Event{
		EventID: eventID, RequestedAt: requestedAt, AuthID: authID, AuthIndex: strings.TrimSpace(record.AuthIndex), AccountLabel: label,
		Provider: strings.TrimSpace(record.Provider), AuthType: strings.TrimSpace(record.AuthType), Model: strings.TrimSpace(record.Model), Alias: strings.TrimSpace(record.Alias),
		Success: !record.Failed, FailureStatus: record.Fail.StatusCode, Detail: detail, Price: resolved, CostNanoUSD: cost, PricingStatus: status,
	}
}

// Query delegates aggregate queries to the durable store.
func (p *StatsPlugin) Query(query Query) ([]Summary, int64, error) {
	if p == nil || p.store == nil {
		return nil, 0, errors.New("usage: stats plugin is not initialized")
	}
	return p.store.QuerySummary(query)
}

// QueryWindow returns an exact timestamp-bounded aggregate over immutable
// events. The method is intentionally separate from Query because the latter
// may use the day aggregate for efficient calendar-range queries.
func (p *StatsPlugin) QueryWindow(query Query, from, to time.Time) (Summary, error) {
	if p == nil || p.store == nil {
		return Summary{}, errors.New("usage: stats plugin is not initialized")
	}
	exactStore, ok := p.store.(interface {
		QueryEventSummary(Query) ([]Summary, int64, error)
	})
	if !ok {
		return Summary{}, errors.New("usage: exact event queries are not supported")
	}
	query.From = from
	query.To = to
	query.GroupBy = "model"
	query.PageSize = 1000
	var total Summary
	total.AuthID = strings.TrimSpace(query.AuthID)
	total.Provider = strings.TrimSpace(query.Provider)
	for page := 1; ; page++ {
		query.Page = page
		items, itemTotal, err := exactStore.QueryEventSummary(query)
		if err != nil {
			return Summary{}, err
		}
		for _, item := range items {
			total.RequestCount += item.RequestCount
			total.SuccessCount += item.SuccessCount
			total.FailedCount += item.FailedCount
			total.InputTokens += item.InputTokens
			total.OutputTokens += item.OutputTokens
			total.ReasoningTokens += item.ReasoningTokens
			total.CacheReadTokens += item.CacheReadTokens
			total.CacheWriteTokens += item.CacheWriteTokens
			total.UnclassifiedTokens += item.UnclassifiedTokens
			total.TotalTokens += item.TotalTokens
			total.PricedRequestCount += item.PricedRequestCount
			total.UnpricedRequestCount += item.UnpricedRequestCount
			total.TotalCostNanoUSD += item.TotalCostNanoUSD
		}
		if int64(page*query.PageSize) >= itemTotal || len(items) == 0 {
			break
		}
	}
	total.TotalCostUSD = formatUSD(total.TotalCostNanoUSD)
	return total, nil
}

// Store exposes the underlying store for management handlers.
func (p *StatsPlugin) Store() StatsStore {
	if p == nil {
		return nil
	}
	return p.store
}

// Close prevents new writes and closes the durable store.
func (p *StatsPlugin) Close() error {
	if p == nil {
		return nil
	}
	if !p.closed.Swap(true) && p.store != nil {
		return p.store.Close()
	}
	return nil
}
