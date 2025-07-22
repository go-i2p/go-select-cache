package selectcache

import (
	"testing"
	"time"
)

// TestConnectionBufferMemoryLeakFixed verifies the memory leak fix
func TestConnectionBufferMemoryLeakFixed(t *testing.T) {
	// Create test dependencies
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

	// Create mock connection
	mockConn := newMockConn()

	// Create caching connection
	cachingConn := NewCachingConnection(mockConn, cache, config, metrics, detector)

	// Simulate multiple HTTP requests to test buffer management
	requestData := []byte("GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n")
	responseData := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 5\r\n\r\nHello")

	maxBufferSizeObserved := 0

	// Simulate 10 requests/responses on the same connection
	for i := 0; i < 10; i++ {
		// Simulate incoming request data
		mockConn.writeToReadBuffer(requestData)

		readBuffer := make([]byte, len(requestData))
		n, err := cachingConn.Read(readBuffer)
		if err != nil {
			t.Errorf("Unexpected error on read %d: %v", i, err)
		}
		if n != len(requestData) {
			t.Errorf("Expected to read %d bytes, got %d", len(requestData), n)
		}

		// Check buffer size after read
		currentRequestBufferSize := len(cachingConn.requestBuffer)
		if currentRequestBufferSize > maxBufferSizeObserved {
			maxBufferSizeObserved = currentRequestBufferSize
		}

		// Simulate outgoing response data
		n, err = cachingConn.Write(responseData)
		if err != nil {
			t.Errorf("Unexpected error on write %d: %v", i, err)
		}
		if n != len(responseData) {
			t.Errorf("Expected to write %d bytes, got %d", len(responseData), n)
		}

		// Check buffer size after write
		currentResponseBufferSize := len(cachingConn.responseBuffer)
		if currentResponseBufferSize > maxBufferSizeObserved {
			maxBufferSizeObserved = currentResponseBufferSize
		}

		// Give some time for background processing
		time.Sleep(1 * time.Millisecond)
	}

	// Final buffer sizes should be reasonable (not continuously growing)
	finalRequestBufferLen := len(cachingConn.requestBuffer)
	finalResponseBufferLen := len(cachingConn.responseBuffer)

	// The fix: buffers should not grow unboundedly
	// They should either be cleared or stay within reasonable limits
	maxReasonableSize := len(requestData) + len(responseData) // Allow for one request+response worth of buffering

	if finalRequestBufferLen > maxReasonableSize {
		t.Errorf("Request buffer too large: %d bytes (max reasonable: %d)", finalRequestBufferLen, maxReasonableSize)
	}

	if finalResponseBufferLen > maxReasonableSize {
		t.Errorf("Response buffer too large: %d bytes (max reasonable: %d)", finalResponseBufferLen, maxReasonableSize)
	}

	// Verify the buffer won't grow beyond the maximum limit
	if maxBufferSizeObserved > 1024*1024 { // 1MB limit
		t.Errorf("Buffer size exceeded maximum limit: %d bytes", maxBufferSizeObserved)
	}

	t.Logf("Memory leak fix verified - Final buffer sizes: Request=%d bytes, Response=%d bytes, Max observed=%d bytes",
		finalRequestBufferLen, finalResponseBufferLen, maxBufferSizeObserved)
}

// TestBufferSizeLimit tests that buffers don't exceed the maximum size limit
func TestBufferSizeLimit(t *testing.T) {
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

	// Try to overflow the buffer with large data
	largeData := make([]byte, 2*1024*1024) // 2MB - larger than maxBufferSize
	for i := range largeData {
		largeData[i] = byte('A')
	}

	mockConn.writeToReadBuffer(largeData)

	readBuffer := make([]byte, len(largeData))
	n, err := cachingConn.Read(readBuffer)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Expected to read %d bytes, got %d", len(largeData), n)
	}

	// Buffer should be cleared due to size limit
	bufferSize := len(cachingConn.requestBuffer)
	if bufferSize > 1024*1024 { // Should not exceed 1MB
		t.Errorf("Buffer size exceeded limit: %d bytes", bufferSize)
	}

	t.Logf("Buffer size limit enforced - Buffer size: %d bytes", bufferSize)
}
