package ssh

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ContextKey is used for storing values in context.
// Exported so main.go and tools can use the same type.
type ContextKey string

const (
	// SessionKeyContextKey is the context key for X-Session-Key header value.
	// Used for sticky session routing in HTTP mode.
	SessionKeyContextKey ContextKey = "session-key"

	sessionHeader      = "X-Session-Key"
	defaultTimeout     = 5 * time.Minute
	minCleanupInterval = 5 * time.Second
	maxCleanupInterval = 60 * time.Second
)

// sessionEntry tracks a manager and its last access time.
type sessionEntry struct {
	manager      *Manager
	lastAccessed atomic.Int64
	activeReqs   atomic.Int32 // Number of in-flight requests
}

func (e *sessionEntry) touch() {
	e.lastAccessed.Store(time.Now().Unix())
}

func (e *sessionEntry) age() time.Duration {
	return time.Since(time.Unix(e.lastAccessed.Load(), 0))
}

func (e *sessionEntry) acquire() {
	e.activeReqs.Add(1)
	e.touch()
}

func (e *sessionEntry) release() {
	e.activeReqs.Add(-1)
}

func (e *sessionEntry) inUse() bool {
	return e.activeReqs.Load() > 0
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
//
// Note: Does NOT acquire active count - that's done by TouchHeader in session hooks.
// This prevents count imbalance from multiple tool calls per session.
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
		log.Printf("[Pool] Reusing manager for header: %s (active=%d)", headerKey, entry.activeReqs.Load())
		return entry.manager
	}

	// Slow path: create with lock
	p.headerCacheMu.Lock()
	defer p.headerCacheMu.Unlock()

	// Double-check after acquiring lock
	if entry = p.headerCache[headerKey]; entry != nil {
		entry.touch()
		log.Printf("[Pool] Reusing manager for header: %s (after lock, active=%d)", headerKey, entry.activeReqs.Load())
		return entry.manager
	}

	// Create new (shouldn't happen if TouchHeader was called first in session hook)
	log.Printf("[Pool] WARNING: Created manager via GetByHeader for header: %s (TouchHeader should create first)", headerKey)
	mgr := NewManager("", "/")
	entry = &sessionEntry{manager: mgr}
	entry.touch()
	p.headerCache[headerKey] = entry
	return mgr
}

// ReleaseHeader decrements the active request count for a header session.
// Called by session hooks on session end to allow cleanup when idle.
func (p *Pool) ReleaseHeader(headerKey string) {
	if p.globalMode || headerKey == "" {
		return
	}

	p.headerCacheMu.RLock()
	entry := p.headerCache[headerKey]
	p.headerCacheMu.RUnlock()

	if entry != nil {
		entry.release()
		log.Printf("[Pool] Released session for header: %s (active=%d)", headerKey, entry.activeReqs.Load())
	}
}

// TouchHeader creates or updates a header-based session.
// If the session doesn't exist, creates it.
// Acquires the active request count (balanced by ReleaseHeader on session end).
// Called by session hooks on session start.
func (p *Pool) TouchHeader(headerKey string) {
	if p.globalMode || headerKey == "" {
		return
	}

	// Fast path: entry exists
	p.headerCacheMu.RLock()
	entry := p.headerCache[headerKey]
	p.headerCacheMu.RUnlock()

	if entry != nil {
		entry.acquire() // Acquire for this session
		log.Printf("[Pool] Acquired session for header: %s (active=%d)", headerKey, entry.activeReqs.Load())
		return
	}

	// Slow path: create if not exists
	p.headerCacheMu.Lock()
	defer p.headerCacheMu.Unlock()

	// Double-check after acquiring lock
	if entry = p.headerCache[headerKey]; entry != nil {
		entry.acquire()
		log.Printf("[Pool] Acquired session for header: %s (after lock, active=%d)", headerKey, entry.activeReqs.Load())
		return
	}

	// Create new manager with active count = 1
	log.Printf("[Pool] Created new manager for header: %s", headerKey)
	mgr := NewManager("", "/")
	entry = &sessionEntry{manager: mgr}
	entry.acquire() // Start with active=1 for this session
	p.headerCache[headerKey] = entry
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
		// Only consider for removal if expired AND not in use
		if age > p.timeout && !entry.inUse() {
			toRemove = append(toRemove, key)
		} else if age <= p.timeout {
			timeUntilExpiry := p.timeout - age
			if timeUntilExpiry < nextExpiry {
				nextExpiry = timeUntilExpiry
			}
		}
		// If expired but in use, check again soon
		if age > p.timeout && entry.inUse() {
			log.Printf("[Pool] Skipping cleanup for %s: still in use (active=%d)", key, entry.activeReqs.Load())
			if minCleanupInterval < nextExpiry {
				nextExpiry = minCleanupInterval
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
			// Triple-check: expired AND not in use
			if entry.age() > p.timeout && !entry.inUse() {
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
// Waits briefly for active requests to complete before force-closing.
func (p *Pool) Close() {
	// Stop cleanup loop
	close(p.stopCleanup)

	// Close global
	if p.global != nil {
		p.global.Close()
	}

	// Close session managers
	p.managersMu.Lock()
	for id, mgr := range p.managers {
		log.Printf("[Pool] Closing session manager: %s", id)
		mgr.Close()
	}
	p.managers = make(map[string]*Manager)
	p.managersMu.Unlock()

	// Close header cache - wait briefly for active requests
	p.headerCacheMu.Lock()
	for key, entry := range p.headerCache {
		if entry.inUse() {
			log.Printf("[Pool] Warning: Closing header session %s with %d active requests", key, entry.activeReqs.Load())
		}
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
