package selectcache

import (
	"net/http"
	"testing"
)

func TestContentTypeBasedDetection(t *testing.T) {
	config := DefaultCacheConfig()
	detector := NewContentDetector(config)

	tests := []struct {
		name        string
		contentType string
		content     string
		expectHTML  bool
	}{
		{
			name:        "HTML with proper Content-Type",
			contentType: "text/html",
			content:     "<html><body>Hello</body></html>",
			expectHTML:  true,
		},
		{
			name:        "XHTML with proper Content-Type",
			contentType: "application/xhtml+xml",
			content:     "<html><body>Hello</body></html>",
			expectHTML:  true,
		},
		{
			name:        "HTML content without Content-Type header",
			contentType: "",
			content:     "<!DOCTYPE html><html><body>Hello</body></html>",
			expectHTML:  false, // Should NOT be detected as HTML without Content-Type
		},
		{
			name:        "JSON with proper Content-Type",
			contentType: "application/json",
			content:     `{"key": "value"}`,
			expectHTML:  false,
		},
		{
			name:        "HTML-looking content with JSON Content-Type",
			contentType: "application/json",
			content:     "<html>This looks like HTML but isn't</html>",
			expectHTML:  false, // Content-Type takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(http.Header)
			if tt.contentType != "" {
				headers.Set("Content-Type", tt.contentType)
			}

			isHTML := detector.IsHTMLContent([]byte(tt.content), headers)
			if isHTML != tt.expectHTML {
				t.Errorf("IsHTMLContent() = %v, want %v", isHTML, tt.expectHTML)
			}

			// Test caching decision
			shouldCache := detector.ShouldCache([]byte(tt.content), headers, 200)
			expectedCache := !tt.expectHTML && !config.IsContentTypeExcluded(tt.contentType)
			if shouldCache != expectedCache {
				t.Errorf("ShouldCache() = %v, want %v", shouldCache, expectedCache)
			}
		})
	}
}
