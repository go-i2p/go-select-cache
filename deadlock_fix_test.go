package selectcache

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestDeadlockFixed verifies that the deadlock issue has been resolved
// This is a negative test confirming the issue from AUDIT.md is resolved
func TestDeadlockFixed(t *testing.T) {
	// Create a pipe for testing
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Create caching connection
	config := &CacheConfig{
		DefaultTTL:  5 * time.Minute,
		MaxMemoryMB: 10,
	}
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)

	cachingConn := NewCachingConnection(server, cache, config, metrics, detector)
	defer cachingConn.Close()

	// Test that read and write can happen concurrently without deadlock
	var wg sync.WaitGroup
	var readErr, writeErr error

	// Start read operation
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 1024)
		_, readErr = cachingConn.Read(buffer)
	}()

	// Start write operation
	wg.Add(1)
	go func() {
		defer wg.Done()
		response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nHello")
		_, writeErr = cachingConn.Write(response)
	}()

	// Handle network I/O coordination
	go func() {
		// Send request to trigger read
		time.Sleep(50 * time.Millisecond)
		client.Write([]byte("GET / HTTP/1.1\r\nHost: test.com\r\n\r\n"))

		// Consume response from write
		time.Sleep(100 * time.Millisecond)
		buffer := make([]byte, 1024)
		client.SetReadDeadline(time.Now().Add(time.Second))
		client.Read(buffer)
	}()

	// Wait for operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Verify operations complete without deadlock (confirming the fix)
	select {
	case <-done:
		// SUCCESS - operations completed without deadlock
		if readErr != nil && readErr.Error() != "EOF" && readErr.Error() != "use of closed network connection" {
			t.Errorf("Unexpected read error: %v", readErr)
		}
		if writeErr != nil && writeErr.Error() != "use of closed network connection" {
			t.Errorf("Unexpected write error: %v", writeErr)
		}
		t.Log("SUCCESS: Read and Write operations completed concurrently without deadlock - fix verified")
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Operations timed out - the deadlock fix is not working properly")
	}
}

// TestConcurrentOperations verifies multiple concurrent read/write operations work properly (confirms fix)
func TestConcurrentOperations(t *testing.T) {
	// Create a pipe for testing
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Create caching connection
	config := &CacheConfig{
		DefaultTTL:  5 * time.Minute,
		MaxMemoryMB: 10,
	}
	metrics := NewCacheMetrics(true)
	cache := NewTTLCache(config, metrics)
	detector := NewContentDetector(config)

	cachingConn := NewCachingConnection(server, cache, config, metrics, detector)
	defer cachingConn.Close()

	// Run multiple operations concurrently
	var wg sync.WaitGroup
	numOps := 5

	// Start multiple read operations
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			buffer := make([]byte, 100)
			cachingConn.Read(buffer)
		}(i)
	}

	// Start multiple write operations
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte("test data")
			cachingConn.Write(data)
		}(i)
	}

	// Coordinate network I/O
	go func() {
		for i := 0; i < numOps; i++ {
			time.Sleep(10 * time.Millisecond)
			client.Write([]byte("data"))

			buffer := make([]byte, 100)
			client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			client.Read(buffer)
		}
	}()

	// Wait for all operations
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("SUCCESS: All concurrent operations completed - deadlock fix verified")
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrent operations timed out - fix not working properly")
	}
}
