// Package tracking is a sub-package of handler infrastructure.
// It is intentionally placed here so that repo/cache.go can import it
// while the logical dependency still points from repo -> handler-layer code.
//
// In a real codebase this would live in handler/ and create a proper cycle.
// archlint detects this as a cross-layer violation: repo -> handler layer.
package tracking

// RequestTracker tracks the most recent HTTP request ID.
// Logically part of the handler layer; placed here for compilation only.
type RequestTracker struct {
	lastID string
}

// NewRequestTracker creates a RequestTracker.
func NewRequestTracker() *RequestTracker { return &RequestTracker{} }

// LastRequestID returns the last recorded request ID.
func (rt *RequestTracker) LastRequestID() string { return rt.lastID }

// Record saves a new request ID.
func (rt *RequestTracker) Record(id string) { rt.lastID = id }
