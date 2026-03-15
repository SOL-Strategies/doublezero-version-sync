package versionsource

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newTestSource creates a Source pointed at the given test server URL.
func newTestSource(serverURL, cluster string) *Source {
	s := New(cluster)
	s.baseURL = serverURL
	return s
}

// makeHandler returns an HTTP handler that serves the given packages list as JSON.
func makeHandler(packages []cloudsmithPackage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(packages)
	}
}

func TestGetRecommendedVersion_ReturnsLatest(t *testing.T) {
	packages := []cloudsmithPackage{
		{Name: "doublezero", Version: "0.7.0-1", Format: "deb", StatusStr: "Completed"},
		{Name: "doublezero", Version: "0.7.1-1", Format: "deb", StatusStr: "Completed"},
		{Name: "doublezero", Version: "0.6.9-1", Format: "deb", StatusStr: "Completed"},
	}
	srv := httptest.NewServer(makeHandler(packages))
	defer srv.Close()

	src := newTestSource(srv.URL, "mainnet-beta")
	v, err := src.GetRecommendedVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Original() != "0.7.1-1" {
		t.Errorf("got %s, want 0.7.1-1", v.Original())
	}
}

func TestGetRecommendedVersion_FetchesOnEveryCall(t *testing.T) {
	// First call returns 0.7.1-1, second call returns 0.7.2-1.
	// If the old cache were still in place the second call would return 0.7.1-1.
	responses := [][]cloudsmithPackage{
		{{Name: "doublezero", Version: "0.7.1-1", Format: "deb", StatusStr: "Completed"}},
		{{Name: "doublezero", Version: "0.7.2-1", Format: "deb", StatusStr: "Completed"}},
	}
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(callCount.Add(1)) - 1
		if idx >= len(responses) {
			idx = len(responses) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses[idx])
	}))
	defer srv.Close()

	src := newTestSource(srv.URL, "mainnet-beta")

	v1, err := src.GetRecommendedVersion()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	v2, err := src.GetRecommendedVersion()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if v1.Original() != "0.7.1-1" {
		t.Errorf("first call: got %s, want 0.7.1-1", v1.Original())
	}
	if v2.Original() != "0.7.2-1" {
		t.Errorf("second call: got %s, want 0.7.2-1 (stale cache would return 0.7.1-1)", v2.Original())
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount.Load())
	}
}

func TestGetRecommendedVersion_IgnoresNonCompletedPackages(t *testing.T) {
	packages := []cloudsmithPackage{
		{Name: "doublezero", Version: "0.8.0-1", Format: "deb", StatusStr: "Uploading"},
		{Name: "doublezero", Version: "0.7.1-1", Format: "deb", StatusStr: "Completed"},
	}
	srv := httptest.NewServer(makeHandler(packages))
	defer srv.Close()

	src := newTestSource(srv.URL, "mainnet-beta")
	v, err := src.GetRecommendedVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Original() != "0.7.1-1" {
		t.Errorf("got %s, want 0.7.1-1 (non-completed package should be ignored)", v.Original())
	}
}

func TestGetRecommendedVersion_ErrorOnNoCompletedPackages(t *testing.T) {
	packages := []cloudsmithPackage{
		{Name: "doublezero", Version: "0.7.1-1", Format: "deb", StatusStr: "Uploading"},
	}
	srv := httptest.NewServer(makeHandler(packages))
	defer srv.Close()

	src := newTestSource(srv.URL, "mainnet-beta")
	_, err := src.GetRecommendedVersion()
	if err == nil {
		t.Fatal("expected error when no completed packages, got nil")
	}
}

func TestGetRecommendedVersion_ErrorOnNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	src := newTestSource(srv.URL, "mainnet-beta")
	_, err := src.GetRecommendedVersion()
	if err == nil {
		t.Fatal("expected error on non-200 response, got nil")
	}
}

func TestGetRecommendedVersion_ErrorOnUnknownCluster(t *testing.T) {
	src := New("unknown-cluster")
	_, err := src.GetRecommendedVersion()
	if err == nil {
		t.Fatal("expected error for unknown cluster, got nil")
	}
}

func TestGetRecommendedVersion_TestnetUsesTestnetRepo(t *testing.T) {
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		packages := []cloudsmithPackage{
			{Name: "doublezero", Version: "0.7.1-1", Format: "deb", StatusStr: "Completed"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(packages)
	}))
	defer srv.Close()

	src := newTestSource(srv.URL, "testnet")
	_, err := src.GetRecommendedVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestPath != "/doublezero-testnet/" {
		t.Errorf("got path %s, want /doublezero-testnet/", requestPath)
	}
}
