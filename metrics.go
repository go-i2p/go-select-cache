package selectcache

import (
	"sync"
	"time"
)

// CacheMetrics collects performance metrics for the caching system
type CacheMetrics struct {
	mu sync.RWMutex

	// Cache operation counters
	hits      uint64
	misses    uint64
	stores    uint64
	evictions uint64
	deletions uint64

	// Memory usage tracking
	totalMemoryBytes uint64
	entryCount       int

	// Performance timing
	totalLookupTime time.Duration
	totalStoreTime  time.Duration
	lookupCount     uint64
	storeCount      uint64

	// Error tracking
	errors map[string]uint64

	enabled bool
}

// NewCacheMetrics creates a new metrics collector
func NewCacheMetrics(enabled bool) *CacheMetrics {
	return &CacheMetrics{
		errors:  make(map[string]uint64),
		enabled: enabled,
	}
}

// RecordHit increments the cache hit counter
func (m *CacheMetrics) RecordHit() {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.hits++
	m.mu.Unlock()
}

// RecordMiss increments the cache miss counter
func (m *CacheMetrics) RecordMiss() {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.misses++
	m.mu.Unlock()
}

// RecordStore increments the cache store counter
func (m *CacheMetrics) RecordStore() {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.stores++
	m.mu.Unlock()
}

// RecordEviction increments the eviction counter
func (m *CacheMetrics) RecordEviction() {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.evictions++
	m.mu.Unlock()
}

// RecordDeletion increments the deletion counter
func (m *CacheMetrics) RecordDeletion() {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.deletions++
	m.mu.Unlock()
}

// RecordLookupTime adds to the total lookup time for average calculation
func (m *CacheMetrics) RecordLookupTime(duration time.Duration) {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.totalLookupTime += duration
	m.lookupCount++
	m.mu.Unlock()
}

// RecordStoreTime adds to the total store time for average calculation
func (m *CacheMetrics) RecordStoreTime(duration time.Duration) {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.totalStoreTime += duration
	m.storeCount++
	m.mu.Unlock()
}

// UpdateMemoryUsage sets the current memory usage
func (m *CacheMetrics) UpdateMemoryUsage(bytes uint64, entryCount int) {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.totalMemoryBytes = bytes
	m.entryCount = entryCount
	m.mu.Unlock()
}

// RecordError increments the error counter for a specific error type
func (m *CacheMetrics) RecordError(errorType string) {
	if !m.enabled {
		return
	}
	m.mu.Lock()
	m.errors[errorType]++
	m.mu.Unlock()
}

// CacheStats represents a snapshot of cache metrics
type CacheStats struct {
	// Operation counts
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	Stores    uint64 `json:"stores"`
	Evictions uint64 `json:"evictions"`
	Deletions uint64 `json:"deletions"`

	// Calculated metrics
	HitRatio        float64 `json:"hit_ratio"`
	AvgLookupTimeMs float64 `json:"avg_lookup_time_ms"`
	AvgStoreTimeMs  float64 `json:"avg_store_time_ms"`

	// Memory usage
	TotalMemoryBytes uint64 `json:"total_memory_bytes"`
	EntryCount       int    `json:"entry_count"`
	AvgEntrySize     uint64 `json:"avg_entry_size"`

	// Error counts
	Errors map[string]uint64 `json:"errors"`
}

// GetStats returns a snapshot of current metrics
func (m *CacheMetrics) GetStats() CacheStats {
	if !m.enabled {
		return CacheStats{
			Errors: make(map[string]uint64),
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := CacheStats{
		Hits:             m.hits,
		Misses:           m.misses,
		Stores:           m.stores,
		Evictions:        m.evictions,
		Deletions:        m.deletions,
		TotalMemoryBytes: m.totalMemoryBytes,
		EntryCount:       m.entryCount,
		Errors:           make(map[string]uint64),
	}

	// Calculate hit ratio
	totalRequests := m.hits + m.misses
	if totalRequests > 0 {
		stats.HitRatio = float64(m.hits) / float64(totalRequests)
	}

	// Calculate average lookup time
	if m.lookupCount > 0 {
		stats.AvgLookupTimeMs = float64(m.totalLookupTime.Nanoseconds()) / float64(m.lookupCount) / 1e6
	}

	// Calculate average store time
	if m.storeCount > 0 {
		stats.AvgStoreTimeMs = float64(m.totalStoreTime.Nanoseconds()) / float64(m.storeCount) / 1e6
	}

	// Calculate average entry size
	if m.entryCount > 0 {
		stats.AvgEntrySize = m.totalMemoryBytes / uint64(m.entryCount)
	}

	// Copy error map
	for k, v := range m.errors {
		stats.Errors[k] = v
	}

	return stats
}

// Reset clears all metrics
func (m *CacheMetrics) Reset() {
	if !m.enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.hits = 0
	m.misses = 0
	m.stores = 0
	m.evictions = 0
	m.deletions = 0
	m.totalMemoryBytes = 0
	m.entryCount = 0
	m.totalLookupTime = 0
	m.totalStoreTime = 0
	m.lookupCount = 0
	m.storeCount = 0
	m.errors = make(map[string]uint64)
}

// IsEnabled returns whether metrics collection is enabled
func (m *CacheMetrics) IsEnabled() bool {
	return m.enabled
}
