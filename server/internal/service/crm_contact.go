package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// CRMContactService contains CRM contact business logic.
type CRMContactService struct {
	productAnalyticsEmitter
	contactRepo    *repository.CRMContactRepository
	activityRepo   *repository.CRMActivityRepository
	timelineRepo   *repository.CRMCompanyTimelineRepository
	entitlementSvc *EntitlementService
	wsPublisher    websocket.EventPublisher
	summaryRefresh CompanySummaryRefreshRequester
}

// SetCompanySummaryRefresh enables linked-account summary invalidation after contact changes.
func (s *CRMContactService) SetCompanySummaryRefresh(refresh CompanySummaryRefreshRequester) *CRMContactService {
	s.summaryRefresh = refresh
	return s
}

// NewCRMContactService creates a new CRMContactService.
func NewCRMContactService(contactRepo *repository.CRMContactRepository) *CRMContactService {
	return &CRMContactService{contactRepo: contactRepo}
}

// SetIdentitySync wires CRM audit activity, linked support identity refreshes,
// and realtime invalidation. The CRM contact remains the authoritative record.
func (s *CRMContactService) SetIdentitySync(
	activityRepo *repository.CRMActivityRepository,
	wsPublisher websocket.EventPublisher,
) *CRMContactService {
	s.activityRepo = activityRepo
	s.wsPublisher = wsPublisher
	return s
}

// SetTimelineRepository enables the unified contact timeline read model.
func (s *CRMContactService) SetTimelineRepository(repo *repository.CRMCompanyTimelineRepository) *CRMContactService {
	s.timelineRepo = repo
	return s
}

func (s *CRMContactService) SetEntitlementService(entitlementSvc *EntitlementService) *CRMContactService {
	s.entitlementSvc = entitlementSvc
	return s
}

// List returns contacts with filters and pagination.
func (s *CRMContactService) List(ctx context.Context, workspaceID string, filters model.CRMContactListFilters, pagination model.PMPagination) ([]model.CRMContact, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	if err := s.requireContactViewEntitlement(ctx, workspaceID); err != nil {
		return nil, 0, err
	}
	return s.contactRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a contact by ID.
func (s *CRMContactService) GetByID(ctx context.Context, id string) (*model.CRMContact, error) {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, fmt.Errorf("contact not found")
	}
	if err := s.requireContactViewEntitlement(ctx, contact.WorkspaceID); err != nil {
		return nil, err
	}
	return contact, nil
}

func (s *CRMContactService) requireContactViewEntitlement(ctx context.Context, workspaceID string) error {
	if s.entitlementSvc == nil {
		return nil
	}
	count, err := s.contactRepo.CountByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.entitlementSvc.RequireLimitUsage(ctx, workspaceID, EntitlementLimitContacts, count, 0)
}

// ListTimeline returns one cursor-paginated page of events directly related to a contact.
func (s *CRMContactService) ListTimeline(
	ctx context.Context,
	workspaceID, contactID, filter, cursor string,
	limit int,
) (*model.CRMTimelinePage, error) {
	if workspaceID == "" || contactID == "" {
		return nil, fmt.Errorf("workspace_id and contact_id are required")
	}
	if s.timelineRepo == nil {
		return nil, fmt.Errorf("contact timeline is not configured")
	}
	contact, err := s.GetByID(ctx, contactID)
	if err != nil {
		return nil, err
	}
	if contact.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("contact not found")
	}
	filter = strings.TrimSpace(filter)
	if filter == "" {
		filter = model.CRMTimelineFilterAll
	}
	if !validCRMTimelineFilter(filter) {
		return nil, fmt.Errorf("invalid timeline filter")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := model.CRMTimelineQuery{Filter: filter, Limit: limit + 1}
	if cursor != "" {
		decoded, err := decodeCRMTimelineCursor(cursor)
		if err != nil {
			return nil, err
		}
		query.CursorAt = &decoded.At
		query.CursorID = decoded.ID
	}
	items, err := s.timelineRepo.ListContact(ctx, workspaceID, contactID, query)
	if err != nil {
		return nil, err
	}
	page := &model.CRMTimelinePage{Data: items}
	if len(items) > limit {
		page.Data = items[:limit]
		last := page.Data[len(page.Data)-1]
		next, err := encodeCRMTimelineCursor(crmTimelineCursor{Version: 1, At: last.OccurredAt, ID: last.ID})
		if err != nil {
			return nil, err
		}
		page.NextCursor = &next
	}
	return page, nil
}

