package selectcache

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patrickmn/go-cache"
)

// TestTypeAssertionPanic tests the critical bug where type assertion can panic
func TestTypeAssertionPanic(t *testing.T) {
	// Create middleware
	middleware := NewDefault()

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)

	// Get the cache key that will be used
	h := sha256.New()
	h.Write([]byte(req.URL.String()))
	h.Write([]byte(req.Header.Get("Accept")))
	h.Write([]byte(req.Header.Get("Accept-Encoding")))
	h.Write([]byte(req.Header.Get("Accept-Language")))
	key := fmt.Sprintf("%x", h.Sum(nil))[:16]

	// Manually corrupt the cache with invalid data
	middleware.GetCacheForTesting().Set(key, "invalid-data-type", cache.DefaultExpiration)

	// Verify the corrupted data is in the cache
	if _, found := middleware.GetCacheForTesting().Get(key); !found {
		t.Fatal("Test setup failed: corrupted data not found in cache")
	}

	// Create a recorder for the test
	recorder := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message": "test"}`))
	}))

	// This should no longer panic with the fix
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic occurred: %v", r)
		}
	}()
	// Make the request that should hit the corrupted cache
	handler.ServeHTTP(recorder, req)

	// Verify the response is the original response (cache miss due to invalid data)
	if recorder.Body.String() != `{"message": "test"}` {
		t.Errorf("Expected original response, got '%s'", recorder.Body.String())
	}

	// Note: After the request, the cache will have a new valid entry,
	// but the important thing is that no panic occurred and the request succeeded
}

// TestInvalidCacheDataRemoval specifically tests that invalid cache data is removed
func TestInvalidCacheDataRemoval(t *testing.T) {
	middleware := NewDefault()
	req := httptest.NewRequest("GET", "/test", nil)

	// Calculate the cache key
	h := sha256.New()
	h.Write([]byte(req.URL.String()))
	h.Write([]byte(req.Header.Get("Accept")))
	h.Write([]byte(req.Header.Get("Accept-Encoding")))
	h.Write([]byte(req.Header.Get("Accept-Language")))
	key := fmt.Sprintf("%x", h.Sum(nil))[:16]

	// Insert invalid data
	middleware.GetCacheForTesting().Set(key, "corrupted-data", cache.DefaultExpiration)

	// Verify invalid data is present
	if _, found := middleware.GetCacheForTesting().Get(key); !found {
		t.Fatal("Setup failed: corrupted data not in cache")
	}

	// Make request that should encounter and remove invalid data
	recorder := httptest.NewRecorder()
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": "valid"}`))
	}))

	handler.ServeHTTP(recorder, req)

	// Verify no panic occurred and response is correct
	if recorder.Body.String() != `{"data": "valid"}` {
		t.Errorf("Expected valid response, got: %s", recorder.Body.String())
	}

	// Verify that after the request, the cache now contains valid data
	if cached, found := middleware.GetCacheForTesting().Get(key); found {
		if cachedResp, ok := cached.(*CachedResponse); ok {
			if string(cachedResp.Body) != `{"data": "valid"}` {
				t.Errorf("Cache contains unexpected data: %s", string(cachedResp.Body))
			}
		} else {
			t.Error("Cache should contain valid CachedResponse after fixing corruption")
		}
	} else {
		t.Error("Cache should contain new valid entry after removing corrupted data")
	}
}

