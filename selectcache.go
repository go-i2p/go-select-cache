// Package selectcache provides HTTP middleware for selective response caching
// using go-cache, with content-type based filtering to exclude HTML responses.
//
// License: MIT (matches go-cache dependency)
package selectcache

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/patrickmn/go-cache"
)

// Middleware provides selective HTTP response caching
type Middleware struct {
	cache         *cache.Cache
	excludeTypes  []string
	includeStatus []int
	hitCount      uint64 // Atomic counter for cache hits
	missCount     uint64 // Atomic counter for cache misses
}

// Config holds configuration for the caching middleware
type Config struct {
	// DefaultTTL is the default time-to-live for cached responses
	DefaultTTL time.Duration
	// CleanupInterval is how often expired items are removed
	CleanupInterval time.Duration
	// ExcludeContentTypes are MIME types that should not be cached
	// Default: ["text/html", "application/xhtml+xml"]
	ExcludeContentTypes []string
	// IncludeStatusCodes are HTTP status codes that should be cached
	// Default: [200]
	IncludeStatusCodes []int
}

// DefaultConfig returns sensible defaults for the middleware
func DefaultConfig() Config {
	return Config{
		DefaultTTL:      15 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		ExcludeContentTypes: []string{
			"text/html",
			"application/xhtml+xml",
		},
		IncludeStatusCodes: []int{200},
	}
}

// New creates a new selective cache middleware with the given configuration
func New(config Config) *Middleware {
	if len(config.ExcludeContentTypes) == 0 {
		config.ExcludeContentTypes = DefaultConfig().ExcludeContentTypes
	}
	if len(config.IncludeStatusCodes) == 0 {
		config.IncludeStatusCodes = DefaultConfig().IncludeStatusCodes
	}

	return &Middleware{
		cache:         cache.New(config.DefaultTTL, config.CleanupInterval),
		excludeTypes:  config.ExcludeContentTypes,
		includeStatus: config.IncludeStatusCodes,
	}
}

// NewDefault creates a middleware with default settings:
// - 15 minute TTL
// - 5 minute cleanup interval
// - Excludes HTML content types
// - Only caches 200 status responses
func NewDefault() *Middleware {
	return New(DefaultConfig())
}

// Handler wraps an http.Handler with selective caching
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET and HEAD requests
		if !m.isCacheableMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		key := m.createCacheKey(r)

		// Try to serve from cache first
		if m.tryServeFromCache(w, r, key) {
			return
		}

		// Handle cache miss with recording and potential storage
		m.handleCacheMiss(w, r, key, next)
	})
}

// HandlerFunc is a convenience method that wraps an http.HandlerFunc
func (m *Middleware) HandlerFunc(next http.HandlerFunc) http.Handler {
	return m.Handler(next)
}

// createCacheKey generates a cache key from the request
func (m *Middleware) createCacheKey(r *http.Request) string {
	// Use the same cache key generation logic as cache.go for consistency
	// but treat GET and HEAD as the same for caching purposes (HEAD reuses GET cache)
	headers := make(map[string]string)

	// Include caching-relevant headers
	for _, header := range []string{"Accept", "Accept-Encoding", "Accept-Language", "Authorization"} {
		if value := r.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	query := ""
	if r.URL.RawQuery != "" {
		query = r.URL.RawQuery
	}

	// For HEAD requests, use GET method in cache key so they share cache entries
	method := r.Method
	if method == "HEAD" {
		method = "GET"
	}

	return GenerateCacheKey(method, r.URL.Path, query, headers)
}

// shouldCache determines if a response should be cached
func (m *Middleware) shouldCache(recorder *ResponseRecorder) bool {
	// Check status code
	statusOK := false
	for _, code := range m.includeStatus {
		if recorder.StatusCode() == code {
			statusOK = true
			break
		}
	}
	if !statusOK {
		return false
	}

	// Check content type exclusions
	contentType := strings.ToLower(recorder.Headers().Get("Content-Type"))
	for _, excludeType := range m.excludeTypes {
		if strings.Contains(contentType, strings.ToLower(excludeType)) {
			return false
		}
	}

	return true
}

// writeCachedResponse writes a cached response to the ResponseWriter
func (m *Middleware) writeCachedResponse(w http.ResponseWriter, r *http.Request, cached *CachedResponse) {
	// Set headers
	for k, v := range cached.Headers {
		w.Header()[k] = v
	}

	// Add cache hit header for debugging
	w.Header().Set("X-Cache-Status", "HIT")

	w.WriteHeader(cached.StatusCode)

	// For HEAD requests, don't write the body
	if r.Method != http.MethodHead {
		w.Write(cached.Body)
	}
}

// Stats returns cache statistics
func (m *Middleware) Stats() (itemCount int, hitCount, missCount uint64) {
	return m.cache.ItemCount(), atomic.LoadUint64(&m.hitCount), atomic.LoadUint64(&m.missCount)
}

// Clear removes all cached responses
func (m *Middleware) Clear() {
	m.cache.Flush()
}

// GetCacheForTesting returns the underlying cache for testing purposes
// This method should only be used in tests
func (m *Middleware) GetCacheForTesting() *cache.Cache {
	return m.cache
}

// Delete removes a specific cached response by URL
// It reconstructs the cache key using the same logic as requests
func (m *Middleware) Delete(url string) {
	// Create a minimal request to generate the cache key
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		// Invalid URL - nothing to delete
		return
	}

	// Generate the cache key using the same logic as requests
	key := m.createCacheKey(req)
	m.cache.Delete(key)
}

// isCacheableMethod checks if the HTTP method is cacheable
func (m *Middleware) isCacheableMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// tryServeFromCache attempts to serve a response from cache
func (m *Middleware) tryServeFromCache(w http.ResponseWriter, r *http.Request, key string) bool {
	cached, found := m.cache.Get(key)
	if !found {
		return false
	}

	cachedResponse, ok := cached.(*CachedResponse)
	if !ok {
		// Invalid cached data - remove it
		m.cache.Delete(key)
		return false
	}

	atomic.AddUint64(&m.hitCount, 1)
	m.writeCachedResponse(w, r, cachedResponse)
	return true
}

// handleCacheMiss processes a cache miss by recording the response and storing if appropriate
func (m *Middleware) handleCacheMiss(w http.ResponseWriter, r *http.Request, key string, next http.Handler) {
	atomic.AddUint64(&m.missCount, 1)

	recorder := NewResponseRecorder(w)
	next.ServeHTTP(recorder, r)

	m.storeResponseIfCacheable(key, recorder)
}

// storeResponseIfCacheable stores the response in cache if it meets caching criteria
func (m *Middleware) storeResponseIfCacheable(key string, recorder *ResponseRecorder) {
	if !m.shouldCache(recorder) {
		return
	}

	cachedResp := &CachedResponse{
		StatusCode: recorder.StatusCode(),
		Headers:    recorder.Headers(),
		Body:       recorder.Body(),
	}
	m.cache.Set(key, cachedResp, cache.DefaultExpiration)
}
