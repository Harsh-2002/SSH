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

	sessionHeader    = "X-Session-Key"
	defaultTimeout   = 5 * time.Minute
	cleanupInterval  = 60 * time.Second
)

// sessionEntry tracks a manager and its last access time.
// Cleanup is based purely on time since last access (not active request count).
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
	timeout     time.Duration
	stopCleanup chan struct{}
}

// NewPool creates a new session pool.
func NewPool(globalMode bool) *Pool {
	pool := &Pool{
		managers:    make(map[string]*Manager),
		headerCache: make(map[string]*sessionEntry),
		globalMode:  globalMode,
		timeout:     defaultTimeout,
		stopCleanup: make(chan struct{}),
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
// Every call touches the session, extending its expiry by the timeout duration.
// Uses double-checked locking for optimal concurrency.
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
		entry.touch() // Extend expiry on every request
		log.Printf("[Pool] Reusing manager for header: %s (expires in %v)", headerKey, p.timeout)
		return entry.manager
	}

	// Slow path: create with lock
	p.headerCacheMu.Lock()
	defer p.headerCacheMu.Unlock()

	// Double-check after acquiring lock
	if entry = p.headerCache[headerKey]; entry != nil {
		entry.touch()
		log.Printf("[Pool] Reusing manager for header: %s (after lock)", headerKey)
		return entry.manager
	}

	// Create new
	log.Printf("[Pool] Created new manager for header: %s (expires in %v)", headerKey, p.timeout)
	mgr := NewManager("", "/")
	entry = &sessionEntry{manager: mgr}
	entry.touch()
	p.headerCache[headerKey] = entry
	return mgr
}

// TouchHeader touches/creates a header-based session to extend its expiry.
// Called on every request to keep the session alive.
func (p *Pool) TouchHeader(headerKey string) {
	if p.globalMode || headerKey == "" {
		return
	}

	// Fast path: entry exists - just touch it
	p.headerCacheMu.RLock()
	entry := p.headerCache[headerKey]
	p.headerCacheMu.RUnlock()

	if entry != nil {
		entry.touch()
		return
	}

	// Slow path: create if not exists
	p.headerCacheMu.Lock()
	defer p.headerCacheMu.Unlock()

	// Double-check after acquiring lock
	if entry = p.headerCache[headerKey]; entry != nil {
		entry.touch()
		return
	}

	// Create new manager
	log.Printf("[Pool] Created new manager for header: %s (expires in %v)", headerKey, p.timeout)
	mgr := NewManager("", "/")
	entry = &sessionEntry{manager: mgr}
	entry.touch()
	p.headerCache[headerKey] = entry
}

// ReleaseHeader is a no-op now. Cleanup is based purely on time, not active count.
// Kept for API compatibility but does nothing.
func (p *Pool) ReleaseHeader(headerKey string) {
	// No-op: cleanup is based on lastAccessed time, not active count
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

// cleanupLoop runs cleanup every 60 seconds for header-based sessions.
func (p *Pool) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCleanup:
			return
		case <-ticker.C:
			p.reap()
		}
	}
}

// reap removes expired header sessions based on last access time.
// Simple logic: if no request for > timeout duration, close and remove.
func (p *Pool) reap() {
	var toRemove []string

	// First pass: identify expired sessions
	p.headerCacheMu.RLock()
	for key, entry := range p.headerCache {
		if entry.age() > p.timeout {
			toRemove = append(toRemove, key)
		}
	}
	sessionCount := len(p.headerCache)
	p.headerCacheMu.RUnlock()

	// Second pass: remove expired (close manager outside lock to prevent deadlock)
	for _, key := range toRemove {
		var mgr *Manager

		p.headerCacheMu.Lock()
		if entry, ok := p.headerCache[key]; ok {
			// Double-check still expired (could have been touched between passes)
			if entry.age() > p.timeout {
				delete(p.headerCache, key)
				mgr = entry.manager
				log.Printf("[Pool] Expired session removed: %s", key)
			}
		}
		p.headerCacheMu.Unlock()

		if mgr != nil {
			mgr.Close()
		}
	}

	// Log only when there are active sessions or something was removed
	if len(toRemove) > 0 || sessionCount > 0 {
		remaining := sessionCount - len(toRemove)
		if remaining > 0 {
			log.Printf("[Pool] Active sessions: %d", remaining)
		}
	}
}

// Close closes all managers. Safe to call multiple times.
func (p *Pool) Close() {
	// Use select to safely close channel (prevents panic on double-close)
	select {
	case <-p.stopCleanup:
		// Already closed
		return
	default:
		close(p.stopCleanup)
	}

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

	// Close header cache
	p.headerCacheMu.Lock()
	for key, entry := range p.headerCache {
		log.Printf("[Pool] Closing header session: %s", key)
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
