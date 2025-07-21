package selectcache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a single cached response with metadata
type CacheEntry struct {
	// Response data
	Data    []byte      `json:"data"`
	Headers http.Header `json:"headers"`

	// Timing information
	ExpiresAt  time.Time `json:"expires_at"`
	AccessTime time.Time `json:"access_time"`
	StoreTime  time.Time `json:"store_time"`

	// Metadata
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// UpdateAccessTime updates the last access time for LRU tracking
func (e *CacheEntry) UpdateAccessTime() {
	e.AccessTime = time.Now()
}

// TTLCache provides thread-safe cache storage with TTL and LRU eviction
type TTLCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	config  *CacheConfig
	metrics *CacheMetrics

	// Memory tracking
	currentMemoryBytes uint64

	// Cleanup timer
	cleanupTimer *time.Timer
	stopCleanup  chan struct{}
	cleanupDone  sync.Once
}

// NewTTLCache creates a new TTL cache with the given configuration
func NewTTLCache(config *CacheConfig, metrics *CacheMetrics) *TTLCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &TTLCache{
		entries:     make(map[string]*CacheEntry),
		config:      config,
		metrics:     metrics,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup routine
	cache.startCleanupRoutine()

	return cache
}

// Get retrieves a cached entry by key
func (c *TTLCache) Get(key string) (*CacheEntry, bool) {
	start := time.Now()
	defer c.recordLookupMetrics(start)

	entry, exists := c.retrieveEntry(key)
	if !exists {
		c.recordCacheMiss()
		return nil, false
	}

	if entry.IsExpired() {
		c.removeExpiredEntry(key, entry)
		return nil, false
	}

	// Update access time for LRU
	entry.UpdateAccessTime()
	c.recordCacheHit()

	return entry, true
}

// recordLookupMetrics records the time taken for cache lookup operations.
func (c *TTLCache) recordLookupMetrics(start time.Time) {
	if c.metrics != nil {
		c.metrics.RecordLookupTime(time.Since(start))
	}
}

// retrieveEntry safely retrieves a cache entry by key using read lock.
func (c *TTLCache) retrieveEntry(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()
	return entry, exists
}

// recordCacheMiss records a cache miss event in metrics if available.
func (c *TTLCache) recordCacheMiss() {
	if c.metrics != nil {
		c.metrics.RecordMiss()
	}
}

// recordCacheHit records a cache hit event in metrics if available.
func (c *TTLCache) recordCacheHit() {
	if c.metrics != nil {
		c.metrics.RecordHit()
	}
}

// removeExpiredEntry removes an expired cache entry and updates memory tracking.
func (c *TTLCache) removeExpiredEntry(key string, entry *CacheEntry) {
	c.mu.Lock()
	delete(c.entries, key)
	c.currentMemoryBytes -= uint64(entry.Size)
	c.mu.Unlock()

	if c.metrics != nil {
		c.metrics.RecordMiss()
		c.metrics.UpdateMemoryUsage(c.currentMemoryBytes, len(c.entries))
	}
}

// createCacheEntry creates a new cache entry with copied data and headers.
func (c *TTLCache) createCacheEntry(data []byte, headers http.Header, ttl time.Duration) *CacheEntry {
	entry := &CacheEntry{
		Data:       make([]byte, len(data)),
		Headers:    make(http.Header),
		ExpiresAt:  time.Now().Add(ttl),
		AccessTime: time.Now(),
		StoreTime:  time.Now(),
		Size:       len(data) + c.calculateHeaderSize(headers),
	}

	// Copy data and headers
	copy(entry.Data, data)
	for k, v := range headers {
		entry.Headers[k] = make([]string, len(v))
		copy(entry.Headers[k], v)
	}

	// Extract content type
	entry.ContentType = headers.Get("Content-Type")
	return entry
}

// checkMemoryLimits verifies cache limits and evicts entries if necessary.
func (c *TTLCache) checkMemoryLimits(entrySize uint64) {
	newMemoryUsage := c.currentMemoryBytes + entrySize
	maxMemoryBytes := uint64(c.config.MaxMemoryMB) * 1024 * 1024

	if newMemoryUsage > maxMemoryBytes || len(c.entries) >= c.config.MaxEntries {
		// Need to evict entries
		evicted := c.evictLRU(newMemoryUsage - maxMemoryBytes + entrySize)
		if c.metrics != nil {
			for i := 0; i < evicted; i++ {
				c.metrics.RecordEviction()
			}
		}
	}
}

// removeExistingEntry removes any existing cache entry for the given key.
func (c *TTLCache) removeExistingEntry(key string) {
	if existingEntry, exists := c.entries[key]; exists {
		c.currentMemoryBytes -= uint64(existingEntry.Size)
	}
}

