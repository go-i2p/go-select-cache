package selectcache

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestCacheAccessTimeRaceCondition reproduces the race condition in cache access time updates
func TestCacheAccessTimeRaceCondition(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL:      5 * time.Minute,
		MaxMemoryMB:     10,
		MaxEntries:      1000,
		CleanupInterval: 1 * time.Minute,
	}

	metrics := &CacheMetrics{}
	cache := NewTTLCache(config, metrics)

	// Add a cache entry
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	err := cache.Set("test-key", []byte("test data"), headers, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	// Create multiple goroutines that simultaneously access the same cache entry
	const numGoroutines = 100
	const accessesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < accessesPerGoroutine; j++ {
				_, exists := cache.Get("test-key")
				if !exists {
					t.Errorf("Cache entry should exist")
				}
				time.Sleep(time.Microsecond) // Small delay to increase chance of race
			}
		}()
	}

	wg.Wait()

	// If we reach here without race condition detection, the test passes
	t.Logf("Successfully completed %d concurrent cache accesses", numGoroutines*accessesPerGoroutine)
}
