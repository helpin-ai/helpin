package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// InstanceSettingsRepository stores the self-hosted server's settings row and
// its server admin flags.
type InstanceSettingsRepository struct {
	db *gorm.DB
}

// NewInstanceSettingsRepository returns a repository backed by db.
func NewInstanceSettingsRepository(db *gorm.DB) *InstanceSettingsRepository {
	return &InstanceSettingsRepository{db: db}
}

// WithTx runs fn in a transaction with a repository bound to it.
func (r *InstanceSettingsRepository) WithTx(ctx context.Context, fn func(txRepo *InstanceSettingsRepository, tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewInstanceSettingsRepository(tx), tx)
	})
}

// Get returns the settings row, or nil when it does not exist yet.
func (r *InstanceSettingsRepository) Get(ctx context.Context) (*model.InstanceSettings, error) {
	var settings model.InstanceSettings
	err := r.db.WithContext(ctx).Where("singleton = ?", true).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get instance settings: %w", err)
	}
	return &settings, nil
}

// Ensure returns the settings row, creating it when missing (for example on a
// database built by AutoMigrate). A new row follows the migration's rule: a
// server that already has accounts keeps open signup and has spent its
// first-account claim; an empty server starts invite-only.
func (r *InstanceSettingsRepository) Ensure(ctx context.Context) (*model.InstanceSettings, error) {
	settings, err := r.Get(ctx)
	if err != nil || settings != nil {
		return settings, err
	}
	hasUsers, err := r.HasUsers(ctx)
	if err != nil {
		return nil, err
	}
	row := model.InstanceSettings{Singleton: true, SignupMode: model.SignupModeInviteOnly}
	if hasUsers {
		now := time.Now().UTC()
		row.SignupMode = model.SignupModeOpen
		row.AdminBootstrappedAt = &now
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create instance settings: %w", err)
	}
	return r.Get(ctx)
}

// ClaimFirstAdmin atomically spends the first-account claim. It returns true
// for exactly one caller: the conditional UPDATE serializes concurrent
// signups on the settings row, and the claim rolls back with a failed signup.
func (r *InstanceSettingsRepository) ClaimFirstAdmin(ctx context.Context, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.InstanceSettings{}).
		Where("singleton = ? AND admin_bootstrapped_at IS NULL", true).
		Updates(map[string]any{"admin_bootstrapped_at": now, "updated_at": now})
	if result.Error != nil {
		return false, fmt.Errorf("claim first server admin: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// MarkBootstrapped spends the first-account claim without granting it, for
// servers that already have accounts.
func (r *InstanceSettingsRepository) MarkBootstrapped(ctx context.Context, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&model.InstanceSettings{}).
		Where("singleton = ? AND admin_bootstrapped_at IS NULL", true).
		Update("admin_bootstrapped_at", now).Error
	if err != nil {
		return fmt.Errorf("mark instance bootstrapped: %w", err)
	}
	return nil
}

// LockSettings takes a row lock on the settings row for the rest of the
// transaction, serializing admin changes. SQLite ignores the lock clause and
// serializes writers on its own.
func (r *InstanceSettingsRepository) LockSettings(ctx context.Context) error {
	var settings model.InstanceSettings
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("singleton = ?", true).First(&settings).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("lock instance settings: %w", err)
	}
	return nil
}

// UpdateSignupPolicy stores the signup mode and allowed domains.
func (r *InstanceSettingsRepository) UpdateSignupPolicy(ctx context.Context, mode string, domains []string, updatedBy string) error {
	return r.update(ctx, map[string]any{
		"signup_mode":            mode,
		"signup_allowed_domains": strings.Join(domains, ","),
		"updated_by":             nullableUUID(updatedBy),
	})
}

// SMTPSettings are the stored application SMTP fields.
type SMTPSettings struct {
	Host, Username, From, TLSMode string
	Port                          int
	PasswordEncrypted             *string
}

