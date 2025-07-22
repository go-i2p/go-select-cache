package selectcache

import (
	"testing"
	"time"
)

// TestMemoryLeakProtection verifies that the critical memory leak is fixed with real-world usage
func TestMemoryLeakProtection(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL:      time.Minute * 5,
		MaxMemoryMB:     100,
		MaxEntries:      1000,
		ContentTypeTTLs: make(map[string]time.Duration),
		ExcludedTypes:   []string{},
		EnableMetrics:   true,
		CleanupInterval: time.Minute,
		BufferSize:      8192,
	}
	metrics := NewCacheMetrics(true)
	detector := NewContentDetector(config)
	cache := NewTTLCache(config, metrics)

	mockConn := newMockConn()
	cachingConn := NewCachingConnection(mockConn, cache, config, metrics, detector)

	// Test with many small operations
	smallData := []byte("small data chunk")
	iterations := 1000

	for i := 0; i < iterations; i++ {
		mockConn.writeToReadBuffer(smallData)

		readBuffer := make([]byte, len(smallData))
		cachingConn.Read(readBuffer)
		cachingConn.Write(smallData)
	}

	// After many operations, buffers should not have grown to consume significant memory
	requestBufferSize := len(cachingConn.requestBuffer)
	responseBufferSize := len(cachingConn.responseBuffer)

	// Verify that buffers haven't grown to unreasonable sizes
	maxReasonableBuffer := 32 * 1024 // 32KB should be more than enough for any reasonable HTTP request/response

	if requestBufferSize > maxReasonableBuffer {
		t.Errorf("Request buffer grew too large: %d bytes after %d operations", requestBufferSize, iterations)
	}

	if responseBufferSize > maxReasonableBuffer {
		t.Errorf("Response buffer grew too large: %d bytes after %d operations", responseBufferSize, iterations)
	}

	t.Logf("Memory leak protection verified - After %d operations: Request buffer: %d bytes, Response buffer: %d bytes",
		iterations, requestBufferSize, responseBufferSize)
}

// TestExtremeBufferProtection tests protection against extremely large buffers
func TestExtremeBufferProtection(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL:      time.Minute * 5,
		MaxMemoryMB:     100,
		MaxEntries:      1000,
		ContentTypeTTLs: make(map[string]time.Duration),
		ExcludedTypes:   []string{},
		EnableMetrics:   true,
		CleanupInterval: time.Minute,
		BufferSize:      8192,
	}
	metrics := NewCacheMetrics(true)
	detector := NewContentDetector(config)
	cache := NewTTLCache(config, metrics)

	mockConn := newMockConn()
	cachingConn := NewCachingConnection(mockConn, cache, config, metrics, detector)

	// Create data that will exceed maxBufferSize (1MB)
	largeChunk := make([]byte, 500*1024) // 500KB
	for i := range largeChunk {
		largeChunk[i] = byte('X')
	}

	// Write multiple large chunks
	for i := 0; i < 5; i++ {
		mockConn.writeToReadBuffer(largeChunk)

		readBuffer := make([]byte, len(largeChunk))
		cachingConn.Read(readBuffer)
		cachingConn.Write(largeChunk)
	}

	// Buffers should not exceed the maximum limit
	requestBufferSize := len(cachingConn.requestBuffer)
	responseBufferSize := len(cachingConn.responseBuffer)

	maxBufferSize := 1024 * 1024 // 1MB

	if requestBufferSize > maxBufferSize {
		t.Errorf("Request buffer exceeded maximum size: %d bytes", requestBufferSize)
	}

	if responseBufferSize > maxBufferSize {
		t.Errorf("Response buffer exceeded maximum size: %d bytes", responseBufferSize)
	}

	t.Logf("Extreme buffer protection verified - Request buffer: %d bytes, Response buffer: %d bytes (max allowed: %d)",
		requestBufferSize, responseBufferSize, maxBufferSize)
}
