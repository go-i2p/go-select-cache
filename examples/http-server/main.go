package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	selectcache "github.com/go-i2p/go-select-cache"
)

// Product represents a product in our store
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

// Order represents a customer order
type Order struct {
	ID       int       `json:"id"`
	Customer string    `json:"customer"`
	Products []Product `json:"products"`
	Total    float64   `json:"total"`
	Date     time.Time `json:"date"`
}

func main() {
	// Create advanced cache configuration
	config := selectcache.Config{
		DefaultTTL:      30 * time.Minute, // Longer TTL for this example
		CleanupInterval: 10 * time.Minute,
		ExcludeContentTypes: []string{
			"text/html",
			"application/xhtml+xml",
			"text/plain", // Also exclude plain text
		},
		IncludeStatusCodes: []int{200, 201, 202}, // Cache more status codes
	}

	// Create cache middleware with custom config
	cache := selectcache.New(config)

	// Products API - will be cached
	http.Handle("/api/products", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=1800") // Browser caching hint

		products := []Product{
			{ID: 1, Name: "Laptop", Price: 999.99, Category: "Electronics", Description: "High-performance laptop"},
			{ID: 2, Name: "Smartphone", Price: 699.99, Category: "Electronics", Description: "Latest smartphone"},
			{ID: 3, Name: "Book", Price: 29.99, Category: "Education", Description: "Programming guide"},
			{ID: 4, Name: "Headphones", Price: 199.99, Category: "Electronics", Description: "Noise-canceling headphones"},
		}

		json.NewEncoder(w).Encode(products)
		fmt.Printf("[%s] Generated products list\n", time.Now().Format("15:04:05"))
	}))

	// Single product API - will be cached
	http.Handle("/api/products/", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Simple product lookup (in real app, you'd parse the ID from URL)
		product := Product{
			ID:          1,
			Name:        "Laptop",
			Price:       999.99,
			Category:    "Electronics",
			Description: "High-performance laptop with SSD and 16GB RAM",
		}

		json.NewEncoder(w).Encode(product)
		fmt.Printf("[%s] Generated single product data\n", time.Now().Format("15:04:05"))
	}))

	// Orders API - will be cached
	http.Handle("/api/orders", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		orders := []Order{
			{
				ID:       1001,
				Customer: "john@example.com",
				Products: []Product{
					{ID: 1, Name: "Laptop", Price: 999.99, Category: "Electronics"},
				},
				Total: 999.99,
				Date:  time.Now().Add(-24 * time.Hour),
			},
			{
				ID:       1002,
				Customer: "jane@example.com",
				Products: []Product{
					{ID: 2, Name: "Smartphone", Price: 699.99, Category: "Electronics"},
					{ID: 4, Name: "Headphones", Price: 199.99, Category: "Electronics"},
				},
				Total: 899.98,
				Date:  time.Now().Add(-12 * time.Hour),
			},
		}

		json.NewEncoder(w).Encode(orders)
		fmt.Printf("[%s] Generated orders list\n", time.Now().Format("15:04:05"))
	}))

	// Analytics API - will be cached
	http.Handle("/api/analytics", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		analytics := map[string]interface{}{
			"total_orders":    156,
			"revenue":         45230.50,
			"avg_order_value": 289.94,
			"top_category":    "Electronics",
			"generated_at":    time.Now(),
			"cache_duration":  "30 minutes",
		}

		json.NewEncoder(w).Encode(analytics)
		fmt.Printf("[%s] Generated analytics data (expensive calculation)\n", time.Now().Format("15:04:05"))
	}))

	// Health check - will be cached
	http.Handle("/api/health", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"version":   "1.0.0",
			"uptime":    "5 days",
		}

		json.NewEncoder(w).Encode(health)
		fmt.Printf("[%s] Generated health check\n", time.Now().Format("15:04:05"))
	}))

	// Admin interface - NOT cached (HTML content type)
	http.Handle("/admin", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		itemCount, hitCount, missCount := cache.Stats()
		hitRatio := float64(hitCount) / float64(hitCount+missCount) * 100

		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Advanced Cache Example - Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .stats { background: #f0f0f0; padding: 20px; border-radius: 5px; }
        .endpoint { margin: 10px 0; }
        .cached { color: green; }
        .not-cached { color: red; }
    </style>
</head>
<body>
    <h1>Advanced HTTP Server Cache Example</h1>
    
    <div class="stats">
        <h2>Cache Statistics</h2>
        <p><strong>Cache Items:</strong> %d</p>
        <p><strong>Cache Hits:</strong> %d</p>
        <p><strong>Cache Misses:</strong> %d</p>
        <p><strong>Hit Ratio:</strong> %.2f%%</p>
        <p><strong>Configuration:</strong> 30min TTL, 10min cleanup</p>
    </div>

    <h2>API Endpoints</h2>
    <div class="endpoint cached">✓ <a href="/api/products">GET /api/products</a> - Products list (cached)</div>
    <div class="endpoint cached">✓ <a href="/api/products/1">GET /api/products/1</a> - Single product (cached)</div>
    <div class="endpoint cached">✓ <a href="/api/orders">GET /api/orders</a> - Orders list (cached)</div>
    <div class="endpoint cached">✓ <a href="/api/analytics">GET /api/analytics</a> - Analytics data (cached)</div>
    <div class="endpoint cached">✓ <a href="/api/health">GET /api/health</a> - Health check (cached)</div>
    <div class="endpoint not-cached">✗ <a href="/admin">GET /admin</a> - This page (NOT cached - HTML)</div>
    
    <h2>Cache Management</h2>
    <div class="endpoint">📊 <a href="/cache/stats">GET /cache/stats</a> - Detailed cache statistics</div>
    <div class="endpoint">🗑️ <strong>POST /cache/clear</strong> - Clear all cache entries</div>
    
    <p><em>Current time: %s</em></p>
    <p><em>This admin page is generated fresh each time (not cached due to HTML content type)</em></p>
</body>
</html>`, itemCount, hitCount, missCount, hitRatio, time.Now().Format("15:04:05"))

		w.Write([]byte(html))
		fmt.Printf("[%s] Generated admin page (not cached)\n", time.Now().Format("15:04:05"))
	}))

	// Enhanced cache statistics endpoint
	http.Handle("/cache/stats", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		itemCount, hitCount, missCount := cache.Stats()
		hitRatio := float64(0)
		if hitCount+missCount > 0 {
			hitRatio = float64(hitCount) / float64(hitCount+missCount) * 100
		}

		stats := map[string]interface{}{
			"cache_items":   itemCount,
			"cache_hits":    hitCount,
			"cache_misses":  missCount,
			"hit_ratio_pct": hitRatio,
			"configuration": map[string]interface{}{
				"ttl":              "30 minutes",
				"cleanup_interval": "10 minutes",
				"excluded_types":   []string{"text/html", "application/xhtml+xml", "text/plain"},
				"included_status":  []int{200, 201, 202},
			},
			"timestamp": time.Now(),
		}

		json.NewEncoder(w).Encode(stats)
		fmt.Printf("[%s] Cache stats: %d items, %d hits, %d misses (%.2f%% hit ratio)\n",
			time.Now().Format("15:04:05"), itemCount, hitCount, missCount, hitRatio)
	}))

	// Enhanced cache clear endpoint
	http.HandleFunc("/cache/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			itemCount, _, _ := cache.Stats()
			cache.Clear()

			response := map[string]interface{}{
				"message":       "Cache cleared successfully",
				"items_cleared": itemCount,
				"timestamp":     time.Now(),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			fmt.Printf("[%s] Cache cleared - removed %d items\n", time.Now().Format("15:04:05"), itemCount)
		} else {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed. Use POST to clear cache.", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("🚀 Advanced HTTP Server with Caching started on :8080")
	fmt.Println("")
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  🌐 http://localhost:8080/admin           - Admin dashboard (HTML, not cached)")
	fmt.Println("  📦 http://localhost:8080/api/products     - Products API (JSON, cached 30min)")
	fmt.Println("  📦 http://localhost:8080/api/products/1   - Single product (JSON, cached 30min)")
	fmt.Println("  📋 http://localhost:8080/api/orders       - Orders API (JSON, cached 30min)")
	fmt.Println("  📊 http://localhost:8080/api/analytics    - Analytics API (JSON, cached 30min)")
	fmt.Println("  ❤️  http://localhost:8080/api/health       - Health check (JSON, cached 30min)")
	fmt.Println("  📈 http://localhost:8080/cache/stats      - Cache statistics (JSON)")
	fmt.Println("  🗑️  curl -X POST http://localhost:8080/cache/clear - Clear cache")
	fmt.Println("")
	fmt.Println("💡 This example demonstrates:")
	fmt.Println("   • Custom cache configuration (30min TTL)")
	fmt.Println("   • Multiple content types and status codes")
	fmt.Println("   • Real-world API endpoints")
	fmt.Println("   • Cache statistics and management")
	fmt.Println("   • HTML exclusion from caching")
	fmt.Println("")
	fmt.Println("🔄 Try making multiple requests to the same endpoint to see caching in action!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
