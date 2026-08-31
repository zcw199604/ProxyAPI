package management

import (
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
)

type accountPricingRequest struct {
	ID              string  `json:"id"`
	AuthID          string  `json:"auth_id"`
	AuthIndex       string  `json:"auth_index"`
	Provider        string  `json:"provider"`
	ModelPattern    string  `json:"model_pattern" binding:"required"`
	BillingMode     string  `json:"billing_mode"`
	InputPrice      *string `json:"input_price"`
	OutputPrice     *string `json:"output_price"`
	CacheReadPrice  *string `json:"cache_read_price"`
	CacheWritePrice *string `json:"cache_write_price"`
	PerRequestPrice *string `json:"per_request_price"`
	ImagePrice      *string `json:"image_price"`
	Priority        int     `json:"priority"`
	Enabled         *bool   `json:"enabled"`
}

func (h *Handler) GetAccountPricing(c *gin.Context) {
	h.mu.Lock()
	plugin := h.statsPlugin
	h.mu.Unlock()
	if plugin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage_statistics_unavailable"})
		return
	}
	overrides, err := plugin.Store().ListOverrides(h.resolveAuthID(c.Query("auth_id"), c.Query("auth_index")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": overrides})
}

func (h *Handler) PutAccountPricing(c *gin.Context) {
	h.mu.Lock()
	plugin := h.statsPlugin
	h.mu.Unlock()
	if plugin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage_statistics_unavailable"})
		return
	}
	var req accountPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = strings.TrimSpace(c.Param("id"))
	}
	if strings.TrimSpace(req.AuthID) == "" {
		req.AuthID = h.resolveAuthID(req.AuthID, req.AuthIndex)
	}
	o, err := pricingRequestToOverride(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_price", "message": err.Error()})
		return
	}
	stored, err := plugin.Store().UpsertOverride(o)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stored)
}

func (h *Handler) resolveAuthID(authID, authIndex string) string {
	authID = strings.TrimSpace(authID)
	if authID != "" {
		return authID
	}
	authIndex = strings.TrimSpace(authIndex)
	if authIndex == "" || h == nil || h.authManager == nil {
		return authID
	}
	for _, auth := range h.authManager.List() {
		if auth != nil && strings.EqualFold(strings.TrimSpace(auth.EnsureIndex()), authIndex) {
			return strings.TrimSpace(auth.ID)
		}
	}
	return authID
}

func (h *Handler) DeleteAccountPricing(c *gin.Context) {
	h.mu.Lock()
	plugin := h.statsPlugin
	h.mu.Unlock()
	if plugin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage_statistics_unavailable"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		id = strings.TrimSpace(c.Query("id"))
	}
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_required"})
		return
	}
	if err := plugin.Store().DisableOverride(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

func pricingRequestToOverride(req accountPricingRequest) (pricing.Override, error) {
	authID := strings.TrimSpace(req.AuthID)
	pattern := strings.ToLower(strings.TrimSpace(req.ModelPattern))
	if authID == "" {
		return pricing.Override{}, &priceError{"auth_id or a valid auth_index is required"}
	}
	o := pricing.Override{ID: strings.TrimSpace(req.ID), AuthID: authID, Provider: strings.TrimSpace(req.Provider), ModelPattern: pattern, BillingMode: strings.TrimSpace(req.BillingMode), Priority: req.Priority, Enabled: req.Enabled == nil || *req.Enabled, UpdatedAt: time.Now().UTC()}
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	var err error
	if o.InputNanoUSDPerToken, err = parsePriceString(req.InputPrice); err != nil {
		return o, err
	}
	if o.OutputNanoUSDPerToken, err = parsePriceString(req.OutputPrice); err != nil {
		return o, err
	}
	if o.CacheReadNanoUSDPerToken, err = parsePriceString(req.CacheReadPrice); err != nil {
		return o, err
	}
	if o.CacheWriteNanoUSDPerToken, err = parsePriceString(req.CacheWritePrice); err != nil {
		return o, err
	}
	if o.PerRequestNanoUSD, err = parsePriceString(req.PerRequestPrice); err != nil {
		return o, err
	}
	if o.ImageNanoUSD, err = parsePriceString(req.ImagePrice); err != nil {
		return o, err
	}
	return o, nil
}

type priceError struct{ message string }

func (e *priceError) Error() string { return e.message }

func parsePriceString(raw *string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" || strings.Contains(value, "/") {
		return nil, &priceError{"price cannot be empty"}
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || rat.Sign() < 0 {
		return nil, &priceError{"price must be a non-negative decimal"}
	}
	rat.Mul(rat, big.NewRat(1_000_000_000, 1))
	if rat.Denom().Cmp(big.NewInt(1)) != 0 || !rat.Num().IsInt64() {
		return nil, &priceError{"price supports at most 9 decimal places"}
	}
	result := rat.Num().Int64()
	return &result, nil
}
