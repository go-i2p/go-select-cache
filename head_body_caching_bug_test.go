package selectcache

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHEADRequestBodyCachingWaste reproduces the bug where HEAD requests
// cache response body data unnecessarily, wasting memory
func TestHEADRequestBodyCachingWaste(t *testing.T) {
	middleware := NewDefault()

	// Create a handler that generates a large response body
	largeResponseBody := make([]byte, 10240) // 10KB response
	for i := range largeResponseBody {
		largeResponseBody[i] = byte('A')
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "10240")
		if r.Method == "GET" {
			w.Write(largeResponseBody)
		}
		// For HEAD requests, only headers should be sent (no body)
	}))

	// First make a GET request to populate cache
	reqGET := httptest.NewRequest("GET", "/api/data", nil)
	recorderGET := httptest.NewRecorder()
	handler.ServeHTTP(recorderGET, reqGET)

	// Verify GET response has body
	if len(recorderGET.Body.Bytes()) != 10240 {
		t.Errorf("GET request should have body of 10240 bytes, got %d", len(recorderGET.Body.Bytes()))
	}

	// Now make a HEAD request to the same URL - this should reuse GET cache
	reqHEAD := httptest.NewRequest("HEAD", "/api/data", nil)
	recorderHEAD := httptest.NewRecorder()
	handler.ServeHTTP(recorderHEAD, reqHEAD)

	// HEAD response should have no body but same headers
	if len(recorderHEAD.Body.Bytes()) != 0 {
		t.Errorf("HEAD response should have no body, got %d bytes", len(recorderHEAD.Body.Bytes()))
	}

	if recorderHEAD.Header().Get("Content-Type") != "text/plain" {
		t.Error("HEAD response should have same headers as GET")
	}

	// Now check if cached entry contains body data
	// Note: Since GET and HEAD share cache keys, HEAD will reuse GET's cache entry
	// This is correct behavior - the issue was when HEAD-only requests cache body data
	key := middleware.createCacheKey(reqGET) // Same key for GET and HEAD
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			bodySize := len(cachedResp.Body)
			t.Logf("Cache contains %d bytes of body data from GET request", bodySize)
			if bodySize == 10240 {
				t.Logf("This is correct: HEAD reuses GET cache entry which includes body data")
			}
		}
	}
}

// TestHEADOnlyRequestBodyCaching tests caching when only HEAD request is made
func TestHEADOnlyRequestBodyCaching(t *testing.T) {
	middleware := NewDefault()

	// Create a handler that would generate a large response body if called with GET
	largeResponseBody := make([]byte, 5120) // 5KB response
	for i := range largeResponseBody {
		largeResponseBody[i] = byte('B')
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "5120")

		// This handler always writes body (simulating real-world scenario where
		// backend doesn't distinguish between GET and HEAD)
		w.Write(largeResponseBody)
	}))

	// Make a HEAD request first (no prior GET)
	reqHEAD := httptest.NewRequest("HEAD", "/api/users", nil)
	recorderHEAD := httptest.NewRecorder()
	handler.ServeHTTP(recorderHEAD, reqHEAD)

	// Check cached entry - this tests the fix
	key := middleware.createCacheKey(reqHEAD)
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			bodySize := len(cachedResp.Body)
			t.Logf("HEAD request cached %d bytes of body data", bodySize)

			// The fix: HEAD requests should not cache body data
			if bodySize > 0 {
				t.Errorf("BUG: HEAD request cached %d bytes of unnecessary body data", bodySize)
				t.Logf("Expected: HEAD requests should cache headers only (0 bytes body)")
				t.Logf("Actual: HEAD request cached full response body, wasting memory")
			} else {
				t.Logf("SUCCESS: HEAD request correctly cached only headers, no body data")
			}

			// Verify headers are still cached
			if cachedResp.Headers.Get("Content-Type") != "application/json" {
				t.Error("Headers should still be cached for HEAD requests")
			}
		}
	} else {
		t.Error("HEAD request should have been cached")
	}
}
