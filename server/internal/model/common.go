package model

import "time"

// APIError represents a standard error response.
type APIError struct {
	Error string `json:"error"`
}

// MessageResponse represents a simple success message.
type MessageResponse struct {
	Message string `json:"message"`
}

// PaginatedResponse wraps a list response with pagination metadata.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total,omitempty"`
	Page       int         `json:"page,omitempty"`
	PerPage    int         `json:"per_page,omitempty"`
	TotalPages int         `json:"total_pages,omitempty"`
}

// Role constants shared across organizations and workspaces.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Timestamps holds common timestamp fields.
type Timestamps struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
