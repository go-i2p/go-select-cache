package selectcache

import (
	"net/http"
	"testing"
)

// TestCacheKeyConsistencyFixed verifies the cache key inconsistency fix between
// selectcache.go and cache.go implementations is working properly
// This is a negative test confirming the issue from AUDIT.md is resolved
func TestCacheKeyConsistencyFixed(t *testing.T) {
	// Create a test HTTP request
	req, err := http.NewRequest("GET", "http://example.com/api/data?id=123&sort=name", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add some headers that affect caching
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", "en-US")

	// Create middleware to test selectcache.go approach
	middleware := &Middleware{}

	// Generate cache key using selectcache.go method
	selectcacheKey := middleware.createCacheKey(req)

	// Generate cache key using cache.go method (the way connection.go uses it)
	headers := map[string]string{
		"Accept":          req.Header.Get("Accept"),
		"Accept-Encoding": req.Header.Get("Accept-Encoding"),
		"Accept-Language": req.Header.Get("Accept-Language"),
	}

	query := ""
	if req.URL.RawQuery != "" {
		query = req.URL.RawQuery
	}

	cacheGoKey := GenerateCacheKey(req.Method, req.URL.Path, query, headers)

	// FIXED: These should now be the same for the same request (confirming the fix)
	if selectcacheKey != cacheGoKey {
		t.Errorf("CACHE KEY INCONSISTENCY DETECTED: Keys should be the same after fix:")
		t.Errorf("  selectcache.go key: %s", selectcacheKey)
		t.Errorf("  cache.go key:      %s", cacheGoKey)
		t.Errorf("This indicates the cache key consistency fix is not working properly")
	} else {
		t.Logf("SUCCESS: Cache key consistency fix verified - both methods generate: %s", selectcacheKey)
	}

	t.Logf("Cache key consistency test results:")
	t.Logf("  Request URL:       %s", req.URL.String())
	t.Logf("  Method:            %s", req.Method)
	t.Logf("  Path:              %s", req.URL.Path)
	t.Logf("  Query:             %s", req.URL.RawQuery)
}

// TestCacheKeyConsistencyAfterFix verifies the fix works (redundant but keeping for completeness)
func TestCacheKeyConsistencyAfterFix(t *testing.T) {
	// This test confirms the fix is working properly

	// Create a test HTTP request
	req, err := http.NewRequest("GET", "http://example.com/api/data?id=123&sort=name", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add some headers that affect caching
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", "en-US")

	// Create middleware to test selectcache.go approach
	middleware := &Middleware{}

	// Generate cache key using selectcache.go method
	selectcacheKey := middleware.createCacheKey(req)

	// Generate cache key using cache.go method (the way connection.go uses it)
	headers := map[string]string{
		"Accept":          req.Header.Get("Accept"),
		"Accept-Encoding": req.Header.Get("Accept-Encoding"),
		"Accept-Language": req.Header.Get("Accept-Language"),
	}

	query := ""
	if req.URL.RawQuery != "" {
		query = req.URL.RawQuery
	}

	cacheGoKey := GenerateCacheKey(req.Method, req.URL.Path, query, headers)

	// FIXED: These should be the same for the same request (confirming the fix works)
	if selectcacheKey != cacheGoKey {
		t.Errorf("Cache keys should be consistent after fix:")
		t.Errorf("  selectcache.go key: %s", selectcacheKey)
		t.Errorf("  cache.go key:      %s", cacheGoKey)
		t.Errorf("This indicates the fix is not working properly")
	} else {
		t.Logf("SUCCESS: Cache key consistency verified - both methods generate: %s", selectcacheKey)
	}
}

// TestMultipleRequestsCacheKeyConsistency verifies consistency across different requests (confirms fix)
func TestMultipleRequestsCacheKeyConsistency(t *testing.T) {
	testCases := []struct {
		name    string
		method  string
		url     string
		headers map[string]string
	}{
		{
			name:   "Simple GET",
			method: "GET",
			url:    "http://example.com/api/users",
			headers: map[string]string{
				"Accept": "application/json",
			},
		},
		{
			name:   "GET with query params",
			method: "GET",
			url:    "http://example.com/api/users?page=1&limit=10",
			headers: map[string]string{
				"Accept":          "application/json",
				"Accept-Encoding": "gzip",
			},
		},
		{
			name:   "HEAD request",
			method: "HEAD",
			url:    "http://example.com/api/status",
			headers: map[string]string{
				"Accept-Language": "en-US",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create HTTP request
			req, err := http.NewRequest(tc.method, tc.url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Set headers
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			// Test consistency between both methods
			middleware := &Middleware{}
			selectcacheKey := middleware.createCacheKey(req)

			query := ""
			if req.URL.RawQuery != "" {
				query = req.URL.RawQuery
			}

			cacheGoKey := GenerateCacheKey(req.Method, req.URL.Path, query, tc.headers)

			// FIXED: With the fix in place, these should be the same
			t.Logf("Request: %s %s", tc.method, tc.url)
			t.Logf("  selectcache.go key: %s", selectcacheKey)
			t.Logf("  cache.go key:      %s", cacheGoKey)

			if selectcacheKey != cacheGoKey {
				t.Errorf("Cache key inconsistency still exists for %s (fix not working properly)", tc.name)
			} else {
				t.Logf("SUCCESS: Cache key consistency verified for %s", tc.name)
			}
		})
	}
}
