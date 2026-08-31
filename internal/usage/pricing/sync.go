package pricing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultCatalogURL = "https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/backend/resources/model-pricing/model_prices_and_context_window.json"

// SyncStatus describes the last loaded/synchronized catalog.
type SyncStatus struct {
	URL          string    `json:"url"`
	Version      string    `json:"version"`
	Hash         string    `json:"hash"`
	UpdatedAt    time.Time `json:"updated_at"`
	ModelCount   int       `json:"model_count"`
	LastSyncAt   time.Time `json:"last_sync_at"`
	LastError    string    `json:"last_error,omitempty"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
}

// SyncService maintains a last-known-good sub2api-compatible catalog.
type SyncService struct {
	mu       sync.RWMutex
	catalog  Catalog
	status   SyncStatus
	url      string
	dir      string
	interval time.Duration
	client   *http.Client
	cancel   context.CancelFunc
	done     chan struct{}
	onUpdate func(Catalog)
}

func NewSyncService(url, dataDir string, interval time.Duration) *SyncService {
	url = strings.TrimSpace(url)
	if url == "" {
		url = DefaultCatalogURL
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	return &SyncService{url: url, dir: dataDir, interval: interval, client: &http.Client{Timeout: 30 * time.Second}, status: SyncStatus{URL: url}}
}

func (s *SyncService) Load() error {
	if s == nil {
		return errors.New("pricing: sync service is nil")
	}
	path := filepath.Join(s.dir, "model-prices.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("pricing: read catalog: %w", err)
	}
	version := "local"
	if meta, errMeta := os.ReadFile(filepath.Join(s.dir, "model-prices.meta.json")); errMeta == nil {
		var m struct{ Version, ETag, LastModified string }
		if json.Unmarshal(meta, &m) == nil {
			if m.Version != "" {
				version = m.Version
			}
			s.mu.Lock()
			s.status.ETag = m.ETag
			s.status.LastModified = m.LastModified
			s.mu.Unlock()
		}
	}
	catalog, err := ParseCatalog(raw, version)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.catalog = catalog
	s.status.Version = catalog.Version
	s.status.Hash = catalog.Hash
	s.status.UpdatedAt = catalog.UpdatedAt
	s.status.ModelCount = len(catalog.Models)
	callback := s.onUpdate
	s.mu.Unlock()
	if callback != nil {
		callback(catalog)
	}
	return nil
}

func (s *SyncService) Sync(ctx context.Context) error {
	if s == nil {
		return errors.New("pricing: sync service is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, s.url, nil)
	if err != nil {
		return err
	}
	s.mu.RLock()
	etag := s.status.ETag
	lastModified := s.status.LastModified
	s.mu.RUnlock()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.setError(err)
		return fmt.Errorf("pricing: download catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		s.mu.Lock()
		s.status.LastSyncAt = time.Now().UTC()
		s.status.LastError = ""
		s.mu.Unlock()
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("pricing source returned HTTP %d", resp.StatusCode)
		s.setError(err)
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		s.setError(err)
		return err
	}
	version := resp.Header.Get("ETag")
	if version == "" {
		sum := sha256.Sum256(raw)
		version = hex.EncodeToString(sum[:8])
	}
	catalog, err := ParseCatalog(raw, version)
	if err != nil {
		s.setError(err)
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		s.setError(err)
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "model-prices-*.tmp")
	if err != nil {
		s.setError(err)
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		s.setError(err)
		return err
	}
	if err = os.Rename(tmpName, filepath.Join(s.dir, "model-prices.json")); err != nil {
		s.setError(err)
		return err
	}
	meta := struct{ Version, ETag, LastModified string }{Version: version, ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	if data, errMeta := json.Marshal(meta); errMeta == nil {
		_ = os.WriteFile(filepath.Join(s.dir, "model-prices.meta.json"), data, 0o644)
	}
	s.mu.Lock()
	s.catalog = catalog
	s.status.Version = catalog.Version
	s.status.Hash = catalog.Hash
	s.status.UpdatedAt = catalog.UpdatedAt
	s.status.ModelCount = len(catalog.Models)
	s.status.LastSyncAt = time.Now().UTC()
	s.status.LastError = ""
	s.status.ETag = resp.Header.Get("ETag")
	s.status.LastModified = resp.Header.Get("Last-Modified")
	callback := s.onUpdate
	s.mu.Unlock()
	if callback != nil {
		callback(catalog)
	}
	return nil
}

func (s *SyncService) setError(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.status.LastSyncAt = time.Now().UTC()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *SyncService) Catalog() Catalog {
	if s == nil {
		return Catalog{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.catalog
	out.Models = make(map[string]Price, len(s.catalog.Models))
	for key, value := range s.catalog.Models {
		out.Models[key] = value.clone()
	}
	return out
}

// SetOnUpdate registers a callback invoked after a new catalog is committed.
func (s *SyncService) SetOnUpdate(callback func(Catalog)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onUpdate = callback
	s.mu.Unlock()
}
func (s *SyncService) Status() SyncStatus {
	if s == nil {
		return SyncStatus{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Models returns sorted model prices with optional substring search and pagination.
func (s *SyncService) Models(search string, page, pageSize int) ([]Price, int, error) {
	if s == nil {
		return nil, 0, errors.New("pricing: sync service is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		return nil, 0, errors.New("pricing: page_size must be <= 1000")
	}
	search = strings.ToLower(strings.TrimSpace(search))
	catalog := s.Catalog()
	keys := make([]string, 0, len(catalog.Models))
	for key := range catalog.Models {
		if search == "" || strings.Contains(key, search) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	total := len(keys)
	start := (page - 1) * pageSize
	if start >= total {
		return []Price{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]Price, 0, end-start)
	for _, key := range keys[start:end] {
		out = append(out, catalog.Models[key])
	}
	return out, total, nil
}

func (s *SyncService) Start(ctx context.Context) {
	if s == nil || s.cancel != nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				_ = s.Sync(runCtx)
			}
		}
	}()
}
func (s *SyncService) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
	s.cancel = nil
	s.done = nil
}
