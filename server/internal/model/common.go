package model

import "time"

// APIError represents a standard error response.
type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
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
	RoleViewer = "viewer"
)

// ErrForbidden is returned when a user lacks permission for an action.
type ErrForbidden struct {
	Message string
}

func (e *ErrForbidden) Error() string { return e.Message }

// Timestamps holds common timestamp fields.
type Timestamps struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
