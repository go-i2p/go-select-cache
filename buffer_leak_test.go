package selectcache

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
	closed      bool
	mu          sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuffer:  bytes.NewBuffer(nil),
		writeBuffer: bytes.NewBuffer(nil),
	}
}

func (m *mockConn) Read(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readBuffer.Read(b)
}

func (m *mockConn) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuffer.Write(b)
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// writeToReadBuffer simulates data coming from the network
func (m *mockConn) writeToReadBuffer(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer.Write(data)
}

// TestConnectionBufferMemoryLeakNowFixed verifies the critical buffer memory leak bug is resolved
// This is a negative test confirming the issue from AUDIT.md is resolved
func TestConnectionBufferMemoryLeakNowFixed(t *testing.T) {
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

	// Simulate multiple HTTP requests to demonstrate buffer growth
	requestData := []byte("GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n")
	responseData := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 5\r\n\r\nHello")

	initialRequestBufferLen := len(cachingConn.requestBuffer)
	initialResponseBufferLen := len(cachingConn.responseBuffer)

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

		// Simulate outgoing response data
		n, err = cachingConn.Write(responseData)
		if err != nil {
			t.Errorf("Unexpected error on write %d: %v", i, err)
		}
		if n != len(responseData) {
			t.Errorf("Expected to write %d bytes, got %d", len(responseData), n)
		}
	}

	// Check if buffers have been properly managed (confirming the fix)
	finalRequestBufferLen := len(cachingConn.requestBuffer)
	finalResponseBufferLen := len(cachingConn.responseBuffer)

	// FIXED: With the fix in place, buffers should NOT grow unboundedly
	// They should be cleared periodically or stay within reasonable limits
	expectedRequestGrowth := len(requestData) * 10   // This was the old buggy behavior
	expectedResponseGrowth := len(responseData) * 10 // This was the old buggy behavior

	// The fix should prevent unbounded growth
	if finalRequestBufferLen == initialRequestBufferLen+expectedRequestGrowth {
		t.Errorf("MEMORY LEAK DETECTED: Request buffer grew unboundedly as before the fix: %d->%d bytes",
			initialRequestBufferLen, finalRequestBufferLen)
		t.Errorf("This indicates the buffer management fix is not working")
	} else {
		t.Logf("SUCCESS: Request buffer growth controlled - grew to %d bytes instead of expected unbounded %d bytes",
			finalRequestBufferLen, initialRequestBufferLen+expectedRequestGrowth)
	}

	if finalResponseBufferLen == initialResponseBufferLen+expectedResponseGrowth {
		t.Errorf("MEMORY LEAK DETECTED: Response buffer grew unboundedly as before the fix: %d->%d bytes",
			initialResponseBufferLen, finalResponseBufferLen)
		t.Errorf("This indicates the buffer management fix is not working")
	} else {
		t.Logf("SUCCESS: Response buffer growth controlled - grew to %d bytes instead of expected unbounded %d bytes",
			finalResponseBufferLen, initialResponseBufferLen+expectedResponseGrowth)
	}

	// Additional check: buffers should stay within reasonable bounds
	maxReasonableSize := 16384 // 16KB threshold from the fix
	if finalRequestBufferLen > maxReasonableSize || finalResponseBufferLen > maxReasonableSize {
		t.Errorf("MEMORY LEAK: Buffers exceeded reasonable size - Request: %d, Response: %d (max: %d)",
			finalRequestBufferLen, finalResponseBufferLen, maxReasonableSize)
	} else {
		t.Logf("SUCCESS: Buffers stayed within reasonable limits - Request: %d, Response: %d (max: %d)",
			finalRequestBufferLen, finalResponseBufferLen, maxReasonableSize)
	}

	// This test confirms the memory leak fix is working
	t.Logf("Memory leak fix verified - Request buffer: %d->%d bytes, Response buffer: %d->%d bytes",
		initialRequestBufferLen, finalRequestBufferLen,
		initialResponseBufferLen, finalResponseBufferLen)
}
