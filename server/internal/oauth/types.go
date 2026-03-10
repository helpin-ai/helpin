package oauth

import "time"

// TokenPair holds OAuth tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}
