package selectcache

import (
	"net/http"
	"testing"
)

// TestGETAndHEADShareCacheEntries verifies that GET and HEAD requests
// to the same URL generate the same cache key (for HEAD to reuse GET cache)
func TestGETAndHEADShareCacheEntries(t *testing.T) {
	middleware := &Middleware{}

	// Create GET request
	reqGET, err := http.NewRequest("GET", "http://example.com/api/data?id=123", nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	reqGET.Header.Set("Accept", "application/json")

	// Create HEAD request to same URL
	reqHEAD, err := http.NewRequest("HEAD", "http://example.com/api/data?id=123", nil)
	if err != nil {
		t.Fatalf("Failed to create HEAD request: %v", err)
	}
	reqHEAD.Header.Set("Accept", "application/json")

	// Generate cache keys
	getKey := middleware.createCacheKey(reqGET)
	headKey := middleware.createCacheKey(reqHEAD)

	// They should be the same so HEAD can reuse GET cache
	if getKey != headKey {
		t.Errorf("GET and HEAD should generate same cache key for cache sharing:")
		t.Errorf("  GET key:  %s", getKey)
		t.Errorf("  HEAD key: %s", headKey)
	}

	t.Logf("Cache key sharing verified: GET and HEAD both use key %s", getKey)
}
