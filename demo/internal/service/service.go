// Package service contains application business logic.
// This is the CLEAN layer - no violations here.
// It depends only on repo (correct direction) and model.
package service

import (
	"demo/internal/model"
	"fmt"
	"time"
)

// UserRepository is the narrow interface service depends on.
// Only the methods service actually uses are declared here - ISP compliant.
type UserRepository interface {
	Save(u *model.User) (int, error)
	FindByID(id int) (*model.User, error)
}

// UserService implements user business logic.
type UserService struct {
	repo UserRepository
}

// NewUserService creates a UserService.
func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Register creates and persists a new user.
func (s *UserService) Register(name, email string) (*model.User, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	u := &model.User{
		Name:      name,
		Email:     email,
		Role:      "user",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err := s.repo.Save(u); err != nil {
		return nil, fmt.Errorf("saving user: %w", err)
	}

	return u, nil
}

// FindByID retrieves a user by ID.
func (s *UserService) FindByID(id int) (*model.User, error) {
	return s.repo.FindByID(id)
}
