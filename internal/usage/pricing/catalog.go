// Package pricing parses model price catalogs and resolves account-specific overrides.
package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	BillingModeToken      = "token"
	BillingModePerRequest = "per_request"
	BillingModeImage      = "image"
)

// Price stores normalized prices in nano-USD per token/request.
// A nil field means that the source did not provide that price.
type Price struct {
	ModelKey                  string
	SourceModel               string
	Provider                  string
	BillingMode               string
	InputNanoUSDPerToken      *int64
	OutputNanoUSDPerToken     *int64
	CacheReadNanoUSDPerToken  *int64
	CacheWriteNanoUSDPerToken *int64
	PerRequestNanoUSD         *int64
	ImageNanoUSD              *int64
}

func (p Price) clone() Price {
	out := p
	out.InputNanoUSDPerToken = cloneInt64(p.InputNanoUSDPerToken)
	out.OutputNanoUSDPerToken = cloneInt64(p.OutputNanoUSDPerToken)
	out.CacheReadNanoUSDPerToken = cloneInt64(p.CacheReadNanoUSDPerToken)
	out.CacheWriteNanoUSDPerToken = cloneInt64(p.CacheWriteNanoUSDPerToken)
	out.PerRequestNanoUSD = cloneInt64(p.PerRequestNanoUSD)
	out.ImageNanoUSD = cloneInt64(p.ImageNanoUSD)
	return out
}

// Catalog is an immutable in-memory snapshot of a model price file.
type Catalog struct {
	Version   string
	Hash      string
	UpdatedAt time.Time
	Models    map[string]Price
}

// ParseCatalog parses the LiteLLM-compatible JSON used by sub2api.
func ParseCatalog(raw []byte, version string) (Catalog, error) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Catalog{}, fmt.Errorf("parse pricing catalog: %w", err)
	}
	if len(entries) == 0 {
		return Catalog{}, fmt.Errorf("parse pricing catalog: no model entries")
	}
	models := make(map[string]Price, len(entries))
	for sourceModel, rawEntry := range entries {
		key := normalizeModel(sourceModel)
		if key == "" {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &fields); err != nil {
			return Catalog{}, fmt.Errorf("parse model %q: %w", sourceModel, err)
		}
		provider, err := optionalString(fields, "litellm_provider")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q provider: %w", sourceModel, err)
		}
		mode, err := optionalString(fields, "mode")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q mode: %w", sourceModel, err)
		}
		if mode == "" {
			mode = BillingModeToken
		}
		price := Price{ModelKey: key, SourceModel: sourceModel, Provider: strings.ToLower(strings.TrimSpace(provider)), BillingMode: mode}
		price.InputNanoUSDPerToken, err = parsePrice(fields, "input_cost_per_token")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q input price: %w", sourceModel, err)
		}
		price.OutputNanoUSDPerToken, err = parsePrice(fields, "output_cost_per_token")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q output price: %w", sourceModel, err)
		}
		price.CacheReadNanoUSDPerToken, err = parsePrice(fields, "cache_read_input_token_cost")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q cache read price: %w", sourceModel, err)
		}
		price.CacheWriteNanoUSDPerToken, err = parsePrice(fields, "cache_creation_input_token_cost")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q cache write price: %w", sourceModel, err)
		}
		price.PerRequestNanoUSD, err = parsePrice(fields, "per_request_cost")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q per-request price: %w", sourceModel, err)
		}
		price.ImageNanoUSD, err = parsePrice(fields, "output_cost_per_image")
		if err != nil {
			return Catalog{}, fmt.Errorf("parse model %q image price: %w", sourceModel, err)
		}
		if price.ImageNanoUSD == nil {
			price.ImageNanoUSD, err = parseFirstPrice(fields, "input_cost_per_image_token", "output_cost_per_image_token", "image_cost_per_token")
			if err != nil {
				return Catalog{}, fmt.Errorf("parse model %q image price: %w", sourceModel, err)
			}
		}
		models[key] = price
	}
	if len(models) == 0 {
		return Catalog{}, fmt.Errorf("parse pricing catalog: no valid model entries")
	}
	sum := sha256.Sum256(raw)
	return Catalog{
		Version:   strings.TrimSpace(version),
		Hash:      hex.EncodeToString(sum[:]),
		UpdatedAt: time.Now().UTC(),
		Models:    models,
	}, nil
}

func parseFirstPrice(fields map[string]json.RawMessage, keys ...string) (*int64, error) {
	for _, key := range keys {
		value, err := parsePrice(fields, key)
		if err != nil || value != nil {
			return value, err
		}
	}
	return nil, nil
}

// Lookup finds a model by its normalized exact key.
func (c Catalog) Lookup(model string) (Price, bool) {
	if len(c.Models) == 0 {
		return Price{}, false
	}
	key := normalizeModel(model)
	price, ok := c.Models[key]
	if !ok {
		return Price{}, false
	}
	return price.clone(), true
}

// Override is a nullable account-specific price rule.
type Override struct {
	ID                        string
	AuthID                    string
	Provider                  string
	ModelPattern              string
	BillingMode               string
	InputNanoUSDPerToken      *int64
	OutputNanoUSDPerToken     *int64
	CacheReadNanoUSDPerToken  *int64
	CacheWriteNanoUSDPerToken *int64
	PerRequestNanoUSD         *int64
	ImageNanoUSD              *int64
	Priority                  int
	Enabled                   bool
	UpdatedAt                 time.Time
}

