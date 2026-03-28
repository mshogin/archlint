// Package handler provides HTTP request handling.
package handler

// VIOLATION: fan-out - this file imports 13 packages.
// Allowed fan-out per archlint config is 5.
// A handler should only know about the service layer; importing repo directly,
// plus a dozen stdlib helpers, exceeds the fan-out budget significantly.
import (
	"demo/internal/config"   // VIOLATION: fan-out #1
	"demo/internal/model"    // VIOLATION: fan-out #2
	"demo/internal/repo"     // VIOLATION: fan-out #3 + layer skip (handler -> repo)
	"demo/internal/tracking" // VIOLATION: fan-out #4
	"encoding/json"          // VIOLATION: fan-out #5
	"errors"                 // VIOLATION: fan-out #6
	"fmt"                    // VIOLATION: fan-out #7
	"log"                    // VIOLATION: fan-out #8
	"math/rand"              // VIOLATION: fan-out #9
	"net/http"               // VIOLATION: fan-out #10
	"os"                     // VIOLATION: fan-out #11
	"strconv"                // VIOLATION: fan-out #12
	"strings"                // VIOLATION: fan-out #13
	"time"                   // VIOLATION: fan-out #14  (total: 14 imports)
)

// UserHandler handles HTTP requests for users.
// VIOLATION: layer skip - it holds a direct reference to *repo.UserRepo
// instead of depending on a service.UserService.
type UserHandler struct {
	// VIOLATION: layer skip - handler depends on repo, skipping the service layer.
	userRepo *repo.UserRepo
	cfg      config.Configurator
	tracker  *tracking.RequestTracker
	logger   *log.Logger
}

// NewUserHandler creates a UserHandler.
// VIOLATION: layer skip - *repo.UserRepo is injected here instead of a service.
func NewUserHandler(r *repo.UserRepo, cfg config.Configurator) *UserHandler {
	return &UserHandler{
		userRepo: r,
		cfg:      cfg,
		tracker:  tracking.NewRequestTracker(),
		logger:   log.New(os.Stderr, "[handler] ", log.LstdFlags),
	}
}

// Tracker exposes the RequestTracker so repo/cache.go can reference it.
func (h *UserHandler) Tracker() *tracking.RequestTracker { return h.tracker }

// CreateUser handles POST /users.
// VIOLATION: layer skip - business logic (validation + persistence) lives in handler,
// not delegated to service.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	reqID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
	h.tracker.Record(reqID)

	var u model.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		h.writeError(w, http.StatusBadRequest, err)
		return
	}

	// VIOLATION: layer skip - validation and persistence belong in service + repo,
	// not in the handler directly.
	if strings.TrimSpace(u.Name) == "" {
		h.writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}

	id, err := h.userRepo.Save(&u) // VIOLATION: direct repo call from handler
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.logger.Printf("created user id=%d host=%s log=%s", id, h.cfg.GetHost(), h.cfg.GetLogLevel())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]int{"id": id})
}

// GetUser handles GET /users/{id}.
// VIOLATION: layer skip - repo access from handler.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimPrefix(r.URL.Path, "/users/")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id: %s", rawID))
		return
	}

	u, err := h.userRepo.FindByID(id) // VIOLATION: direct repo call from handler
	if err != nil {
		h.writeError(w, http.StatusNotFound, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}

// writeError writes a JSON error response.
func (h *UserHandler) writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
