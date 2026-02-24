package auth

import "github.com/golang-jwt/jwt/v5"

// Claims are the JWT payload fields.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// LoginRequest is the JSON body for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	Token    string      `json:"token"`
	UserID   string      `json:"user_id"`
	Username string      `json:"username"`
	Email    string      `json:"email"`
	Role     string      `json:"role"`
}

// Context keys stored in Gin context after JWT validation.
const (
	CtxUserID   = "user_id"
	CtxUsername = "username"
	CtxEmail    = "user_email"
	CtxRole     = "role"
)
