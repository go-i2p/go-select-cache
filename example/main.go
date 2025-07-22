package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	selectcache "github.com/go-i2p/go-select-cache"
)

// User represents a simple user data structure
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Data represents some API data
type Data struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

func main() {
	// Create cache middleware with default settings
	cache := selectcache.NewDefault()

	// API users endpoint - returns JSON (will be cached)
	http.Handle("/api/users", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		users := []User{
			{ID: 1, Name: "Alice", Age: 30},
			{ID: 2, Name: "Bob", Age: 25},
			{ID: 3, Name: "Charlie", Age: 35},
		}
		json.NewEncoder(w).Encode(users)
		fmt.Printf("Generated users data at %s\n", time.Now().Format("15:04:05"))
	}))

	// API data endpoint - returns JSON (will be cached)
	http.Handle("/api/data", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := Data{
			Message:   "Hello from cached API",
			Timestamp: time.Now(),
			Count:     42,
		}
		json.NewEncoder(w).Encode(data)
		fmt.Printf("Generated data response at %s\n", time.Now().Format("15:04:05"))
	}))

	// HTML endpoint - returns HTML (will NOT be cached due to content-type exclusion)
	http.Handle("/", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Cache Example</title>
</head>
<body>
    <h1>Go Select Cache Example</h1>
    <p>Current time: ` + time.Now().Format("15:04:05") + `</p>
    <ul>
        <li><a href="/api/users">API Users (cached JSON)</a></li>
        <li><a href="/api/data">API Data (cached JSON)</a></li>
        <li><a href="/cache/stats">Cache Statistics</a></li>
        <li><a href="/cache/clear">Clear Cache (POST)</a></li>
    </ul>
    <p>Note: This HTML page is NOT cached due to content-type exclusion.</p>
</body>
</html>`
		w.Write([]byte(html))
		fmt.Printf("Generated HTML page at %s (not cached)\n", time.Now().Format("15:04:05"))
	}))

	// Cache statistics endpoint
	http.Handle("/cache/stats", cache.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemCount, hitCount, missCount := cache.Stats()
		w.Header().Set("Content-Type", "application/json")
		stats := map[string]interface{}{
			"items":  itemCount,
			"hits":   hitCount,
			"misses": missCount,
			"ratio":  float64(hitCount) / float64(hitCount+missCount),
		}
		json.NewEncoder(w).Encode(stats)
		fmt.Printf("Cache stats: %d items, %d hits, %d misses\n", itemCount, hitCount, missCount)
	}))

	// Cache clear endpoint
	http.HandleFunc("/cache/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			cache.Clear()
			w.Write([]byte("Cache cleared successfully"))
			fmt.Println("Cache cleared")
		} else {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed. Use POST to clear cache.", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Starting HTTP server on :8080")
	fmt.Println("Try these URLs:")
	fmt.Println("  http://localhost:8080/           - HTML page (not cached)")
	fmt.Println("  http://localhost:8080/api/users  - JSON API (cached)")
	fmt.Println("  http://localhost:8080/api/data   - JSON API (cached)")
	fmt.Println("  http://localhost:8080/cache/stats - Cache statistics")
	fmt.Println("  curl -X POST http://localhost:8080/cache/clear - Clear cache")
	fmt.Println("")
	fmt.Println("The JSON endpoints will be cached for 15 minutes.")
	fmt.Println("Try requesting the same endpoint multiple times to see caching in action!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
