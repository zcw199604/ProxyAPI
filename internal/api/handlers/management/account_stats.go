package management

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	internalusage "github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
)

// GetAccountStats returns durable provider-attempt aggregates. No prompts,
// API keys, OAuth tokens, or upstream response bodies are exposed.
func (h *Handler) GetAccountStats(c *gin.Context) {
	h.mu.Lock()
	plugin := h.statsPlugin
	h.mu.Unlock()
	if plugin == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage_statistics_unavailable"})
		return
	}
	query, err := parseStatsQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_query", "message": err.Error()})
		return
	}
	items, total, err := plugin.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": query.Page, "page_size": query.PageSize})
}

func parseStatsQuery(c *gin.Context) (internalusage.Query, error) {
	q := internalusage.Query{AuthID: strings.TrimSpace(c.Query("auth_id")), Provider: strings.TrimSpace(c.Query("provider")), Model: strings.TrimSpace(c.Query("model")), GroupBy: strings.TrimSpace(c.Query("group_by")), Page: 1, PageSize: 100}
	parseTime := func(key string) (time.Time, error) {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			return time.Time{}, nil
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			return parsed, nil
		}
		return time.Parse("2006-01-02", raw)
	}
	var err error
	if q.From, err = parseTime("from"); err != nil {
		return q, err
	}
	if q.To, err = parseTime("to"); err != nil {
		return q, err
	}
	if q.From != (time.Time{}) && q.To != (time.Time{}) && !q.To.After(q.From) {
		return q, strconv.ErrSyntax
	}
	if raw := c.Query("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return q, strconv.ErrSyntax
		}
		q.Page = value
	}
	if raw := c.Query("page_size"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 1000 {
			return q, strconv.ErrRange
		}
		q.PageSize = value
	}
	return q, nil
}
