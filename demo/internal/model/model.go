// Package model defines domain entities.
package model

import (
	"fmt"
	"strings"
	"time"
)

// User is a domain entity.
// VIOLATION: god class - 15+ methods, violates Single Responsibility Principle.
type User struct {
	ID        int
	Name      string
	Email     string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Address   string
	Phone     string
	Status    string
	Score     float64
}

// --- Identity methods ---

// GetID returns user ID.
func (u *User) GetID() int { return u.ID }

// GetName returns user name.
func (u *User) GetName() string { return u.Name }

// GetEmail returns user email.
func (u *User) GetEmail() string { return u.Email }

// SetEmail sets user email.
func (u *User) SetEmail(email string) { u.Email = strings.TrimSpace(email) }

// IsAdmin checks if user is admin.
func (u *User) IsAdmin() bool { return u.Role == "admin" }

// --- Validation methods ---

// Validate validates user fields.
// VIOLATION: god class - validation belongs in a separate validator.
func (u *User) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("name required")
	}
	if u.Email == "" {
		return fmt.Errorf("email required")
	}
	if !strings.Contains(u.Email, "@") {
		return fmt.Errorf("invalid email: %s", u.Email)
	}
	return nil
}

// --- Formatting methods ---

// FormatName formats user name for display.
// VIOLATION: god class - formatting belongs in a presenter/formatter layer.
func (u *User) FormatName() string {
	return strings.Title(strings.ToLower(u.Name))
}

// FormatEmail returns lowercase email.
func (u *User) FormatEmail() string { return strings.ToLower(u.Email) }

// FormatPhone formats phone number.
func (u *User) FormatPhone() string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, u.Phone)
	if len(digits) == 11 {
		return fmt.Sprintf("+%s (%s) %s-%s-%s",
			digits[:1], digits[1:4], digits[4:7], digits[7:9], digits[9:11])
	}
	return u.Phone
}

// --- Audit methods ---

// Touch updates the UpdatedAt timestamp.
// VIOLATION: god class - audit logic belongs in a repository or audit service.
func (u *User) Touch() { u.UpdatedAt = time.Now() }

// Age returns seconds since user creation.
func (u *User) Age() float64 { return time.Since(u.CreatedAt).Seconds() }

// IsStale returns true if not updated in 90 days.
func (u *User) IsStale() bool { return time.Since(u.UpdatedAt) > 90*24*time.Hour }

// --- Business logic methods ---

// Activate activates the user.
// VIOLATION: god class - business state machine belongs in service layer.
func (u *User) Activate() { u.Status = "active" }

// Deactivate deactivates the user.
func (u *User) Deactivate() { u.Status = "inactive" }

// IsActive returns true if user is active.
func (u *User) IsActive() bool { return u.Status == "active" }

// UpdateScore updates user score.
// VIOLATION: god class - scoring belongs in a dedicated scoring service.
func (u *User) UpdateScore(delta float64) { u.Score += delta }

// String implements Stringer for debug output.
func (u *User) String() string {
	return fmt.Sprintf("User{ID:%d Name:%q Email:%q Role:%q Status:%q Score:%.2f}",
		u.ID, u.Name, u.Email, u.Role, u.Status, u.Score)
}
