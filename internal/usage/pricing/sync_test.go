package pricing

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncServiceLoadsAndUsesETag(t *testing.T) {
	raw := `{"gpt-test":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000002}}`
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()
	dir := t.TempDir()
	service := NewSyncService(server.URL, dir, time.Hour)
	if err := service.Sync(nil); err != nil {
		t.Fatal(err)
	}
	if service.Status().ModelCount != 1 || requests != 1 {
		t.Fatalf("status = %+v requests=%d", service.Status(), requests)
	}
	if err := service.Sync(nil); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || service.Status().LastError != "" {
		t.Fatalf("etag sync status = %+v requests=%d", service.Status(), requests)
	}
	if _, err := os.Stat(filepath.Join(dir, "model-prices.json")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncServiceKeepsLastGoodCatalogOnInvalidResponse(t *testing.T) {
	valid := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if valid {
			_, _ = w.Write([]byte(`{"gpt-test":{"input_cost_per_token":0.000001}}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	service := NewSyncService(server.URL, t.TempDir(), time.Hour)
	if err := service.Sync(nil); err != nil {
		t.Fatal(err)
	}
	valid = false
	if err := service.Sync(nil); err == nil {
		t.Fatal("Sync() error = nil for invalid response")
	}
	if _, ok := service.Catalog().Lookup("gpt-test"); !ok {
		t.Fatal("last good catalog was replaced")
	}
}
