package ssh

import (
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	t.Run("normal mode creates empty pool", func(t *testing.T) {
		pool := NewPool(false)
		defer pool.Close()

		if pool.globalMode {
			t.Error("expected globalMode to be false")
		}
		if pool.global != nil {
			t.Error("expected global manager to be nil in normal mode")
		}
	})

	t.Run("global mode creates shared manager", func(t *testing.T) {
		pool := NewPool(true)
		defer pool.Close()

		if !pool.globalMode {
			t.Error("expected globalMode to be true")
		}
		if pool.global == nil {
			t.Error("expected global manager to exist in global mode")
		}
	})
}

func TestPoolSessionManagement(t *testing.T) {
	pool := NewPool(false)
	defer pool.Close()

	sessionID := "test-session-123"

	t.Run("CreateSession creates new manager", func(t *testing.T) {
		pool.CreateSession(sessionID)

		mgr := pool.Get(sessionID)
		if mgr == nil {
			t.Error("expected manager to exist after CreateSession")
		}
	})

	t.Run("Get returns nil for non-existent session", func(t *testing.T) {
		mgr := pool.Get("non-existent")
		if mgr != nil {
			t.Error("expected nil for non-existent session")
		}
	})

	t.Run("DestroySession removes manager", func(t *testing.T) {
		pool.DestroySession(sessionID)

		mgr := pool.Get(sessionID)
		if mgr != nil {
			t.Error("expected manager to be nil after DestroySession")
		}
	})
}

func TestPoolGlobalMode(t *testing.T) {
	pool := NewPool(true)
	defer pool.Close()

	t.Run("Get returns global manager regardless of session ID", func(t *testing.T) {
		mgr1 := pool.Get("session-1")
		mgr2 := pool.Get("session-2")

		if mgr1 != mgr2 {
			t.Error("expected same global manager for all sessions")
		}
		if mgr1 != pool.global {
			t.Error("expected Get to return global manager")
		}
	})

	t.Run("CreateSession is no-op in global mode", func(t *testing.T) {
		pool.CreateSession("new-session")
		// Should not panic or create new manager
		if len(pool.managers) != 0 {
			t.Error("expected no per-session managers in global mode")
		}
	})
}

func TestPoolHeaderBasedPooling(t *testing.T) {
	pool := NewPool(false)
	defer pool.Close()

	headerKey := "api-key-12345"

	t.Run("GetByHeader creates new manager on first call", func(t *testing.T) {
		mgr := pool.GetByHeader(headerKey)
		if mgr == nil {
			t.Error("expected manager to be created")
		}
	})

	t.Run("GetByHeader returns same manager on subsequent calls", func(t *testing.T) {
		mgr1 := pool.GetByHeader(headerKey)
		mgr2 := pool.GetByHeader(headerKey)

		if mgr1 != mgr2 {
			t.Error("expected same manager for same header key")
		}
	})

	t.Run("GetByHeader returns nil for empty key", func(t *testing.T) {
		mgr := pool.GetByHeader("")
		if mgr != nil {
			t.Error("expected nil for empty header key")
		}
	})

	t.Run("different header keys get different managers", func(t *testing.T) {
		mgr1 := pool.GetByHeader("key-a")
		mgr2 := pool.GetByHeader("key-b")

		if mgr1 == mgr2 {
			t.Error("expected different managers for different keys")
		}
	})
}

