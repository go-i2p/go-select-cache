package selectcache

import (
	"net/http"
	"testing"
	"time"
)

// TestTTLConfigurationAppliedInTransportLayer verifies that per-content-type
// TTL configuration is properly applied in the transport layer caching
// This is a negative test confirming the issue from AUDIT.md is resolved
func TestTTLConfigurationAppliedInTransportLayer(t *testing.T) {
	// Create config with specific TTL for different content types
	config := &CacheConfig{
		DefaultTTL: 5 * time.Minute,
		ContentTypeTTLs: map[string]time.Duration{
			"application/json":       10 * time.Minute,
			"text/css":               60 * time.Minute,
			"application/javascript": 45 * time.Minute,
		},
		MaxMemoryMB: 10,
		MaxEntries:  100,
	}

	detector := NewContentDetector(config)

	testCases := []struct {
		name         string
		contentType  string
		expectedTTL  time.Duration
		statusCode   int
		responseBody []byte
	}{
		{
			name:         "JSON content uses configured TTL",
			contentType:  "application/json",
			expectedTTL:  10 * time.Minute,
			statusCode:   200,
			responseBody: []byte(`{"test": "data"}`),
		},
		{
			name:         "CSS content uses configured TTL",
			contentType:  "text/css",
			expectedTTL:  60 * time.Minute,
			statusCode:   200,
			responseBody: []byte("body { color: red; }"),
		},
		{
			name:         "JavaScript content uses configured TTL",
			contentType:  "application/javascript",
			expectedTTL:  45 * time.Minute,
			statusCode:   200,
			responseBody: []byte("function test() { return true; }"),
		},
		{
			name:         "Unknown content type uses default TTL",
			contentType:  "application/unknown",
			expectedTTL:  5 * time.Minute,
			statusCode:   200,
			responseBody: []byte("unknown content"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create headers with the content type
			headers := http.Header{}
			headers.Set("Content-Type", tc.contentType)

			// Analyze the response
			analysis := detector.AnalyzeResponse(tc.responseBody, headers, tc.statusCode)

			// FIXED: Verify the TTL is correctly determined (confirming the fix works)
			if !analysis.IsCacheable {
				t.Fatal("Response should be cacheable")
			}

			if analysis.RecommendedTTL != tc.expectedTTL {
				t.Errorf("TTL CONFIGURATION NOT APPLIED: Expected TTL %v, got %v for content type %s",
					tc.expectedTTL, analysis.RecommendedTTL, tc.contentType)
				t.Errorf("This indicates the TTL configuration fix is not working properly")
			} else {
				t.Logf("SUCCESS: TTL configuration correctly applied - %v for %s", tc.expectedTTL, tc.contentType)
			}

			// Also verify content type is correctly parsed
			if analysis.ContentType != tc.contentType {
				t.Errorf("Expected content type %s, got %s",
					tc.contentType, analysis.ContentType)
			}
		})
	}
}

// TestTTLConfigurationWithContentTypeParams verifies TTL handling with
// content type parameters (e.g., "application/json; charset=utf-8")
func TestTTLConfigurationWithContentTypeParams(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL: 5 * time.Minute,
		ContentTypeTTLs: map[string]time.Duration{
			"application/json": 30 * time.Minute,
		},
		MaxMemoryMB: 10,
		MaxEntries:  100,
	}

	detector := NewContentDetector(config)

	testCases := []struct {
		name                string
		contentTypeHeader   string
		expectedTTL         time.Duration
		expectedContentType string
	}{
		{
			name:                "Content type with charset",
			contentTypeHeader:   "application/json; charset=utf-8",
			expectedTTL:         30 * time.Minute,
			expectedContentType: "application/json",
		},
		{
			name:                "Content type with multiple params",
			contentTypeHeader:   "application/json; charset=utf-8; boundary=something",
			expectedTTL:         30 * time.Minute,
			expectedContentType: "application/json",
		},
		{
			name:                "Content type with spaces",
			contentTypeHeader:   " application/json ; charset=utf-8 ",
			expectedTTL:         30 * time.Minute,
			expectedContentType: "application/json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("Content-Type", tc.contentTypeHeader)
			responseBody := []byte(`{"test": "data"}`)

			analysis := detector.AnalyzeResponse(responseBody, headers, 200)

			if !analysis.IsCacheable {
				t.Fatal("Response should be cacheable")
			}

			if analysis.RecommendedTTL != tc.expectedTTL {
				t.Errorf("Expected TTL %v, got %v for content type header %s",
					tc.expectedTTL, analysis.RecommendedTTL, tc.contentTypeHeader)
			}

			if analysis.ContentType != tc.expectedContentType {
				t.Errorf("Expected parsed content type %s, got %s",
					tc.expectedContentType, analysis.ContentType)
			}
		})
	}
}

// TestGetTTLForContentTypeFunction tests the config function directly
func TestGetTTLForContentTypeFunction(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL: 15 * time.Minute,
		ContentTypeTTLs: map[string]time.Duration{
			"text/html":  30 * time.Minute,
			"text/css":   60 * time.Minute,
			"image/jpeg": 24 * time.Hour,
		},
	}

	testCases := []struct {
		contentType string
		expectedTTL time.Duration
	}{
		{"text/html", 30 * time.Minute},
		{"text/css", 60 * time.Minute},
		{"image/jpeg", 24 * time.Hour},
		{"application/json", 15 * time.Minute}, // Default TTL
		{"unknown/type", 15 * time.Minute},     // Default TTL
	}

	for _, tc := range testCases {
		t.Run(tc.contentType, func(t *testing.T) {
			ttl := config.GetTTLForContentType(tc.contentType)
			if ttl != tc.expectedTTL {
				t.Errorf("Expected TTL %v for %s, got %v",
					tc.expectedTTL, tc.contentType, ttl)
			}
		})
	}
}
