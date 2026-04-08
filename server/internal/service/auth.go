package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"gorm.io/gorm"
)

const passwordResetTTL = time.Hour

type authEmailSender interface {
	SendPasswordResetEmail(to, fullName, resetURL string) error
}

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo          *repository.UserRepository
	passwordResetRepo *repository.PasswordResetTokenRepository
	organizationRepo  *repository.OrganizationRepository
	jwtManager        *auth.JWTManager
	s3Client          *storage.S3Client
	emailClient       authEmailSender
	appBaseURL        string
	logger            *slog.Logger
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	userRepo *repository.UserRepository,
	passwordResetRepo *repository.PasswordResetTokenRepository,
	organizationRepo *repository.OrganizationRepository,
	jwtManager *auth.JWTManager,
	s3Client *storage.S3Client,
	emailClient authEmailSender,
	appBaseURL string,
) *AuthService {
	return &AuthService{
		userRepo:          userRepo,
		passwordResetRepo: passwordResetRepo,
		organizationRepo:  organizationRepo,
		jwtManager:        jwtManager,
		s3Client:          s3Client,
		emailClient:       emailClient,
		appBaseURL:        strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
		logger:            slog.Default().With("service", "auth"),
	}
}

// Signup creates a new user account and returns auth tokens.
func (s *AuthService) Signup(ctx context.Context, req model.SignupRequest) (*model.AuthResponse, error) {
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		return nil, fmt.Errorf("email, password, and full_name are required")
	}

	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to check existing user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		s.logger.InfoContext(ctx, "signup attempted with existing email", "email", req.Email)
		return nil, fmt.Errorf("email already in use")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash password during signup", "email", req.Email, "error", err)
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, req.Email, hash, req.FullName)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create user", "email", req.Email, "error", err)
		return nil, fmt.Errorf("create user: %w", err)
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, false)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens after signup", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "user signed up", "user_id", user.ID, "email", user.Email)

	// Auto-create a default organization for the new user.
	s.autoCreateOrganization(ctx, user)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserProfile(user),
	}, nil
}

// autoCreateOrganization creates a default organization for a newly registered user.
func (s *AuthService) autoCreateOrganization(ctx context.Context, user *model.User) {
	if s.organizationRepo == nil {
		return
	}
	firstName := strings.SplitN(strings.TrimSpace(user.FullName), " ", 2)[0]
	if firstName == "" {
		firstName = "My"
	}
	orgName := firstName + "'s Organization"
	orgSlug := slugifyOrg(orgName)

	org, err := s.organizationRepo.Create(ctx, orgName, orgSlug, user.ID, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to auto-create organization", "error", err, "user_id", user.ID)
		return
	}
	if _, err := s.organizationRepo.AddMember(ctx, org.ID, user.ID, "owner"); err != nil {
		s.logger.ErrorContext(ctx, "failed to add user as org owner", "error", err, "org_id", org.ID, "user_id", user.ID)
	}
	s.logger.InfoContext(ctx, "auto-created organization", "org_id", org.ID, "org_name", orgName, "user_id", user.ID)
}

// slugifyOrg converts a name to a URL-friendly slug.
func slugifyOrg(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// Signin authenticates a user and returns auth tokens.
func (s *AuthService) Signin(ctx context.Context, req model.SigninRequest) (*model.AuthResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to find user during signin", "email", req.Email, "error", err)
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		s.logger.InfoContext(ctx, "signin attempted with unknown email", "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		s.logger.InfoContext(ctx, "signin failed invalid password", "user_id", user.ID, "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, req.RememberMe)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens after signin", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "user signed in", "user_id", user.ID, "email", user.Email, "remember_me", req.RememberMe)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserProfile(user),
	}, nil
}

// GetProfile returns the profile for the given user ID.
func (s *AuthService) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// UpdateProfile updates the authenticated user's profile.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req model.UpdateProfileRequest) (*model.UserProfile, error) {
	user, err := s.userRepo.Update(ctx, userID, req.FullName, req.AvatarURL, req.AvatarStyle, req.AvatarSeed, req.AvatarBackgroundMode, req.AvatarBackgroundColor, req.DefaultWorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	profile := toUserProfile(user)
	return &profile, nil
}

