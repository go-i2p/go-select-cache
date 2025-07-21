package selectcache

import (
	"net/http"
	"testing"
	"time"
)

func TestContentDetector_IsHTMLContent(t *testing.T) {
	config := DefaultCacheConfig()
	detector := NewContentDetector(config)

	tests := []struct {
		name       string
		response   []byte
		headers    http.Header
		expectHTML bool
	}{
		{
			name:     "HTML content-type header",
			response: []byte("<h1>Hello</h1>"),
			headers: http.Header{
				"Content-Type": []string{"text/html; charset=utf-8"},
			},
			expectHTML: true,
		},
		{
			name:     "XHTML content-type header",
			response: []byte("<html></html>"),
			headers: http.Header{
				"Content-Type": []string{"application/xhtml+xml"},
			},
			expectHTML: true,
		},
		{
			name:       "HTML content without header - not detected",
			response:   []byte("<!DOCTYPE html><html><head></head><body></body></html>"),
			headers:    http.Header{},
			expectHTML: false, // Without Content-Type header, not detected as HTML
		},
		{
			name:       "HTML tags without header - not detected",
			response:   []byte("<html><head><title>Test</title></head><body><h1>Test</h1></body></html>"),
			headers:    http.Header{},
			expectHTML: false, // Without Content-Type header, not detected as HTML
		},
		{
			name:     "JSON content",
			response: []byte(`{"name": "test", "value": 123}`),
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expectHTML: false,
		},
		{
			name:       "Plain text",
			response:   []byte("This is just plain text content without any HTML tags."),
			headers:    http.Header{},
			expectHTML: false,
		},
		{
			name:       "Empty content",
			response:   []byte(""),
			headers:    http.Header{},
			expectHTML: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isHTML := detector.IsHTMLContent(tt.response, tt.headers)
			if isHTML != tt.expectHTML {
				t.Errorf("IsHTMLContent() = %v, want %v", isHTML, tt.expectHTML)
			}
		})
	}
}

func TestContentDetector_ShouldCache(t *testing.T) {
	config := DefaultCacheConfig()
	detector := NewContentDetector(config)

	tests := []struct {
		name        string
		response    []byte
		headers     http.Header
		statusCode  int
		shouldCache bool
	}{
		{
			name:     "JSON API response - should cache",
			response: []byte(`{"data": "test"}`),
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			statusCode:  200,
			shouldCache: true,
		},
		{
			name:     "HTML page - should not cache",
			response: []byte("<!DOCTYPE html><html><body><h1>Test</h1></body></html>"),
			headers: http.Header{
				"Content-Type": []string{"text/html; charset=utf-8"},
			},
			statusCode:  200,
			shouldCache: false,
		},
		{
			name:     "Error response - should not cache",
			response: []byte(`{"error": "not found"}`),
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			statusCode:  404,
			shouldCache: false,
		},
		{
			name:     "CSS file - should cache",
			response: []byte("body { color: red; }"),
			headers: http.Header{
				"Content-Type": []string{"text/css"},
			},
			statusCode:  200,
			shouldCache: true,
		},
		{
			name:     "Image file - should cache",
			response: make([]byte, 1024), // Simulated image data
			headers: http.Header{
				"Content-Type": []string{"image/png"},
			},
			statusCode:  200,
			shouldCache: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldCache := detector.ShouldCache(tt.response, tt.headers, tt.statusCode)
			if shouldCache != tt.shouldCache {
				t.Errorf("ShouldCache() = %v, want %v", shouldCache, tt.shouldCache)
			}
		})
	}
}

