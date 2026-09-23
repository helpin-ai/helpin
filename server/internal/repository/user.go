package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UserRepository handles database operations for the users table.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user and returns the created user.
func (r *UserRepository) Create(ctx context.Context, email, passwordHash, fullName string) (*model.User, error) {
	user := &model.User{
		Email:        email,
		PasswordHash: passwordHash,
		FullName:     fullName,
	}
	return r.CreateUser(ctx, user)
}

// CreateUser inserts a new user row using the provided model.
func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// GetByEmail looks up a user by their email address, case-insensitively.
//
// The lookup is case-insensitive because email addresses are case-insensitive
// for routing purposes and callers normalize input to lowercase, while some
// legacy rows were stored with mixed case. A case-sensitive match would miss
// those rows and cause auth flows to mint duplicate accounts for the same
// person (see the users table's case-sensitive uniqueness gap).
//
// Results are ordered by id so that, in the (transient) event that duplicate
// case-variant rows still exist, the lookup deterministically resolves to the
// lowest-id row — the same account the merge migration keeps as the survivor.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.WithContext(ctx).Where("LOWER(email) = LOWER(?)", email).Order("id").First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// GetByID looks up a user by their ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// GrantPlatformAdminByEmails marks existing users with matching emails as platform admins.
func (r *UserRepository) GrantPlatformAdminByEmails(ctx context.Context, emails []string) (int64, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		cleaned := strings.ToLower(strings.TrimSpace(email))
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	if len(normalized) == 0 {
		return 0, nil
	}

	result := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("lower(email) IN ?", normalized).
		Update("is_platform_admin", true)
	if result.Error != nil {
		return 0, fmt.Errorf("grant platform admin: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// Update modifies a user's profile fields.
func (r *UserRepository) Update(ctx context.Context, id string, fullName, avatarURL, avatarStyle, avatarSeed, avatarBackgroundMode, avatarBackgroundColor, defaultWorkspaceID *string) (*model.User, error) {
	updates := map[string]interface{}{}
	if fullName != nil {
		updates["full_name"] = *fullName
	}
	if avatarURL != nil {
		updates["avatar_url"] = *avatarURL
	}
	if avatarStyle != nil {
		updates["avatar_style"] = *avatarStyle
	}
	if avatarSeed != nil {
		updates["avatar_seed"] = *avatarSeed
	}
	if avatarBackgroundMode != nil {
		updates["avatar_background_mode"] = *avatarBackgroundMode
	}
	if avatarBackgroundColor != nil {
		updates["avatar_background_color"] = *avatarBackgroundColor
	}
	if defaultWorkspaceID != nil {
		updates["default_workspace_id"] = *defaultWorkspaceID
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	user := &model.User{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(user).Error; err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

// UpdatePassword updates a user's password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}

// MarkEmailVerified marks a user's email as verified if it has not already
// been verified, which also lifts a pending signup verification.
func (r *UserRepository) MarkEmailVerified(ctx context.Context, id string, verifiedAt time.Time) (*model.User, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ? AND email_verified_at IS NULL", id).
		Updates(map[string]any{"email_verified_at": verifiedAt, "signup_verification_pending": false}).Error; err != nil {
		return nil, fmt.Errorf("mark email verified: %w", err)
	}
	return r.GetByID(ctx, id)
}

// LinkGoogleSubject stores the Google subject identifier for a user.
func (r *UserRepository) LinkGoogleSubject(ctx context.Context, id, subject string) (*model.User, error) {
	updates := map[string]interface{}{"google_subject": strings.TrimSpace(subject)}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("link google subject: %w", err)
	}
	return r.GetByID(ctx, id)
}

// UpsertTwoFactor stores the current encrypted TOTP secret and recovery codes.
func (r *UserRepository) UpsertTwoFactor(ctx context.Context, id string, secretEncrypted *string, verified bool, recoveryCodesEncrypted *string) (*model.User, error) {
	updates := map[string]interface{}{
		"totp_secret_encrypted":    secretEncrypted,
		"totp_verified":            verified,
		"recovery_codes_encrypted": recoveryCodesEncrypted,
	}

	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("upsert user 2fa: %w", err)
	}
	return r.GetByID(ctx, id)
}

// UpdateRecoveryCodes replaces the encrypted recovery codes for a user.
func (r *UserRepository) UpdateRecoveryCodes(ctx context.Context, id string, recoveryCodesEncrypted *string) (*model.User, error) {
	updates := map[string]interface{}{
		"recovery_codes_encrypted": recoveryCodesEncrypted,
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update user recovery codes: %w", err)
	}
	return r.GetByID(ctx, id)
}

// MarkTwoFactorVerified activates TOTP for the user.
func (r *UserRepository) MarkTwoFactorVerified(ctx context.Context, id string) (*model.User, error) {
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("totp_verified", true).Error; err != nil {
		return nil, fmt.Errorf("mark user 2fa verified: %w", err)
	}
	return r.GetByID(ctx, id)
}

// ClearTwoFactor removes all stored 2FA state from the user.
func (r *UserRepository) ClearTwoFactor(ctx context.Context, id string) (*model.User, error) {
	updates := map[string]interface{}{
		"totp_secret_encrypted":    nil,
		"totp_verified":            false,
		"recovery_codes_encrypted": nil,
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("clear user 2fa: %w", err)
	}
	return r.GetByID(ctx, id)
}

// WithTx runs fn in a transaction and passes a repository bound to that transaction.
func (r *UserRepository) WithTx(ctx context.Context, fn func(txRepo *UserRepository, tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewUserRepository(tx), tx)
	})
}
