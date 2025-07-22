package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	selectcache "github.com/go-i2p/go-select-cache"
)

func main() {
	// Create base TCP listener
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Create advanced cache configuration for transport layer
	config := selectcache.DefaultCacheConfig()
	config.MaxMemoryMB = 256                    // 256MB cache limit
	config.MaxEntries = 5000                    // Maximum 5000 cached entries
	config.DefaultTTL = 20 * time.Minute        // 20 minute default TTL
	config.CleanupInterval = 5 * time.Minute    // Clean up every 5 minutes
	config.EnableMetrics = true                 // Enable detailed metrics
	config.BufferSize = 8192                    // 8KB read buffer
	config.ConnectionTimeout = 30 * time.Second // 30 second connection timeout

	// Configure per-content-type TTLs
	config.ContentTypeTTLs = map[string]time.Duration{
		"application/json": 15 * time.Minute, // JSON responses cached for 15 minutes
		"text/css":         60 * time.Minute, // CSS files cached for 1 hour
		"text/javascript":  60 * time.Minute, // JS files cached for 1 hour
		"image/png":        24 * time.Hour,   // Images cached for 24 hours
		"image/jpeg":       24 * time.Hour,   // Images cached for 24 hours
	}

	// Exclude certain content types from caching
	config.ExcludedTypes = []string{
		"text/html",                // HTML pages not cached
		"application/xhtml+xml",    // XHTML not cached
		"text/event-stream",        // Server-sent events not cached
		"application/octet-stream", // Binary streams not cached
	}

	fmt.Println("🔧 Cache Configuration:")
	fmt.Printf("   Memory Limit: %dMB\n", config.MaxMemoryMB)
	fmt.Printf("   Max Entries: %d\n", config.MaxEntries)
	fmt.Printf("   Default TTL: %v\n", config.DefaultTTL)
	fmt.Printf("   Buffer Size: %d bytes\n", config.BufferSize)
	fmt.Printf("   Metrics: %v\n", config.EnableMetrics)

	// Wrap listener with caching capabilities
	cachingListener := selectcache.NewCachingListener(listener, config)
	defer cachingListener.Close()

	// Create HTTP handler for our test server
	handler := createHTTPHandler(cachingListener)

	// Create HTTP server
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Println("🚀 TCP Server with Transport-Layer Caching started on :8080")
	fmt.Println("")
	fmt.Println("🔗 This example demonstrates transport-layer caching that intercepts")
	fmt.Println("   and caches responses at the connection level, before they reach")
	fmt.Println("   the HTTP middleware layer.")
	fmt.Println("")
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  🌐 http://localhost:8080/                - Main page (HTML, not cached)")
	fmt.Println("  📊 http://localhost:8080/api/data        - JSON API (cached 15min)")
	fmt.Println("  🖼️  http://localhost:8080/static/logo.png - Image (cached 24h)")
	fmt.Println("  🎨 http://localhost:8080/static/style.css - CSS (cached 1h)")
	fmt.Println("  📜 http://localhost:8080/static/app.js    - JavaScript (cached 1h)")
	fmt.Println("  📈 http://localhost:8080/cache/metrics    - Transport cache metrics")
	fmt.Println("")
	fmt.Println("💡 Transport-layer caching features:")
	fmt.Println("   • Intercepts at TCP connection level")
	fmt.Println("   • Per-content-type TTL configuration")
	fmt.Println("   • Memory and entry limits")
	fmt.Println("   • Detailed performance metrics")
	fmt.Println("   • Automatic content detection")
	fmt.Println("")
	fmt.Println("🔄 Try making multiple requests to see transport-layer caching!")

	// Start serving with caching listener
	log.Fatal(server.Serve(cachingListener))
}