func TestSessionEntry(t *testing.T) {
	mgr := NewManager("", "/")
	entry := &sessionEntry{manager: mgr}

	t.Run("touch sets lastAccessed", func(t *testing.T) {
		// Before touch, lastAccessed should be 0
		if entry.lastAccessed.Load() != 0 {
			t.Error("expected initial lastAccessed to be 0")
		}

		entry.touch()
		if entry.lastAccessed.Load() == 0 {
			t.Error("expected lastAccessed to be set after touch")
		}
	})

	t.Run("age returns correct duration", func(t *testing.T) {
		// Set lastAccessed to 2 seconds ago
		entry.lastAccessed.Store(time.Now().Add(-2 * time.Second).Unix())
		age := entry.age()

		if age < time.Second || age > 3*time.Second {
			t.Errorf("expected age around 2s, got %v", age)
		}
	})

	t.Run("acquire and release track active requests", func(t *testing.T) {
		entry2 := &sessionEntry{manager: mgr}
		
		if entry2.activeReqs.Load() != 0 {
			t.Error("expected initial activeReqs to be 0")
		}
		if entry2.inUse() {
			t.Error("expected inUse to be false initially")
		}

		entry2.acquire()
		if entry2.activeReqs.Load() != 1 {
			t.Errorf("expected activeReqs=1 after acquire, got %d", entry2.activeReqs.Load())
		}
		if !entry2.inUse() {
			t.Error("expected inUse to be true after acquire")
		}

		entry2.acquire()
		if entry2.activeReqs.Load() != 2 {
			t.Errorf("expected activeReqs=2 after second acquire, got %d", entry2.activeReqs.Load())
		}

		entry2.release()
		if entry2.activeReqs.Load() != 1 {
			t.Errorf("expected activeReqs=1 after release, got %d", entry2.activeReqs.Load())
		}

		entry2.release()
		if entry2.activeReqs.Load() != 0 {
			t.Errorf("expected activeReqs=0 after second release, got %d", entry2.activeReqs.Load())
		}
		if entry2.inUse() {
			t.Error("expected inUse to be false after all releases")
		}
	})
}

func TestPoolTouchHeaderAcquireRelease(t *testing.T) {
	pool := NewPool(false)
	defer pool.Close()

	headerKey := "test-session-key"

	t.Run("TouchHeader creates and acquires", func(t *testing.T) {
		pool.TouchHeader(headerKey)
		
		pool.headerCacheMu.RLock()
		entry := pool.headerCache[headerKey]
		pool.headerCacheMu.RUnlock()

		if entry == nil {
			t.Fatal("expected entry to be created")
		}
		if entry.activeReqs.Load() != 1 {
			t.Errorf("expected activeReqs=1 after TouchHeader, got %d", entry.activeReqs.Load())
		}
	})

	t.Run("second TouchHeader increments active count", func(t *testing.T) {
		pool.TouchHeader(headerKey)
		
		pool.headerCacheMu.RLock()
		entry := pool.headerCache[headerKey]
		pool.headerCacheMu.RUnlock()

		if entry.activeReqs.Load() != 2 {
			t.Errorf("expected activeReqs=2 after second TouchHeader, got %d", entry.activeReqs.Load())
		}
	})

	t.Run("ReleaseHeader decrements active count", func(t *testing.T) {
		pool.ReleaseHeader(headerKey)
		
		pool.headerCacheMu.RLock()
		entry := pool.headerCache[headerKey]
		pool.headerCacheMu.RUnlock()

		if entry.activeReqs.Load() != 1 {
			t.Errorf("expected activeReqs=1 after ReleaseHeader, got %d", entry.activeReqs.Load())
		}
	})

	t.Run("GetByHeader returns same manager without changing active count", func(t *testing.T) {
		pool.headerCacheMu.RLock()
		beforeCount := pool.headerCache[headerKey].activeReqs.Load()
		pool.headerCacheMu.RUnlock()

		mgr := pool.GetByHeader(headerKey)
		if mgr == nil {
			t.Fatal("expected manager to be returned")
		}

		pool.headerCacheMu.RLock()
		afterCount := pool.headerCache[headerKey].activeReqs.Load()
		pool.headerCacheMu.RUnlock()

		if beforeCount != afterCount {
			t.Errorf("GetByHeader changed activeReqs: before=%d, after=%d", beforeCount, afterCount)
		}
	})
}
