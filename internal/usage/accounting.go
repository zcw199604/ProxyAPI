// Package usage persists request usage and computes account-level statistics.
package usage

import (
	"math"
	"math/big"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

const (
	pricingStatusPriced   = "priced"
	pricingStatusUnpriced = "unpriced"
	PricingStatusPriced   = pricingStatusPriced
	PricingStatusUnpriced = pricingStatusUnpriced
)

// calculateCost computes token-mode cost from the canonical non-overlapping breakdown.
func calculateCost(detail coreusage.Detail, price pricing.ResolvedPrice) (int64, string) {
	price = effectivePrice(price)
	breakdown := normalizedBreakdown(detail)
	if !breakdown.Valid() {
		return 0, pricingStatusUnpriced
	}
	if !hasAnyPrice(price) {
		return 0, pricingStatusUnpriced
	}

	total := new(big.Int)
	priced := true
	add := func(tokens int64, unitPrice *int64) {
		if tokens <= 0 {
			return
		}
		if unitPrice == nil {
			priced = false
			return
		}
		term := new(big.Int).Mul(big.NewInt(tokens), big.NewInt(*unitPrice))
		total.Add(total, term)
	}
	add(breakdown.Input.UncachedTokens, price.InputNanoUSDPerToken)
	add(breakdown.Input.CacheReadTokens, price.CacheReadNanoUSDPerToken)
	add(breakdown.Input.CacheWriteTokens, price.CacheWriteNanoUSDPerToken)
	add(breakdown.Output.TotalTokens, price.OutputNanoUSDPerToken)
	if !priced || !total.IsInt64() {
		return 0, pricingStatusUnpriced
	}
	return total.Int64(), pricingStatusPriced
}

func normalizedBreakdown(detail coreusage.Detail) coreusage.TokenBreakdown {
	if detail.TokenBreakdown.Valid() {
		return detail.TokenBreakdown
	}
	input := detail.InputTokens
	output := detail.OutputTokens
	if input < 0 {
		input = 0
	}
	if output < 0 {
		output = 0
	}
	cacheRead := detail.CacheReadTokens
	cacheWrite := detail.CacheCreationTokens
	if cacheRead < 0 {
		cacheRead = 0
	}
	if cacheWrite < 0 {
		cacheWrite = 0
	}
	if cacheRead > input || cacheWrite > input-cacheRead {
		cacheRead, cacheWrite = 0, 0
	}
	reasoning := detail.ReasoningTokens
	if reasoning < 0 || reasoning > output {
		reasoning = 0
	}
	total := detail.TotalTokens
	if total <= 0 {
		if output > math.MaxInt64-input {
			total = 0
		} else {
			total = input + output
		}
	}
	return coreusage.NewSubsetTokenBreakdown(input, cacheRead, cacheWrite, output, reasoning, total)
}

func hasAnyPrice(price pricing.ResolvedPrice) bool {
	price = effectivePrice(price)
	return price.InputNanoUSDPerToken != nil ||
		price.OutputNanoUSDPerToken != nil ||
		price.CacheReadNanoUSDPerToken != nil ||
		price.CacheWriteNanoUSDPerToken != nil ||
		price.PerRequestNanoUSD != nil ||
		price.ImageNanoUSD != nil
}

func calculateEventCost(detail coreusage.Detail, price pricing.ResolvedPrice, requestCount, imageCount int) (int64, string) {
	price = effectivePrice(price)
	if requestCount <= 0 {
		requestCount = 1
	}
	mode := price.BillingMode
	if mode == pricing.BillingModePerRequest {
		if price.PerRequestNanoUSD == nil {
			return 0, pricingStatusUnpriced
		}
		return multiplyPrice(int64(requestCount), *price.PerRequestNanoUSD)
	}
	if mode == pricing.BillingModeImage {
		if imageCount <= 0 {
			imageCount = requestCount
		}
		if price.ImageNanoUSD == nil {
			return 0, pricingStatusUnpriced
		}
		return multiplyPrice(int64(imageCount), *price.ImageNanoUSD)
	}
	return calculateCost(detail, price)
}

func effectivePrice(price pricing.ResolvedPrice) pricing.ResolvedPrice {
	if price.InputNanoUSDPerToken == nil && price.OutputNanoUSDPerToken == nil && price.CacheReadNanoUSDPerToken == nil && price.CacheWriteNanoUSDPerToken == nil && price.PerRequestNanoUSD == nil && price.ImageNanoUSD == nil && price.Price.ModelKey != "" {
		price.InputNanoUSDPerToken = price.Price.InputNanoUSDPerToken
		price.OutputNanoUSDPerToken = price.Price.OutputNanoUSDPerToken
		price.CacheReadNanoUSDPerToken = price.Price.CacheReadNanoUSDPerToken
		price.CacheWriteNanoUSDPerToken = price.Price.CacheWriteNanoUSDPerToken
		price.PerRequestNanoUSD = price.Price.PerRequestNanoUSD
		price.ImageNanoUSD = price.Price.ImageNanoUSD
	}
	return price
}

func multiplyPrice(count, unit int64) (int64, string) {
	if count < 0 || unit < 0 {
		return 0, pricingStatusUnpriced
	}
	result := new(big.Int).Mul(big.NewInt(count), big.NewInt(unit))
	if !result.IsInt64() || result.Sign() < 0 || result.Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return 0, pricingStatusUnpriced
	}
	return result.Int64(), pricingStatusPriced
}
