# TCP Server with Transport-Layer Caching Example

This example demonstrates the advanced transport-layer caching capabilities of go-select-cache that operate at the TCP connection level.

## Features

- 🚀 **Transport-layer caching** - Operates at TCP connection level
- 🎯 **Per-content-type TTLs** - Different cache durations for different content types
- 📊 **Advanced configuration** - Memory limits, entry limits, buffer sizes
- 🔍 **Automatic content detection** - Analyzes raw network traffic
- 📈 **Performance metrics** - Detailed caching statistics
- 🌐 **Multi-content support** - HTML, JSON, CSS, JavaScript, Images

## How to Run

```bash
cd examples/tcp-server  
go run main.go
```

The server will start on `http://localhost:8080`

## Transport-Layer Cache Configuration

```go
config := selectcache.DefaultCacheConfig()
config.MaxMemoryMB = 256                    // 256MB cache limit
config.MaxEntries = 5000                    // Maximum 5000 cached entries
config.DefaultTTL = 20 * time.Minute        // 20 minute default TTL
config.EnableMetrics = true                 // Enable detailed metrics
config.BufferSize = 8192                    // 8KB read buffer

// Per-content-type TTLs
config.ContentTypeTTLs = map[string]time.Duration{
    "application/json": 15 * time.Minute, // JSON responses
    "text/css":         60 * time.Minute, // CSS files  
    "text/javascript":  60 * time.Minute, // JS files
    "image/png":        24 * time.Hour,   // Images
    "image/jpeg":       24 * time.Hour,   // Images
}
```

## Available Endpoints

### Main Application
- 🌐 `http://localhost:8080/` - Main HTML page (not cached)

### Content with Different Cache TTLs
- 📊 `http://localhost:8080/api/data` - JSON API (cached 15 minutes)
- 🖼️ `http://localhost:8080/static/logo.png` - PNG image (cached 24 hours)
- 🎨 `http://localhost:8080/static/style.css` - CSS stylesheet (cached 1 hour)
- 📜 `http://localhost:8080/static/app.js` - JavaScript (cached 1 hour)

### Cache Information
- 📈 `http://localhost:8080/cache/metrics` - Transport cache metrics and configuration

## How Transport-Layer Caching Works

1. **TCP Connection Interception**: The caching listener wraps the base TCP listener
2. **Traffic Analysis**: Analyzes raw network traffic to detect HTTP responses
3. **Content Detection**: Automatically identifies content types from HTTP headers
4. **Response Caching**: Caches responses at the connection level, before application handlers
5. **Direct Serving**: Serves cached responses directly from the transport layer

## Testing the Example

### 1. Basic Functionality Test
```bash
# Test different content types
curl http://localhost:8080/                    # HTML (not cached)
curl http://localhost:8080/api/data           # JSON (cached 15min)
curl http://localhost:8080/static/style.css   # CSS (cached 1h)
curl http://localhost:8080/static/app.js      # JS (cached 1h)
curl http://localhost:8080/static/logo.png    # PNG (cached 24h)
```

### 2. Cache Performance Test
```bash
# Test JSON API caching performance
echo "First request (should be slower):"
time curl -s http://localhost:8080/api/data > /dev/null

echo "Second request (should be faster from cache):"  
time curl -s http://localhost:8080/api/data > /dev/null
```

### 3. Browser Testing
1. Open `http://localhost:8080/` in your browser
2. Open Developer Tools → Network tab
3. Refresh the page multiple times
4. Notice that static assets (CSS, JS, images) are served from cache

### 4. Cache Metrics
```bash
# View transport cache configuration and metrics
curl http://localhost:8080/cache/metrics | jq
```

## Expected Behavior

### Caching Behavior by Content Type:
- **HTML pages** (`text/html`): ❌ Not cached (excluded)
- **JSON APIs** (`application/json`): ✅ Cached for 15 minutes
- **CSS files** (`text/css`): ✅ Cached for 1 hour
- **JavaScript** (`text/javascript`): ✅ Cached for 1 hour  
- **Images** (`image/png`, `image/jpeg`): ✅ Cached for 24 hours

### Performance Characteristics:
- **Cache Miss**: Normal response time, data generated fresh
- **Cache Hit**: Extremely fast response, served directly from transport layer
- **Memory Management**: Automatic cleanup based on TTL and memory limits

## Key Features Demonstrated

1. **Transport-Layer Operation**: Caching happens before HTTP middleware
2. **Content-Type Awareness**: Different TTLs for different content types
3. **Memory Management**: Configurable memory and entry limits
4. **Automatic Detection**: No manual content type configuration needed
5. **Performance Optimization**: Static assets cached for long periods
6. **Flexible Configuration**: Per-content-type TTL customization

## Comparison with Middleware Caching

| Feature | Middleware Caching | Transport-Layer Caching |
|---------|-------------------|------------------------|
| **Operation Level** | HTTP middleware | TCP connection |
| **Performance** | Fast | Extremely fast |
| **Setup Complexity** | Simple | Advanced |
| **Content Detection** | Manual | Automatic |
| **Memory Management** | Basic | Advanced |
| **Use Cases** | API responses | Static assets + APIs |

This transport-layer approach is ideal for applications serving mixed content (APIs, static assets) where maximum performance is required.
