package selectcache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDeleteMethodWithHEADRequests tests if Delete can remove HEAD-cached entries
func TestDeleteMethodWithHEADRequests(t *testing.T) {
	// Create middleware with cache
	config := Config{
		DefaultTTL:          5 * time.Minute,
		CleanupInterval:     1 * time.Minute,
		IncludeStatusCodes:  []int{200},
		ExcludeContentTypes: []string{"text/html"}, // Don't exclude JSON
	}

	middleware := New(config)

	// Create a test server that serves both GET and HEAD
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.Write([]byte(`{"message": "test data"}`))
		}
		// HEAD requests don't write body
	})

	// Wrap with middleware
	wrappedHandler := middleware.Handler(handler)

	// Test URL
	testURL := "/test-endpoint"

	// 1. Make a HEAD request to cache the response
	t.Log("Step 1: Making HEAD request to cache response")
	headReq := httptest.NewRequest("HEAD", testURL, nil)
	headResp := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(headResp, headReq)

	if headResp.Code != 200 {
		t.Fatalf("HEAD request failed: %d", headResp.Code)
	}

	// 2. Verify the response was cached by making a GET request (should be cache hit)
	t.Log("Step 2: Making GET request to verify cache hit")
	getReq := httptest.NewRequest("GET", testURL, nil)
	getResp := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(getResp, getReq)

	if getResp.Code != 200 {
		t.Fatalf("GET request failed: %d", getResp.Code)
	}

	// Check cache stats - should have some hits and misses
	itemCount, hitCount, missCount := middleware.Stats()
	t.Logf("Cache stats after requests: Items=%d, Hits=%d, Misses=%d", itemCount, hitCount, missCount)

	// 3. Now try to delete the cached entry using the URL
	t.Log("Step 3: Deleting cached entry using Delete method")
	middleware.Delete("http://example.com" + testURL)

	// 4. Make another GET request - should be a cache miss now
	t.Log("Step 4: Making another GET request to verify deletion")
	getReq2 := httptest.NewRequest("GET", testURL, nil)
	getResp2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(getResp2, getReq2)

	if getResp2.Code != 200 {
		t.Fatalf("Second GET request failed: %d", getResp2.Code)
	}

	// Check cache stats - should have more misses if delete worked correctly
	finalItemCount, finalHitCount, finalMissCount := middleware.Stats()
	t.Logf("Final cache stats: Items=%d, Hits=%d, Misses=%d", finalItemCount, finalHitCount, finalMissCount)

	if finalMissCount <= missCount {
		t.Errorf("Delete method failed to remove HEAD-cached entry. Expected more misses after delete.")
		t.Errorf("Stats before delete: Items=%d, Hits=%d, Misses=%d", itemCount, hitCount, missCount)
		t.Errorf("Stats after delete: Items=%d, Hits=%d, Misses=%d", finalItemCount, finalHitCount, finalMissCount)
	} else {
		t.Logf("SUCCESS: Delete method successfully removed HEAD-cached entry")
	}
}
