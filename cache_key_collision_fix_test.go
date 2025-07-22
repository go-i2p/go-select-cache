package selectcache

import (
	"testing"
)

// TestCacheKeyCollisionFix tests that header sanitization prevents cache key collisions
func TestCacheKeyCollisionFix(t *testing.T) {
	// Critical test case: headers that would collide without proper sanitization
	headers1 := map[string]string{
		"A": "b|C=d", // Contains pipe and equals that could confuse parsing
	}
	headers2 := map[string]string{
		"A": "b", // Separate headers that without sanitization
		"C": "d", // would generate the same raw key string
	}

	key1 := GenerateCacheKey("GET", "/test", "", headers1)
	key2 := GenerateCacheKey("GET", "/test", "", headers2)

	t.Logf("Key 1 (A='b|C=d'): %s", key1)
	t.Logf("Key 2 (A='b', C='d'): %s", key2)

	if key1 == key2 {
		t.Errorf("CRITICAL: Cache key collision detected: '%s' == '%s'", key1, key2)
		t.Errorf("This indicates the sanitization fix is not working properly")
	} else {
		t.Logf("SUCCESS: No collision detected - sanitization is working correctly")
	}

	// Additional edge cases
	testCases := []struct {
		name    string
		headers map[string]string
	}{
		{
			name:    "pipe in header value",
			headers: map[string]string{"Accept": "text/html|application/json"},
		},
		{
			name:    "equals in header value",
			headers: map[string]string{"Custom": "key=value"},
		},
		{
			name:    "backslash in header value",
			headers: map[string]string{"Path": "C:\\Windows\\System32"},
		},
		{
			name:    "multiple special chars",
			headers: map[string]string{"Complex": "a=b|c\\d=e"},
		},
	}

	var keys []string
	for _, tc := range testCases {
		key := GenerateCacheKey("GET", "/test", "", tc.headers)
		keys = append(keys, key)
		t.Logf("Key for %s: %s", tc.name, key)
	}

	// Ensure all keys are unique
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] == keys[j] {
				t.Errorf("Duplicate key found between case %d and %d: %s", i, j, keys[i])
			}
		}
	}

	t.Logf("All edge case tests passed - header sanitization working correctly")
}
