package websocket

import "context"

// PresenceProvider abstracts presence state storage for viewing, typing,
// and visitor-online tracking. Both the in-memory PresenceState (dev/tests)
// and RedisPresence (production multi-pod) implement this interface.
//
// All conn-scoped methods accept a connID to support multiple tabs per user.
// The in-memory implementation ignores TTL/refresh (keys never expire) but
// still tracks connID for correctness.
type PresenceProvider interface {
	// Viewing — conn-scoped (one agent can view from multiple tabs).
	SetViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) (changed bool, err error)
	ClearViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) (changed bool, err error)
	GetViewers(ctx context.Context, workspaceID, conversationID string) ([]string, error)
	// GetActiveViewing returns the conversationID this connection is currently viewing (empty if none).
	GetActiveViewing(ctx context.Context, workspaceID, userID, connID string) (string, error)
	RefreshViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) error

	// Typing — per-user with connID ownership tag (last-writer-wins).
	SetTyping(ctx context.Context, workspaceID, conversationID, userID, connID, content string) error
	ClearTyping(ctx context.Context, workspaceID, conversationID, userID, connID string) (changed bool, err error)
	GetTypers(ctx context.Context, workspaceID, conversationID string) (map[string]string, error)

	// ClearAllForConn clears all viewing and typing state for a user+conn,
	// returning the affected conversation IDs.
	ClearAllForConn(ctx context.Context, workspaceID, userID, connID string) (viewingCleared []string, typingCleared []string, err error)

	// GetSnapshot returns viewers + typers for a conversation.
	GetSnapshot(ctx context.Context, workspaceID, conversationID string) (PresenceSnapshot, error)

	// Online visitors — conn-scoped (one visitor can have multiple widget tabs).
	SetVisitorOnline(ctx context.Context, workspaceID, anonymousID, connID string) error
	SetVisitorOffline(ctx context.Context, workspaceID, anonymousID, connID string) (lastConn bool, err error)
	IsVisitorOnline(ctx context.Context, workspaceID, anonymousID string) (bool, error)
	GetOnlineVisitors(ctx context.Context, workspaceID string) ([]string, error)
	RefreshVisitorOnline(ctx context.Context, workspaceID, anonymousID, connID string) error

	// RefreshAllForConn refreshes all active keys for a user+conn (called on support:ping).
	RefreshAllForConn(ctx context.Context, workspaceID, userID, connID string) error
}
