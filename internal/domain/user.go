// Package domain contains business entities and domain models.
package domain

import "time"

// Role represents a user role in the system.
type Role string

// User roles.
const (
	RoleUser     Role = "user"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

// IsValid checks if the role is valid.
func (r Role) IsValid() bool {
	switch r {
	case RoleUser, RoleOperator, RoleAdmin:
		return true
	}
	return false
}

// HasPermission checks if this role has at least the permissions of the required role.
func (r Role) HasPermission(required Role) bool {
	roleHierarchy := map[Role]int{
		RoleUser:     1,
		RoleOperator: 2,
		RoleAdmin:    3,
	}
	return roleHierarchy[r] >= roleHierarchy[required]
}

// User represents a user account in the system.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"first_name,omitempty"`
	LastName     string    `json:"last_name,omitempty"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RefreshToken represents a refresh token stored in the database.
type RefreshToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}
