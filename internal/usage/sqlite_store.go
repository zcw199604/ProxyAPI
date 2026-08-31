package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage/pricing"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
	_ "modernc.org/sqlite"
)

const sqliteSchemaVersion = 3

// SQLiteStore is the default single-instance durable statistics store.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLiteStore opens (and migrates) a SQLite statistics database.
func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("usage: sqlite path is empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("usage: create sqlite directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("usage: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// NewSQLiteStore is an alias for OpenSQLiteStore for embedders following a constructor naming convention.
func NewSQLiteStore(path string) (*SQLiteStore, error) { return OpenSQLiteStore(path) }

func (s *SQLiteStore) migrate() error {
	if s == nil || s.db == nil {
		return errors.New("usage: sqlite store is nil")
	}
	statements := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS usage_schema (version INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS usage_events (
			event_id TEXT PRIMARY KEY,
			requested_at TEXT NOT NULL,
			day_key TEXT NOT NULL,
			auth_id TEXT NOT NULL,
			auth_index TEXT,
			account_label TEXT,
			provider TEXT NOT NULL,
			auth_type TEXT,
			model TEXT NOT NULL,
			alias TEXT,
			success INTEGER NOT NULL,
			failure_status INTEGER NOT NULL,
			detail_json BLOB NOT NULL,
			price_json BLOB,
			cost_nano_usd INTEGER NOT NULL,
			pricing_status TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_requested ON usage_events(requested_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_account ON usage_events(auth_id, requested_at)`,
		`CREATE TABLE IF NOT EXISTS usage_daily (
			day_key TEXT NOT NULL,
			auth_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			failed_count INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_write_tokens INTEGER NOT NULL DEFAULT 0,
			unclassified_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			priced_request_count INTEGER NOT NULL DEFAULT 0,
			unpriced_request_count INTEGER NOT NULL DEFAULT 0,
			total_cost_nano_usd INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY(day_key, auth_id, provider, model)
		)`,
		`CREATE TABLE IF NOT EXISTS usage_price_overrides (
			id TEXT PRIMARY KEY,
			auth_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT '',
			model_pattern TEXT NOT NULL,
			billing_mode TEXT NOT NULL DEFAULT '',
			input_nano_usd INTEGER,
			output_nano_usd INTEGER,
			cache_read_nano_usd INTEGER,
			cache_write_nano_usd INTEGER,
			per_request_nano_usd INTEGER,
			image_nano_usd INTEGER,
			priority INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("usage: migrate sqlite: %w", err)
		}
	}
	var version int
	err := s.db.QueryRow(`SELECT version FROM usage_schema LIMIT 1`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.Exec(`INSERT INTO usage_schema(version) VALUES (?)`, sqliteSchemaVersion)
	} else if err == nil && version != sqliteSchemaVersion {
		_, err = s.db.Exec(`UPDATE usage_schema SET version = ?`, sqliteSchemaVersion)
	}
	if err != nil {
		return fmt.Errorf("usage: update sqlite schema version: %w", err)
	}
	return nil
}

func (s *SQLiteStore) AppendEvent(event Event) error {
	if s == nil || s.db == nil {
		return errors.New("usage: sqlite store is nil")
	}
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = uuid.NewString()
	}
	requestedAt := event.RequestedAt.UTC()
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	dayKey := requestedAt.Format("2006-01-02")
	authID := strings.TrimSpace(event.AuthID)
	if authID == "" {
		authID = "unknown"
	}
	provider := strings.TrimSpace(event.Provider)
	model := strings.TrimSpace(event.Model)
	if provider == "" {
		provider = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	detail, err := json.Marshal(event.Detail)
	if err != nil {
		return fmt.Errorf("usage: marshal detail: %w", err)
	}
	var priceJSON []byte
	if event.Price.Source != "" || event.Price.ModelKey != "" {
		priceJSON, err = json.Marshal(event.Price)
		if err != nil {
			return fmt.Errorf("usage: marshal price: %w", err)
		}
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("usage: begin usage transaction: %w", err)
	}
	result, err := tx.Exec(`INSERT OR IGNORE INTO usage_events
		(event_id, requested_at, day_key, auth_id, auth_index, account_label, provider, auth_type, model, alias,
		success, failure_status, detail_json, price_json, cost_nano_usd, pricing_status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventID, requestedAt.Format(time.RFC3339Nano), dayKey, authID, event.AuthIndex, event.AccountLabel,
		provider, event.AuthType, model, event.Alias, boolInt(event.Success), event.FailureStatus, detail,
		priceJSON, event.CostNanoUSD, event.PricingStatus, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("usage: insert event: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return tx.Commit()
	}
	b := normalizedBreakdown(event.Detail)
	_, err = tx.Exec(`INSERT INTO usage_daily
		(day_key, auth_id, provider, model, request_count, success_count, failed_count,
		 input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens,
		 unclassified_tokens, total_tokens, priced_request_count, unpriced_request_count, total_cost_nano_usd)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(day_key, auth_id, provider, model) DO UPDATE SET
		request_count = request_count + 1,
		success_count = success_count + excluded.success_count,
		failed_count = failed_count + excluded.failed_count,
		input_tokens = input_tokens + excluded.input_tokens,
		output_tokens = output_tokens + excluded.output_tokens,
		reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
		cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
		cache_write_tokens = cache_write_tokens + excluded.cache_write_tokens,
		unclassified_tokens = unclassified_tokens + excluded.unclassified_tokens,
		total_tokens = total_tokens + excluded.total_tokens,
		priced_request_count = priced_request_count + excluded.priced_request_count,
		unpriced_request_count = unpriced_request_count + excluded.unpriced_request_count,
		total_cost_nano_usd = total_cost_nano_usd + excluded.total_cost_nano_usd`,
		dayKey, authID, provider, model, boolInt(event.Success), boolInt(!event.Success), b.Input.TotalTokens, b.Output.TotalTokens,
		b.Output.ReasoningTokens, b.Input.CacheReadTokens, b.Input.CacheWriteTokens, b.UnclassifiedTokens,
		b.TotalTokens, pricedInt(event.PricingStatus), unpricedInt(event.PricingStatus), event.CostNanoUSD)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("usage: update daily aggregate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("usage: commit usage event: %w", err)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func pricedInt(status string) int {
	if status == pricingStatusPriced {
		return 1
	}
	return 0
}
func unpricedInt(status string) int {
	if status == pricingStatusUnpriced {
		return 1
	}
	return 0
}

func (s *SQLiteStore) QuerySummary(query Query) ([]Summary, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, errors.New("usage: sqlite store is nil")
	}
	where := []string{"1=1"}
	args := []any{}
	if !query.From.IsZero() {
		where = append(where, "day_key >= ?")
		args = append(args, query.From.UTC().Format("2006-01-02"))
	}
	if !query.To.IsZero() {
		where = append(where, "day_key < ?")
		args = append(args, query.To.UTC().Format("2006-01-02"))
	}
	if strings.TrimSpace(query.AuthID) != "" {
		where = append(where, "auth_id = ?")
		args = append(args, strings.TrimSpace(query.AuthID))
	}
	if strings.TrimSpace(query.Provider) != "" {
		where = append(where, "provider = ?")
		args = append(args, strings.TrimSpace(query.Provider))
	}
	if strings.TrimSpace(query.Model) != "" {
		where = append(where, "model = ?")
		args = append(args, strings.TrimSpace(query.Model))
	}
	group := "auth_id, provider"
	selectBucket := "''"
	switch strings.ToLower(strings.TrimSpace(query.GroupBy)) {
	case "day":
		group = "day_key, auth_id, provider"
		selectBucket = "day_key"
	case "model":
		group = "auth_id, provider, model"
	case "day_model", "day-model":
		group = "day_key, auth_id, provider, model"
		selectBucket = "day_key"
	}
	base := ` FROM usage_daily WHERE ` + strings.Join(where, " AND ")
	countQuery := `SELECT COUNT(*) FROM (SELECT ` + group + base + ` GROUP BY ` + group + `)`
	var total int64
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("usage: count summaries: %w", err)
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		return nil, 0, errors.New("usage: page_size must be <= 1000")
	}
	querySQL := `SELECT ` + selectBucket + ` AS bucket, auth_id, provider, CASE WHEN instr('` + group + `','model') > 0 THEN model ELSE '' END,
		SUM(request_count), SUM(success_count), SUM(failed_count), SUM(input_tokens), SUM(output_tokens), SUM(reasoning_tokens),
		SUM(cache_read_tokens), SUM(cache_write_tokens), SUM(unclassified_tokens), SUM(total_tokens), SUM(priced_request_count),
		SUM(unpriced_request_count), SUM(total_cost_nano_usd)` + base + ` GROUP BY ` + group + ` ORDER BY auth_id, provider, bucket LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("usage: query summaries: %w", err)
	}
	defer rows.Close()
	var out []Summary
	for rows.Next() {
		var item Summary
		if err := rows.Scan(&item.Bucket, &item.AuthID, &item.Provider, &item.Model, &item.RequestCount, &item.SuccessCount, &item.FailedCount,
			&item.InputTokens, &item.OutputTokens, &item.ReasoningTokens, &item.CacheReadTokens, &item.CacheWriteTokens,
			&item.UnclassifiedTokens, &item.TotalTokens, &item.PricedRequestCount, &item.UnpricedRequestCount, &item.TotalCostNanoUSD); err != nil {
			return nil, 0, fmt.Errorf("usage: scan summary: %w", err)
		}
		item.TotalCostUSD = formatUSD(item.TotalCostNanoUSD)
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// QueryEventSummary aggregates immutable usage events using exact timestamp
// bounds. It is used for rolling quota windows where day-level aggregation
// would include requests outside the requested interval.
func (s *SQLiteStore) QueryEventSummary(query Query) ([]Summary, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, errors.New("usage: sqlite store is nil")
	}
	where := []string{"1=1"}
	args := []any{}
	if !query.From.IsZero() {
		where = append(where, "requested_at >= ?")
		args = append(args, query.From.UTC().Format(time.RFC3339Nano))
	}
	if !query.To.IsZero() {
		where = append(where, "requested_at < ?")
		args = append(args, query.To.UTC().Format(time.RFC3339Nano))
	}
	if authID := strings.TrimSpace(query.AuthID); authID != "" {
		where = append(where, "auth_id = ?")
		args = append(args, authID)
	}
	if provider := strings.TrimSpace(query.Provider); provider != "" {
		where = append(where, "provider = ?")
		args = append(args, provider)
	}
	if model := strings.TrimSpace(query.Model); model != "" {
		where = append(where, "model = ?")
		args = append(args, model)
	}

	rows, err := s.db.Query(`SELECT event_id, day_key, auth_id, provider, model, success,
		detail_json, cost_nano_usd, pricing_status FROM usage_events WHERE `+strings.Join(where, " AND ")+` ORDER BY requested_at, event_id`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("usage: query event summaries: %w", err)
	}
	defer rows.Close()

	groupBy := strings.ToLower(strings.TrimSpace(query.GroupBy))
	if groupBy == "day-model" {
		groupBy = "day_model"
	}
	type aggregate struct {
		Summary
		key string
	}
	aggregates := make(map[string]*aggregate)
	for rows.Next() {
		var eventID, day, authID, provider, model, status string
		var success, cost int64
		var detailJSON []byte
		if err := rows.Scan(&eventID, &day, &authID, &provider, &model, &success, &detailJSON, &cost, &status); err != nil {
			return nil, 0, fmt.Errorf("usage: scan event summary: %w", err)
		}
		key := eventID
		bucket := eventID
		itemModel := model
		switch groupBy {
		case "day":
			key = day + "\x00" + authID + "\x00" + provider
			bucket = day
			itemModel = ""
		case "model":
			key = authID + "\x00" + provider + "\x00" + model
			bucket = ""
		case "day_model":
			key = day + "\x00" + authID + "\x00" + provider + "\x00" + model
			bucket = day
		default:
			key = authID + "\x00" + provider
			bucket = ""
			itemModel = ""
		}
		item := aggregates[key]
		if item == nil {
			item = &aggregate{Summary: Summary{Bucket: bucket, AuthID: authID, Provider: provider, Model: itemModel}, key: key}
			aggregates[key] = item
		}
		item.RequestCount++
		if success != 0 {
			item.SuccessCount++
		} else {
			item.FailedCount++
		}
		var detail coreusage.Detail
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			return nil, 0, fmt.Errorf("usage: decode event detail: %w", err)
		}
		breakdown := normalizedBreakdown(detail)
		item.InputTokens += breakdown.Input.TotalTokens
		item.OutputTokens += breakdown.Output.TotalTokens
		item.ReasoningTokens += breakdown.Output.ReasoningTokens
		item.CacheReadTokens += breakdown.Input.CacheReadTokens
		item.CacheWriteTokens += breakdown.Input.CacheWriteTokens
		item.UnclassifiedTokens += breakdown.UnclassifiedTokens
		item.TotalTokens += breakdown.TotalTokens
		if status == pricingStatusPriced {
			item.PricedRequestCount++
		} else if status == pricingStatusUnpriced {
			item.UnpricedRequestCount++
		}
		item.TotalCostNanoUSD += cost
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("usage: iterate event summaries: %w", err)
	}

	ordered := make([]*aggregate, 0, len(aggregates))
	for _, item := range aggregates {
		item.TotalCostUSD = formatUSD(item.TotalCostNanoUSD)
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		return nil, 0, errors.New("usage: page_size must be <= 1000")
	}
	start := (page - 1) * pageSize
	if start >= len(ordered) {
		return []Summary{}, int64(len(ordered)), nil
	}
	end := start + pageSize
	if end > len(ordered) {
		end = len(ordered)
	}
	out := make([]Summary, 0, end-start)
	for _, item := range ordered[start:end] {
		out = append(out, item.Summary)
	}
	return out, int64(len(ordered)), nil
}

func formatUSD(nano int64) string { return fmt.Sprintf("%.9f", float64(nano)/1_000_000_000) }

func (s *SQLiteStore) ListOverrides(authID string) ([]pricing.Override, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("usage: sqlite store is nil")
	}
	query := `SELECT id, auth_id, provider, model_pattern, billing_mode, input_nano_usd, output_nano_usd, cache_read_nano_usd,
		cache_write_nano_usd, per_request_nano_usd, image_nano_usd, priority, enabled, updated_at FROM usage_price_overrides`
	args := []any{}
	if strings.TrimSpace(authID) != "" {
		query += " WHERE auth_id = ?"
		args = append(args, strings.TrimSpace(authID))
	}
	query += " ORDER BY priority DESC, updated_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pricing.Override
	for rows.Next() {
		var o pricing.Override
		var enabled int
		var updated string
		var in, outp, cr, cw, req, img sql.NullInt64
		if err := rows.Scan(&o.ID, &o.AuthID, &o.Provider, &o.ModelPattern, &o.BillingMode, &in, &outp, &cr, &cw, &req, &img, &o.Priority, &enabled, &updated); err != nil {
			return nil, err
		}
		o.InputNanoUSDPerToken = nullInt64(in)
		o.OutputNanoUSDPerToken = nullInt64(outp)
		o.CacheReadNanoUSDPerToken = nullInt64(cr)
		o.CacheWriteNanoUSDPerToken = nullInt64(cw)
		o.PerRequestNanoUSD = nullInt64(req)
		o.ImageNanoUSD = nullInt64(img)
		o.Enabled = enabled != 0
		o.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, o)
	}
	return out, rows.Err()
}

func nullInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func (s *SQLiteStore) UpsertOverride(o pricing.Override) (pricing.Override, error) {
	if s == nil || s.db == nil {
		return pricing.Override{}, errors.New("usage: sqlite store is nil")
	}
	if strings.TrimSpace(o.ID) == "" {
		o.ID = uuid.NewString()
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO usage_price_overrides(id, auth_id, provider, model_pattern, billing_mode, input_nano_usd, output_nano_usd,
		cache_read_nano_usd, cache_write_nano_usd, per_request_nano_usd, image_nano_usd, priority, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET auth_id=excluded.auth_id, provider=excluded.provider, model_pattern=excluded.model_pattern,
		billing_mode=excluded.billing_mode, input_nano_usd=excluded.input_nano_usd, output_nano_usd=excluded.output_nano_usd,
		cache_read_nano_usd=excluded.cache_read_nano_usd, cache_write_nano_usd=excluded.cache_write_nano_usd,
		per_request_nano_usd=excluded.per_request_nano_usd, image_nano_usd=excluded.image_nano_usd, priority=excluded.priority,
		enabled=excluded.enabled, updated_at=excluded.updated_at`, o.ID, strings.TrimSpace(o.AuthID), strings.ToLower(strings.TrimSpace(o.Provider)), strings.ToLower(strings.TrimSpace(o.ModelPattern)), o.BillingMode,
		o.InputNanoUSDPerToken, o.OutputNanoUSDPerToken, o.CacheReadNanoUSDPerToken, o.CacheWriteNanoUSDPerToken, o.PerRequestNanoUSD, o.ImageNanoUSD, o.Priority, boolInt(o.Enabled), o.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return pricing.Override{}, fmt.Errorf("usage: upsert override: %w", err)
	}
	return o, nil
}

func (s *SQLiteStore) DisableOverride(id string) error {
	if s == nil || s.db == nil {
		return errors.New("usage: sqlite store is nil")
	}
	result, err := s.db.Exec(`UPDATE usage_price_overrides SET enabled = 0, updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RebuildAggregates recomputes the daily table from the immutable event log.
func (s *SQLiteStore) RebuildAggregates() error {
	if s == nil || s.db == nil {
		return errors.New("usage: sqlite store is nil")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM usage_daily`); err != nil {
		_ = tx.Rollback()
		return err
	}
	rows, err := tx.Query(`SELECT day_key, auth_id, provider, model, success, detail_json, cost_nano_usd, pricing_status FROM usage_events`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day, authID, provider, model, status string
		var success, cost int64
		var detailJSON []byte
		if err := rows.Scan(&day, &authID, &provider, &model, &success, &detailJSON, &cost, &status); err != nil {
			_ = tx.Rollback()
			return err
		}
		var detail coreusage.Detail
		if err := json.Unmarshal(detailJSON, &detail); err != nil {
			_ = tx.Rollback()
			return err
		}
		b := normalizedBreakdown(detail)
		if _, err := tx.Exec(`INSERT INTO usage_daily(day_key, auth_id, provider, model, request_count, success_count, failed_count, input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens, unclassified_tokens, total_tokens, priced_request_count, unpriced_request_count, total_cost_nano_usd) VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(day_key, auth_id, provider, model) DO UPDATE SET request_count=request_count+1, success_count=success_count+excluded.success_count, failed_count=failed_count+excluded.failed_count, input_tokens=input_tokens+excluded.input_tokens, output_tokens=output_tokens+excluded.output_tokens, reasoning_tokens=reasoning_tokens+excluded.reasoning_tokens, cache_read_tokens=cache_read_tokens+excluded.cache_read_tokens, cache_write_tokens=cache_write_tokens+excluded.cache_write_tokens, unclassified_tokens=unclassified_tokens+excluded.unclassified_tokens, total_tokens=total_tokens+excluded.total_tokens, priced_request_count=priced_request_count+excluded.priced_request_count, unpriced_request_count=unpriced_request_count+excluded.unpriced_request_count, total_cost_nano_usd=total_cost_nano_usd+excluded.total_cost_nano_usd`, day, authID, provider, model, success, 1-success, b.Input.TotalTokens, b.Output.TotalTokens, b.Output.ReasoningTokens, b.Input.CacheReadTokens, b.Input.CacheWriteTokens, b.UnclassifiedTokens, b.TotalTokens, pricedInt(status), unpricedInt(status), cost); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// PurgeBefore removes old immutable events and rebuilds daily aggregates.
func (s *SQLiteStore) PurgeBefore(cutoff time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("usage: sqlite store is nil")
	}
	if _, err := s.db.Exec(`DELETE FROM usage_events WHERE requested_at < ?`, cutoff.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return s.RebuildAggregates()
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
