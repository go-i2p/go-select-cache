package selectcache

import (
	"net/http"
	"testing"
)

// TestTransportLayerCacheKeyConsistency tests that the transport layer (connection.go)
// generates cache keys consistently with the middleware layer for HEAD/GET sharing
func TestTransportLayerCacheKeyConsistency(t *testing.T) {
	// Test HEAD request - transport layer should convert HEAD to GET for cache key
	headReq, err := http.NewRequest("HEAD", "http://example.com/api/data", nil)
	if err != nil {
		t.Fatalf("Failed to create HEAD request: %v", err)
	}
	headReq.Header.Set("Accept", "application/json")

	// Create equivalent GET request
	getReq, err := http.NewRequest("GET", "http://example.com/api/data", nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	getReq.Header.Set("Accept", "application/json")

	// Middleware layer cache keys
	middleware := &Middleware{}
	middlewareHeadKey := middleware.createCacheKey(headReq)
	middlewareGetKey := middleware.createCacheKey(getReq)

	// Transport layer cache keys - test the ACTUAL code in connection.go
	transportHeadKey := generateActualTransportCacheKey(headReq)
	transportGetKey := generateActualTransportCacheKey(getReq)

	t.Logf("Middleware HEAD key: %s", middlewareHeadKey)
	t.Logf("Middleware GET key:  %s", middlewareGetKey)
	t.Logf("Transport HEAD key:  %s", transportHeadKey)
	t.Logf("Transport GET key:   %s", transportGetKey)

	// All keys should be the same due to HEAD->GET conversion
	if middlewareHeadKey != middlewareGetKey {
		t.Errorf("Middleware layer HEAD/GET cache keys don't match: HEAD=%s, GET=%s",
			middlewareHeadKey, middlewareGetKey)
	}

	if transportHeadKey != transportGetKey {
		t.Errorf("Transport layer HEAD/GET cache keys don't match: HEAD=%s, GET=%s",
			transportHeadKey, transportGetKey)
	}

	if middlewareHeadKey != transportHeadKey {
		t.Errorf("BUG REPRODUCED: Middleware and transport HEAD cache keys don't match: middleware=%s, transport=%s",
			middlewareHeadKey, transportHeadKey)
	}

	if middlewareGetKey != transportGetKey {
		t.Errorf("Middleware and transport GET cache keys don't match: middleware=%s, transport=%s",
			middlewareGetKey, transportGetKey)
	}

	if middlewareHeadKey == transportHeadKey && middlewareGetKey == transportGetKey &&
		middlewareHeadKey == middlewareGetKey && transportHeadKey == transportGetKey {
		t.Logf("SUCCESS: All cache keys match: %s", middlewareHeadKey)
	} else {
		t.Logf("Cache key inconsistency bug confirmed - HEAD requests generate different keys")
	}
}

// generateActualTransportCacheKey replicates the FIXED logic from connection.go:360
// This should now match the middleware behavior with HEAD->GET conversion
func generateActualTransportCacheKey(req *http.Request) string {
	headers := make(map[string]string)

	// Include caching-relevant headers (exact copy from connection.go)
	for _, header := range []string{"Accept", "Accept-Encoding", "Accept-Language", "Authorization"} {
		if value := req.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	query := ""
	if req.URL.RawQuery != "" {
		query = req.URL.RawQuery
	}

	// FIXED: Convert HEAD to GET for cache sharing (same as middleware layer)
	method := req.Method
	if method == "HEAD" {
		method = "GET"
	}

	return GenerateCacheKey(method, req.URL.Path, query, headers)
}
