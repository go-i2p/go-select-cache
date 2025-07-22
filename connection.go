package selectcache

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// Maximum buffer size to prevent memory leaks - 1MB should be sufficient for most HTTP requests/responses
	maxBufferSize = 1024 * 1024
)

// CachingConnection wraps a net.Conn to provide transparent response caching
type CachingConnection struct {
	net.Conn
	id       string
	cache    *TTLCache
	config   *CacheConfig
	metrics  *CacheMetrics
	detector *ContentDetector

	// Request/response tracking
	readMu         sync.Mutex   // Protects read operations and request buffer
	writeMu        sync.Mutex   // Protects write operations and response buffer
	stateMu        sync.RWMutex // Protects shared connection state
	requestBuffer  []byte
	responseBuffer []byte
	isHTTPRequest  bool
	cacheKey       string
	currentRequest *http.Request

	// Connection state
	closed   bool
	readPos  int
	writePos int

	// Timeouts
	readDeadline  time.Time
	writeDeadline time.Time

	// Close callback
	closeCallback func()
}

// NewCachingConnection creates a new caching connection wrapper
func NewCachingConnection(conn net.Conn, cache *TTLCache, config *CacheConfig, metrics *CacheMetrics, detector *ContentDetector) *CachingConnection {
	id := generateConnectionID()

	return &CachingConnection{
		Conn:     conn,
		id:       id,
		cache:    cache,
		config:   config,
		metrics:  metrics,
		detector: detector,
	}
}

// ID returns the unique identifier for this connection
func (c *CachingConnection) ID() string {
	return c.id
}

// Read intercepts read operations to analyze requests
func (c *CachingConnection) Read(b []byte) (int, error) {
	// Check closed state without holding any locks for long
	c.stateMu.RLock()
	closed := c.closed
	c.stateMu.RUnlock()

	if closed {
		return 0, io.EOF
	}

	// Read from underlying connection first (no locks held)
	n, err := c.Conn.Read(b)
	if err != nil {
		return n, err
	}
	// Only lock for buffer operations
	c.readMu.Lock()

	// Check buffer size limit to prevent memory leaks
	if len(c.requestBuffer)+n > maxBufferSize {
		// Clear buffer and reset to prevent unbounded growth
		c.requestBuffer = c.requestBuffer[:0]
	}

	c.requestBuffer = append(c.requestBuffer, b[:n]...)

	// Check if we need to parse HTTP request
	needsParsing := !c.isHTTPRequest && len(c.requestBuffer) > 0
	requestBufferCopy := make([]byte, len(c.requestBuffer))
	copy(requestBufferCopy, c.requestBuffer)

	// If buffer is getting large and we can't parse HTTP, clear it
	if len(c.requestBuffer) > 8192 && !c.isHTTPRequest {
		c.requestBuffer = c.requestBuffer[:0]
	}

	c.readMu.Unlock()

	// Parse request outside of locks if needed
	if needsParsing {
		c.tryParseHTTPRequestFromBuffer(requestBufferCopy)
	}

	return n, err
}

// Write intercepts write operations to cache responses
func (c *CachingConnection) Write(b []byte) (int, error) {
	// Check for cached response first
	if cached, written := c.tryServeCachedResponse(b); cached {
		return written, nil
	}

	// Write data and capture response
	n, err := c.writeAndBufferResponse(b)
	if err != nil {
		return n, err
	}

	// Check if response analysis is needed
	c.checkAndAnalyzeResponse(b)

	return n, err
}

// tryServeCachedResponse checks if a cached response exists and serves it if found.
func (c *CachingConnection) tryServeCachedResponse(b []byte) (bool, int) {
	c.stateMu.RLock()
	closed := c.closed
	cacheKey := c.cacheKey
	c.stateMu.RUnlock()

	if closed {
		return true, 0 // Signal error will be handled upstream
	}

	if cacheKey != "" {
		if entry, found := c.cache.Get(cacheKey); found {
			// Clear cache key to prevent subsequent cache lookups on same connection
			c.stateMu.Lock()
			c.cacheKey = ""
			c.stateMu.Unlock()
			// Return cached response
			cachedData := c.buildHTTPResponse(entry)
			written, _ := c.writeCachedResponse(cachedData, len(b))
			return true, written
		}
	}

	return false, 0
}

// writeAndBufferResponse writes data to the underlying connection and buffers it for analysis.
func (c *CachingConnection) writeAndBufferResponse(b []byte) (int, error) {
	// Check if connection is closed
	c.stateMu.RLock()
	closed := c.closed
	c.stateMu.RUnlock()

	if closed {
		return 0, io.ErrClosedPipe
	}

	// Write to underlying connection first
	n, err := c.Conn.Write(b)
	if err != nil {
		return n, err
	}
	// Only lock for buffer operations
	c.writeMu.Lock()

	// Check buffer size limit to prevent memory leaks
	if len(c.responseBuffer)+len(b) > maxBufferSize {
		// Clear buffer and reset to prevent unbounded growth
		c.responseBuffer = c.responseBuffer[:0]
	}

	c.responseBuffer = append(c.responseBuffer, b...)

	// If response buffer is getting large and we haven't analyzed yet, clear it periodically
	// This prevents memory buildup for non-HTTP traffic or failed parsing
	if len(c.responseBuffer) > 16384 { // 16KB threshold
		c.responseBuffer = c.responseBuffer[:0]
	}

	c.writeMu.Unlock()

	return n, err
}

