package usage

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestCalculateCostDoesNotDoubleCountReasoningTokens(t *testing.T) {
	inputPrice := int64(100)
	outputPrice := int64(200)
	price := pricing.ResolvedPrice{
		InputNanoUSDPerToken:  &inputPrice,
		OutputNanoUSDPerToken: &outputPrice,
	}
	detail := coreusage.Detail{
		InputTokens:     10,
		OutputTokens:    7,
		ReasoningTokens: 3,
		TotalTokens:     17,
		TokenBreakdown:  coreusage.NewSeparateReasoningTokenBreakdown(10, 0, 0, 4, 3, 17),
	}
	cost, status := calculateCost(detail, price)
	if status != pricingStatusPriced {
		t.Fatalf("calculateCost() status = %q, want priced", status)
	}
	// 10*100 + (4+3)*200 = 2400 nano-USD. Reasoning is already in output.
	if cost != 2400 {
		t.Fatalf("calculateCost() = %d, want 2400", cost)
	}
}

func TestCalculateCostMarksMissingPriceUnpriced(t *testing.T) {
	detail := coreusage.Detail{InputTokens: 10, TotalTokens: 10}
	cost, status := calculateCost(detail, pricing.ResolvedPrice{})
	if cost != 0 || status != pricingStatusUnpriced {
		t.Fatalf("calculateCost() = (%d, %q), want (0, unpriced)", cost, status)
	}
}
