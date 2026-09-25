package model

import "time"

// User represents a row in the users table.
type User struct {
	ID                     string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email                  string     `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash           string     `json:"-" gorm:"not null"`
	FullName               string     `json:"full_name" gorm:"not null"`
	EmailVerifiedAt        *time.Time `json:"email_verified_at"`
	GoogleSubject          *string    `json:"-" gorm:"uniqueIndex"`
	AvatarURL              *string    `json:"avatar_url"`
	AvatarStyle            *string    `json:"avatar_style"`
	AvatarSeed             *string    `json:"avatar_seed"`
	AvatarBackgroundMode   *string    `json:"avatar_background_mode"`
	AvatarBackgroundColor  *string    `json:"avatar_background_color"`
	DefaultWorkspaceID     *string    `json:"default_workspace_id" gorm:"type:uuid"`
	TOTPSecretEncrypted    *string    `json:"-" gorm:"column:totp_secret_encrypted;type:text"`
	TOTPVerified           bool       `json:"-" gorm:"column:totp_verified;not null;default:false"`
	RecoveryCodesEncrypted *string    `json:"-" gorm:"column:recovery_codes_encrypted;type:text"`
	IsPlatformAdmin        bool       `json:"is_platform_admin" gorm:"column:is_platform_admin;not null;default:false"`
	// IsServerAdmin marks a self-hosted server's administrators (Community).
	IsServerAdmin bool `json:"is_server_admin" gorm:"column:is_server_admin;not null;default:false"`
	// SignupVerificationPending blocks sign-in until the email is verified; set
	// for accounts created through domain-restricted signup.
	SignupVerificationPending bool      `json:"-" gorm:"column:signup_verification_pending;not null;default:false"`
	CreatedAt                 time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (User) TableName() string { return "users" }

// SignupRequest is the payload for POST /api/auth/signup.
type SignupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	FullName    string `json:"full_name"`
	AnonymousID string `json:"anonymous_id,omitempty"`
}

// SigninRequest is the payload for POST /api/auth/signin.
type SigninRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// DemoSigninRequest is the payload for POST /api/auth/demo. Email is the
// visitor's own address (optional unless the server requires it); it is only
// forwarded to the lead webhook and never becomes an account.
type DemoSigninRequest struct {
	Email string `json:"email"`
}

// AuthResponse is returned after successful authentication.
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         UserProfile `json:"user"`
	// VerificationRequired is set instead of tokens when the new account must
	// verify its email before signing in (domain-restricted signup).
	VerificationRequired bool `json:"-"`
}

// SignupVerificationResponse is returned by signup when the account must
// verify its email before it can sign in.
type SignupVerificationResponse struct {
	VerificationRequired bool   `json:"verification_required"`
	Email                string `json:"email"`
}

// SigninResponse is returned after a signin attempt.
type SigninResponse struct {
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	User         *UserProfile `json:"user,omitempty"`
	Requires2FA  bool         `json:"requires_2fa,omitempty"`
	TwoFAToken   string       `json:"two_fa_token,omitempty"`
}

// UserProfile is the public user representation.
type UserProfile struct {
	ID                    string     `json:"id"`
	Email                 string     `json:"email"`
	FullName              string     `json:"full_name"`
	AvatarURL             *string    `json:"avatar_url"`
	AvatarStyle           *string    `json:"avatar_style"`
	AvatarSeed            *string    `json:"avatar_seed"`
	AvatarBackgroundMode  *string    `json:"avatar_background_mode"`
	AvatarBackgroundColor *string    `json:"avatar_background_color"`
	DefaultWorkspaceID    *string    `json:"default_workspace_id"`
	TwoFAEnabled          bool       `json:"two_fa_enabled"`
	IsPlatformAdmin       bool       `json:"is_platform_admin"`
	IsServerAdmin         bool       `json:"is_server_admin"`
	EmailVerified         bool       `json:"email_verified"`
	EmailVerifiedAt       *time.Time `json:"email_verified_at,omitempty"`
	MFASatisfiedInToken   bool       `json:"mfa_satisfied_in_token,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// UpdateProfileRequest is the payload for PUT /api/auth/me.
type UpdateProfileRequest struct {
	FullName              *string `json:"full_name"`
	AvatarURL             *string `json:"avatar_url"`
	AvatarStyle           *string `json:"avatar_style"`
	AvatarSeed            *string `json:"avatar_seed"`
	AvatarBackgroundMode  *string `json:"avatar_background_mode"`
	AvatarBackgroundColor *string `json:"avatar_background_color"`
	DefaultWorkspaceID    *string `json:"default_workspace_id"`
}

// ChangePasswordRequest is the payload for PUT /api/auth/change-password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ForgotPasswordRequest is the payload for POST /api/auth/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the payload for POST /api/auth/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// RefreshTokenRequest is the payload for POST /api/auth/refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TwoFASetupRequest is the payload for POST /api/auth/2fa/setup.
type TwoFASetupRequest struct {
	Password string `json:"password"`
}

// TwoFAVerifyRequest is the payload for POST /api/auth/2fa/verify.
type TwoFAVerifyRequest struct {
	TOTPCode string `json:"totp_code"`
}

// TwoFASigninRequest is the payload for POST /api/auth/2fa/verify-signin.
type TwoFASigninRequest struct {
	TwoFAToken   string `json:"two_fa_token"`
	TOTPCode     string `json:"totp_code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

// TwoFAStepUpRequest verifies an existing signed-in user and returns tokens
// marked as MFA-satisfied.
type TwoFAStepUpRequest struct {
	TOTPCode     string `json:"totp_code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

// TwoFADisableRequest is the payload for DELETE /api/auth/2fa.
type TwoFADisableRequest struct {
	Password string `json:"password"`
}

// TwoFARegenerateRequest is the payload for POST /api/auth/2fa/regenerate-recovery-codes.
type TwoFARegenerateRequest struct {
	Password string `json:"password"`
	TOTPCode string `json:"totp_code"`
}

// TwoFAStatusResponse returns the active 2FA state for the authenticated user.
type TwoFAStatusResponse struct {
	Enabled bool `json:"enabled"`
}

// TwoFASetupResponse returns the provisioning URI and recovery codes for a pending setup.
type TwoFASetupResponse struct {
	ProvisioningURI string   `json:"provisioning_uri"`
	RecoveryCodes   []string `json:"recovery_codes"`
}

// RecoveryCodesResponse returns newly generated recovery codes.
type RecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}