// SaveSMTP stores application SMTP settings.
func (r *InstanceSettingsRepository) SaveSMTP(ctx context.Context, smtp SMTPSettings, updatedBy string, now time.Time) error {
	return r.update(ctx, map[string]any{
		"smtp_host":               smtp.Host,
		"smtp_port":               smtp.Port,
		"smtp_username":           smtp.Username,
		"smtp_password_encrypted": smtp.PasswordEncrypted,
		"smtp_from":               smtp.From,
		"smtp_tls_mode":           smtp.TLSMode,
		"smtp_updated_at":         now,
		"updated_by":              nullableUUID(updatedBy),
	})
}

// ClearSMTP removes the stored application SMTP settings.
func (r *InstanceSettingsRepository) ClearSMTP(ctx context.Context, updatedBy string, now time.Time) error {
	return r.update(ctx, map[string]any{
		"smtp_host":               nil,
		"smtp_port":               nil,
		"smtp_username":           nil,
		"smtp_password_encrypted": nil,
		"smtp_from":               nil,
		"smtp_tls_mode":           nil,
		"smtp_updated_at":         now,
		"updated_by":              nullableUUID(updatedBy),
	})
}

func (r *InstanceSettingsRepository) update(ctx context.Context, updates map[string]any) error {
	if _, err := r.Ensure(ctx); err != nil {
		return err
	}
	updates["updated_at"] = time.Now().UTC()
	err := r.db.WithContext(ctx).Model(&model.InstanceSettings{}).Where("singleton = ?", true).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("update instance settings: %w", err)
	}
	return nil
}

// HasUsers reports whether any account exists.
func (r *InstanceSettingsRepository) HasUsers(ctx context.Context) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Limit(1).Count(&count).Error; err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}

// ListServerAdmins returns the server admins, oldest account first.
func (r *InstanceSettingsRepository) ListServerAdmins(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).Where("is_server_admin = ?", true).Order("created_at ASC, id ASC").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("list server admins: %w", err)
	}
	return users, nil
}

// CountServerAdmins returns how many accounts are server admins.
func (r *InstanceSettingsRepository) CountServerAdmins(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("is_server_admin = ?", true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count server admins: %w", err)
	}
	return count, nil
}

// SetServerAdmin grants or revokes server admin for one account.
func (r *InstanceSettingsRepository) SetServerAdmin(ctx context.Context, userID string, admin bool) error {
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("is_server_admin", admin).Error
	if err != nil {
		return fmt.Errorf("set server admin: %w", err)
	}
	return nil
}

// GrantServerAdminByEmails marks existing accounts with the given (already
// normalized) emails as server admins.
func (r *InstanceSettingsRepository) GrantServerAdminByEmails(ctx context.Context, emails []string) (int64, error) {
	if len(emails) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&model.User{}).
		Where("lower(email) IN ? AND is_server_admin = ?", emails, false).
		Update("is_server_admin", true)
	if result.Error != nil {
		return 0, fmt.Errorf("grant server admin by email: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// UpgradeAdminCandidate returns the account that should administer a server
// that has accounts but no server admin: the owner of the oldest
// organization (the person who set the server up), otherwise the oldest
// account. It returns "" when there are no accounts.
func (r *InstanceSettingsRepository) UpgradeAdminCandidate(ctx context.Context) (string, error) {
	var ownerIDs []string
	err := r.db.WithContext(ctx).Table("organizations").
		Joins("JOIN users ON users.id = organizations.owner_id").
		Order("organizations.created_at ASC, organizations.id ASC").
		Limit(1).Pluck("organizations.owner_id", &ownerIDs).Error
	if err != nil {
		return "", fmt.Errorf("find oldest organization owner: %w", err)
	}
	if len(ownerIDs) > 0 && ownerIDs[0] != "" {
		return ownerIDs[0], nil
	}
	var userIDs []string
	if err := r.db.WithContext(ctx).Model(&model.User{}).Order("created_at ASC, id ASC").Limit(1).Pluck("id", &userIDs).Error; err != nil {
		return "", fmt.Errorf("find oldest account: %w", err)
	}
	if len(userIDs) == 0 {
		return "", nil
	}
	return userIDs[0], nil
}

func nullableUUID(id string) any {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return id
}