// UploadAvatar uploads a user avatar to S3 and saves the public URL.
func (s *AuthService) UploadAvatar(ctx context.Context, userID string, body io.Reader, size int64, contentType string) (*model.UserProfile, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage not configured")
	}

	// Delete old avatar from S3 if it exists.
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.AvatarURL != nil && *user.AvatarURL != "" {
		oldKey := fmt.Sprintf("users/%s/avatar/%s", userID, filepath.Base(*user.AvatarURL))
		_ = s.s3Client.DeleteObject(ctx, oldKey)
	}

	ext := ".png"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	}
	key := fmt.Sprintf("users/%s/avatar/%s%s", userID, uuid.New().String(), ext)

	if err := s.s3Client.PutObject(ctx, key, contentType, size, body, true); err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	avatarURL := s.s3Client.PublicURL(key)
	user, err = s.userRepo.Update(ctx, userID, nil, &avatarURL, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	s.logger.InfoContext(ctx, "avatar uploaded", "user_id", userID)
	profile := toUserProfile(user)
	return &profile, nil
}

// DeleteAvatar removes the user's avatar.
func (s *AuthService) DeleteAvatar(ctx context.Context, userID string) (*model.UserProfile, error) {
	// Delete old avatar from S3 if it exists.
	existing, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("delete avatar: %w", err)
	}
	if existing != nil && existing.AvatarURL != nil && *existing.AvatarURL != "" && s.s3Client != nil {
		oldKey := fmt.Sprintf("users/%s/avatar/%s", userID, filepath.Base(*existing.AvatarURL))
		_ = s.s3Client.DeleteObject(ctx, oldKey)
	}

	emptyURL := ""
	user, err := s.userRepo.Update(ctx, userID, nil, &emptyURL, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("delete avatar: %w", err)
	}

	s.logger.InfoContext(ctx, "avatar deleted", "user_id", userID)
	profile := toUserProfile(user)
	return &profile, nil
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		s.logger.ErrorContext(ctx, "invalid refresh token", "error", err)
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user during token refresh", "user_id", claims.UserID, "error", err)
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		s.logger.ErrorContext(ctx, "user not found during token refresh", "user_id", claims.UserID)
		return nil, fmt.Errorf("user not found")
	}

	accessToken, newRefresh, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, false)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate tokens during refresh", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	s.logger.InfoContext(ctx, "token refreshed", "user_id", user.ID)

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		User:         toUserProfile(user),
	}, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, req model.ChangePasswordRequest) error {
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return fmt.Errorf("current_password and new_password are required")
	}
	if len(req.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user during password change", "user_id", userID, "error", err)
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		s.logger.ErrorContext(ctx, "user not found during password change", "user_id", userID)
		return fmt.Errorf("user not found")
	}

	if err := auth.CheckPassword(req.CurrentPassword, user.PasswordHash); err != nil {
		s.logger.InfoContext(ctx, "password change failed invalid current password", "user_id", userID)
		return fmt.Errorf("current password is incorrect")
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash new password", "user_id", userID, "error", err)
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		s.logger.ErrorContext(ctx, "failed to update password", "user_id", userID, "error", err)
		return err
	}
	if s.passwordResetRepo != nil {
		if err := s.passwordResetRepo.InvalidateAllForUser(ctx, userID, time.Now().UTC()); err != nil {
			s.logger.ErrorContext(ctx, "failed to invalidate reset tokens after password change", "user_id", userID, "error", err)
		}
	}

	s.logger.InfoContext(ctx, "password changed", "user_id", userID)
	return nil
}

