package selectcache

import (
	"testing"
	"time"
)

func TestSimpleIntegration(t *testing.T) {
	// Test that the basic components work together
	config := DefaultCacheConfig()
	config.DefaultTTL = 1 * time.Second
	config.EnableMetrics = true

	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	detector := NewContentDetector(config)

	// Test caching workflow
	t.Run("Complete caching workflow", func(t *testing.T) {
		// Simulate a cacheable response
		response := []byte(`{"message": "test"}`)
		headers := make(map[string][]string)
		headers["Content-Type"] = []string{"application/json"}
		statusCode := 200

		// Analyze the response
		analysis := detector.AnalyzeResponse(response, headers, statusCode)

		if !analysis.IsCacheable {
			t.Errorf("JSON response should be cacheable")
		}

		if analysis.IsHTML {
			t.Errorf("JSON response should not be detected as HTML")
		}

		// Generate cache key
		cacheKey := GenerateCacheKey("GET", "/api/data", "", map[string]string{
			"Accept": "application/json",
		})

		// Store in cache
		err := cache.Set(cacheKey, response, headers, analysis.RecommendedTTL)
		if err != nil {
			t.Fatalf("Failed to cache response: %v", err)
		}

		// Retrieve from cache
		entry, found := cache.Get(cacheKey)
		if !found {
			t.Fatalf("Failed to retrieve cached response")
		}

		// Verify cached data
		if string(entry.Data) != string(response) {
			t.Errorf("Cached data mismatch")
		}

		if entry.Headers["Content-Type"][0] != "application/json" {
			t.Errorf("Cached headers mismatch")
		}

		// Check metrics
		stats := metrics.GetStats()
		if stats.Hits == 0 {
			t.Errorf("Expected cache hit to be recorded")
		}

		if stats.Stores == 0 {
			t.Errorf("Expected cache store to be recorded")
		}
	})

	t.Run("HTML exclusion workflow", func(t *testing.T) {
		// Simulate an HTML response
		response := []byte(`<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Test</h1></body></html>`)
		headers := make(map[string][]string)
		headers["Content-Type"] = []string{"text/html; charset=utf-8"}
		statusCode := 200

		// Analyze the response
		analysis := detector.AnalyzeResponse(response, headers, statusCode)

		if analysis.IsCacheable {
			t.Errorf("HTML response should not be cacheable")
		}

		if !analysis.IsHTML {
			t.Errorf("HTML response should be detected as HTML")
		}
	})
}

func TestCacheKeyConsistency(t *testing.T) {
	// Test that cache keys are generated consistently
	tests := []struct {
		name        string
		method1     string
		path1       string
		query1      string
		headers1    map[string]string
		method2     string
		path2       string
		query2      string
		headers2    map[string]string
		shouldMatch bool
	}{
		{
			name:        "identical requests",
			method1:     "GET",
			path1:       "/api/data",
			query1:      "id=123",
			headers1:    map[string]string{"Accept": "application/json"},
			method2:     "GET",
			path2:       "/api/data",
			query2:      "id=123",
			headers2:    map[string]string{"Accept": "application/json"},
			shouldMatch: true,
		},
		{
			name:        "different methods",
			method1:     "GET",
			path1:       "/api/data",
			query1:      "",
			headers1:    map[string]string{},
			method2:     "POST",
			path2:       "/api/data",
			query2:      "",
			headers2:    map[string]string{},
			shouldMatch: false,
		},
		{
			name:        "different paths",
			method1:     "GET",
			path1:       "/api/data",
			query1:      "",
			headers1:    map[string]string{},
			method2:     "GET",
			path2:       "/api/users",
			query2:      "",
			headers2:    map[string]string{},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := GenerateCacheKey(tt.method1, tt.path1, tt.query1, tt.headers1)
			key2 := GenerateCacheKey(tt.method2, tt.path2, tt.query2, tt.headers2)

			if tt.shouldMatch && key1 != key2 {
				t.Errorf("Keys should match but don't: %s vs %s", key1, key2)
			}

			if !tt.shouldMatch && key1 == key2 {
				t.Errorf("Keys should not match but do: %s", key1)
			}
		})
	}
}
