// Package user contains the User aggregate and its value objects.
package user

import (
	"fmt"
	"time"
)

// Role represents the user's role within the system.
type Role string

const (
	RoleDoctor    Role = "DOCTOR"
	RolePatient   Role = "PATIENT"
	RoleCaretaker Role = "CARETAKER"
)

// IsValid returns true if the role is a known value.
func (r Role) IsValid() bool {
	switch r {
	case RoleDoctor, RolePatient, RoleCaretaker:
		return true
	}
	return false
}

// User is the aggregate root for identity and access management.
type User struct {
	id           string
	email        string
	passwordHash string
	role         Role
	firstName    string
	lastName     string
	isActive     bool
	createdAt    time.Time
	updatedAt    time.Time
}

// NewUser creates a new User validating domain invariants.
// passwordHash must already be a bcrypt hash — hashing is an application concern.
func NewUser(id, email, passwordHash, firstName, lastName string, role Role) (*User, error) {
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if passwordHash == "" {
		return nil, fmt.Errorf("password hash is required")
	}
	if firstName == "" {
		return nil, fmt.Errorf("first name is required")
	}
	if lastName == "" {
		return nil, fmt.Errorf("last name is required")
	}
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role %q", role)
	}

	now := time.Now().UTC()
	return &User{
		id:           id,
		email:        email,
		passwordHash: passwordHash,
		role:         role,
		firstName:    firstName,
		lastName:     lastName,
		isActive:     true,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

// Reconstitute rebuilds a User from persisted data (no invariant re-validation).
func Reconstitute(id, email, passwordHash, firstName, lastName string, role Role, isActive bool, createdAt, updatedAt time.Time) *User {
	return &User{
		id:           id,
		email:        email,
		passwordHash: passwordHash,
		role:         role,
		firstName:    firstName,
		lastName:     lastName,
		isActive:     isActive,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// Read-only accessors.
func (u *User) ID() string           { return u.id }
func (u *User) Email() string        { return u.email }
func (u *User) PasswordHash() string { return u.passwordHash }
func (u *User) Role() Role           { return u.role }
func (u *User) FirstName() string    { return u.firstName }
func (u *User) LastName() string     { return u.lastName }
func (u *User) FullName() string     { return u.firstName + " " + u.lastName }
func (u *User) IsActive() bool       { return u.isActive }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// UpdateProfile updates mutable profile fields.
func (u *User) UpdateProfile(firstName, lastName string) error {
	if firstName == "" {
		return fmt.Errorf("first name is required")
	}
	if lastName == "" {
		return fmt.Errorf("last name is required")
	}
	u.firstName = firstName
	u.lastName = lastName
	u.updatedAt = time.Now().UTC()
	return nil
}

// ChangePassword replaces the stored password hash.
func (u *User) ChangePassword(newHash string) error {
	if newHash == "" {
		return fmt.Errorf("password hash is required")
	}
	u.passwordHash = newHash
	u.updatedAt = time.Now().UTC()
	return nil
}

// Deactivate marks the user as inactive.
func (u *User) Deactivate() {
	u.isActive = false
	u.updatedAt = time.Now().UTC()
}
