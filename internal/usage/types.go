package usage

import (
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

// Event is the normalized, persistent representation of one provider attempt.
type Event struct {
	EventID       string
	RequestedAt   time.Time
	AuthID        string
	AuthIndex     string
	AccountLabel  string
	Provider      string
	AuthType      string
	RequestID     string
	Model         string
	Alias         string
	Success       bool
	FailureStatus int
	Detail        coreusage.Detail
	Price         pricing.ResolvedPrice
	CostNanoUSD   int64
	PricingStatus string
}

// Query controls account usage aggregation.
type Query struct {
	From     time.Time
	To       time.Time
	AuthID   string
	Provider string
	Model    string
	GroupBy  string
	Page     int
	PageSize int
}

// Summary is an aggregate over one account/model/day bucket.
type Summary struct {
	Bucket               string `json:"bucket,omitempty"`
	AuthID               string `json:"auth_id"`
	Provider             string `json:"provider"`
	Model                string `json:"model,omitempty"`
	RequestCount         int64  `json:"request_count"`
	SuccessCount         int64  `json:"success_count"`
	FailedCount          int64  `json:"failed_count"`
	InputTokens          int64  `json:"input_tokens"`
	OutputTokens         int64  `json:"output_tokens"`
	ReasoningTokens      int64  `json:"reasoning_tokens"`
	CacheReadTokens      int64  `json:"cache_read_tokens"`
	CacheWriteTokens     int64  `json:"cache_write_tokens"`
	UnclassifiedTokens   int64  `json:"unclassified_tokens"`
	TotalTokens          int64  `json:"total_tokens"`
	PricedRequestCount   int64  `json:"priced_request_count"`
	UnpricedRequestCount int64  `json:"unpriced_request_count"`
	TotalCostNanoUSD     int64  `json:"-"`
	TotalCostUSD         string `json:"total_cost_usd"`
}

// StatsStore persists events and exposes aggregate queries.
type StatsStore interface {
	AppendEvent(event Event) error
	QuerySummary(query Query) ([]Summary, int64, error)
	ListOverrides(authID string) ([]pricing.Override, error)
	UpsertOverride(override pricing.Override) (pricing.Override, error)
	DisableOverride(id string) error
	Close() error
}
