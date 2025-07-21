package selectcache

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestIntegration_HTTPServerCaching(t *testing.T) {
	// Create a test HTTP server
	requestCount := 0
	mux := http.NewServeMux()

	// API endpoint that should be cached
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{"message": "API data", "request_num": %d}`, requestCount)
		w.Write([]byte(response))
	})

	// HTML endpoint that should NOT be cached
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/html")
		html := fmt.Sprintf("<h1>Page view #%d</h1>", requestCount)
		w.Write([]byte(html))
	})

	// Create a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Configure caching
	config := &CacheConfig{
		DefaultTTL:        1 * time.Minute,
		MaxMemoryMB:       10,
		MaxEntries:        100,
		ExcludedTypes:     []string{"text/html"},
		EnableMetrics:     true,
		CleanupInterval:   10 * time.Second,
		BufferSize:        4096,
		ConnectionTimeout: 5 * time.Second,
	}

	// Wrap with caching
	cachingListener := NewCachingListener(listener, config)
	defer cachingListener.Close()

	// Start HTTP server
	server := &http.Server{Handler: mux}
	go server.Serve(cachingListener)
	defer server.Close()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	// Test 1: API endpoint should be cached
	t.Run("API endpoint caching", func(t *testing.T) {
		// First request
		resp1, err := http.Get(baseURL + "/api/data")
		if err != nil {
			t.Fatalf("First request failed: %v", err)
		}
		defer resp1.Body.Close()

		body1, _ := io.ReadAll(resp1.Body)

		// Second request (should be cached)
		resp2, err := http.Get(baseURL + "/api/data")
		if err != nil {
			t.Fatalf("Second request failed: %v", err)
		}
		defer resp2.Body.Close()

		body2, _ := io.ReadAll(resp2.Body)

		// Responses should be identical (cached)
		if !bytes.Equal(body1, body2) {
			t.Errorf("API responses differ, caching not working:\nFirst: %s\nSecond: %s", body1, body2)
		}

		// Check for cache hit header
		if resp2.Header.Get("X-Cache-Status") != "HIT" {
			t.Errorf("Expected cache hit header, got: %s", resp2.Header.Get("X-Cache-Status"))
		}
	})

	// Test 2: HTML endpoint should NOT be cached
	t.Run("HTML endpoint not cached", func(t *testing.T) {
		initialCount := requestCount

		// First HTML request
		resp1, err := http.Get(baseURL + "/")
		if err != nil {
			t.Fatalf("First HTML request failed: %v", err)
		}
		defer resp1.Body.Close()
		io.ReadAll(resp1.Body)

		// Second HTML request (should NOT be cached)
		resp2, err := http.Get(baseURL + "/")
		if err != nil {
			t.Fatalf("Second HTML request failed: %v", err)
		}
		defer resp2.Body.Close()
		io.ReadAll(resp2.Body)

		// Both requests should have been processed (not cached)
		expectedRequests := initialCount + 2
		if requestCount != expectedRequests {
			t.Errorf("Expected %d requests processed, got %d", expectedRequests, requestCount)
		}

		// Should not have cache hit header
		if resp2.Header.Get("X-Cache-Status") == "HIT" {
			t.Errorf("HTML should not be cached, but got cache hit")
		}
	})

	// Test 3: Cache statistics
	t.Run("Cache statistics", func(t *testing.T) {
		stats := cachingListener.GetStats()

		if stats.CacheStats.Hits == 0 {
			t.Errorf("Expected cache hits, got 0")
		}

		if stats.CacheStats.Misses == 0 {
			t.Errorf("Expected cache misses, got 0")
		}

		if stats.CacheSize == 0 {
			t.Errorf("Expected cache entries, got 0")
		}

		t.Logf("Cache stats: Hits=%d, Misses=%d, HitRatio=%.2f%%, Entries=%d",
			stats.CacheStats.Hits, stats.CacheStats.Misses,
			stats.CacheStats.HitRatio*100, stats.CacheSize)
	})
}

func TestIntegration_CacheExpiration(t *testing.T) {
	// Create a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Configure very short TTL for testing
	config := &CacheConfig{
		DefaultTTL:        200 * time.Millisecond, // Very short TTL
		MaxMemoryMB:       10,
		MaxEntries:        100,
		ExcludedTypes:     []string{"text/html"},
		EnableMetrics:     true,
		CleanupInterval:   50 * time.Millisecond,
		BufferSize:        4096,
		ConnectionTimeout: 5 * time.Second,
	}

	cachingListener := NewCachingListener(listener, config)
	defer cachingListener.Close()

	// Simple counter endpoint
	requestCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/counter", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"count": %d}`, requestCount)))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(cachingListener)
	defer server.Close()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	// First request
	resp1, err := http.Get(baseURL + "/counter")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	resp1.Body.Close()

	// Second request immediately (should be cached)
	resp2, err := http.Get(baseURL + "/counter")
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	resp2.Body.Close()

	if resp2.Header.Get("X-Cache-Status") != "HIT" {
		t.Errorf("Expected cache hit, got: %s", resp2.Header.Get("X-Cache-Status"))
	}

	// Wait for cache expiration
	time.Sleep(300 * time.Millisecond)

	// Third request after expiration (should NOT be cached)
	resp3, err := http.Get(baseURL + "/counter")
	if err != nil {
		t.Fatalf("Third request failed: %v", err)
	}
	resp3.Body.Close()

	if resp3.Header.Get("X-Cache-Status") == "HIT" {
		t.Errorf("Expected cache miss after expiration, but got cache hit")
	}

	// Should have processed 3 requests (all requests go through handler, cache hits replace response)
	if requestCount != 3 {
		t.Errorf("Expected 3 requests processed, got %d", requestCount)
	}
}

func TestIntegration_ConcurrentConnections(t *testing.T) {
	// Create a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	config := DefaultCacheConfig()
	config.EnableMetrics = true

	cachingListener := NewCachingListener(listener, config)
	defer cachingListener.Close()

	// Simple endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message": "test"}`))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(cachingListener)
	defer server.Close()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	// Make concurrent requests
	const numRequests = 50
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer func() { done <- true }()

			resp, err := http.Get(baseURL + "/test")
			if err != nil {
				t.Errorf("Request failed: %v", err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Check that we handled multiple concurrent connections
	stats := cachingListener.GetStats()
	totalRequests := stats.CacheStats.Hits + stats.CacheStats.Misses

	if totalRequests < numRequests {
		t.Errorf("Expected at least %d requests processed, got %d", numRequests, totalRequests)
	}

	t.Logf("Processed %d requests: %d hits, %d misses",
		totalRequests, stats.CacheStats.Hits, stats.CacheStats.Misses)
}