// TestDeleteMethodBug tests that the Delete method doesn't work correctly
func TestDeleteMethodBug(t *testing.T) {
	middleware := NewDefault()

	// Make a request to populate the cache
	req := httptest.NewRequest("GET", "/api/users", nil)
	recorder := httptest.NewRecorder()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"users": []}`))
	}))

	// First request - should populate cache
	handler.ServeHTTP(recorder, req)
	if recorder.Body.String() != `{"users": []}` {
		t.Fatalf("First request failed: %s", recorder.Body.String())
	}

	// Verify it's cached by making another request and checking for cache hit header
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, req)

	if recorder2.Header().Get("X-Cache-Status") != "HIT" {
		t.Error("Second request should have been a cache hit")
	}

	// Now try to delete using the URL (this should work with the fix)
	middleware.Delete("/api/users")

	// Make another request - should be a cache miss since we deleted it
	recorder3 := httptest.NewRecorder()
	handler.ServeHTTP(recorder3, req)

	// With the fix, this should now be a cache miss
	if recorder3.Header().Get("X-Cache-Status") == "HIT" {
		t.Error("Request should have been a cache miss after deletion, but got cache hit")
	}

	// Verify the response is still correct (served fresh)
	if recorder3.Body.String() != `{"users": []}` {
		t.Errorf("Expected fresh response after deletion, got: %s", recorder3.Body.String())
	}
}

// TestStatsMethodBug tests that the Stats method doesn't track hits/misses correctly
func TestStatsMethodBug(t *testing.T) {
	middleware := NewDefault()

	// Initial stats should show no items
	itemCount, hitCount, missCount := middleware.Stats()
	if itemCount != 0 || hitCount != 0 || missCount != 0 {
		t.Errorf("Initial stats should be 0,0,0 but got %d,%d,%d", itemCount, hitCount, missCount)
	}

	// Make a request to populate the cache (cache miss)
	req := httptest.NewRequest("GET", "/api/data", nil)
	recorder1 := httptest.NewRecorder()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": "test"}`))
	}))

	// First request - cache miss
	handler.ServeHTTP(recorder1, req)

	// Stats should show 1 item and 1 miss (should work with the fix)
	itemCount, hitCount, missCount = middleware.Stats()
	if itemCount != 1 {
		t.Errorf("Expected 1 cached item, got %d", itemCount)
	}
	if hitCount != 0 {
		t.Errorf("Expected 0 hits after first request, got %d", hitCount)
	}
	if missCount != 1 {
		t.Errorf("Expected 1 miss after first request, got %d", missCount)
	}

	// Second request - cache hit
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, req)

	// Verify it was a cache hit
	if recorder2.Header().Get("X-Cache-Status") != "HIT" {
		t.Error("Second request should have been a cache hit")
	}

	// Stats should show 1 hit, 1 miss (should work with the fix)
	itemCount, hitCount, missCount = middleware.Stats()
	if itemCount != 1 {
		t.Errorf("Expected 1 cached item, got %d", itemCount)
	}
	if hitCount != 1 {
		t.Errorf("Expected 1 hit after second request, got %d", hitCount)
	}
	if missCount != 1 {
		t.Errorf("Expected 1 miss total, got %d", missCount)
	}
}

// TestStatsAccuracy tests that stats accurately count hits and misses
func TestStatsAccuracy(t *testing.T) {
	middleware := NewDefault()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"path": "` + r.URL.Path + `"}`))
	}))

	// Make requests to different URLs (cache misses)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/item%d", i), nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}

	// Should have 3 items, 0 hits, 3 misses
	itemCount, hitCount, missCount := middleware.Stats()
	if itemCount != 3 || hitCount != 0 || missCount != 3 {
		t.Errorf("After 3 unique requests: expected (3,0,3), got (%d,%d,%d)", itemCount, hitCount, missCount)
	}

	// Make requests to same URLs (cache hits)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/item%d", i), nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		// Verify cache hit
		if recorder.Header().Get("X-Cache-Status") != "HIT" {
			t.Errorf("Request to /api/item%d should have been a cache hit", i)
		}
	}

	// Should have 3 items, 3 hits, 3 misses
	itemCount, hitCount, missCount = middleware.Stats()
	if itemCount != 3 || hitCount != 3 || missCount != 3 {
		t.Errorf("After 3 hits: expected (3,3,3), got (%d,%d,%d)", itemCount, hitCount, missCount)
	}
}

// TestHEADRequestSupport tests that HEAD requests should be cached like GET requests
func TestHEADRequestSupport(t *testing.T) {
	middleware := NewDefault()

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "15")
		if r.Method == "GET" {
			w.Write([]byte(`{"data": "test"}`))
		}
		// For HEAD requests, only headers are sent (no body)
	}))

	// Make a GET request first to populate cache
	reqGET := httptest.NewRequest("GET", "/api/data", nil)
	recorderGET := httptest.NewRecorder()
	handler.ServeHTTP(recorderGET, reqGET)

	// Verify GET response
	if recorderGET.Body.String() != `{"data": "test"}` {
		t.Errorf("GET response incorrect: %s", recorderGET.Body.String())
	}

	// Make a second GET request - should be cached
	recorderGET2 := httptest.NewRecorder()
	handler.ServeHTTP(recorderGET2, reqGET)
	if recorderGET2.Header().Get("X-Cache-Status") != "HIT" {
		t.Error("Second GET request should be a cache hit")
	}

	// Now make a HEAD request to the same URL
	reqHEAD := httptest.NewRequest("HEAD", "/api/data", nil)
	recorderHEAD := httptest.NewRecorder()
	handler.ServeHTTP(recorderHEAD, reqHEAD)

	// HEAD request should be served from cache with the fix
	if recorderHEAD.Header().Get("X-Cache-Status") != "HIT" {
		t.Error("HEAD request should be served from cache (same as GET)")
	}

	// HEAD response should have same headers as GET but no body
	if recorderHEAD.Header().Get("Content-Type") != "application/json" {
		t.Error("HEAD response should have same Content-Type as GET")
	}
	if recorderHEAD.Body.Len() != 0 {
		t.Error("HEAD response should have no body")
	}

	// Stats should show the HEAD request as a cache hit
	_, hitCount, _ := middleware.Stats()
	if hitCount < 2 { // Should have at least 2 hits (GET + HEAD)
		t.Errorf("Expected at least 2 cache hits (GET + HEAD), got %d", hitCount)
	}
}
