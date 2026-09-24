package model

import "time"

// Signup modes for a self-hosted server.
const (
	// SignupModeOpen lets anyone who can reach the server create an account.
	SignupModeOpen = "open"
	// SignupModeInviteOnly admits new accounts only through invitations.
	SignupModeInviteOnly = "invite_only"
	// SignupModeDomains admits addresses on the allowed email domains. New
	// accounts must verify their email before they can sign in.
	SignupModeDomains = "domains"
)

// Server admin sources reported by the admin list.
const (
	// ServerAdminSourceGranted is an admin assigned in the app (or the first account).
	ServerAdminSourceGranted = "granted"
	// ServerAdminSourceEnv is an admin designated by HELPIN_ADMIN_EMAILS.
	ServerAdminSourceEnv = "env"
)

// Application email settings sources.
const (
	AppEmailSourceEnv      = "env"
	AppEmailSourceDatabase = "database"
	AppEmailSourceNone     = "none"
)

// InstanceSettings is the single row of server-wide settings for a
// self-hosted installation. SMTPPasswordEncrypted is AES-256-GCM ciphertext.
type InstanceSettings struct {
	ID                    string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Singleton             bool       `json:"-" gorm:"not null;default:true;uniqueIndex"`
	SignupMode            string     `json:"signup_mode" gorm:"not null;default:'invite_only'"`
	SignupAllowedDomains  string     `json:"-" gorm:"not null;default:''"`
	AdminBootstrappedAt   *time.Time `json:"admin_bootstrapped_at"`
	SMTPHost              *string    `json:"-" gorm:"column:smtp_host"`
	SMTPPort              *int       `json:"-" gorm:"column:smtp_port;type:integer"`
	SMTPUsername          *string    `json:"-" gorm:"column:smtp_username"`
	SMTPPasswordEncrypted *string    `json:"-" gorm:"column:smtp_password_encrypted;type:text"`
	SMTPFrom              *string    `json:"-" gorm:"column:smtp_from"`
	SMTPTLSMode           *string    `json:"-" gorm:"column:smtp_tls_mode"`
	SMTPUpdatedAt         *time.Time `json:"-" gorm:"column:smtp_updated_at"`
	UpdatedBy             *string    `json:"-" gorm:"type:uuid"`
	CreatedAt             time.Time  `json:"created_at" gorm:"autoCreateTime;not null;default:now()"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"autoUpdateTime;not null;default:now()"`
}

// TableName returns the instance settings table.
func (InstanceSettings) TableName() string { return "instance_settings" }

// SignupPolicyResponse is the signup policy as server admins see it.
type SignupPolicyResponse struct {
	Mode           string   `json:"mode"`
	AllowedDomains []string `json:"allowed_domains"`
	// AppEmailConfigured reports whether verification email can be sent, which
	// domain-restricted signup requires.
	AppEmailConfigured bool `json:"app_email_configured"`
	// RecommendInviteOnly is true while signup is open to anyone.
	RecommendInviteOnly bool `json:"recommend_invite_only"`
}

// UpdateSignupPolicyRequest changes the signup policy.
type UpdateSignupPolicyRequest struct {
	Mode           string   `json:"mode"`
	AllowedDomains []string `json:"allowed_domains"`
}

// PublicSignupPolicy is the part of the signup policy the sign-in and
// registration pages need.
type PublicSignupPolicy struct {
	Mode           string   `json:"signup_mode"`
	AllowedDomains []string `json:"signup_allowed_domains"`
	// FirstUser is true until the first account exists; that account may
	// always sign up and becomes the server admin.
	FirstUser bool `json:"signup_first_user"`
}

// ServerAdmin is one account that administers the server.
type ServerAdmin struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	// Source is "env" when HELPIN_ADMIN_EMAILS designates the account; such
	// admins cannot be removed in the app.
	Source string `json:"source"`
}

// GrantServerAdminRequest makes an existing account a server admin.
type GrantServerAdminRequest struct {
	Email string `json:"email"`
}

// AppEmailSettingsResponse describes application email without secrets.
type AppEmailSettingsResponse struct {
	// Source is env (read-only server configuration), database (saved in the
	// app) or none.
	Source string `json:"source"`
	// Provider is smtp or postmark; empty when not configured.
	Provider string `json:"provider"`
	Editable bool   `json:"editable"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	From     string `json:"from"`
	TLSMode  string `json:"tls_mode"`
	// PasswordSet reports whether a password is stored; the password itself is
	// never returned.
	PasswordSet bool `json:"password_set"`
	// Problem explains why stored settings cannot be used; empty otherwise.
	Problem string `json:"problem,omitempty"`
}

// UpdateAppEmailSettingsRequest saves application SMTP settings. A nil
// Password keeps the stored password; an empty string clears it.
type UpdateAppEmailSettingsRequest struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Username string  `json:"username"`
	Password *string `json:"password"`
	From     string  `json:"from"`
	TLSMode  string  `json:"tls_mode"`
}
