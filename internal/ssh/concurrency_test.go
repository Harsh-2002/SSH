package ssh

import (
	"sync"
	"testing"
)

// TestPoolConcurrency verifies thread safety under high contention.
// This is a "source of truth" stress test for the double-checked locking implementation.
func TestPoolConcurrency(t *testing.T) {
	pool := NewPool(false)
	defer pool.Close()

	const (
		numGoroutines = 50
		numKeys       = 5
		iterations    = 100
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Hammer the pool with concurrent requests for the same set of keys
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "shared-key" // All hit the same key to test locking
			
			for j := 0; j < iterations; j++ {
				mgr := pool.GetByHeader(key)
				if mgr == nil {
					t.Errorf("routine %d iter %d: expected manager, got nil", id, j)
					return
				}
				
				// Verify we always get the SAME manager instance for the same key
				// This proves the double-checked locking correctly prevents duplicates
				mgr2 := pool.GetByHeader(key)
				if mgr != mgr2 {
					t.Errorf("routine %d iter %d: got different manager instances for same key", id, j)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	pool.headerCacheMu.RLock()
	count := len(pool.headerCache)
	pool.headerCacheMu.RUnlock()

	if count != 1 {
		t.Errorf("expected exactly 1 manager in cache, got %d", count)
	}
}

// TestAliasGenerationConcurrency ensures generated aliases never conflict under load.
func TestAliasGenerationConcurrency(t *testing.T) {
	mgr := NewManager("", "/")
	defer mgr.Close()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	type result struct {
		alias string
		err   error
	}
	results := make(chan result, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// Everyone tries to get "admin@server" at the same time
			alias := mgr.generateAlias("admin", "server")
			
			// Simulate registering it immediately (like Connect would)
			mgr.mu.Lock()
			mgr.connections[alias] = &Client{}
			mgr.mu.Unlock()
			
			results <- result{alias, nil}
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[string]bool)
	for res := range results {
		if seen[res.alias] {
			t.Errorf("duplicate alias generated: %s", res.alias)
		}
		seen[res.alias] = true
	}
}
