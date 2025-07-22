# Basic HTTP Middleware Example

This example demonstrates the basic usage of go-select-cache as HTTP middleware.

## Features

- ✅ JSON API endpoints (cached for 15 minutes)
- ❌ HTML pages (excluded from caching)
- 📊 Cache statistics endpoint
- 🗑️ Cache clearing functionality

## How to Run

```bash
cd example
go run main.go
```

The server will start on `http://localhost:8080`

## Test Endpoints

### Cached Endpoints (JSON)
```bash
# API users - will be cached
curl http://localhost:8080/api/users

# API data - will be cached  
curl http://localhost:8080/api/data

# Cache statistics
curl http://localhost:8080/cache/stats
```

### Not Cached (HTML)
```bash
# Main page - HTML content is excluded from caching
curl http://localhost:8080/
```

### Cache Management
```bash
# Clear cache
curl -X POST http://localhost:8080/cache/clear
```

## How It Works

1. **Middleware Setup**: Creates a cache middleware with default settings
2. **Request Filtering**: Only caches GET and HEAD requests
3. **Content-Type Filtering**: Excludes HTML content types from caching
4. **Cache Storage**: Uses in-memory cache with 15-minute TTL
5. **Cache Lookup**: Subsequent requests are served from cache when available

## Expected Behavior

- First request to `/api/users` or `/api/data` will generate fresh data
- Subsequent requests within 15 minutes will be served from cache (much faster)
- HTML pages are always generated fresh (not cached)
- Cache statistics show hits, misses, and cache size

## Testing Cache Behavior

1. Make a request to `/api/users` and note the timestamp
2. Make the same request again immediately - should return the same cached data
3. Check `/cache/stats` to see cache hits increase
4. Wait 15+ minutes and request again - new data will be generated
