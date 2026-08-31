package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetPricingStatus(c *gin.Context) {
	h.mu.Lock()
	service := h.pricingService
	h.mu.Unlock()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "pricing_unavailable"})
		return
	}
	c.JSON(http.StatusOK, service.Status())
}

func (h *Handler) GetPricingModels(c *gin.Context) {
	h.mu.Lock()
	service := h.pricingService
	h.mu.Unlock()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "pricing_unavailable"})
		return
	}
	items, total, err := service.Models(c.Query("search"), intQuery(c.Query("page"), 1), intQuery(c.Query("page_size"), 100))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) PostPricingSync(c *gin.Context) {
	h.mu.Lock()
	service := h.pricingService
	h.mu.Unlock()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "pricing_unavailable"})
		return
	}
	if err := service.Sync(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "sync_failed", "message": err.Error(), "status": service.Status()})
		return
	}
	c.JSON(http.StatusOK, service.Status())
}

func intQuery(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		value = value*10 + int(ch-'0')
		if value > 1000000 {
			return fallback
		}
	}
	if value == 0 {
		return fallback
	}
	return value
}
