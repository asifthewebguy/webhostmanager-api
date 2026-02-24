package users

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleDeveloper  Role = "developer"
	RoleViewer     Role = "viewer"
)

// AllRoles in privilege order (highest first).
var AllRoles = []Role{RoleSuperAdmin, RoleAdmin, RoleDeveloper, RoleViewer}

func (r Role) Valid() bool {
	for _, v := range AllRoles {
		if r == v {
			return true
		}
	}
	return false
}

// User is the GORM model for the users table.
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username     string     `gorm:"uniqueIndex;not null"                           json:"username"`
	Email        string     `gorm:"uniqueIndex;not null"                           json:"email"`
	PasswordHash string     `gorm:"not null"                                       json:"-"`
	Role         Role       `gorm:"not null;default:'viewer'"                      json:"role"`
	IsActive     bool       `gorm:"not null;default:true"                          json:"is_active"`
	LastLoginAt  *time.Time `                                                       json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `                                                       json:"created_at"`
	UpdatedAt    time.Time  `                                                       json:"updated_at"`
}

func (User) TableName() string { return "users" }

// CreateUserRequest is the JSON body for creating a new user.
type CreateUserRequest struct {
	Username        string `json:"username"         binding:"required,min=3,max=100"`
	Email           string `json:"email"            binding:"required,email"`
	Password        string `json:"password"         binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	Role            Role   `json:"role"             binding:"required"`
}

// UpdateUserRequest is the JSON body for updating an existing user.
type UpdateUserRequest struct {
	Email    string `json:"email"     binding:"omitempty,email"`
	Password string `json:"password"  binding:"omitempty,min=8"`
	Role     Role   `json:"role"      binding:"omitempty"`
	IsActive *bool  `json:"is_active"`
}

// UserResponse is a safe user representation (no password hash).
type UserResponse struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Role        Role       `json:"role"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Role:        u.Role,
		IsActive:    u.IsActive,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