// Create creates a contact.
func (s *CRMContactService) Create(ctx context.Context, req model.CreateCRMContactRequest) (*model.CRMContact, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.FirstName) == "" {
		return nil, fmt.Errorf("workspace_id and first_name are required")
	}
	if s.entitlementSvc != nil {
		count, err := s.contactRepo.CountByWorkspace(ctx, req.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if err := s.entitlementSvc.RequireLimitUsage(ctx, req.WorkspaceID, EntitlementLimitContacts, count, 1); err != nil {
			return nil, err
		}
	}

	displayID, err := s.contactRepo.GetNextDisplayID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}

	lifecycleStage := model.CRMLifecycleSubscriber
	if req.LifecycleStage != nil && *req.LifecycleStage != "" {
		lifecycleStage = *req.LifecycleStage
	}
	leadStatus := model.CRMLeadStatusNew
	if req.LeadStatus != nil && *req.LeadStatus != "" {
		leadStatus = *req.LeadStatus
	}

	contact := &model.CRMContact{
		WorkspaceID:      req.WorkspaceID,
		DisplayID:        displayID,
		FirstName:        strings.TrimSpace(req.FirstName),
		LastName:         req.LastName,
		Email:            req.Email,
		Phone:            req.Phone,
		JobTitle:         req.JobTitle,
		Description:      req.Description,
		Labels:           model.StringArray(req.Labels),
		PrimaryLocation:  req.PrimaryLocation,
		CountryCode:      req.CountryCode,
		CountryName:      req.CountryName,
		LinkedInURL:      req.LinkedInURL,
		FacebookURL:      req.FacebookURL,
		InstagramURL:     req.InstagramURL,
		AngelListURL:     req.AngelListURL,
		XURL:             req.XURL,
		LifecycleStage:   lifecycleStage,
		LeadStatus:       leadStatus,
		OwnerMemberID:    req.OwnerMemberID,
		AvatarURL:        req.AvatarURL,
		Source:           req.Source,
		CustomProperties: model.JSONB(req.CustomProperties),
	}

	if err := s.contactRepo.Create(ctx, contact); err != nil {
		return nil, err
	}
	s.trackProductEvent(ctx, ProductAnalyticsEvent{
		SemanticKey: "crm_contact_created:" + contact.ID,
		WorkspaceID: contact.WorkspaceID, Name: "crm_contact_created", Source: "api",
		OccurredAt: contact.CreatedAt,
		Attributes: map[string]any{"entity_id": contact.ID, "lifecycle_stage": contact.LifecycleStage, "module": "crm"},
	})
	s.requestCompanySummaryRefresh(ctx, contact.WorkspaceID, contact.ID)
	return contact, nil
}