// storeCacheEntry stores the entry and updates metrics.
func (c *TTLCache) storeCacheEntry(key string, entry *CacheEntry) {
	c.entries[key] = entry
	c.currentMemoryBytes += uint64(entry.Size)

	if c.metrics != nil {
		c.metrics.RecordStore()
		c.metrics.UpdateMemoryUsage(c.currentMemoryBytes, len(c.entries))
	}
}

// Set stores a cache entry with the specified TTL
func (c *TTLCache) Set(key string, data []byte, headers http.Header, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		if c.metrics != nil {
			c.metrics.RecordStoreTime(time.Since(start))
		}
	}()

	entry := c.createCacheEntry(data, headers, ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkMemoryLimits(uint64(entry.Size))
	c.removeExistingEntry(key)
	c.storeCacheEntry(key, entry)

	return nil
}

// Delete removes a cache entry by key
func (c *TTLCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		delete(c.entries, key)
		c.currentMemoryBytes -= uint64(entry.Size)

		if c.metrics != nil {
			c.metrics.RecordDeletion()
			c.metrics.UpdateMemoryUsage(c.currentMemoryBytes, len(c.entries))
		}
		return true
	}

	return false
}

// Clear removes all cache entries
func (c *TTLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryCount := len(c.entries)
	c.entries = make(map[string]*CacheEntry)
	c.currentMemoryBytes = 0

	if c.metrics != nil {
		for i := 0; i < entryCount; i++ {
			c.metrics.RecordDeletion()
		}
		c.metrics.UpdateMemoryUsage(0, 0)
	}
}

// Size returns the current number of entries in the cache
func (c *TTLCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// MemoryUsage returns the current memory usage in bytes
func (c *TTLCache) MemoryUsage() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentMemoryBytes
}

// Close stops the cleanup routine and releases resources
func (c *TTLCache) Close() {
	c.cleanupDone.Do(func() {
		close(c.stopCleanup)
		if c.cleanupTimer != nil {
			c.cleanupTimer.Stop()
		}
	})
}

// evictLRU removes least recently used entries to free up the specified amount of memory
// Must be called with write lock held
func (c *TTLCache) evictLRU(bytesToFree uint64) int {
	if len(c.entries) == 0 {
		return 0
	}

	// Create a slice of entries sorted by access time (oldest first)
	type entryWithKey struct {
		key   string
		entry *CacheEntry
	}

	entries := make([]entryWithKey, 0, len(c.entries))
	for key, entry := range c.entries {
		entries = append(entries, entryWithKey{key: key, entry: entry})
	}

	// Sort by access time (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].entry.AccessTime.Before(entries[j].entry.AccessTime)
	})

	var freedBytes uint64
	evicted := 0

	for _, e := range entries {
		delete(c.entries, e.key)
		freedBytes += uint64(e.entry.Size)
		evicted++

		if freedBytes >= bytesToFree {
			break
		}
	}

	c.currentMemoryBytes -= freedBytes
	return evicted
}

// startCleanupRoutine starts the background cleanup routine
func (c *TTLCache) startCleanupRoutine() {
	c.cleanupTimer = time.NewTimer(c.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-c.cleanupTimer.C:
				c.cleanupExpired()
				c.cleanupTimer.Reset(c.config.CleanupInterval)
			case <-c.stopCleanup:
				return
			}
		}
	}()
}

// cleanupExpired removes all expired entries
func (c *TTLCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var freedBytes uint64
	deleted := 0

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			freedBytes += uint64(entry.Size)
			deleted++
		}
	}

	c.currentMemoryBytes -= freedBytes

	if c.metrics != nil && deleted > 0 {
		for i := 0; i < deleted; i++ {
			c.metrics.RecordDeletion()
		}
		c.metrics.UpdateMemoryUsage(c.currentMemoryBytes, len(c.entries))
	}
}

// calculateHeaderSize estimates the memory size of HTTP headers
func (c *TTLCache) calculateHeaderSize(headers http.Header) int {
	size := 0
	for k, v := range headers {
		size += len(k)
		for _, val := range v {
			size += len(val)
		}
	}
	return size
}

// GenerateCacheKey creates a consistent cache key from request characteristics
func GenerateCacheKey(method, path, query string, headers map[string]string) string {
	var keyParts []string

	// Add request method
	keyParts = append(keyParts, method)

	// Add request path
	keyParts = append(keyParts, path)

	// Add sorted query parameters
	if query != "" {
		keyParts = append(keyParts, "query="+query)
	}

	// Add sorted headers that affect caching
	if len(headers) > 0 {
		var headerKeys []string
		for k := range headers {
			headerKeys = append(headerKeys, k)
		}
		sort.Strings(headerKeys)

		for _, k := range headerKeys {
			keyParts = append(keyParts, k+"="+headers[k])
		}
	}

	// Create hash of the key components
	keyString := strings.Join(keyParts, "|")
	hash := sha256.Sum256([]byte(keyString))
	return hex.EncodeToString(hash[:])[:16] // 16 chars for cache key
}
