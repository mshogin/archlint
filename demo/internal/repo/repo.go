// Package repo provides data access for users.
package repo

import (
	"demo/internal/model"
	"fmt"
)

// UserRepo handles persistence for users.
type UserRepo struct {
	store map[int]*model.User
	next  int
}

// NewUserRepo creates a new in-memory UserRepo.
func NewUserRepo() *UserRepo {
	return &UserRepo{store: make(map[int]*model.User)}
}

// Save persists a user and returns its assigned ID.
func (r *UserRepo) Save(u *model.User) (int, error) {
	if u == nil {
		return 0, fmt.Errorf("user must not be nil")
	}
	r.next++
	u.ID = r.next
	r.store[u.ID] = u
	return u.ID, nil
}

// FindByID retrieves a user by ID.
func (r *UserRepo) FindByID(id int) (*model.User, error) {
	u, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

// Delete removes a user by ID.
func (r *UserRepo) Delete(id int) error {
	if _, ok := r.store[id]; !ok {
		return fmt.Errorf("user %d not found", id)
	}
	delete(r.store, id)
	return nil
}

// Count returns total number of stored users.
func (r *UserRepo) Count() int { return len(r.store) }
