# Advanced HTTP Server Example

This example demonstrates advanced usage of go-select-cache with custom configuration, multiple endpoints, and enhanced features.

## Features

- 🔧 Custom cache configuration (30-minute TTL)
- 📦 Multiple JSON API endpoints
- 🌐 Admin dashboard (HTML, not cached)
- 📊 Enhanced cache statistics
- 🗑️ Advanced cache management
- ✅ Multiple status codes cached (200, 201, 202)

## How to Run

```bash
cd examples/http-server
go run main.go
```

The server will start on `http://localhost:8080`

## Available Endpoints

### Admin Interface
- 🌐 `http://localhost:8080/admin` - Admin dashboard with cache statistics (HTML, not cached)

### API Endpoints (All Cached for 30 minutes)
- 📦 `http://localhost:8080/api/products` - Products list
- 📦 `http://localhost:8080/api/products/1` - Single product  
- 📋 `http://localhost:8080/api/orders` - Orders list
- 📊 `http://localhost:8080/api/analytics` - Analytics data (expensive calculation)
- ❤️ `http://localhost:8080/api/health` - Health check

### Cache Management
- 📈 `http://localhost:8080/cache/stats` - Detailed cache statistics (JSON)
- 🗑️ `curl -X POST http://localhost:8080/cache/clear` - Clear all cache entries

## Custom Configuration

This example uses a custom cache configuration:

```go
config := selectcache.Config{
    DefaultTTL:      30 * time.Minute, // Longer TTL
    CleanupInterval: 10 * time.Minute,
    ExcludeContentTypes: []string{
        "text/html",
        "application/xhtml+xml", 
        "text/plain",           // Also exclude plain text
    },
    IncludeStatusCodes: []int{200, 201, 202}, // Cache more status codes
}
```

## Testing the Example

### 1. Basic Cache Testing
```bash
# Test products API caching
curl http://localhost:8080/api/products
curl http://localhost:8080/api/products  # Should be served from cache

# Check cache statistics
curl http://localhost:8080/cache/stats
```

### 2. Performance Testing
```bash
# Test analytics endpoint (simulates expensive calculation)
time curl http://localhost:8080/api/analytics  # First request (slow)
time curl http://localhost:8080/api/analytics  # Cached request (fast)
```

### 3. Admin Dashboard
Visit `http://localhost:8080/admin` in your browser to see:
- Real-time cache statistics
- Hit ratio percentage
- Configuration details
- Links to all available endpoints

### 4. Cache Management
```bash
# Get detailed statistics
curl http://localhost:8080/cache/stats | jq

# Clear cache and verify
curl -X POST http://localhost:8080/cache/clear
curl http://localhost:8080/cache/stats  # Should show 0 items
```

## Expected Behavior

- **First requests**: Generate fresh data, show in console output
- **Cached requests**: Served from cache (no console output), much faster response
- **HTML pages**: Always generated fresh (admin dashboard)
- **Cache statistics**: Show hit ratio, item count, configuration
- **30-minute TTL**: Cached responses expire after 30 minutes

## Key Features Demonstrated

1. **Custom Configuration**: Longer TTL, custom content type exclusions
2. **Multiple Status Codes**: Caches 200, 201, and 202 responses
3. **Real-world APIs**: Products, orders, analytics endpoints
4. **Performance Monitoring**: Detailed cache statistics and metrics
5. **Browser Integration**: HTML admin interface for easy monitoring
6. **Cache Management**: Clear cache and view real-time statistics
