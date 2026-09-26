package main

import (
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// newAppEmailConfig returns the application email source that lets server
// admins save SMTP settings in the app, or nil when server administration is
// disabled (Enterprise). envSender, built from SMTP_* or POSTMARK_*, takes
// precedence over saved settings.
func newAppEmailConfig(db *gorm.DB, cfg *config.Config, envSender email.AppSender) *service.AppEmailConfigService {
	if !serverAdminEnabled {
		return nil
	}
	return service.NewAppEmailConfigService(repository.NewInstanceSettingsRepository(db), service.AppEmailConfigOptions{
		EnvSender: envSender,
		EnvSMTP: email.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.SMTPFrom, TLSMode: cfg.SMTPTLSMode,
		},
		EncryptionKey: resolveInstanceSettingsEncryptionKey(cfg),
	})
}

// newInstanceService returns server administration, or nil when it is
// disabled (Enterprise).
func newInstanceService(db *gorm.DB, cfg *config.Config, mailReady func() bool) *service.InstanceService {
	if !serverAdminEnabled {
		return nil
	}
	return service.NewInstanceService(repository.NewInstanceSettingsRepository(db), repository.NewUserRepository(db), service.InstanceServiceOptions{
		AdminEmails: cfg.ServerAdminEmails,
		MailReady:   mailReady,
	})
}

// resolveInstanceSettingsEncryptionKey returns the key that protects the
// SMTP password saved in the app: CRM_ENCRYPTION_KEY, which every Community
// install generates and which already protects other stored credentials.
func resolveInstanceSettingsEncryptionKey(cfg *config.Config) []byte {
	key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CRMEncryptionKey))
	if err != nil {
		slog.Warn("invalid CRM_ENCRYPTION_KEY; SMTP passwords cannot be saved in the app", "error", err)
		return nil
	}
	return key
}
