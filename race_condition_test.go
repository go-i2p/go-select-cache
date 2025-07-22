package selectcache

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRaceConditionInCloseMethod tests for potential deadlock in Close() method
// when multiple goroutines are accessing Read/Write/Close simultaneously
func TestRaceConditionInCloseMethod(t *testing.T) {
	for i := 0; i < 10; i++ { // Run multiple iterations to increase chance of catching race condition
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			config := &CacheConfig{
				DefaultTTL:  5 * time.Minute,
				MaxMemoryMB: 10,
			}
			metrics := NewCacheMetrics(true)
			cache := NewTTLCache(config, metrics)
			detector := NewContentDetector(config)

			cachingConn := NewCachingConnection(server, cache, config, metrics, detector)
			var wg sync.WaitGroup
			var mu sync.Mutex
			var readErr, writeErr, closeErr error

			// Helper functions to safely set errors
			setReadErr := func(err error) {
				mu.Lock()
				defer mu.Unlock()
				if readErr == nil {
					readErr = err
				}
			}

			setWriteErr := func(err error) {
				mu.Lock()
				defer mu.Unlock()
				if writeErr == nil {
					writeErr = err
				}
			}

			setCloseErr := func(err error) {
				mu.Lock()
				defer mu.Unlock()
				closeErr = err
			}

			// Simulate high concurrency scenario with different lock ordering
			// Start multiple Read operations
			for j := 0; j < 3; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 5; k++ {
						buffer := make([]byte, 100)
						_, err := cachingConn.Read(buffer)
						if err != nil {
							setReadErr(err)
							return
						}
						time.Sleep(1 * time.Millisecond)
					}
				}()
			}

			// Start multiple Write operations
			for j := 0; j < 3; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 5; k++ {
						data := []byte("test data")
						_, err := cachingConn.Write(data)
						if err != nil {
							setWriteErr(err)
							return
						}
						time.Sleep(1 * time.Millisecond)
					}
				}()
			}

			// Start Close operation in the middle
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond) // Let read/write operations start
				setCloseErr(cachingConn.Close())
			}()

			// Handle network I/O to prevent blocking
			go func() {
				for {
					buffer := make([]byte, 1024)
					n, err := client.Read(buffer)
					if err != nil {
						return
					}
					if n > 0 {
						client.Write([]byte("response"))
					}
				}
			}()

			// Wait for completion with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Test passed - no deadlock
				t.Logf("SUCCESS: No deadlock detected in iteration %d", i)
			case <-time.After(5 * time.Second):
				t.Fatalf("DEADLOCK: Operations did not complete within timeout - potential race condition")
			}
			// Check for errors (other than expected ones after close)
			mu.Lock()
			checkReadErr := readErr
			checkWriteErr := writeErr
			checkCloseErr := closeErr
			mu.Unlock()

			if checkReadErr != nil && !isConnectionClosed(checkReadErr) {
				t.Errorf("Unexpected read error: %v", checkReadErr)
			}
			if checkWriteErr != nil && !isConnectionClosed(checkWriteErr) {
				t.Errorf("Unexpected write error: %v", checkWriteErr)
			}
			if checkCloseErr != nil {
				t.Errorf("Close error: %v", checkCloseErr)
			}
		})
	}
}

// TestSpecificLockOrderingIssue tests the specific issue identified in Close() method
func TestSpecificLockOrderingIssue(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	config := &CacheConfig{
		DefaultTTL:  5 * time.Minute,
		MaxMemoryMB: 10,
	}
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)

	cachingConn := NewCachingConnection(server, cache, config, metrics, detector)

	var wg sync.WaitGroup

	// Scenario: Goroutine 1 holds readMu, tries to get stateMu
	// Goroutine 2 holds stateMu (in Close), tries to get readMu
	// This could create a deadlock

	wg.Add(2)

	// Goroutine 1: Hold readMu for extended time, then access state
	go func() {
		defer wg.Done()
		buffer := make([]byte, 100)
		for i := 0; i < 20; i++ {
			cachingConn.Read(buffer)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Goroutine 2: Try to close (which holds stateMu then tries readMu/writeMu)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // Let read operation start
		cachingConn.Close()
	}()

	// Network I/O handler
	go func() {
		for {
			buffer := make([]byte, 1024)
			n, err := client.Read(buffer)
			if err != nil {
				return
			}
			if n > 0 {
				client.Write([]byte("HTTP/1.1 200 OK\r\n\r\nOK"))
			}
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("SUCCESS: No deadlock in specific lock ordering scenario")
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK: Specific lock ordering issue detected")
	}
}

// Helper function to check if error is due to connection being closed
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "closed") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "broken pipe")
}
