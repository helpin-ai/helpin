package middleware

import "context"

type contextKey string

const (
	userIDKey  contextKey = "user_id"
	userEmailKey contextKey = "user_email"
)

// WithUserID stores the user ID in the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithUserEmail stores the user email in the context.
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
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