func TestContentDetector_GetContentType(t *testing.T) {
	config := DefaultCacheConfig()
	detector := NewContentDetector(config)

	tests := []struct {
		name         string
		headers      http.Header
		expectedType string
	}{
		{
			name: "JSON content type",
			headers: http.Header{
				"Content-Type": []string{"application/json; charset=utf-8"},
			},
			expectedType: "application/json",
		},
		{
			name: "HTML content type",
			headers: http.Header{
				"Content-Type": []string{"text/html; charset=utf-8"},
			},
			expectedType: "text/html",
		},
		{
			name:         "Missing content type",
			headers:      http.Header{},
			expectedType: "application/octet-stream",
		},
		{
			name: "Plain text",
			headers: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			expectedType: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentType := detector.GetContentType(tt.headers)
			if contentType != tt.expectedType {
				t.Errorf("GetContentType() = %v, want %v", contentType, tt.expectedType)
			}
		})
	}
}

func TestContentDetector_DetectContentTypeFromBytes(t *testing.T) {
	config := DefaultCacheConfig()
	detector := NewContentDetector(config)

	tests := []struct {
		name         string
		data         []byte
		expectedType string
	}{
		{
			name:         "JSON data",
			data:         []byte(`{"key": "value"}`),
			expectedType: "application/json",
		},
		{
			name:         "JSON array",
			data:         []byte(`[{"id": 1}, {"id": 2}]`),
			expectedType: "application/json",
		},
		{
			name:         "XML data",
			data:         []byte(`<?xml version="1.0"?><root><item>test</item></root>`),
			expectedType: "application/xml",
		},
		{
			name:         "HTML data without content-type - detected as plain text",
			data:         []byte(`<!DOCTYPE html><html><head><title>Test</title></head></html>`),
			expectedType: "text/plain",
		},
		{
			name:         "PNG image",
			data:         []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expectedType: "image/png",
		},
		{
			name:         "JPEG image",
			data:         []byte{0xFF, 0xD8, 0xFF, 0xE0},
			expectedType: "image/jpeg",
		},
		{
			name:         "GIF image",
			data:         []byte("GIF89a"),
			expectedType: "image/gif",
		},
		{
			name:         "PDF document",
			data:         []byte("%PDF-1.4"),
			expectedType: "application/pdf",
		},
		{
			name:         "Plain text",
			data:         []byte("This is just plain text content."),
			expectedType: "text/plain",
		},
		{
			name:         "Binary data",
			data:         []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE},
			expectedType: "application/octet-stream",
		},
		{
			name:         "Empty data",
			data:         []byte{},
			expectedType: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentType := detector.DetectContentTypeFromBytes(tt.data)
			if contentType != tt.expectedType {
				t.Errorf("DetectContentTypeFromBytes() = %v, want %v", contentType, tt.expectedType)
			}
		})
	}
}

func TestContentDetector_AnalyzeResponse(t *testing.T) {
	config := DefaultCacheConfig()
	config.ContentTypeTTLs = map[string]time.Duration{
		"application/json": 5 * time.Minute,
		"image/png":        1 * time.Hour,
	}
	detector := NewContentDetector(config)

	tests := []struct {
		name            string
		response        []byte
		headers         http.Header
		statusCode      int
		expectCacheable bool
		expectHTML      bool
	}{
		{
			name:     "Cacheable JSON API",
			response: []byte(`{"data": "test"}`),
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			statusCode:      200,
			expectCacheable: true,
			expectHTML:      false,
		},
		{
			name:     "Non-cacheable HTML",
			response: []byte("<!DOCTYPE html><html><body>Test</body></html>"),
			headers: http.Header{
				"Content-Type": []string{"text/html"},
			},
			statusCode:      200,
			expectCacheable: false,
			expectHTML:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := detector.AnalyzeResponse(tt.response, tt.headers, tt.statusCode)

			if analysis.IsCacheable != tt.expectCacheable {
				t.Errorf("IsCacheable = %v, want %v", analysis.IsCacheable, tt.expectCacheable)
			}

			if analysis.IsHTML != tt.expectHTML {
				t.Errorf("IsHTML = %v, want %v", analysis.IsHTML, tt.expectHTML)
			}

			if analysis.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %v, want %v", analysis.StatusCode, tt.statusCode)
			}

			if analysis.Size != len(tt.response) {
				t.Errorf("Size = %v, want %v", analysis.Size, len(tt.response))
			}
		})
	}
}
