package service

import "context"

// CompanySummaryRefreshRequester is implemented by CRMSummaryService. Source
// services depend on this narrow contract so summary refresh remains best
// effort and cannot block their primary mutations.
type CompanySummaryRefreshRequester interface {
	RequestCompanyRefreshForObject(ctx context.Context, workspaceID, objectType, objectID string) error
}