// ResolvedPrice contains the effective price and its provenance.
type ResolvedPrice struct {
	Price
	// Flattened fields keep the type convenient for callers using struct literals.
	InputNanoUSDPerToken      *int64
	OutputNanoUSDPerToken     *int64
	CacheReadNanoUSDPerToken  *int64
	CacheWriteNanoUSDPerToken *int64
	PerRequestNanoUSD         *int64
	ImageNanoUSD              *int64
	Source                    string
	Version                   string
}

// Resolver applies account overrides before the global catalog.
type Resolver struct {
	Catalog   Catalog
	Overrides []Override
}

// Resolve returns a price for authID/provider/model, if one is known.
func (r Resolver) Resolve(authID, provider, model string) (ResolvedPrice, bool) {
	base, found := r.Catalog.Lookup(model)
	selected, hasOverride := selectOverride(r.Overrides, authID, provider, model)
	if !found && !hasOverride {
		return ResolvedPrice{}, false
	}
	if !found {
		base = Price{ModelKey: normalizeModel(model), Provider: strings.ToLower(strings.TrimSpace(provider)), BillingMode: BillingModeToken}
	}
	if hasOverride {
		applyOverride(&base, selected)
		if base.ModelKey == "" {
			base.ModelKey = normalizeModel(model)
		}
		if base.Provider == "" {
			base.Provider = strings.ToLower(strings.TrimSpace(provider))
		}
		return resolvedPrice(base, "account_override", selected.ID), true
	}
	return resolvedPrice(base, "sub2api", r.Catalog.Version), true
}

func resolvedPrice(base Price, source, version string) ResolvedPrice {
	return ResolvedPrice{Price: base, InputNanoUSDPerToken: cloneInt64(base.InputNanoUSDPerToken), OutputNanoUSDPerToken: cloneInt64(base.OutputNanoUSDPerToken), CacheReadNanoUSDPerToken: cloneInt64(base.CacheReadNanoUSDPerToken), CacheWriteNanoUSDPerToken: cloneInt64(base.CacheWriteNanoUSDPerToken), PerRequestNanoUSD: cloneInt64(base.PerRequestNanoUSD), ImageNanoUSD: cloneInt64(base.ImageNanoUSD), Source: source, Version: version}
}

func selectOverride(overrides []Override, authID, provider, model string) (Override, bool) {
	authID = strings.TrimSpace(authID)
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = normalizeModel(model)
	type candidate struct {
		rule Override
		rank int
	}
	var candidates []candidate
	for _, rule := range overrides {
		if !rule.Enabled || strings.TrimSpace(rule.AuthID) != authID {
			continue
		}
		ruleProvider := strings.ToLower(strings.TrimSpace(rule.Provider))
		if ruleProvider != "" && ruleProvider != provider {
			continue
		}
		pattern := normalizeModel(rule.ModelPattern)
		rank := 0
		switch {
		case pattern == model:
			rank = 3
		case strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")):
			rank = 2
		case pattern == "*" || pattern == "" || pattern == "default":
			rank = 1
		}
		if rank > 0 {
			candidates = append(candidates, candidate{rule: rule, rank: rank})
		}
	}
	if len(candidates) == 0 {
		return Override{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank > candidates[j].rank
		}
		if candidates[i].rule.Priority != candidates[j].rule.Priority {
			return candidates[i].rule.Priority > candidates[j].rule.Priority
		}
		if !candidates[i].rule.UpdatedAt.Equal(candidates[j].rule.UpdatedAt) {
			return candidates[i].rule.UpdatedAt.After(candidates[j].rule.UpdatedAt)
		}
		return candidates[i].rule.ID < candidates[j].rule.ID
	})
	return candidates[0].rule, true
}

func applyOverride(dst *Price, rule Override) {
	if dst == nil {
		return
	}
	if mode := strings.TrimSpace(rule.BillingMode); mode != "" {
		dst.BillingMode = mode
	}
	if rule.InputNanoUSDPerToken != nil {
		dst.InputNanoUSDPerToken = cloneInt64(rule.InputNanoUSDPerToken)
	}
	if rule.OutputNanoUSDPerToken != nil {
		dst.OutputNanoUSDPerToken = cloneInt64(rule.OutputNanoUSDPerToken)
	}
	if rule.CacheReadNanoUSDPerToken != nil {
		dst.CacheReadNanoUSDPerToken = cloneInt64(rule.CacheReadNanoUSDPerToken)
	}
	if rule.CacheWriteNanoUSDPerToken != nil {
		dst.CacheWriteNanoUSDPerToken = cloneInt64(rule.CacheWriteNanoUSDPerToken)
	}
	if rule.PerRequestNanoUSD != nil {
		dst.PerRequestNanoUSD = cloneInt64(rule.PerRequestNanoUSD)
	}
	if rule.ImageNanoUSD != nil {
		dst.ImageNanoUSD = cloneInt64(rule.ImageNanoUSD)
	}
}

func optionalString(fields map[string]json.RawMessage, key string) (string, error) {
	raw, ok := fields[key]
	if !ok || string(raw) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func parsePrice(fields map[string]json.RawMessage, key string) (*int64, error) {
	raw, ok := fields[key]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return nil, err
	}
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("invalid numeric value %q", number.String())
	}
	if value < 0 {
		return nil, fmt.Errorf("price must be non-negative")
	}
	nano := value * 1_000_000_000
	if nano > float64(math.MaxInt64) {
		return nil, fmt.Errorf("price is too large")
	}
	rounded := math.Round(nano)
	result := int64(rounded)
	return &result, nil
}

func normalizeModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "models/")
	return strings.TrimSpace(model)
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