// ForgotPassword creates a single-use reset token and emails a reset link when the user exists.
func (s *AuthService) ForgotPassword(ctx context.Context, req model.ForgotPasswordRequest) error {
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	if emailAddr == "" {
		return fmt.Errorf("email is required")
	}

	user, err := s.userRepo.GetByEmail(ctx, emailAddr)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to look up user for forgot password", "email", emailAddr, "error", err)
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		s.logger.InfoContext(ctx, "forgot password requested for unknown email", "email", emailAddr)
		return nil
	}
	if s.passwordResetRepo == nil {
		s.logger.ErrorContext(ctx, "password reset repository not configured", "user_id", user.ID)
		return fmt.Errorf("password reset is not configured")
	}

	rawToken, tokenHash, err := generatePasswordResetToken()
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate password reset token", "user_id", user.ID, "error", err)
		return err
	}

	now := time.Now().UTC()
	resetToken := &model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(passwordResetTTL),
	}

	if err := s.passwordResetRepo.WithTx(ctx, func(txRepo *repository.PasswordResetTokenRepository, tx *gorm.DB) error {
		if err := txRepo.InvalidateAllForUser(ctx, user.ID, now); err != nil {
			return err
		}
		if err := txRepo.Create(ctx, resetToken); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to persist password reset token", "user_id", user.ID, "error", err)
		return err
	}

	resetURL := buildPasswordResetURL(s.appBaseURL, rawToken)
	if s.emailClient == nil {
		s.logger.WarnContext(ctx, "password reset requested but email client not configured", "user_id", user.ID, "email", emailAddr)
		return nil
	}
	if err := s.emailClient.SendPasswordResetEmail(user.Email, user.FullName, resetURL); err != nil {
		s.logger.ErrorContext(ctx, "failed to send password reset email", "user_id", user.ID, "email", emailAddr, "error", err)
	}

	s.logger.InfoContext(ctx, "password reset requested", "user_id", user.ID, "email", emailAddr)
	return nil
}

// ResetPassword validates a password reset token and sets a new password.
func (s *AuthService) ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error {
	token := strings.TrimSpace(req.Token)
	if token == "" || req.Password == "" {
		return fmt.Errorf("token and password are required")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if s.passwordResetRepo == nil {
		return fmt.Errorf("password reset is not configured")
	}

	tokenHash := hashPasswordResetToken(token)
	now := time.Now().UTC()

	return s.passwordResetRepo.WithTx(ctx, func(txRepo *repository.PasswordResetTokenRepository, txDB *gorm.DB) error {
		tokenRow, err := txRepo.GetActiveByTokenHash(ctx, tokenHash, now)
		if err != nil {
			return err
		}
		if tokenRow == nil {
			return fmt.Errorf("password reset link is invalid or has expired")
		}

		userRepo := repository.NewUserRepository(txDB)
		user, err := userRepo.GetByID(ctx, tokenRow.UserID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if user == nil {
			return fmt.Errorf("user not found")
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		used, err := txRepo.MarkUsed(ctx, tokenRow.ID, now)
		if err != nil {
			return err
		}
		if !used {
			return fmt.Errorf("password reset link is invalid or has expired")
		}

		if err := userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := txRepo.InvalidateOtherTokensForUser(ctx, user.ID, tokenRow.ID, now); err != nil {
			return err
		}

		s.logger.InfoContext(ctx, "password reset completed", "user_id", user.ID)
		return nil
	})
}

func toUserProfile(u *model.User) model.UserProfile {
	return model.UserProfile{
		ID:                    u.ID,
		Email:                 u.Email,
		FullName:              u.FullName,
		AvatarURL:             u.AvatarURL,
		AvatarStyle:           u.AvatarStyle,
		AvatarSeed:            u.AvatarSeed,
		AvatarBackgroundMode:  u.AvatarBackgroundMode,
		AvatarBackgroundColor: u.AvatarBackgroundColor,
		DefaultWorkspaceID:    u.DefaultWorkspaceID,
		CreatedAt:             u.CreatedAt,
	}
}

func generatePasswordResetToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate password reset token: %w", err)
	}
	rawToken := hex.EncodeToString(b)
	return rawToken, hashPasswordResetToken(rawToken), nil
}

func hashPasswordResetToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func buildPasswordResetURL(appBaseURL, token string) string {
	base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/reset-password?token=%s", base, url.QueryEscape(token))
}