// checkAndAnalyzeResponse determines if response analysis is needed and triggers it.
func (c *CachingConnection) checkAndAnalyzeResponse(b []byte) {
	c.writeMu.Lock()
	responseBufferCopy := make([]byte, len(c.responseBuffer))
	copy(responseBufferCopy, c.responseBuffer)
	needsAnalysis := c.shouldAnalyzeResponse(b)
	c.writeMu.Unlock()

	if needsAnalysis {
		c.stateMu.RLock()
		cacheKey := c.cacheKey
		c.stateMu.RUnlock()
		c.analyzeAndCacheResponseFromBuffer(responseBufferCopy, cacheKey)
	}
}

// shouldAnalyzeResponse determines if the current response data should be analyzed for caching.
func (c *CachingConnection) shouldAnalyzeResponse(b []byte) bool {
	if len(c.responseBuffer) == 0 {
		return false
	}

	headerEnd := bytes.Index(c.responseBuffer, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(c.responseBuffer, []byte("\n\n"))
	}

	// Only analyze if we have headers and this write might be the last one
	// (heuristic: small write after headers suggests end of response)
	return headerEnd != -1 && (len(b) < 1024 ||
		bytes.Contains(b, []byte("\r\n\r\n")) ||
		bytes.Contains(b, []byte("\n\n")))
}

// Close closes the connection and performs cleanup
func (c *CachingConnection) Close() error {
	// Check if already closed first (without holding locks for long)
	c.stateMu.RLock()
	alreadyClosed := c.closed
	c.stateMu.RUnlock()

	if alreadyClosed {
		return nil
	}

	// Clear buffers first (avoiding lock ordering issues)
	// Do this before acquiring stateMu to prevent deadlock
	c.readMu.Lock()
	c.requestBuffer = nil
	c.readMu.Unlock()

	c.writeMu.Lock()
	c.responseBuffer = nil
	c.writeMu.Unlock()

	// Now acquire state lock and set closed flag
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	// Double-check to prevent race condition
	if c.closed {
		return nil
	}

	c.closed = true

	// Call the close callback if set
	if c.closeCallback != nil {
		c.closeCallback()
	}

	return c.Conn.Close()
}

// SetCloseCallback sets a callback function to be called when the connection closes
func (c *CachingConnection) SetCloseCallback(callback func()) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.closeCallback = callback
}