func createHTTPHandler(cachingListener *selectcache.CachingListener) http.Handler {
	mux := http.NewServeMux()

	// Main HTML page - not cached due to content type
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Transport-Layer Caching Example</title>
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/app.js"></script>
</head>
<body>
    <div class="container">
        <h1>🚀 Transport-Layer Caching Demo</h1>
        <p>Current time: <strong>%s</strong></p>
        <p>This page is generated fresh each time (HTML not cached)</p>
        
        <h2>Test Endpoints</h2>
        <ul>
            <li><a href="/api/data">📊 JSON API Data</a> (cached 15 minutes)</li>
            <li><a href="/static/logo.png">🖼️ Logo Image</a> (cached 24 hours)</li>
            <li><a href="/static/style.css">🎨 CSS Stylesheet</a> (cached 1 hour)</li>
            <li><a href="/static/app.js">📜 JavaScript</a> (cached 1 hour)</li>
            <li><a href="/cache/metrics">📈 Cache Metrics</a></li>
        </ul>
        
        <h2>How It Works</h2>
        <p>This server uses <strong>transport-layer caching</strong> that operates at the TCP connection level:</p>
        <ul>
            <li>🔍 Analyzes raw network traffic</li>
            <li>🏷️ Detects HTTP responses and content types</li>
            <li>💾 Caches responses before they reach application handlers</li>
            <li>⚡ Serves cached responses directly from the connection layer</li>
            <li>🎯 Uses different TTLs per content type</li>
        </ul>
        
        <div class="info">
            <p><strong>Try this:</strong> Open browser dev tools and make multiple requests 
            to the same endpoint. You'll see cached responses served much faster!</p>
        </div>
    </div>
</body>
</html>`, time.Now().Format("15:04:05"))
		w.Write([]byte(html))
		fmt.Printf("[%s] Generated main page (not cached)\n", time.Now().Format("15:04:05"))
	})

	// JSON API endpoint - will be cached for 15 minutes
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Generated-At", time.Now().Format(time.RFC3339))

		data := fmt.Sprintf(`{
  "message": "Data from transport-cached API",
  "timestamp": "%s",
  "server": "tcp-caching-example",
  "cached_for": "15 minutes",
  "random_number": %d,
  "transport_layer": true
}`, time.Now().Format(time.RFC3339), time.Now().Unix()%1000)

		w.Write([]byte(data))
		fmt.Printf("[%s] Generated JSON API response (cached 15min)\n", time.Now().Format("15:04:05"))
	})

	// CSS file - will be cached for 1 hour
	mux.HandleFunc("/static/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Header().Set("X-Generated-At", time.Now().Format(time.RFC3339))

		css := `/* Transport-Layer Cached CSS */
body { 
    font-family: Arial, sans-serif; 
    margin: 40px; 
    background-color: #f8f9fa;
}
.container { 
    max-width: 800px; 
    margin: 0 auto; 
    background: white;
    padding: 30px;
    border-radius: 8px;
    box-shadow: 0 2px 10px rgba(0,0,0,0.1);
}
h1 { 
    color: #2c3e50; 
    border-bottom: 3px solid #3498db;
    padding-bottom: 10px;
}
h2 { 
    color: #34495e; 
    margin-top: 30px;
}
.info { 
    background: #e8f5e8; 
    padding: 15px; 
    border-radius: 5px; 
    border-left: 4px solid #27ae60;
    margin-top: 20px;
}
ul { 
    line-height: 1.6; 
}
a { 
    color: #3498db; 
    text-decoration: none; 
}
a:hover { 
    text-decoration: underline; 
}
/* Generated at: ` + time.Now().Format("15:04:05") + ` */`

		w.Write([]byte(css))
		fmt.Printf("[%s] Generated CSS file (cached 1h)\n", time.Now().Format("15:04:05"))
	})

	// JavaScript file - will be cached for 1 hour
	mux.HandleFunc("/static/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("X-Generated-At", time.Now().Format(time.RFC3339))

		js := `// Transport-Layer Cached JavaScript
console.log('Transport-layer caching demo loaded at ` + time.Now().Format("15:04:05") + `');

// Demo function to test API caching
function testAPICaching() {
    const startTime = performance.now();
    
    fetch('/api/data')
        .then(response => response.json())
        .then(data => {
            const endTime = performance.now();
            const responseTime = Math.round(endTime - startTime);
            
            console.log('API Response:', data);
            console.log('Response time:', responseTime + 'ms');
            
            if (responseTime < 10) {
                console.log('🚀 Fast response - likely served from transport cache!');
            } else {
                console.log('⏱️ Slower response - likely generated fresh');
            }
        })
        .catch(error => console.error('Error:', error));
}

// Auto-test caching every 10 seconds
setInterval(testAPICaching, 10000);

// Test immediately
testAPICaching();

document.addEventListener('DOMContentLoaded', function() {
    console.log('🎯 Transport-layer caching demo ready!');
    console.log('📊 Check Network tab to see caching behavior');
});`

		w.Write([]byte(js))
		fmt.Printf("[%s] Generated JavaScript file (cached 1h)\n", time.Now().Format("15:04:05"))
	})

	// Placeholder image - will be cached for 24 hours
	mux.HandleFunc("/static/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Generated-At", time.Now().Format(time.RFC3339))

		// Simple 1x1 PNG (base64 decoded)
		pngData := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
			0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
			0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x62, 0x00, 0x02, 0x00, 0x00,
			0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
			0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
		}

		w.Write(pngData)
		fmt.Printf("[%s] Generated PNG image (cached 24h)\n", time.Now().Format("15:04:05"))
	})

	// Cache metrics endpoint - shows transport layer cache statistics
	mux.HandleFunc("/cache/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get metrics from the caching listener
		// Note: In a real implementation, you'd expose these metrics from the CachingListener
		metrics := map[string]interface{}{
			"transport_cache": map[string]interface{}{
				"description":        "Transport-layer cache operating at TCP connection level",
				"max_memory_mb":      256,
				"max_entries":        5000,
				"default_ttl":        "20 minutes",
				"cleanup_interval":   "5 minutes",
				"buffer_size":        8192,
				"connection_timeout": "30 seconds",
				"metrics_enabled":    true,
			},
			"content_type_ttls": map[string]string{
				"application/json": "15 minutes",
				"text/css":         "1 hour",
				"text/javascript":  "1 hour",
				"image/png":        "24 hours",
				"image/jpeg":       "24 hours",
			},
			"excluded_types": []string{
				"text/html",
				"application/xhtml+xml",
				"text/event-stream",
				"application/octet-stream",
			},
			"cache_behavior": map[string]interface{}{
				"intercepts_at":     "TCP connection level",
				"operates_before":   "HTTP middleware",
				"content_detection": "automatic",
				"memory_managed":    true,
			},
			"timestamp": time.Now(),
		}

		// In a real implementation, you'd get actual cache statistics here
		// For demo purposes, we'll show the configuration
		fmt.Printf("[%s] Generated cache metrics\n", time.Now().Format("15:04:05"))

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
		}
	})

	return mux
}
