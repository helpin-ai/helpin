package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMActivityService contains CRM activity business logic.
type CRMActivityService struct {
	activityRepo   *repository.CRMActivityRepository
	summaryRefresh CompanySummaryRefreshRequester
	signalStarter  interface {
		StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error
	}
	signalRepo *repository.CRMSignalRepository
}

// SetCompanySummaryRefresh enables account-summary invalidation after activity changes.
func (s *CRMActivityService) SetCompanySummaryRefresh(refresh CompanySummaryRefreshRequester) *CRMActivityService {
	s.summaryRefresh = refresh
	return s
}

// SetSignalDetection enables asynchronous evidence-backed signal analysis for
// user-authored notes, calls, and meeting notes.
func (s *CRMActivityService) SetSignalDetection(starter interface {
	StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error
}, signalRepo *repository.CRMSignalRepository) *CRMActivityService {
	s.signalStarter = starter
	s.signalRepo = signalRepo
	return s
}

// NewCRMActivityService creates a new CRMActivityService.
func NewCRMActivityService(activityRepo *repository.CRMActivityRepository) *CRMActivityService {
	return &CRMActivityService{activityRepo: activityRepo}
}

// List returns activities with filters and pagination.
func (s *CRMActivityService) List(ctx context.Context, workspaceID string, filters model.CRMActivityListFilters, pagination model.PMPagination) ([]model.CRMActivity, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.activityRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns an activity by ID.
func (s *CRMActivityService) GetByID(ctx context.Context, id string) (*model.CRMActivity, error) {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, fmt.Errorf("activity not found")
	}
	return activity, nil
}

// Create creates an activity.
func (s *CRMActivityService) Create(ctx context.Context, req model.CreateCRMActivityRequest) (*model.CRMActivity, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !isValidActivityType(req.ActivityType) {
		return nil, fmt.Errorf("invalid activity_type")
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	activity := &model.CRMActivity{
		WorkspaceID:   req.WorkspaceID,
		ActivityType:  req.ActivityType,
		ContactID:     req.ContactID,
		CompanyID:     req.CompanyID,
		DealID:        req.DealID,
		OwnerMemberID: req.OwnerMemberID,
		Subject:       req.Subject,
		Body:          req.Body,
		OccurredAt:    occurredAt,
		Metadata:      model.JSONB(req.Metadata),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		return nil, err
	}
	s.requestCompanySummaryRefresh(ctx, activity)
	s.enqueueSignalDetection(ctx, activity)
	return activity, nil
}

// Update updates an activity.
func (s *CRMActivityService) Update(ctx context.Context, id string, req model.UpdateCRMActivityRequest) (*model.CRMActivity, error) {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, fmt.Errorf("activity not found")
	}
	if activity.Metadata != nil && activity.Metadata["immutable"] == true {
		return nil, fmt.Errorf("system activity is immutable")
	}
	previousSourceType := crmActivitySignalSourceType(activity.ActivityType)

	if req.ActivityType != nil {
		if !isValidActivityType(*req.ActivityType) {
			return nil, fmt.Errorf("invalid activity_type")
		}
		activity.ActivityType = *req.ActivityType
	}
	if req.ContactID != nil {
		activity.ContactID = req.ContactID
	}
	if req.CompanyID != nil {
		activity.CompanyID = req.CompanyID
	}
	if req.DealID != nil {
		activity.DealID = req.DealID
	}
	if req.Subject != nil {
		activity.Subject = req.Subject
	}
	if req.Body != nil {
		activity.Body = req.Body
	}
	if req.OccurredAt != nil {
		activity.OccurredAt = *req.OccurredAt
	}
	if req.Metadata != nil {
		activity.Metadata = model.JSONB(req.Metadata)
	}

	if err := s.activityRepo.Update(ctx, activity); err != nil {
		return nil, err
	}
	if currentSourceType := crmActivitySignalSourceType(activity.ActivityType); s.signalRepo != nil && previousSourceType != "" && previousSourceType != currentSourceType {
		if err := s.signalRepo.ReconcileAutomatedSignalsForSource(ctx, activity.WorkspaceID, previousSourceType, activity.ID, nil); err != nil {
			slog.WarnContext(ctx, "updated CRM activity retained buyer signals from its previous type", "error", err, "activity_id", activity.ID)
		}
	}
	s.requestCompanySummaryRefresh(ctx, activity)
	s.enqueueSignalDetection(ctx, activity)
	return activity, nil
}

