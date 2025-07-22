package selectcache

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHEADRequestBodyCachingFixed verifies the fix for HEAD requests not caching unnecessary body data
// This is a negative test confirming the issue from AUDIT.md is resolved
func TestHEADRequestBodyCachingFixed(t *testing.T) {
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

	// Check if cached entry contains body data
	// With the fix: GET and HEAD share cache keys, HEAD reuses GET's cache entry
	// This is correct behavior - the issue was when HEAD-only requests cache body data
	key := middleware.createCacheKey(reqGET) // Same key for GET and HEAD
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			bodySize := len(cachedResp.Body)
			t.Logf("Cache contains %d bytes of body data from GET request", bodySize)
			if bodySize == 10240 {
				t.Logf("SUCCESS: This is correct behavior - HEAD reuses GET cache entry which includes body data")
			} else {
				t.Errorf("Expected cached body size to be 10240 bytes from GET request, got %d", bodySize)
			}
		}
	} else {
		t.Error("Expected cache entry to exist from GET request")
	}
}

// TestHEADOnlyRequestBodyCachingFixed verifies HEAD-only requests don't cache body data (confirms fix)
func TestHEADOnlyRequestBodyCachingFixed(t *testing.T) {
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

	// Check cached entry - this verifies the fix is working
	key := middleware.createCacheKey(reqHEAD)
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			bodySize := len(cachedResp.Body)
			t.Logf("HEAD request cached %d bytes of body data", bodySize)

			// FIXED: HEAD requests should not cache body data
			if bodySize > 0 {
				t.Errorf("HEAD REQUEST BODY CACHING BUG DETECTED: HEAD request cached %d bytes of unnecessary body data", bodySize)
				t.Errorf("Expected: HEAD requests should cache headers only (0 bytes body)")
				t.Errorf("This indicates the HEAD request fix is not working properly")
			} else {
				t.Logf("SUCCESS: HEAD request correctly cached only headers, no body data")
			}

			// Verify headers are still cached
			if cachedResp.Headers.Get("Content-Type") != "application/json" {
				t.Error("Headers should still be cached for HEAD requests")
			} else {
				t.Logf("SUCCESS: Headers correctly cached for HEAD request")
			}
		}
	} else {
		t.Error("HEAD request should have been cached")
	}
}
