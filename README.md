# go-select-cache

HTTP response caching middleware for Go that selectively caches responses based on content type, excluding HTML to keep dynamic content fresh.

## Features

- HTTP middleware for response caching
- Content-type based filtering (excludes HTML by default) 
- Configurable TTL for cached responses
- Cache statistics (hit/miss counts)
- Support for GET and HEAD requests only

## Installation

```bash
go get github.com/go-i2p/go-select-cache
```

## Dependencies

- `github.com/patrickmn/go-cache` - Used for simplified HTTP middleware caching
- Custom TTL cache implementation with LRU eviction for advanced transport-layer caching

## Quick Start

### HTTP Server with Middleware Caching

```go
package main

import (
    "encoding/json"
    "net/http"
    "time"
    
    "github.com/go-i2p/go-select-cache"
)

func main() {
    // Create cache middleware
    cache := selectcache.NewDefault()
    // Or with custom config:
    // config := selectcache.Config{
    //     DefaultTTL:           15 * time.Minute,
    //     CleanupInterval:      5 * time.Minute,
    //     ExcludeContentTypes:  []string{"text/html", "application/xhtml+xml"},
    //     IncludeStatusCodes:   []int{200},
    // }
    // cache := selectcache.New(config)

    mux := http.NewServeMux()
    mux.HandleFunc("/api/data", apiHandler)    // Will be cached
    mux.HandleFunc("/", htmlHandler)           // Will NOT be cached (HTML)
    
    // Wrap with cache middleware
    server := &http.Server{
        Addr:    ":8080",
        Handler: cache.Handler(mux),
    }
    server.ListenAndServe()
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"message": "This will be cached"}`))
}

func htmlHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte("<h1>This HTML won't be cached</h1>"))
}
```

## Configuration

```go
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
```

### Default Configuration

```go
config := selectcache.DefaultConfig()
// Returns:
// DefaultTTL: 15 minutes
// CleanupInterval: 5 minutes  
// ExcludeContentTypes: ["text/html", "application/xhtml+xml"]
// IncludeStatusCodes: [200]
```

## What Gets Cached

- Only GET and HEAD requests are cached
- Only responses with 200 status code (configurable)
- All content types EXCEPT those in the exclusion list

### Default Behavior
- ✅ **CACHED**: `application/json`, `image/*`, `text/css`, `application/javascript`, etc.
- ❌ **NOT CACHED**: `text/html`, `application/xhtml+xml` (to keep HTML pages dynamic)

## API Reference

### Core Functions

```go
// Create middleware with default settings
func NewDefault() *Middleware

// Create middleware with custom configuration  
func New(config Config) *Middleware

// Get default configuration
func DefaultConfig() Config
```

### Middleware Methods

```go
// Wrap an HTTP handler with caching
func (m *Middleware) Handler(next http.Handler) http.Handler

// Wrap an HTTP handler function with caching
func (m *Middleware) HandlerFunc(next http.HandlerFunc) http.Handler

// Get cache statistics (itemCount, hitCount, missCount)
func (m *Middleware) Stats() (int, uint64, uint64)

// Clear all cached responses
func (m *Middleware) Clear()

// Delete specific cached response by URL
func (m *Middleware) Delete(url string)
```

### Usage Examples

```go
// Get cache statistics
itemCount, hitCount, missCount := cache.Stats()
fmt.Printf("Cache: %d items, %d hits, %d misses\n", itemCount, hitCount, missCount)

// Add cache stats endpoint
http.HandleFunc("/cache/stats", func(w http.ResponseWriter, r *http.Request) {
    itemCount, hitCount, missCount := cache.Stats()
    fmt.Fprintf(w, "Items: %d, Hits: %d, Misses: %d", itemCount, hitCount, missCount)
})

// Clear cache endpoint  
http.HandleFunc("/cache/clear", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        cache.Clear()
        w.Write([]byte("Cache cleared"))
    }
})
```

## Advanced Transport-Layer Caching

For advanced use cases, the library also provides transport-layer caching that can intercept and cache responses at the connection level:

### CachingListener

```go
package main

import (
    "net"
    "net/http"
    
    "github.com/go-i2p/go-select-cache"
)

func main() {
    // Create base listener
    listener, err := net.Listen("tcp", ":8080")
    if err != nil {
        panic(err)
    }
    
    // Create advanced cache config
    config := selectcache.DefaultCacheConfig()
    config.MaxMemoryMB = 256
    config.MaxEntries = 5000
    
    // Wrap with caching listener
    cachingListener := selectcache.NewCachingListener(listener, config)
    defer cachingListener.Close()
    
    // Use with HTTP server
    server := &http.Server{Handler: yourHandler}
    server.Serve(cachingListener)
}
```

### Advanced Configuration

```go
type CacheConfig struct {
    // DefaultTTL is the default time-to-live for cached responses
    DefaultTTL time.Duration
    
    // ContentTypeTTLs provides per-content-type TTL overrides
    ContentTypeTTLs map[string]time.Duration
    
    // MaxMemoryMB is the maximum memory in megabytes for cache storage
    MaxMemoryMB int64
    
    // MaxEntries is the maximum number of cache entries
    MaxEntries int
    
    // ExcludedTypes are content types that should never be cached
    ExcludedTypes []string
    
    // EnableMetrics determines if performance metrics are collected
    EnableMetrics bool
    
    // CleanupInterval is how often expired entries are removed
    CleanupInterval time.Duration
    
    // BufferSize is the size of the read buffer for connection analysis
    BufferSize int
    
    // ConnectionTimeout is the maximum time to wait for connection analysis
    ConnectionTimeout time.Duration
}
```

## Examples

Complete working examples are available in the `example/` and `examples/` directories:

```bash
# Run the basic HTTP middleware example
cd example
go run main.go

# Test with curl
curl http://localhost:8080/api/users  # Will be cached
curl http://localhost:8080/api/data   # Will be cached  
curl http://localhost:8080/           # HTML - not cached
curl http://localhost:8080/cache/stats # Check cache statistics

# Run advanced examples
cd examples/http-server
go run main.go

cd examples/tcp-server  
go run main.go
```

## How It Works

1. **Request Filtering**: Only caches GET and HEAD requests
2. **Response Capture**: Uses `ResponseRecorder` to capture response data  
3. **Content-Type Check**: Excludes configured content types (HTML by default)
4. **Status Code Check**: Only caches configured status codes (200 by default)
5. **Cache Storage**: Stores responses in memory with TTL using patrickmn/go-cache
6. **Cache Lookup**: Subsequent requests check cache first using SHA256-based keys

## License

MIT License