package selectcache

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestHTTPParsingPerformanceComparison compares custom parsing vs standard library
func TestHTTPParsingPerformanceComparison(t *testing.T) {
	// Create test HTTP response data
	responseData := createTestHTTPResponse()

	// Find header/body boundary
	headerEnd := bytes.Index(responseData, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		t.Fatal("Could not find header boundary")
	}

	headerData := responseData[:headerEnd]
	bodyData := responseData[headerEnd+4:]

	// Create a caching connection for testing
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)
	conn := &mockConnection{}
	cachingConn := NewCachingConnection(conn, cache, config, metrics, detector)

	t.Run("CustomParsing", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			resp, err := cachingConn.parseHTTPResponse(headerData, bodyData)
			if err != nil {
				t.Fatalf("Custom parsing failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
		customDuration := time.Since(start)
		t.Logf("Custom parsing took: %v", customDuration)
	})

	t.Run("StandardLibraryParsing", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			resp, err := parseHTTPResponseWithStandardLibrary(responseData)
			if err != nil {
				t.Fatalf("Standard library parsing failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
		stdDuration := time.Since(start)
		t.Logf("Standard library parsing took: %v", stdDuration)
	})
}

// TestHTTPParsingCompatibility ensures both methods produce equivalent results
func TestHTTPParsingCompatibility(t *testing.T) {
	responseData := createTestHTTPResponse()

	// Find header/body boundary
	headerEnd := bytes.Index(responseData, []byte("\r\n\r\n"))
	headerData := responseData[:headerEnd]
	bodyData := responseData[headerEnd+4:]

	// Create a caching connection for testing
	config := DefaultCacheConfig()
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)
	conn := &mockConnection{}
	cachingConn := NewCachingConnection(conn, cache, config, metrics, detector)

	// Parse with custom method
	customResp, err := cachingConn.parseHTTPResponse(headerData, bodyData)
	if err != nil {
		t.Fatalf("Custom parsing failed: %v", err)
	}

	// Parse with standard library
	stdResp, err := parseHTTPResponseWithStandardLibrary(responseData)
	if err != nil {
		t.Fatalf("Standard library parsing failed: %v", err)
	}

	// Compare results
	if customResp.StatusCode != stdResp.StatusCode {
		t.Errorf("Status codes differ: custom=%d, std=%d", customResp.StatusCode, stdResp.StatusCode)
	}

	// Compare important headers
	testHeaders := []string{"Content-Type", "Content-Length", "Cache-Control"}
	for _, header := range testHeaders {
		customVal := customResp.Header.Get(header)
		stdVal := stdResp.Header.Get(header)
		if customVal != stdVal {
			t.Errorf("Header %s differs: custom=%q, std=%q", header, customVal, stdVal)
		}
	}
}

// parseHTTPResponseWithStandardLibrary demonstrates using Go's standard library
func parseHTTPResponseWithStandardLibrary(responseData []byte) (*http.Response, error) {
	reader := bufio.NewReader(bytes.NewReader(responseData))

	// Create a mock request (required by ReadResponse)
	req := &http.Request{
		Method: "GET",
	}

	return http.ReadResponse(reader, req)
}

// createTestHTTPResponse creates a sample HTTP response for testing
func createTestHTTPResponse() []byte {
	response := strings.Join([]string{
		"HTTP/1.1 200 OK",
		"Content-Type: application/json",
		"Content-Length: 27",
		"Cache-Control: max-age=3600",
		"Server: nginx/1.18.0",
		"Date: " + time.Now().Format(http.TimeFormat),
		"",
		`{"message": "Hello World"}`,
	}, "\r\n")

	return []byte(response)
}

// mockConnection implements net.Conn for testing
type mockConnection struct {
	data []byte
	pos  int
}

func (m *mockConnection) Read(b []byte) (int, error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n := copy(b, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockConnection) Write(b []byte) (int, error) {
	return len(b), nil
}

func (m *mockConnection) Close() error {
	return nil
}

func (m *mockConnection) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:8080"}
}

func (m *mockConnection) RemoteAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:12345"}
}

func (m *mockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	address string
}

func (m *mockAddr) Network() string {
	return "tcp"
}

func (m *mockAddr) String() string {
	return m.address
}
