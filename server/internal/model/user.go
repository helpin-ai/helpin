package model

import "time"

// User represents a row in the users table.
type User struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"not null"`
	FullName     string    `json:"full_name" gorm:"not null"`
	AvatarURL    *string   `json:"avatar_url"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (User) TableName() string { return "users" }

// SignupRequest is the payload for POST /api/auth/signup.
type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// SigninRequest is the payload for POST /api/auth/signin.
type SigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned after successful authentication.
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         UserProfile `json:"user"`
}

// UserProfile is the public user representation.
type UserProfile struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	AvatarURL *string   `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateProfileRequest is the payload for PUT /api/auth/me.
type UpdateProfileRequest struct {
	FullName  *string `json:"full_name"`
	AvatarURL *string `json:"avatar_url"`
}

// RefreshTokenRequest is the payload for POST /api/auth/refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}