// SetDeadline sets both read and write deadlines
func (c *CachingConnection) SetDeadline(t time.Time) error {
	c.stateMu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.stateMu.Unlock()

	return c.Conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline
func (c *CachingConnection) SetReadDeadline(t time.Time) error {
	c.stateMu.Lock()
	c.readDeadline = t
	c.stateMu.Unlock()

	return c.Conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *CachingConnection) SetWriteDeadline(t time.Time) error {
	c.stateMu.Lock()
	c.writeDeadline = t
	c.stateMu.Unlock()

	return c.Conn.SetWriteDeadline(t)
}

// tryParseHTTPRequestFromBuffer attempts to parse an HTTP request from the provided buffer
func (c *CachingConnection) tryParseHTTPRequestFromBuffer(requestBuffer []byte) {
	// Look for end of HTTP headers (double CRLF)
	headerEnd := bytes.Index(requestBuffer, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		// Try LF only
		headerEnd = bytes.Index(requestBuffer, []byte("\n\n"))
		if headerEnd == -1 {
			return // Not enough data yet
		}
	}

	// Parse the request
	reader := bytes.NewReader(requestBuffer[:headerEnd+4])
	bufReader := bufio.NewReader(reader)

	req, err := http.ReadRequest(bufReader)
	if err != nil {
		return // Not a valid HTTP request
	}

	// Update shared state with proper locking
	c.stateMu.Lock()
	c.isHTTPRequest = true
	c.currentRequest = req
	c.stateMu.Unlock()

	// Clear request buffer after successful parsing to prevent memory leaks
	c.readMu.Lock()
	c.requestBuffer = c.requestBuffer[:0]
	c.readMu.Unlock()

	// Generate cache key for GET and HEAD requests
	if req.Method == "GET" || req.Method == "HEAD" {
		headers := make(map[string]string)

		// Include caching-relevant headers
		for _, header := range []string{"Accept", "Accept-Encoding", "Accept-Language", "Authorization"} {
			if value := req.Header.Get(header); value != "" {
				headers[header] = value
			}
		}

		query := ""
		if req.URL.RawQuery != "" {
			query = req.URL.RawQuery
		}

		cacheKey := GenerateCacheKey(req.Method, req.URL.Path, query, headers)

		// Update cache key with proper locking
		c.stateMu.Lock()
		c.cacheKey = cacheKey
		c.stateMu.Unlock()
	}
}

// analyzeAndCacheResponseFromBuffer analyzes the response from the provided buffer and caches it if appropriate
func (c *CachingConnection) analyzeAndCacheResponseFromBuffer(responseBuffer []byte, cacheKey string) {
	// Safely read shared state
	c.stateMu.RLock()
	isHTTPRequest := c.isHTTPRequest
	c.stateMu.RUnlock()

	if !isHTTPRequest || cacheKey == "" || len(responseBuffer) == 0 {
		return
	}

	// Look for end of HTTP headers
	headerEnd := bytes.Index(responseBuffer, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(responseBuffer, []byte("\n\n"))
		if headerEnd == -1 {
			return // Headers not complete yet
		}
	}

	// Parse response headers
	headerData := responseBuffer[:headerEnd]
	bodyData := responseBuffer[headerEnd+4:]

	resp, err := c.parseHTTPResponse(headerData, bodyData)
	if err != nil {
		return // Invalid response
	}

	// Analyze response for caching
	analysis := c.detector.AnalyzeResponse(bodyData, resp.Header, resp.StatusCode)

	if analysis.IsCacheable {
		// Store in cache
		ttl := analysis.RecommendedTTL
		if ttl == 0 {
			ttl = c.config.DefaultTTL
		}

		err := c.cache.Set(cacheKey, bodyData, resp.Header, ttl)
		if err != nil && c.metrics != nil {
			c.metrics.RecordError("cache_store_failed")
		}
	}

	// Clear response buffer after successful analysis to prevent memory leaks
	c.writeMu.Lock()
	c.responseBuffer = c.responseBuffer[:0]
	c.writeMu.Unlock()
}

// parseHTTPResponse parses HTTP response headers and creates a response object
func (c *CachingConnection) parseHTTPResponse(headerData, bodyData []byte) (*http.Response, error) {
	// Parse status line
	lines := bytes.Split(headerData, []byte("\n"))
	if len(lines) == 0 {
		return nil, fmt.Errorf("no status line")
	}

	statusLine := string(bytes.TrimSpace(lines[0]))
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid status line")
	}

	// Parse status code
	var statusCode int
	if _, err := fmt.Sscanf(parts[1], "%d", &statusCode); err != nil {
		return nil, fmt.Errorf("invalid status code: %s", parts[1])
	}

	// Parse headers
	headers := make(http.Header)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(string(lines[i]))
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers.Add(key, value)
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Header:     headers,
	}

	return resp, nil
}

// buildHTTPResponse constructs an HTTP response from a cache entry
func (c *CachingConnection) buildHTTPResponse(entry *CacheEntry) []byte {
	var buf bytes.Buffer

	// Status line (assume HTTP/1.1 and 200 OK for cached responses)
	buf.WriteString("HTTP/1.1 200 OK\r\n")

	// Headers
	for key, values := range entry.Headers {
		for _, value := range values {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// Add cache-specific headers
	buf.WriteString("X-Cache-Status: HIT\r\n")
	buf.WriteString(fmt.Sprintf("X-Cache-Age: %d\r\n", int(time.Since(entry.StoreTime).Seconds())))

	// End of headers
	buf.WriteString("\r\n")

	// Body
	buf.Write(entry.Data)

	return buf.Bytes()
}

// writeCachedResponse writes a cached response directly to the underlying connection
func (c *CachingConnection) writeCachedResponse(data []byte, originalLength int) (int, error) {
	_, err := c.Conn.Write(data)
	if err == nil && c.metrics != nil {
		c.metrics.RecordHit()
	}
	// Return the length of the original write that was requested
	return originalLength, err
}

// generateConnectionID creates a unique identifier for the connection
func generateConnectionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// LocalAddr returns the local network address
func (c *CachingConnection) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *CachingConnection) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// GetStats returns statistics for this connection
func (c *CachingConnection) GetStats() ConnectionStats {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	// Also need to safely read the buffer sizes
	c.readMu.Lock()
	requestSize := len(c.requestBuffer)
	c.readMu.Unlock()

	c.writeMu.Lock()
	responseSize := len(c.responseBuffer)
	c.writeMu.Unlock()

	return ConnectionStats{
		ID:            c.id,
		IsHTTPRequest: c.isHTTPRequest,
		HasCacheKey:   c.cacheKey != "",
		RequestSize:   requestSize,
		ResponseSize:  responseSize,
		LocalAddr:     c.LocalAddr().String(),
		RemoteAddr:    c.RemoteAddr().String(),
		Closed:        c.closed,
	}
}

// ConnectionStats contains statistics for a caching connection
type ConnectionStats struct {
	ID            string `json:"id"`
	IsHTTPRequest bool   `json:"is_http_request"`
	HasCacheKey   bool   `json:"has_cache_key"`
	RequestSize   int    `json:"request_size"`
	ResponseSize  int    `json:"response_size"`
	LocalAddr     string `json:"local_addr"`
	RemoteAddr    string `json:"remote_addr"`
	Closed        bool   `json:"closed"`
}
