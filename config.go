// Package selectcache provides transport-layer caching middleware for Go's net.Listener
// that intercepts and caches network responses with configurable TTL management.
//
// License: MIT
package selectcache

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CacheConfig holds configuration for the transport-layer caching middleware
type CacheConfig struct {
	// DefaultTTL is the default time-to-live for cached responses
	DefaultTTL time.Duration `json:"default_ttl"`

	// ContentTypeTTLs provides per-content-type TTL overrides
	ContentTypeTTLs map[string]time.Duration `json:"content_type_ttls"`

	// MaxMemoryMB is the maximum memory in megabytes for cache storage
	MaxMemoryMB int64 `json:"max_memory_mb"`

	// MaxEntries is the maximum number of cache entries
	MaxEntries int `json:"max_entries"`

	// ExcludedTypes are content types that should never be cached
	ExcludedTypes []string `json:"excluded_types"`

	// EnableMetrics determines if performance metrics are collected
	EnableMetrics bool `json:"enable_metrics"`

	// CleanupInterval is how often expired entries are removed
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// BufferSize is the size of the read buffer for connection analysis
	BufferSize int `json:"buffer_size"`

	// ConnectionTimeout is the maximum time to wait for connection analysis
	ConnectionTimeout time.Duration `json:"connection_timeout"`
}

// DefaultCacheConfig returns sensible defaults for the caching middleware
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:      15 * time.Minute,
		ContentTypeTTLs: make(map[string]time.Duration),
		MaxMemoryMB:     512,   // 512MB default limit
		MaxEntries:      10000, // 10k entries default
		ExcludedTypes: []string{
			"text/html",
			"application/xhtml+xml",
		},
		EnableMetrics:     true,
		CleanupInterval:   5 * time.Minute,
		BufferSize:        8192, // 8KB buffer for analysis
		ConnectionTimeout: 30 * time.Second,
	}
}

// Validate checks the configuration for invalid values
func (c *CacheConfig) Validate() error {
	if err := c.validateTimeSettings(); err != nil {
		return err
	}

	if err := c.validateResourceLimits(); err != nil {
		return err
	}

	if err := c.validateNetworkSettings(); err != nil {
		return err
	}

	if err := c.validateContentTypeTTLs(); err != nil {
		return err
	}

	return nil
}

// validateTimeSettings validates time-related configuration values
func (c *CacheConfig) validateTimeSettings() error {
	if c.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be positive, got %v", c.DefaultTTL)
	}

	if c.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive, got %v", c.CleanupInterval)
	}

	return nil
}

// validateResourceLimits validates memory and entry limit configuration values
func (c *CacheConfig) validateResourceLimits() error {
	if c.MaxMemoryMB <= 0 {
		return fmt.Errorf("max memory must be positive, got %d MB", c.MaxMemoryMB)
	}

	if c.MaxEntries <= 0 {
		return fmt.Errorf("max entries must be positive, got %d", c.MaxEntries)
	}

	return nil
}

// validateNetworkSettings validates network-related configuration values
func (c *CacheConfig) validateNetworkSettings() error {
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive, got %d", c.BufferSize)
	}

	if c.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive, got %v", c.ConnectionTimeout)
	}

	return nil
}

// validateContentTypeTTLs validates TTL values for configured content types
func (c *CacheConfig) validateContentTypeTTLs() error {
	for contentType, ttl := range c.ContentTypeTTLs {
		if ttl <= 0 {
			return fmt.Errorf("TTL for content type %s must be positive, got %v", contentType, ttl)
		}
	}

	return nil
}

// LoadFromJSON loads configuration from JSON bytes
func (c *CacheConfig) LoadFromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// ToJSON serializes the configuration to JSON
func (c *CacheConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// GetTTLForContentType returns the TTL for a specific content type,
// falling back to DefaultTTL if no specific TTL is configured
func (c *CacheConfig) GetTTLForContentType(contentType string) time.Duration {
	if ttl, exists := c.ContentTypeTTLs[contentType]; exists {
		return ttl
	}
	return c.DefaultTTL
}

// IsContentTypeExcluded checks if a content type should be excluded from caching
func (c *CacheConfig) IsContentTypeExcluded(contentType string) bool {
	contentTypeLower := strings.ToLower(contentType)
	for _, excluded := range c.ExcludedTypes {
		if strings.Contains(contentTypeLower, strings.ToLower(excluded)) {
			return true
		}
	}
	return false
}
