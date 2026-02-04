package ssh

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	sessionHeader = "X-Session-Key"
	defaultTimeout = 5 * time.Minute
	minCleanupInterval = 5 * time.Second
	maxCleanupInterval = 60 * time.Second
)

// sessionEntry tracks a manager and its last access time.
type sessionEntry struct {
	manager      *Manager
	lastAccessed atomic.Int64
}

func (e *sessionEntry) touch() {
	e.lastAccessed.Store(time.Now().Unix())
}

func (e *sessionEntry) age() time.Duration {
	return time.Since(time.Unix(e.lastAccessed.Load(), 0))
}

// Pool manages SSH Managers for multiple MCP sessions.
// Supports three modes:
// 1. Global: Single shared manager (-global flag)
// 2. Header-based: Per X-Session-Key header (HTTP mode)
// 3. Session-based: Per MCP session ID (default)
type Pool struct {
	// Per-session managers (keyed by session ID)
	managers   map[string]*Manager
	managersMu sync.RWMutex

	// Header-based cache (keyed by X-Session-Key header)
	headerCache   map[string]*sessionEntry
	headerCacheMu sync.RWMutex

	// Global mode
	globalMode bool
	global     *Manager

	// Cleanup
	timeout      time.Duration
	nextInterval time.Duration
	stopCleanup  chan struct{}
}

// NewPool creates a new session pool.
func NewPool(globalMode bool) *Pool {
	pool := &Pool{
		managers:     make(map[string]*Manager),
		headerCache:  make(map[string]*sessionEntry),
		globalMode:   globalMode,
		timeout:      defaultTimeout,
		nextInterval: 30 * time.Second,
		stopCleanup:  make(chan struct{}),
	}

	if globalMode {
		pool.global = NewManager("", "/")
		log.Println("[Pool] Running in global mode - single shared manager")
	} else {
		// Start cleanup goroutine
		go pool.cleanupLoop()
		log.Printf("[Pool] Started with %v session timeout", pool.timeout)
	}

	return pool
}

// Get returns the Manager for the session ID.
func (p *Pool) Get(sessionID string) *Manager {
	if p.globalMode {
		return p.global
	}

	p.managersMu.RLock()
	mgr := p.managers[sessionID]
	p.managersMu.RUnlock()
	return mgr
}

// GetByHeader returns a Manager for the given header key.
// Uses double-checked locking for optimal concurrency:
// - Fast path: Existing sessions without lock
// - Slow path: New sessions with lock
func (p *Pool) GetByHeader(headerKey string) *Manager {
	if p.globalMode {
		return p.global
	}

	if headerKey == "" {
		return nil
	}

	// Fast path: check without lock (atomic read)
	p.headerCacheMu.RLock()
	entry := p.headerCache[headerKey]
	p.headerCacheMu.RUnlock()

	if entry != nil {
		entry.touch()
		log.Printf("[Pool] Reusing header session: %s (fast path)", headerKey)
		return entry.manager
	}

	// Slow path: create with lock
	p.headerCacheMu.Lock()
	defer p.headerCacheMu.Unlock()

	// Double-check after acquiring lock
	if entry = p.headerCache[headerKey]; entry != nil {
		entry.touch()
		log.Printf("[Pool] Reusing header session: %s (slow path)", headerKey)
		return entry.manager
	}

	// Create new
	log.Printf("[Pool] Creating new header session: %s", headerKey)
	mgr := NewManager("", "/")
	entry = &sessionEntry{manager: mgr}
	entry.touch()
	p.headerCache[headerKey] = entry
	return mgr
}

// CreateSession creates a new Manager for the session.
func (p *Pool) CreateSession(sessionID string) {
	if p.globalMode {
		return
	}

	p.managersMu.Lock()
	defer p.managersMu.Unlock()

	if _, exists := p.managers[sessionID]; exists {
		return
	}

	p.managers[sessionID] = NewManager("", "/")
	log.Printf("[Pool] Created manager for session %s", sessionID)
}

// DestroySession removes and closes the Manager.
func (p *Pool) DestroySession(sessionID string) {
	if p.globalMode {
		return
	}

	p.managersMu.Lock()
	defer p.managersMu.Unlock()

	mgr, exists := p.managers[sessionID]
	if !exists {
		return
	}

	mgr.Close()
	delete(p.managers, sessionID)
	log.Printf("[Pool] Destroyed manager for session %s", sessionID)
}

// cleanupLoop runs adaptive cleanup for header-based sessions.
func (p *Pool) cleanupLoop() {
	for {
		select {
		case <-p.stopCleanup:
			return
		case <-time.After(p.nextInterval):
			p.reap()
		}
	}
}

// reap removes expired header sessions and calculates next interval.
func (p *Pool) reap() {
	var toRemove []string
	nextExpiry := time.Duration(1<<63 - 1) // max duration

	// First pass: identify expired and calculate next expiry
	p.headerCacheMu.RLock()
	for key, entry := range p.headerCache {
		age := entry.age()
		if age > p.timeout {
			toRemove = append(toRemove, key)
		} else {
			timeUntilExpiry := p.timeout - age
			if timeUntilExpiry < nextExpiry {
				nextExpiry = timeUntilExpiry
			}
		}
	}
	sessionCount := len(p.headerCache)
	p.headerCacheMu.RUnlock()

	// Second pass: remove expired (with close outside main lock)
	for _, key := range toRemove {
		var mgr *Manager

		p.headerCacheMu.Lock()
		if entry, ok := p.headerCache[key]; ok {
			// Double-check inside lock
			if entry.age() > p.timeout {
				delete(p.headerCache, key)
				mgr = entry.manager
				log.Printf("[Pool] Cleaning up idle header session: %s", key)
			}
		}
		p.headerCacheMu.Unlock()

		if mgr != nil {
			mgr.Close()
		}
	}

	// Adaptive sleep interval
	if sessionCount == 0 || nextExpiry == time.Duration(1<<63-1) {
		p.nextInterval = maxCleanupInterval
	} else {
		p.nextInterval = nextExpiry + time.Second
		if p.nextInterval < minCleanupInterval {
			p.nextInterval = minCleanupInterval
		}
		if p.nextInterval > maxCleanupInterval {
			p.nextInterval = maxCleanupInterval
		}
	}

	if len(toRemove) > 0 || sessionCount > 0 {
		log.Printf("[Pool] Cleanup: removed=%d, active=%d, next_check=%v", 
			len(toRemove), sessionCount-len(toRemove), p.nextInterval)
	}
}

// Close closes all managers.
func (p *Pool) Close() {
	// Stop cleanup loop
	close(p.stopCleanup)

	// Close global
	if p.global != nil {
		p.global.Close()
	}

	// Close session managers
	p.managersMu.Lock()
	for _, mgr := range p.managers {
		mgr.Close()
	}
	p.managers = make(map[string]*Manager)
	p.managersMu.Unlock()

	// Close header cache
	p.headerCacheMu.Lock()
	for _, entry := range p.headerCache {
		entry.manager.Close()
	}
	p.headerCache = make(map[string]*sessionEntry)
	p.headerCacheMu.Unlock()

	log.Println("[Pool] All managers closed")
}

// SessionHeader returns the header name used for session keys.
func SessionHeader() string {
	return sessionHeader
}