// Delete removes an activity.
func (s *CRMActivityService) Delete(ctx context.Context, id string) error {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if activity == nil {
		return fmt.Errorf("activity not found")
	}
	if err := s.activityRepo.Delete(ctx, id); err != nil {
		return err
	}
	if s.signalRepo != nil {
		if sourceType := crmActivitySignalSourceType(activity.ActivityType); sourceType != "" {
			if err := s.signalRepo.ReconcileAutomatedSignalsForSource(ctx, activity.WorkspaceID, sourceType, activity.ID, nil); err != nil {
				slog.WarnContext(ctx, "deleted CRM activity retained stale buyer signals", "error", err, "activity_id", activity.ID)
			}
		}
	}
	s.requestCompanySummaryRefresh(ctx, activity)
	return nil
}

func (s *CRMActivityService) enqueueSignalDetection(ctx context.Context, activity *model.CRMActivity) {
	if s == nil || s.signalStarter == nil || activity == nil {
		return
	}
	sourceType := crmActivitySignalSourceType(activity.ActivityType)
	body := strings.TrimSpace(derefString(activity.Body))
	if sourceType == "" {
		return
	}
	if body == "" {
		if s.signalRepo != nil {
			if err := s.signalRepo.ReconcileAutomatedSignalsForSource(ctx, activity.WorkspaceID, sourceType, activity.ID, nil); err != nil {
				slog.WarnContext(ctx, "empty CRM activity retained stale buyer signals", "error", err, "activity_id", activity.ID)
			}
		}
		return
	}
	versionAt := activity.UpdatedAt
	if versionAt.IsZero() {
		versionAt = time.Now().UTC()
	}
	payload := model.SignalSourcePayload{
		SourceType: sourceType, SourceID: activity.ID, WorkspaceID: activity.WorkspaceID,
		ContactID: activity.ContactID, DealID: activity.DealID, CompanyID: activity.CompanyID,
		Subject: strings.TrimSpace(derefString(activity.Subject)), Body: body,
		Direction: "bilateral", OccurredAt: activity.OccurredAt,
	}
	key := fmt.Sprintf("activity-%s-%d", activity.ID, versionAt.UnixNano())
	if err := s.signalStarter.StartSignalDetection(ctx, key, []model.SignalSourcePayload{payload}); err != nil {
		slog.WarnContext(ctx, "CRM activity buyer signal enqueue failed", "error", err, "activity_id", activity.ID)
	}
}

func (s *CRMActivityService) requestCompanySummaryRefresh(ctx context.Context, activity *model.CRMActivity) {
	if s == nil || s.summaryRefresh == nil || activity == nil {
		return
	}
	objects := []struct{ objectType, objectID string }{}
	if activity.CompanyID != nil {
		objects = append(objects, struct{ objectType, objectID string }{model.CRMObjectCompany, *activity.CompanyID})
	}
	if activity.ContactID != nil {
		objects = append(objects, struct{ objectType, objectID string }{model.CRMObjectContact, *activity.ContactID})
	}
	if activity.DealID != nil {
		objects = append(objects, struct{ objectType, objectID string }{model.CRMObjectDeal, *activity.DealID})
	}
	for _, object := range objects {
		if object.objectID == "" {
			continue
		}
		if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, activity.WorkspaceID, object.objectType, object.objectID); err != nil {
			slog.ErrorContext(ctx, "failed to request company summary refresh from crm activity", "error", err, "activity_id", activity.ID, "object_type", object.objectType, "object_id", object.objectID)
		}
	}
}

func isValidActivityType(t string) bool {
	switch t {
	case model.CRMActivityNote, model.CRMActivityCall, model.CRMActivityMeeting, model.CRMActivityEmail:
		return true
	default:
		return false
	}
}
