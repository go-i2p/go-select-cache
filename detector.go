package selectcache

import (
	"bytes"
	"net/http"
	"strings"
	"time"
)

// ContentDetector analyzes response data to determine content type and cacheability
type ContentDetector struct {
	config *CacheConfig
}

// NewContentDetector creates a new content detector with the given configuration
func NewContentDetector(config *CacheConfig) *ContentDetector {
	return &ContentDetector{
		config: config,
	}
}

// IsHTMLContent determines if the response contains HTML content based on Content-Type header
func (d *ContentDetector) IsHTMLContent(response []byte, headers http.Header) bool {
	// Only use Content-Type header examination
	contentType := headers.Get("Content-Type")
	return d.isHTMLContentType(contentType)
}

// ShouldCache determines if a response should be cached based on content analysis
func (d *ContentDetector) ShouldCache(response []byte, headers http.Header, statusCode int) bool {
	// Check if status code is cacheable (typically 200, 301, 304, etc.)
	if !d.isCacheableStatusCode(statusCode) {
		return false
	}

	// Check content type exclusions
	contentType := headers.Get("Content-Type")
	if d.config.IsContentTypeExcluded(contentType) {
		return false // Excluded means don't cache
	}

	// Check for HTML content using multiple detection strategies
	if d.IsHTMLContent(response, headers) {
		return false // Don't cache HTML
	}

	// Check response size limits (avoid caching very large responses)
	if len(response) > int(d.config.MaxMemoryMB)*1024*1024/10 { // Max 10% of total cache for single entry
		return false
	}

	return true
}

// GetContentType extracts and normalizes the content type from headers
func (d *ContentDetector) GetContentType(headers http.Header) string {
	contentType := headers.Get("Content-Type")
	if contentType == "" {
		return "application/octet-stream" // Default for unknown content
	}

	// Extract main type (before semicolon)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}

	return strings.TrimSpace(strings.ToLower(contentType))
}

// isHTMLContentType checks if the Content-Type header indicates HTML
func (d *ContentDetector) isHTMLContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	contentTypeLower := strings.ToLower(contentType)

	// Common HTML content types
	htmlTypes := []string{
		"text/html",
		"application/xhtml+xml",
		"application/xhtml",
	}

	for _, htmlType := range htmlTypes {
		if strings.Contains(contentTypeLower, htmlType) {
			return true
		}
	}

	return false
}

// isCacheableStatusCode checks if the HTTP status code indicates a cacheable response
func (d *ContentDetector) isCacheableStatusCode(statusCode int) bool {
	// Common cacheable status codes
	cacheableStatus := []int{
		200, // OK
		201, // Created
		300, // Multiple Choices
		301, // Moved Permanently
		302, // Found
		304, // Not Modified
		307, // Temporary Redirect
		308, // Permanent Redirect
		410, // Gone (can be cached as "not found")
	}

	for _, code := range cacheableStatus {
		if statusCode == code {
			return true
		}
	}

	return false
}

// AnalyzeResponse performs comprehensive analysis of a response for caching decisions
func (d *ContentDetector) AnalyzeResponse(response []byte, headers http.Header, statusCode int) *ResponseAnalysis {
	analysis := &ResponseAnalysis{
		StatusCode:  statusCode,
		ContentType: d.GetContentType(headers),
		Size:        len(response),
		IsHTML:      d.IsHTMLContent(response, headers),
		IsCacheable: false,
	}

	// Determine cacheability
	analysis.IsCacheable = d.ShouldCache(response, headers, statusCode)

	// Set TTL based on content type
	if analysis.IsCacheable {
		analysis.RecommendedTTL = d.config.GetTTLForContentType(analysis.ContentType)
	}

	return analysis
}

// ResponseAnalysis contains the results of response content analysis
type ResponseAnalysis struct {
	StatusCode     int           `json:"status_code"`
	ContentType    string        `json:"content_type"`
	Size           int           `json:"size"`
	IsHTML         bool          `json:"is_html"`
	IsCacheable    bool          `json:"is_cacheable"`
	RecommendedTTL time.Duration `json:"recommended_ttl"`
}

// DetectContentTypeFromBytes attempts to detect content type from response bytes
// This is a fallback when Content-Type header is missing or unreliable
func (d *ContentDetector) DetectContentTypeFromBytes(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	// Try structured data detection first
	if contentType := d.detectStructuredData(data); contentType != "" {
		return contentType
	}

	// Try binary format detection
	if contentType := d.detectBinaryFormats(data); contentType != "" {
		return contentType
	}

	// Check if it's plain text (printable ASCII)
	if d.isPlainText(data) {
		return "text/plain"
	}

	return "application/octet-stream"
}

// detectStructuredData attempts to identify JSON and XML content types
func (d *ContentDetector) detectStructuredData(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ""
	}

	// JSON detection
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return "application/json"
	}

	// XML detection
	if bytes.HasPrefix(trimmed, []byte("<?xml")) {
		return "application/xml"
	}

	return ""
}

// detectBinaryFormats attempts to identify common binary file formats
func (d *ContentDetector) detectBinaryFormats(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	// Image formats
	if contentType := d.detectImageFormats(data); contentType != "" {
		return contentType
	}

	// PDF detection
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "application/pdf"
	}

	return ""
}

// detectImageFormats identifies common image file formats by their signatures
func (d *ContentDetector) detectImageFormats(data []byte) string {
	// JPEG
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}

	// PNG
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return "image/png"
	}

	// GIF
	if bytes.HasPrefix(data, []byte("GIF8")) {
		return "image/gif"
	}

	return ""
}

// isPlainText checks if the data appears to be plain text
func (d *ContentDetector) isPlainText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Sample first 512 bytes
	sampleSize := len(data)
	if sampleSize > 512 {
		sampleSize = 512
	}

	nonPrintable := 0
	for i := 0; i < sampleSize; i++ {
		b := data[i]
		// Allow printable ASCII, tabs, newlines, carriage returns
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 {
			continue
		}
		nonPrintable++
	}

	// If more than 5% non-printable characters, likely binary
	return float64(nonPrintable)/float64(sampleSize) < 0.05
}
