package selectcache

import (
	"net"
	"sync"
)

// CachingListener wraps a net.Listener to provide transparent caching of responses
type CachingListener struct {
	wrapped  net.Listener
	cache    *TTLCache
	config   *CacheConfig
	metrics  *CacheMetrics
	detector *ContentDetector

	// Connection tracking
	activeConns sync.Map // map[string]*CachingConnection
	connCounter uint64   // Atomic counter for connection IDs
}

// NewCachingListener creates a new caching listener that wraps the provided listener
func NewCachingListener(listener net.Listener, config *CacheConfig) *CachingListener {
	if config == nil {
		config = DefaultCacheConfig()
	}

	if err := config.Validate(); err != nil {
		panic("invalid cache configuration: " + err.Error())
	}

	metrics := NewCacheMetrics(config.EnableMetrics)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)

	return &CachingListener{
		wrapped:  listener,
		cache:    cache,
		config:   config,
		metrics:  metrics,
		detector: detector,
	}
}

// Accept waits for and returns the next connection to the listener
func (cl *CachingListener) Accept() (net.Conn, error) {
	conn, err := cl.wrapped.Accept()
	if err != nil {
		return nil, err
	}

	// Wrap the connection with caching capabilities
	cachingConn := NewCachingConnection(conn, cl.cache, cl.config, cl.metrics, cl.detector)

	// Track the connection
	connID := cachingConn.ID()
	cl.activeConns.Store(connID, cachingConn)

	// Set up cleanup callback for when connection closes
	cachingConn.SetCloseCallback(func() {
		cl.activeConns.Delete(connID)
	})

	return cachingConn, nil
}

// Close closes the listener and all active connections
func (cl *CachingListener) Close() error {
	// Close cache resources first
	cl.cache.Close()

	// Close wrapped listener
	return cl.wrapped.Close()
}

// Addr returns the listener's network address
func (cl *CachingListener) Addr() net.Addr {
	return cl.wrapped.Addr()
}

// GetCache returns the underlying cache for management operations
func (cl *CachingListener) GetCache() *TTLCache {
	return cl.cache
}

// GetMetrics returns the current cache metrics
func (cl *CachingListener) GetMetrics() *CacheMetrics {
	return cl.metrics
}

// GetConfig returns the cache configuration
func (cl *CachingListener) GetConfig() *CacheConfig {
	return cl.config
}

// GetStats returns comprehensive statistics about the caching listener
func (cl *CachingListener) GetStats() ListenerStats {
	cacheStats := cl.metrics.GetStats()

	// Count active connections
	activeConnCount := 0
	cl.activeConns.Range(func(key, value interface{}) bool {
		activeConnCount++
		return true
	})

	return ListenerStats{
		CacheStats:        cacheStats,
		ActiveConnections: activeConnCount,
		CacheSize:         cl.cache.Size(),
		CacheMemoryUsage:  cl.cache.MemoryUsage(),
		ListenerAddress:   cl.wrapped.Addr().String(),
	}
}

// ClearCache removes all cached entries
func (cl *CachingListener) ClearCache() {
	cl.cache.Clear()
}

// UpdateConfig updates the cache configuration (note: some changes require restart)
func (cl *CachingListener) UpdateConfig(newConfig *CacheConfig) error {
	if err := newConfig.Validate(); err != nil {
		return err
	}

	// Update configuration
	cl.config = newConfig

	// Update detector with new config
	cl.detector = NewContentDetector(newConfig)

	return nil
}

// ListenerStats contains comprehensive statistics about the caching listener
type ListenerStats struct {
	CacheStats        CacheStats `json:"cache_stats"`
	ActiveConnections int        `json:"active_connections"`
	CacheSize         int        `json:"cache_size"`
	CacheMemoryUsage  uint64     `json:"cache_memory_usage"`
	ListenerAddress   string     `json:"listener_address"`
}
