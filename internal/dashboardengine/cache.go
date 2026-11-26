package dashboardengine

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// A short TTL cache for compiled widget data (compile.go), keyed by the
// widget's own source definition -- so two widgets with an identical
// EntitySource share one cache entry, and the same widget's next request
// within the TTL skips EntityResolver entirely. Deliberately not part of
// EntityResolver or the resolve step itself: caching belongs at the
// compiled-data boundary (compile.go's own doc comment explains why),
// not wrapped around every individual xolu call.
//
// In-memory, per DashboardEngine instance. DashboardEngine is expected to
// be a long-lived value (constructed once at server startup, held on
// UIHandler, reused across requests -- same lifecycle formengine.FormEngine
// already has), so cache state persists naturally across requests without
// any wiring beyond the DashboardEngine itself.

type cacheEntry struct {
	value   map[string]any
	expires time.Time
}

type widgetCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	ttl     time.Duration
	now     func() time.Time // overridable for tests
}

func newWidgetCache(ttl time.Duration) *widgetCache {
	return &widgetCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
		now:     time.Now,
	}
}

func (c *widgetCache) get(key string) (map[string]any, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().After(e.expires) {
		return nil, false
	}
	return e.value, true
}

func (c *widgetCache) set(key string, value map[string]any) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{value: value, expires: c.now().Add(c.ttl)}
}

// sourceCacheKey builds a stable cache key from an EntitySource -- its own
// JSON encoding, plus CurrentUserID appended explicitly. EntitySource has
// no pointers/funcs, so encoding/json produces a deterministic key for
// equal values (Go map key ordering in JSON output is sorted by
// encoding/json's own rules, not insertion order, so this is stable
// across calls, not just within one). CurrentUserID carries json:"-" (it's
// not part of the schema, populated internally from RenderContext) so
// json.Marshal alone would never vary the key by user -- appended by hand
// here instead, since a scope: "self" audit_log source must never let one
// user's cached result answer another user's request.
func sourceCacheKey(widgetType string, s EntitySource) string {
	b, err := json.Marshal(s)
	if err != nil {
		// A source that can't marshal can't be meaningfully cached or
		// compared for equality either -- every render is a fresh miss,
		// which is correct (if conservative) rather than a cache that
		// silently never matches anything.
		return ""
	}
	return fmt.Sprintf("%s:%s:user=%d", widgetType, b, s.CurrentUserID)
}
