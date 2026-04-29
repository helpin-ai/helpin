package middleware

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

type contextKey string

const (
	userIDKey      contextKey = "user_id"
	userEmailKey   contextKey = "user_email"
	workspaceIDKey contextKey = "workspace_id"
	claimsKey      contextKey = "claims"
)

// WithUserID stores the user ID in the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithUserEmail stores the user email in the context.
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
}

// WithClaims stores the validated JWT claims in context.
func WithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFrom retrieves validated JWT claims from context.
func ClaimsFrom(ctx context.Context) *auth.Claims {
	v, _ := ctx.Value(claimsKey).(*auth.Claims)
	return v
}

// GetUserID retrieves the user ID from the context.
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// GetUserEmail retrieves the user email from the context.
func GetUserEmail(ctx context.Context) string {
	v, _ := ctx.Value(userEmailKey).(string)
	return v
}

// WithWorkspaceID stores the workspace ID in context.
func WithWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, workspaceIDKey, workspaceID)
}

// GetWorkspaceID retrieves workspace ID from context.
func GetWorkspaceID(ctx context.Context) string {
	v, _ := ctx.Value(workspaceIDKey).(string)
	return v
}
