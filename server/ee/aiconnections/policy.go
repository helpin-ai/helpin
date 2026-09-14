//go:build ee

// Package aiconnections supplies SaaS availability and the flat BYOK tariff.
package aiconnections

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

var ErrBYOKUnavailable = errors.New("BYOK is not enabled for this workspace or its token tariff is not configured")

type Tariff struct {
	Version            string `gorm:"primaryKey"`
	Currency           string
	MicrousdPerMillion int64
	AccountingVersion  string
}

func (Tariff) TableName() string { return "ai_byok_tariffs" }

type WorkspaceSettings struct {
	WorkspaceID   string `gorm:"primaryKey;type:uuid"`
	BYOKEnabled   bool   `gorm:"column:byok_enabled"`
	TariffVersion *string
}

func (WorkspaceSettings) TableName() string { return "workspace_ai_billing_settings" }

type Policy struct{ db *gorm.DB }

func NewPolicy(db *gorm.DB) *Policy { return &Policy{db: db} }

// ResolveConnectionPolicy runs before credential refresh or fallback. It never
// performs a provider probe, reserves money, or edits the workspace's flag.
func (p *Policy) ResolveConnectionPolicy(ctx context.Context, workspace string, connection *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
	if connection == nil || connection.WorkspaceID != workspace {
		return nil, ErrBYOKUnavailable
	}
	if connection.Funding == "managed" {
		if connection.Scope != "workspace" || connection.UserID != nil || connection.Provider == "openai_chatgpt" {
			return nil, ErrBYOKUnavailable
		}
		return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingHelpinHosted}, nil
	}
	if connection.Funding != "customer" || p == nil || p.db == nil {
		return nil, ErrBYOKUnavailable
	}
	var tariff Tariff
	err := p.db.WithContext(ctx).Table("ai_byok_tariffs AS tariff").Select("tariff.*").
		Joins("JOIN workspace_ai_billing_settings AS settings ON settings.tariff_version = tariff.version").
		Where("settings.workspace_id = ? AND settings.byok_enabled = ?", workspace, true).Take(&tariff).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBYOKUnavailable
	}
	if err != nil {
		return nil, err
	}
	flat := &aiusage.FlatTokenTariff{Version: tariff.Version, Currency: tariff.Currency,
		MicrousdPerMillion: &tariff.MicrousdPerMillion, AccountingVersion: tariff.AccountingVersion}
	if err := flat.Validate(); err != nil {
		return nil, ErrBYOKUnavailable
	}
	return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: flat}, nil
}
