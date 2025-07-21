package selectcache

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestTTLCache_SetAndGet(t *testing.T) {
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	// Test data
	key := "test-key"
	data := []byte("test data")
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	ttl := 5 * time.Minute

	// Set entry
	err := cache.Set(key, data, headers, ttl)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get entry
	entry, found := cache.Get(key)
	if !found {
		t.Fatalf("Get() failed to find entry")
	}

	// Verify data
	if string(entry.Data) != string(data) {
		t.Errorf("Data mismatch: got %s, want %s", string(entry.Data), string(data))
	}

	// Verify headers
	if entry.Headers.Get("Content-Type") != headers.Get("Content-Type") {
		t.Errorf("Headers mismatch: got %s, want %s",
			entry.Headers.Get("Content-Type"), headers.Get("Content-Type"))
	}

	// Verify TTL
	if entry.IsExpired() {
		t.Errorf("Entry should not be expired")
	}
}

func TestTTLCache_Expiration(t *testing.T) {
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	key := "expire-test"
	data := []byte("test data")
	headers := make(http.Header)
	ttl := 100 * time.Millisecond // Very short TTL

	// Set entry
	err := cache.Set(key, data, headers, ttl)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should be available immediately
	_, found := cache.Get(key)
	if !found {
		t.Fatalf("Entry should be available immediately")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, found = cache.Get(key)
	if found {
		t.Errorf("Entry should be expired")
	}
}

func TestTTLCache_MemoryLimit(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL:        time.Hour,
		MaxMemoryMB:       1, // Very small limit: 1MB
		MaxEntries:        1000,
		ExcludedTypes:     []string{},
		EnableMetrics:     true,
		CleanupInterval:   time.Minute,
		BufferSize:        4096,
		ConnectionTimeout: 30 * time.Second,
	}

	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	// Create large data that exceeds memory limit
	largeData := make([]byte, 512*1024) // 512KB each
	headers := make(http.Header)

	// Add entries until memory limit is reached
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("large-key-%d", i)
		err := cache.Set(key, largeData, headers, time.Hour)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	// Memory usage should be bounded
	memoryUsage := cache.MemoryUsage()
	maxMemoryBytes := uint64(config.MaxMemoryMB) * 1024 * 1024

	// Allow some overhead for headers and metadata
	if memoryUsage > maxMemoryBytes*2 {
		t.Errorf("Memory usage %d exceeds reasonable limit of %d",
			memoryUsage, maxMemoryBytes*2)
	}
}

func TestTTLCache_Delete(t *testing.T) {
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	key := "delete-test"
	data := []byte("test data")
	headers := make(http.Header)

	// Set entry
	err := cache.Set(key, data, headers, time.Hour)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify it exists
	_, found := cache.Get(key)
	if !found {
		t.Fatalf("Entry should exist")
	}

	// Delete entry
	deleted := cache.Delete(key)
	if !deleted {
		t.Errorf("Delete() should return true")
	}

	// Verify it's gone
	_, found = cache.Get(key)
	if found {
		t.Errorf("Entry should be deleted")
	}

	// Delete non-existent entry
	deleted = cache.Delete("non-existent")
	if deleted {
		t.Errorf("Delete() should return false for non-existent key")
	}
}

func TestTTLCache_Clear(t *testing.T) {
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	defer cache.Close()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		data := []byte(fmt.Sprintf("data-%d", i))
		headers := make(http.Header)

		err := cache.Set(key, data, headers, time.Hour)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	// Verify entries exist
	if cache.Size() != 5 {
		t.Errorf("Expected 5 entries, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	if cache.Size() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", cache.Size())
	}

	if cache.MemoryUsage() != 0 {
		t.Errorf("Expected 0 memory usage after clear, got %d", cache.MemoryUsage())
	}
}

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		query    string
		headers  map[string]string
		expected bool // Whether keys should be equal
	}{
		{
			name:     "same parameters should generate same key",
			method:   "GET",
			path:     "/api/data",
			query:    "id=123",
			headers:  map[string]string{"Accept": "application/json"},
			expected: true,
		},
		{
			name:     "different method should generate different key",
			method:   "POST",
			path:     "/api/data",
			query:    "id=123",
			headers:  map[string]string{"Accept": "application/json"},
			expected: false,
		},
	}

	baseKey := GenerateCacheKey("GET", "/api/data", "id=123",
		map[string]string{"Accept": "application/json"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := GenerateCacheKey(tt.method, tt.path, tt.query, tt.headers)

			if tt.expected && key != baseKey {
				t.Errorf("Expected same key, got different: %s vs %s", key, baseKey)
			}

			if !tt.expected && key == baseKey {
				t.Errorf("Expected different key, got same: %s", key)
			}
		})
	}
}
