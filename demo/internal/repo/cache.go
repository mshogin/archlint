// Package repo provides data access for users.
package repo

// VIOLATION: cross-layer dependency - repo imports handler-layer infrastructure.
//
// The "tracking" package is logically part of the handler layer (it tracks HTTP
// request IDs). A repository must never depend on anything from the handler layer.
// Correct direction: handler -> service -> repo  (one way, downward only)
// Actual direction here: repo -> tracking (handler layer) - WRONG direction!
//
// archlint detects this as a forbidden cross-layer import:
//   repo (layer 4) -> tracking/handler (layer 1) = upward dependency violation.
import (
	"demo/internal/tracking" // VIOLATION: repo -> handler-layer dependency (wrong direction)
	"fmt"
)

// CachedRepo wraps UserRepo with a write-through cache.
// It uses tracking.RequestTracker (handler-layer) for cache-key generation,
// which creates a forbidden upward dependency.
type CachedRepo struct {
	base    *UserRepo
	tracker *tracking.RequestTracker // VIOLATION: repo depends on handler-layer type
	cache   map[string]string
}

// NewCachedRepo creates a CachedRepo.
// VIOLATION: tracker comes from the handler layer - wrong direction of dependency.
func NewCachedRepo(base *UserRepo, tracker *tracking.RequestTracker) *CachedRepo {
	return &CachedRepo{
		base:    base,
		tracker: tracker,
		cache:   make(map[string]string),
	}
}

// GetCached returns a cached string representation or fetches from the base repo.
func (c *CachedRepo) GetCached(id int) (string, error) {
	// VIOLATION: using handler-layer tracker in repo logic
	lastReq := c.tracker.LastRequestID()
	cacheKey := fmt.Sprintf("%d:%s", id, lastReq)

	if v, ok := c.cache[cacheKey]; ok {
		return v, nil
	}

	u, err := c.base.FindByID(id)
	if err != nil {
		return "", err
	}

	result := u.String()
	c.cache[cacheKey] = result
	return result, nil
}
