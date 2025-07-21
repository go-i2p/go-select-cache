package selectcache

import (
	"testing"
	"time"
)

func TestCacheConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *CacheConfig
		wantError bool
	}{
		{
			name:      "valid default config",
			config:    DefaultCacheConfig(),
			wantError: false,
		},
		{
			name: "invalid default TTL",
			config: &CacheConfig{
				DefaultTTL:        0,
				MaxMemoryMB:       100,
				MaxEntries:        1000,
				CleanupInterval:   time.Minute,
				BufferSize:        4096,
				ConnectionTimeout: 30 * time.Second,
			},
			wantError: true,
		},
		{
			name: "invalid max memory",
			config: &CacheConfig{
				DefaultTTL:        time.Minute,
				MaxMemoryMB:       -1,
				MaxEntries:        1000,
				CleanupInterval:   time.Minute,
				BufferSize:        4096,
				ConnectionTimeout: 30 * time.Second,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("CacheConfig.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCacheConfig_GetTTLForContentType(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL: 10 * time.Minute,
		ContentTypeTTLs: map[string]time.Duration{
			"application/json": 5 * time.Minute,
			"image/png":        1 * time.Hour,
		},
	}

	tests := []struct {
		name        string
		contentType string
		expectedTTL time.Duration
	}{
		{
			name:        "configured content type",
			contentType: "application/json",
			expectedTTL: 5 * time.Minute,
		},
		{
			name:        "unconfigured content type",
			contentType: "text/plain",
			expectedTTL: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := config.GetTTLForContentType(tt.contentType)
			if ttl != tt.expectedTTL {
				t.Errorf("GetTTLForContentType() = %v, want %v", ttl, tt.expectedTTL)
			}
		})
	}
}
