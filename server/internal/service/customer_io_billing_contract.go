package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CustomerIOBillingInsights is optional edition metadata. Identity syncing still
// works when community supplies no financial projection.
type CustomerIOBillingInsights interface {
	WorkspaceSummary(context.Context, string) (*model.BillingSummary, error)
	OrganizationSummary(context.Context, string) (model.CustomerIOOrganizationSummary, error)
}
