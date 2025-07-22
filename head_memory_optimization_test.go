package selectcache

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHEADRequestMemoryOptimization verifies that HEAD requests don't cache unnecessary body data
func TestHEADRequestMemoryOptimization(t *testing.T) {
	middleware := NewDefault()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"large": "response data that should not be cached for HEAD requests"}`))
	}))

	// Make a HEAD request
	req := httptest.NewRequest("HEAD", "/api/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	// Check that cached entry doesn't contain body data for HEAD requests
	key := middleware.createCacheKey(req)
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			if len(cachedResp.Body) > 0 {
				t.Errorf("HEAD request should not cache body data, got %d bytes", len(cachedResp.Body))
			}

			// Verify headers are still cached
			if cachedResp.Headers.Get("Content-Type") != "application/json" {
				t.Error("HEAD request should still cache headers")
			}

			t.Logf("✅ HEAD request optimization verified: 0 bytes body cached, headers preserved")
		}
	} else {
		t.Error("HEAD request should be cached")
	}
}