// Seed creates a batch of synthetic contacts for list-performance testing.
func (s *CRMContactService) Seed(ctx context.Context, req model.SeedCRMContactsRequest) (*model.SeedCRMContactsResponse, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	count := req.Count
	if count <= 0 {
		count = 500
	}
	if count > 2000 {
		return nil, fmt.Errorf("count cannot exceed 2000")
	}

	existingCount, err := s.contactRepo.CountByWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if s.entitlementSvc != nil {
		if err := s.entitlementSvc.RequireLimitUsage(ctx, req.WorkspaceID, EntitlementLimitContacts, existingCount, int64(count)); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	contacts := make([]model.CRMContact, 0, count)
	for i := 0; i < count; i++ {
		sequence := int(existingCount) + i + 1
		firstName, lastName := seededContactName(sequence)
		email := fmt.Sprintf("seed-contact-%d@helpin.test", sequence)
		phone := fmt.Sprintf("+1-555-%04d", sequence%10000)
		jobTitle := seededJobTitle(i)
		source := "seed"
		createdAt := now.Add(-time.Duration(i) * time.Minute)

		contacts = append(contacts, model.CRMContact{
			WorkspaceID:      req.WorkspaceID,
			DisplayID:        fmt.Sprintf("CON-%d", sequence),
			FirstName:        firstName,
			LastName:         &lastName,
			Email:            &email,
			Phone:            &phone,
			JobTitle:         &jobTitle,
			LifecycleStage:   seededLifecycleStage(i),
			LeadStatus:       seededLeadStatus(i),
			Source:           &source,
			CustomProperties: model.JSONB{"seeded": true},
			CreatedAt:        createdAt,
			UpdatedAt:        createdAt,
		})
	}

	if err := s.contactRepo.CreateInBatches(ctx, contacts, 100); err != nil {
		return nil, err
	}

	return &model.SeedCRMContactsResponse{Created: len(contacts)}, nil
}

// Update updates a contact without a human actor, such as from an internal automation.
func (s *CRMContactService) Update(
	ctx context.Context,
	id string,
	req model.UpdateCRMContactRequest,
) (*model.CRMContact, error) {
	return s.UpdateWithActor(ctx, id, req, "", "")
}

// UpdateWithActor updates a contact and records the human actor for audit activity.
func (s *CRMContactService) UpdateWithActor(
	ctx context.Context,
	id string,
	req model.UpdateCRMContactRequest,
	actorUserID, actorMemberID string,
) (*model.CRMContact, error) {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, fmt.Errorf("contact not found")
	}

	oldEmail := normalizedOptionalString(contact.Email)
	if req.FirstName != nil {
		name := strings.TrimSpace(*req.FirstName)
		if name == "" {
			return nil, fmt.Errorf("first_name cannot be empty")
		}
		contact.FirstName = name
	}
	if req.LastName != nil {
		contact.LastName = req.LastName
	}
	if req.Email != nil {
		contact.Email = req.Email
	}
	if req.Phone != nil {
		contact.Phone = req.Phone
	}
	if req.JobTitle != nil {
		contact.JobTitle = req.JobTitle
	}
	if req.Description != nil {
		contact.Description = req.Description
	}
	if req.Labels != nil {
		contact.Labels = model.StringArray(req.Labels)
	}
	if req.PrimaryLocation != nil {
		contact.PrimaryLocation = req.PrimaryLocation
	}
	if req.CountryCode != nil {
		contact.CountryCode = req.CountryCode
	}
	if req.CountryName != nil {
		contact.CountryName = req.CountryName
	}
	if req.LinkedInURL != nil {
		contact.LinkedInURL = req.LinkedInURL
	}
	if req.FacebookURL != nil {
		contact.FacebookURL = req.FacebookURL
	}
	if req.InstagramURL != nil {
		contact.InstagramURL = req.InstagramURL
	}
	if req.AngelListURL != nil {
		contact.AngelListURL = req.AngelListURL
	}
	if req.XURL != nil {
		contact.XURL = req.XURL
	}
	if req.LifecycleStage != nil {
		contact.LifecycleStage = *req.LifecycleStage
	}
	if req.LeadStatus != nil {
		contact.LeadStatus = *req.LeadStatus
	}
	if req.ClearOwner {
		contact.OwnerMemberID = nil
	} else if req.OwnerMemberID != nil {
		contact.OwnerMemberID = req.OwnerMemberID
	}
	if req.AvatarURL != nil {
		contact.AvatarURL = req.AvatarURL
	}
	if req.Source != nil {
		contact.Source = req.Source
	}
	if req.CustomProperties != nil {
		contact.CustomProperties = model.JSONB(req.CustomProperties)
	}

	newEmail := normalizedOptionalString(contact.Email)

	update := func(
		contactRepo *repository.CRMContactRepository,
		activityRepo *repository.CRMActivityRepository,
	) error {
		if err := contactRepo.Update(ctx, contact); err != nil {
			return err
		}
		if activityRepo != nil && oldEmail != newEmail {
			activity := emailChangedActivity(contact, oldEmail, newEmail, actorUserID, actorMemberID)
			if err := activityRepo.Create(ctx, activity); err != nil {
				return err
			}
		}
		return nil
	}

	if s.activityRepo != nil {
		err = s.contactRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return update(
				s.contactRepo.WithTx(tx),
				s.activityRepo.WithTx(tx),
			)
		})
	} else {
		err = update(s.contactRepo, nil)
	}
	if err != nil {
		return nil, err
	}

	if s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "crm_contact",
			EntityID:    contact.ID,
			WorkspaceID: contact.WorkspaceID,
			ActorID:     actorUserID,
		})
	}
	s.requestCompanySummaryRefresh(ctx, contact.WorkspaceID, contact.ID)
	return contact, nil
}

func normalizedOptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func emailChangedActivity(
	contact *model.CRMContact,
	oldEmail, newEmail, actorUserID, actorMemberID string,
) *model.CRMActivity {
	subject := "Email changed"
	body := fmt.Sprintf("%s → %s", displayActivityEmail(oldEmail), displayActivityEmail(newEmail))
	var ownerMemberID *string
	if strings.TrimSpace(actorMemberID) != "" {
		ownerMemberID = &actorMemberID
	}
	return &model.CRMActivity{
		WorkspaceID:   contact.WorkspaceID,
		ActivityType:  model.CRMActivityNote,
		ContactID:     &contact.ID,
		OwnerMemberID: ownerMemberID,
		Subject:       &subject,
		Body:          &body,
		OccurredAt:    time.Now().UTC(),
		Metadata: model.JSONB{
			"event_type":    "contact_email_changed",
			"old_email":     oldEmail,
			"new_email":     newEmail,
			"actor_user_id": actorUserID,
			"immutable":     true,
		},
	}
}

func displayActivityEmail(email string) string {
	if strings.TrimSpace(email) == "" {
		return "No email"
	}
	return email
}

// Delete removes a contact.
func (s *CRMContactService) Delete(ctx context.Context, id string) error {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if contact == nil {
		return fmt.Errorf("contact not found")
	}
	s.requestCompanySummaryRefresh(ctx, contact.WorkspaceID, contact.ID)
	return s.contactRepo.Delete(ctx, id)
}

func (s *CRMContactService) requestCompanySummaryRefresh(ctx context.Context, workspaceID, contactID string) {
	if s == nil || s.summaryRefresh == nil {
		return
	}
	if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, workspaceID, model.CRMObjectContact, contactID); err != nil {
		slog.ErrorContext(ctx, "failed to request company summary refresh from contact", "error", err, "workspace_id", workspaceID, "contact_id", contactID)
	}
}

func seededContactName(sequence int) (string, string) {
	firstNames := []string{"Avery", "Jordan", "Taylor", "Parker", "Morgan", "Riley", "Casey", "Drew"}
	lastNames := []string{"Stone", "Bennett", "Coleman", "Reed", "Foster", "Hayes", "Morris", "Brooks"}
	first := firstNames[(sequence-1)%len(firstNames)]
	last := fmt.Sprintf("%s %d", lastNames[(sequence-1)%len(lastNames)], sequence)
	return first, last
}

func seededJobTitle(index int) string {
	jobTitles := []string{
		"Revenue Operations Manager",
		"Growth Lead",
		"Customer Success Director",
		"VP Sales",
		"Marketing Operations Manager",
		"Partnerships Manager",
	}
	return jobTitles[index%len(jobTitles)]
}

func seededLifecycleStage(index int) string {
	stages := []string{
		model.CRMLifecycleSubscriber,
		model.CRMLifecycleLead,
		model.CRMLifecycleMarketingQualified,
		model.CRMLifecycleSalesQualified,
		model.CRMLifecycleOpportunity,
		model.CRMLifecycleCustomer,
	}
	return stages[index%len(stages)]
}

func seededLeadStatus(index int) string {
	statuses := []string{
		model.CRMLeadStatusNew,
		model.CRMLeadStatusOpen,
		model.CRMLeadStatusInProgress,
		model.CRMLeadStatusUnqualified,
	}
	return statuses[index%len(statuses)]
}
