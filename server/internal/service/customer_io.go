package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const defaultCustomerIOWorkspaceObjectTypeID = "1"
const defaultCustomerIOOrganizationObjectTypeID = "2"

type CustomerIOTrackConfig struct {
	SiteID                   string
	APIKey                   string
	Region                   string
	WorkspaceObjectTypeID    string
	OrganizationObjectTypeID string
	Endpoint                 string
	HTTPClient               *http.Client
}

type CustomerIOTrackClient struct {
	siteID                   string
	apiKey                   string
	workspaceObjectTypeID    string
	organizationObjectTypeID string
	endpoint                 string
	httpClient               *http.Client
}

// CustomerIODeliveryError describes a failed Customer.io Track API delivery.
// A zero StatusCode identifies a transport failure; Err remains available to
// errors.Is and errors.As through Unwrap.
type CustomerIODeliveryError struct {
	StatusCode int
	RetryAfter time.Duration
	Body       string
	Err        error
}

func (e *CustomerIODeliveryError) Error() string {
	if e == nil {
		return "customer.io delivery error"
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("post customer.io entity: unexpected status %d", e.StatusCode)
	}
	return fmt.Sprintf("post customer.io entity: %v", e.Err)
}

// Unwrap returns the underlying transport error, when present.
func (e *CustomerIODeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type CustomerIOWorkspaceIdentity struct {
	Workspace          *model.Workspace
	Billing            *BillingSummary
	RelationshipUserID string
	RelationshipRole   string
	RelationshipStatus string
	TeamMemberships    int
}

type CustomerIOOrganizationIdentity struct {
	Organization       *model.Organization
	RelationshipUserID string
	RelationshipRole   string
	MemberCount        int
	WorkspaceCount     int
	TrialingWorkspaces int
	ActiveWorkspaces   int
	LockedWorkspaces   int
	HighestPlan        string
	HasTrialWorkspace  bool
	HasPaidWorkspace   bool
	PaidWorkspaceCount int
	MonthlyDueCents    int
}

type CustomerIOEvent struct {
	UserID     string
	EventID    string
	Name       string
	OccurredAt time.Time
	Attributes map[string]any
}

type customerIOEntityPayload struct {
	Type             string                       `json:"type"`
	Identifiers      map[string]string            `json:"identifiers"`
	Action           string                       `json:"action"`
	ID               string                       `json:"id,omitempty"`
	Name             string                       `json:"name,omitempty"`
	Timestamp        int64                        `json:"timestamp,omitempty"`
	Attributes       map[string]any               `json:"attributes,omitempty"`
	CIORelationships []customerIORelationshipBody `json:"cio_relationships,omitempty"`
}

type customerIORelationshipBody struct {
	Identifiers            map[string]string `json:"identifiers"`
	RelationshipAttributes map[string]any    `json:"relationship_attributes,omitempty"`
}

func NewCustomerIOTrackClient(cfg CustomerIOTrackConfig) *CustomerIOTrackClient {
	siteID := strings.TrimSpace(cfg.SiteID)
	apiKey := strings.TrimSpace(cfg.APIKey)
	objectTypeID := strings.TrimSpace(cfg.WorkspaceObjectTypeID)
	if objectTypeID == "" {
		objectTypeID = defaultCustomerIOWorkspaceObjectTypeID
	}
	orgObjectTypeID := strings.TrimSpace(cfg.OrganizationObjectTypeID)
	if orgObjectTypeID == "" {
		orgObjectTypeID = defaultCustomerIOOrganizationObjectTypeID
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &CustomerIOTrackClient{
		siteID:                   siteID,
		apiKey:                   apiKey,
		workspaceObjectTypeID:    objectTypeID,
		organizationObjectTypeID: orgObjectTypeID,
		endpoint:                 customerIOEndpoint(cfg.Region, cfg.Endpoint),
		httpClient:               httpClient,
	}
}

func (c *CustomerIOTrackClient) Enabled() bool {
	return c != nil && c.siteID != "" && c.apiKey != ""
}

func (c *CustomerIOTrackClient) IdentifyUser(ctx context.Context, user *model.User) error {
	if !c.Enabled() || user == nil || strings.TrimSpace(user.ID) == "" {
		return nil
	}
	payload := customerIOEntityPayload{
		Type:        "person",
		Identifiers: map[string]string{"id": user.ID},
		Action:      "identify",
		Attributes:  customerIOUserAttributes(user),
	}
	return c.postEntity(ctx, payload)
}

func (c *CustomerIOTrackClient) TrackEvent(ctx context.Context, event CustomerIOEvent) error {
	if !c.Enabled() || strings.TrimSpace(event.UserID) == "" || strings.TrimSpace(event.Name) == "" {
		return nil
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := customerIOEntityPayload{
		Type:        "person",
		Identifiers: map[string]string{"id": event.UserID},
		Action:      "event",
		ID:          strings.TrimSpace(event.EventID),
		Name:        strings.TrimSpace(event.Name),
		Timestamp:   occurredAt.UTC().Unix(),
		Attributes:  customerIOEventAttributes(event.Attributes),
	}
	return c.postEntity(ctx, payload)
}

func (c *CustomerIOTrackClient) DeletePersonRelationship(ctx context.Context, userID, objectTypeID, objectID string) error {
	if !c.Enabled() || strings.TrimSpace(userID) == "" || strings.TrimSpace(objectTypeID) == "" || strings.TrimSpace(objectID) == "" {
		return nil
	}
	payload := customerIOEntityPayload{
		Type:        "person",
		Identifiers: map[string]string{"id": userID},
		Action:      "delete_relationships",
		CIORelationships: []customerIORelationshipBody{{
			Identifiers: map[string]string{
				"object_type_id": objectTypeID,
				"object_id":      objectID,
			},
		}},
	}
	return c.postEntity(ctx, payload)
}

func (c *CustomerIOTrackClient) IdentifyWorkspace(ctx context.Context, input CustomerIOWorkspaceIdentity) error {
	if !c.Enabled() || input.Workspace == nil || strings.TrimSpace(input.Workspace.ID) == "" {
		return nil
	}
	payload := customerIOEntityPayload{
		Type: "object",
		Identifiers: map[string]string{
			"object_type_id": c.workspaceObjectTypeID,
			"object_id":      input.Workspace.ID,
		},
		Action:     "identify",
		Attributes: customerIOWorkspaceAttributes(input),
	}
	relationshipUserID := strings.TrimSpace(input.RelationshipUserID)
	if relationshipUserID == "" {
		relationshipUserID = strings.TrimSpace(input.Workspace.OwnerID)
	}
	if relationshipUserID != "" {
		payload.CIORelationships = []customerIORelationshipBody{{
			Identifiers: map[string]string{"id": relationshipUserID},
			RelationshipAttributes: compactAttributes(map[string]any{
				"workspace_role":         customerIOFirstNonBlank(input.RelationshipRole, "owner"),
				"membership_status":      customerIOFirstNonBlank(input.RelationshipStatus, model.WorkspaceMemberStatusActive),
				"team_membership_count":  input.TeamMemberships,
				"relationship_source":    "helpin_backend",
				"relationship_synced_at": time.Now().UTC(),
			}),
		}}
	}
	return c.postEntity(ctx, payload)
}

func (c *CustomerIOTrackClient) IdentifyOrganization(ctx context.Context, input CustomerIOOrganizationIdentity) error {
	if !c.Enabled() || input.Organization == nil || strings.TrimSpace(input.Organization.ID) == "" {
		return nil
	}
	payload := customerIOEntityPayload{
		Type: "object",
		Identifiers: map[string]string{
			"object_type_id": c.organizationObjectTypeID,
			"object_id":      input.Organization.ID,
		},
		Action:     "identify",
		Attributes: customerIOOrganizationAttributes(input),
	}
	relationshipUserID := strings.TrimSpace(input.RelationshipUserID)
	if relationshipUserID == "" {
		relationshipUserID = strings.TrimSpace(input.Organization.OwnerID)
	}
	if relationshipUserID != "" {
		payload.CIORelationships = []customerIORelationshipBody{{
			Identifiers: map[string]string{"id": relationshipUserID},
			RelationshipAttributes: compactAttributes(map[string]any{
				"organization_role":      customerIOFirstNonBlank(input.RelationshipRole, "owner"),
				"relationship_source":    "helpin_backend",
				"relationship_synced_at": time.Now().UTC(),
			}),
		}}
	}
	return c.postEntity(ctx, payload)
}

func (c *CustomerIOTrackClient) postEntity(ctx context.Context, payload customerIOEntityPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal customer.io entity payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.endpoint, "/")+"/api/v2/entity", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create customer.io request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.siteID+":"+c.apiKey)))

	res, err := c.httpClient.Do(req)
	if err != nil {
		return &CustomerIODeliveryError{Err: err}
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, 4096))
	closeErr := res.Body.Close()
	if readErr != nil {
		return &CustomerIODeliveryError{StatusCode: res.StatusCode, Err: fmt.Errorf("read response body: %w", readErr)}
	}
	if closeErr != nil {
		return &CustomerIODeliveryError{StatusCode: res.StatusCode, Err: fmt.Errorf("close response body: %w", closeErr)}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &CustomerIODeliveryError{
			StatusCode: res.StatusCode,
			RetryAfter: customerIORetryAfter(res.Header.Get("Retry-After"), time.Now()),
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return nil
}

func customerIORetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	retryAt, err := http.ParseTime(value)
	if err != nil || !retryAt.After(now) {
		return 0
	}
	return retryAt.Sub(now)
}

func customerIOEventAttributes(attributes map[string]any) map[string]any {
	attrs := make(map[string]any, len(attributes))
	for key, value := range attributes {
		attrs[key] = value
	}
	delete(attrs, "recipient")
	delete(attrs, "from_address")
	delete(attrs, "reply_to")
	return attrs
}

func customerIOEventULID(outboxID, recipientUserID string, occurredAt time.Time) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"helpin-customerio-event-v1",
		outboxID,
		recipientUserID,
	}, "\x00")))
	id, err := ulid.New(ulid.Timestamp(occurredAt), bytes.NewReader(digest[:10]))
	if err != nil {
		panic(fmt.Sprintf("derive customer.io event ULID: %v", err))
	}
	return id.String()
}

func customerIOEndpoint(region, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimRight(strings.TrimSpace(override), "/")
	}
	if strings.EqualFold(strings.TrimSpace(region), "eu") {
		return "https://track-eu.customer.io"
	}
	return "https://track.customer.io"
}

func customerIOUserAttributes(user *model.User) map[string]any {
	firstName, lastName := splitFullName(user.FullName)
	return compactAttributes(map[string]any{
		"email":                  user.Email,
		"full_name":              user.FullName,
		"first_name":             firstName,
		"last_name":              lastName,
		"created_at":             user.CreatedAt,
		"updated_at":             user.UpdatedAt,
		"email_verified":         user.EmailVerifiedAt != nil,
		"email_verified_at":      user.EmailVerifiedAt,
		"default_workspace_id":   customerIOStringValue(user.DefaultWorkspaceID),
		"two_fa_enabled":         user.TOTPVerified,
		"avatar_style":           customerIOStringValue(user.AvatarStyle),
		"avatar_background_mode": customerIOStringValue(user.AvatarBackgroundMode),
		"signup_domain":          customerIOEmailDomain(user.Email),
		"profile_complete":       strings.TrimSpace(user.FullName) != "" && strings.TrimSpace(user.Email) != "",
	})
}

func customerIOWorkspaceAttributes(input CustomerIOWorkspaceIdentity) map[string]any {
	ws := input.Workspace
	attrs := map[string]any{
		"name":                        ws.Name,
		"workspace_id":                ws.ID,
		"workspace_slug":              ws.Slug,
		"workspace_key":               ws.WorkspaceKey,
		"organization_id":             customerIOStringValue(ws.OrganizationID),
		"owner_user_id":               ws.OwnerID,
		"website":                     customerIOStringValue(ws.WebsiteURL),
		"website_domain":              customerIOURLHost(customerIOStringValue(ws.WebsiteURL)),
		"timezone":                    ws.Timezone,
		"created_at":                  ws.CreatedAt,
		"updated_at":                  ws.UpdatedAt,
		"has_company_product_context": strings.TrimSpace(customerIOStringValue(ws.CompanyProductContext)) != "",
	}
	if input.Billing != nil {
		b := input.Billing
		attrs["plan"] = b.Plan
		attrs["billing_status"] = b.Status
		attrs["billing_interval"] = b.BillingInterval
		attrs["trialing"] = b.Trialing
		attrs["trial_ends_at"] = b.TrialEndsAt
		attrs["trial_days_left"] = trialDaysLeft(b.TrialEndsAt)
		attrs["locked"] = b.Locked
		attrs["credits_used"] = b.CreditsUsed
		attrs["credits_remaining"] = b.CreditsRemaining
		attrs["included_credits"] = b.IncludedCredits
		attrs["seat_limit"] = b.SeatLimit
		attrs["seat_usage"] = b.SeatUsage
		attrs["seat_over_limit"] = b.SeatOverLimit
		attrs["on_demand_enabled"] = b.OnDemandEnabled
		attrs["on_demand_available"] = b.OnDemandAvailable
		attrs["manage_billing_enabled"] = b.ManageBillingEnabled
		attrs["cancel_at_period_end"] = b.CancelAtPeriodEnd
	}
	return compactAttributes(attrs)
}

func customerIOOrganizationAttributes(input CustomerIOOrganizationIdentity) map[string]any {
	org := input.Organization
	return compactAttributes(map[string]any{
		"name":                     org.Name,
		"organization_id":          org.ID,
		"organization_slug":        org.Slug,
		"owner_user_id":            org.OwnerID,
		"logo_url":                 customerIOStringValue(org.LogoURL),
		"has_logo":                 strings.TrimSpace(customerIOStringValue(org.LogoURL)) != "",
		"created_at":               org.CreatedAt,
		"updated_at":               org.UpdatedAt,
		"member_count":             input.MemberCount,
		"workspace_count":          input.WorkspaceCount,
		"trialing_workspace_count": input.TrialingWorkspaces,
		"active_workspace_count":   input.ActiveWorkspaces,
		"locked_workspace_count":   input.LockedWorkspaces,
		"highest_plan":             input.HighestPlan,
		"has_trial_workspace":      input.HasTrialWorkspace,
		"has_paid_workspace":       input.HasPaidWorkspace,
		"paid_workspace_count":     input.PaidWorkspaceCount,
		"monthly_due_cents":        input.MonthlyDueCents,
	})
}

type CustomerIOIdentityService struct {
	client        *CustomerIOTrackClient
	userRepo      *repository.UserRepository
	workspaceRepo *repository.WorkspaceRepository
	orgRepo       *repository.OrganizationRepository
	billingRepo   *repository.BillingRepository
	logger        *slog.Logger
}

func NewCustomerIOIdentityService(client *CustomerIOTrackClient, userRepo *repository.UserRepository, workspaceRepo *repository.WorkspaceRepository, orgRepo *repository.OrganizationRepository, billingRepo *repository.BillingRepository) *CustomerIOIdentityService {
	return &CustomerIOIdentityService{
		client:        client,
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		orgRepo:       orgRepo,
		billingRepo:   billingRepo,
		logger:        slog.Default().With("service", "customer_io_identity"),
	}
}

func (s *CustomerIOIdentityService) Enabled() bool {
	return s != nil && s.client != nil && s.client.Enabled()
}

func (s *CustomerIOIdentityService) SyncUser(ctx context.Context, user *model.User) {
	if !s.Enabled() || user == nil {
		return
	}
	if err := s.client.IdentifyUser(ctx, user); err != nil {
		s.logger.ErrorContext(ctx, "failed to sync customer.io user", "error", err, "user_id", user.ID)
	}
}

// TrackUserSignedUp records account creation after the person identity exists.
// Delivery is best-effort so Customer.io can never fail account creation.
func (s *CustomerIOIdentityService) TrackUserSignedUp(ctx context.Context, user *model.User, signupMethod string) {
	if !s.Enabled() || user == nil || strings.TrimSpace(user.ID) == "" {
		return
	}
	occurredAt := user.CreatedAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	if err := s.client.TrackEvent(ctx, CustomerIOEvent{
		UserID:     user.ID,
		Name:       "user_signed_up",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"email":         user.Email,
			"full_name":     user.FullName,
			"signup_method": strings.TrimSpace(signupMethod),
		},
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to track customer.io signup", "error", err, "user_id", user.ID)
	}
}

func (s *CustomerIOIdentityService) SyncUserByID(ctx context.Context, userID string) {
	if !s.Enabled() || s.userRepo == nil || strings.TrimSpace(userID) == "" {
		return
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load user for customer.io sync", "error", err, "user_id", userID)
		return
	}
	s.SyncUser(ctx, user)
}

func (s *CustomerIOIdentityService) TrackEvent(ctx context.Context, event CustomerIOEvent) {
	if !s.Enabled() {
		return
	}
	if err := s.client.TrackEvent(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "failed to track customer.io event", "error", err, "event_name", event.Name, "user_id", event.UserID, "workspace_id", event.Attributes["workspace_id"])
	}
}

// TrackOutboxEvent delivers a durable event with a stable per-recipient ID.
func (s *CustomerIOIdentityService) TrackOutboxEvent(ctx context.Context, outboxID string, event CustomerIOEvent) error {
	if !s.Enabled() {
		return nil
	}
	event.EventID = customerIOEventULID(outboxID, event.UserID, event.OccurredAt)
	return s.client.TrackEvent(ctx, event)
}

// TrackWorkspaceEvent fans a workspace-scoped event out to its active members.
// The recipient's workspace role is included in the event so Customer.io can
// target owner/admin campaigns without relying on a global person role.
func (s *CustomerIOIdentityService) TrackWorkspaceEvent(ctx context.Context, workspaceID, name string, occurredAt time.Time, attributes map[string]any) {
	if !s.Enabled() || s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(name) == "" {
		return
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil {
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to load workspace for customer.io event", "error", err, "workspace_id", workspaceID)
		}
		return
	}
	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load workspace members for customer.io event", "error", err, "workspace_id", workspaceID)
		return
	}
	if len(members) == 0 {
		members = []model.MemberWithUser{{UserID: workspace.OwnerID, Role: model.RoleOwner}}
	}
	billing := s.workspaceBillingSummary(ctx, workspaceID)
	base := customerIOWorkspaceEventAttributes(attributes, workspace)
	if billing != nil {
		base["plan"] = billing.Plan
		base["billing_status"] = billing.Status
		base["trialing"] = billing.Trialing
		base["trial_ends_at"] = billing.TrialEndsAt
	}
	for _, member := range members {
		props := cloneAnalyticsAttributes(base)
		props["workspace_role"] = member.Role
		props["membership_status"] = model.WorkspaceMemberStatusActive
		s.TrackEvent(ctx, CustomerIOEvent{
			UserID:     member.UserID,
			Name:       name,
			OccurredAt: occurredAt,
			Attributes: props,
		})
	}
}

func customerIOWorkspaceEventAttributes(attributes map[string]any, workspace *model.Workspace) map[string]any {
	base := cloneAnalyticsAttributes(attributes)
	if workspace == nil {
		return base
	}
	base["workspace_id"] = workspace.ID
	base["workspace_name"] = workspace.Name
	base["workspace_slug"] = workspace.Slug
	if workspace.OrganizationID != nil {
		base["organization_id"] = *workspace.OrganizationID
	}
	return base
}

func (s *CustomerIOIdentityService) DeleteWorkspaceRelationship(ctx context.Context, workspaceID, userID string) {
	if !s.Enabled() || s.client == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return
	}
	if err := s.client.DeletePersonRelationship(ctx, userID, s.client.workspaceObjectTypeID, workspaceID); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete customer.io workspace relationship", "error", err, "workspace_id", workspaceID, "user_id", userID)
	}
}

func (s *CustomerIOIdentityService) DeleteOrganizationRelationship(ctx context.Context, organizationID, userID string) {
	if !s.Enabled() || s.client == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(userID) == "" {
		return
	}
	if err := s.client.DeletePersonRelationship(ctx, userID, s.client.organizationObjectTypeID, organizationID); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete customer.io organization relationship", "error", err, "organization_id", organizationID, "user_id", userID)
	}
}

func cloneAnalyticsAttributes(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source)+4)
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func (s *CustomerIOIdentityService) SyncWorkspace(ctx context.Context, workspaceID, relationshipUserID string) {
	if !s.Enabled() || s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	if strings.TrimSpace(relationshipUserID) == "" {
		s.SyncWorkspaceMembers(ctx, workspaceID)
		return
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load workspace for customer.io sync", "error", err, "workspace_id", workspaceID)
		return
	}
	if workspace == nil {
		return
	}
	billingSummary := s.workspaceBillingSummary(ctx, workspaceID)
	role, status, teamMemberships := s.relationshipDetails(ctx, workspaceID, relationshipUserID)
	if err := s.identifyWorkspaceRelationship(ctx, workspace, billingSummary, relationshipUserID, role, status, teamMemberships); err != nil {
		s.logger.ErrorContext(ctx, "failed to sync customer.io workspace", "error", err, "workspace_id", workspaceID)
	}
	if workspace.OrganizationID != nil && strings.TrimSpace(*workspace.OrganizationID) != "" {
		s.SyncOrganization(ctx, *workspace.OrganizationID, relationshipUserID)
	}
}

// SyncWorkspaceMembers upserts a workspace object and every active member
// relationship. It is used by authoritative backend flows so relationship
// state does not depend on a user visiting the workspace in the browser.
func (s *CustomerIOIdentityService) SyncWorkspaceMembers(ctx context.Context, workspaceID string) {
	if !s.Enabled() || s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil {
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to load workspace for customer.io member sync", "error", err, "workspace_id", workspaceID)
		}
		return
	}
	billingSummary := s.workspaceBillingSummary(ctx, workspaceID)
	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load workspace members for customer.io sync", "error", err, "workspace_id", workspaceID)
		return
	}
	if len(members) == 0 {
		members = []model.MemberWithUser{{UserID: workspace.OwnerID, Role: model.RoleOwner}}
	}
	for _, member := range members {
		if err := s.identifyWorkspaceRelationship(ctx, workspace, billingSummary, member.UserID, member.Role, model.WorkspaceMemberStatusActive, 0); err != nil {
			s.logger.ErrorContext(ctx, "failed to sync customer.io workspace relationship", "error", err, "workspace_id", workspaceID, "user_id", member.UserID)
		}
	}
	if workspace.OrganizationID != nil && strings.TrimSpace(*workspace.OrganizationID) != "" {
		s.SyncOrganization(ctx, *workspace.OrganizationID, "")
	}
}

// RefreshWorkspaceForOutbox updates current workspace and relationship state
// and returns delivery errors to the durable worker. found is false when the
// workspace was deleted after the event was enqueued.
func (s *CustomerIOIdentityService) RefreshWorkspaceForOutbox(ctx context.Context, workspaceID string) (found bool, err error) {
	if !s.Enabled() || s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return false, nil
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("load workspace for Customer.io refresh: %w", err)
	}
	if workspace == nil {
		return false, nil
	}
	billingSummary := s.workspaceBillingSummary(ctx, workspaceID)
	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return true, fmt.Errorf("load workspace members for Customer.io refresh: %w", err)
	}
	if len(members) == 0 {
		members = []model.MemberWithUser{{UserID: workspace.OwnerID, Role: model.RoleOwner}}
	}
	for _, member := range members {
		if err := s.identifyWorkspaceRelationship(ctx, workspace, billingSummary, member.UserID, member.Role, model.WorkspaceMemberStatusActive, 0); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (s *CustomerIOIdentityService) workspaceBillingSummary(ctx context.Context, workspaceID string) *BillingSummary {
	if s.billingRepo == nil {
		return nil
	}
	billing, err := s.billingRepo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load billing for customer.io sync", "error", err, "workspace_id", workspaceID)
		return nil
	}
	if billing == nil {
		return nil
	}
	return customerIOBillingSummary(ctx, billing, s.workspaceRepo)
}

func (s *CustomerIOIdentityService) identifyWorkspaceRelationship(ctx context.Context, workspace *model.Workspace, billing *BillingSummary, userID, role, status string, teamMemberships int) error {
	return s.client.IdentifyWorkspace(ctx, CustomerIOWorkspaceIdentity{
		Workspace:          workspace,
		Billing:            billing,
		RelationshipUserID: userID,
		RelationshipRole:   role,
		RelationshipStatus: status,
		TeamMemberships:    teamMemberships,
	})
}

func (s *CustomerIOIdentityService) SyncOrganization(ctx context.Context, orgID, relationshipUserID string) {
	if !s.Enabled() || s.orgRepo == nil || strings.TrimSpace(orgID) == "" {
		return
	}
	if strings.TrimSpace(relationshipUserID) == "" {
		s.SyncOrganizationMembers(ctx, orgID)
		return
	}
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load organization for customer.io sync", "error", err, "organization_id", orgID)
		return
	}
	if org == nil {
		return
	}
	summary := s.organizationSummary(ctx, orgID)
	role := "owner"
	if relationshipUserID != "" {
		if resolvedRole, err := s.orgRepo.GetMemberRole(ctx, orgID, relationshipUserID); err == nil && strings.TrimSpace(resolvedRole) != "" {
			role = resolvedRole
		}
	}
	if err := s.client.IdentifyOrganization(ctx, CustomerIOOrganizationIdentity{
		Organization:       org,
		RelationshipUserID: relationshipUserID,
		RelationshipRole:   role,
		MemberCount:        summary.MemberCount,
		WorkspaceCount:     summary.WorkspaceCount,
		TrialingWorkspaces: summary.TrialingWorkspaces,
		ActiveWorkspaces:   summary.ActiveWorkspaces,
		LockedWorkspaces:   summary.LockedWorkspaces,
		HighestPlan:        summary.HighestPlan,
		HasTrialWorkspace:  summary.HasTrialWorkspace,
		HasPaidWorkspace:   summary.HasPaidWorkspace,
		PaidWorkspaceCount: summary.PaidWorkspaceCount,
		MonthlyDueCents:    summary.MonthlyDueCents,
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to sync customer.io organization", "error", err, "organization_id", orgID)
	}
}

// SyncOrganizationMembers upserts an organization object and every member
// relationship, including each member's organization role.
func (s *CustomerIOIdentityService) SyncOrganizationMembers(ctx context.Context, orgID string) {
	if !s.Enabled() || s.orgRepo == nil || strings.TrimSpace(orgID) == "" {
		return
	}
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil || org == nil {
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to load organization for customer.io member sync", "error", err, "organization_id", orgID)
		}
		return
	}
	summary := s.organizationSummary(ctx, orgID)
	members, err := s.orgRepo.ListMembers(ctx, orgID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load organization members for customer.io sync", "error", err, "organization_id", orgID)
		return
	}
	if len(members) == 0 {
		members = []model.MemberWithUser{{UserID: org.OwnerID, Role: model.RoleOwner}}
	}
	for _, member := range members {
		if err := s.client.IdentifyOrganization(ctx, CustomerIOOrganizationIdentity{
			Organization:       org,
			RelationshipUserID: member.UserID,
			RelationshipRole:   member.Role,
			MemberCount:        summary.MemberCount,
			WorkspaceCount:     summary.WorkspaceCount,
			TrialingWorkspaces: summary.TrialingWorkspaces,
			ActiveWorkspaces:   summary.ActiveWorkspaces,
			LockedWorkspaces:   summary.LockedWorkspaces,
			HighestPlan:        summary.HighestPlan,
			HasTrialWorkspace:  summary.HasTrialWorkspace,
			HasPaidWorkspace:   summary.HasPaidWorkspace,
			PaidWorkspaceCount: summary.PaidWorkspaceCount,
			MonthlyDueCents:    summary.MonthlyDueCents,
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to sync customer.io organization relationship", "error", err, "organization_id", orgID, "user_id", member.UserID)
		}
	}
}

type customerIOOrganizationSummary struct {
	MemberCount        int
	WorkspaceCount     int
	TrialingWorkspaces int
	ActiveWorkspaces   int
	LockedWorkspaces   int
	HighestPlan        string
	HasTrialWorkspace  bool
	HasPaidWorkspace   bool
	PaidWorkspaceCount int
	MonthlyDueCents    int
}

func (s *CustomerIOIdentityService) organizationSummary(ctx context.Context, orgID string) customerIOOrganizationSummary {
	var summary customerIOOrganizationSummary
	if s.orgRepo != nil {
		if members, err := s.orgRepo.ListMembers(ctx, orgID); err == nil {
			summary.MemberCount = len(members)
		} else {
			s.logger.ErrorContext(ctx, "failed to count organization members for customer.io sync", "error", err, "organization_id", orgID)
		}
	}
	if s.billingRepo == nil {
		return summary
	}
	rows, err := s.billingRepo.ListWorkspaceBillingsForOrg(ctx, orgID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load organization billing rollup for customer.io sync", "error", err, "organization_id", orgID)
		return summary
	}
	summary.WorkspaceCount = len(rows)
	for _, row := range rows {
		billing := row.Billing
		if billing.Status == model.BillingStatusTrialing {
			summary.TrialingWorkspaces++
			summary.HasTrialWorkspace = true
		}
		if billing.Status == model.BillingStatusActive {
			summary.ActiveWorkspaces++
			if billing.StripeSubscriptionID != nil || billing.Plan == model.BillingPlanFounder {
				summary.HasPaidWorkspace = true
				summary.PaidWorkspaceCount++
			}
		}
		if billingStatusLocked(billing.Status) || billing.Status == model.BillingStatusPastDue || billing.Status == model.BillingStatusUnpaid {
			summary.LockedWorkspaces++
		}
		if billingPlanRank(billing.Plan) > billingPlanRank(summary.HighestPlan) {
			summary.HighestPlan = billing.Plan
		}
		if billing.Status == model.BillingStatusActive {
			summary.MonthlyDueCents += monthlyDueCentsForCustomerIO(billing.Plan, billing.BillingInterval)
		}
	}
	return summary
}

func (s *CustomerIOIdentityService) relationshipDetails(ctx context.Context, workspaceID, userID string) (string, string, int) {
	if s.workspaceRepo == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return "owner", model.WorkspaceMemberStatusActive, 0
	}
	member, err := s.workspaceRepo.GetMembership(ctx, workspaceID, userID)
	if err != nil || member == nil {
		return "owner", model.WorkspaceMemberStatusActive, 0
	}
	return member.Role, member.Status, 0
}

func customerIOBillingSummary(ctx context.Context, billing *model.WorkspaceBilling, workspaceRepo *repository.WorkspaceRepository) *BillingSummary {
	if billing == nil {
		return nil
	}
	includedCredits := includedCreditsForPlan(billing.Plan)
	if billing.IncludedCredits > 0 {
		includedCredits = billing.IncludedCredits
	}
	remaining := includedCredits - billing.CreditsUsed
	if remaining < 0 {
		remaining = 0
	}
	summary := &BillingSummary{
		WorkspaceID:            billing.WorkspaceID,
		Plan:                   billing.Plan,
		Status:                 billing.Status,
		BillingInterval:        billing.BillingInterval,
		Trialing:               billing.Status == model.BillingStatusTrialing,
		TrialEndsAt:            billing.TrialEndsAt,
		CurrentPeriodStart:     billing.CurrentPeriodStart,
		CurrentPeriodEnd:       billing.CurrentPeriodEnd,
		IncludedCredits:        includedCredits,
		CreditsUsed:            billing.CreditsUsed,
		CreditsRemaining:       remaining,
		OnDemandEnabled:        billing.OnDemandEnabled,
		OnDemandAvailable:      billingCanUseOnDemand(billing),
		StripeCustomerID:       billing.StripeCustomerID,
		StripeSubscriptionID:   billing.StripeSubscriptionID,
		PendingPlan:            billing.PendingPlan,
		PendingBillingInterval: billing.PendingBillingInterval,
		PendingChangeAt:        billing.PendingChangeAt,
		CancelAtPeriodEnd:      billing.CancelAtPeriodEnd,
		CanceledAt:             billing.CanceledAt,
		Locked:                 billingStatusLocked(billing.Status),
		ManageBillingEnabled:   billing.StripeCustomerID != nil,
		OnDemandBlocksInvoiced: billing.OnDemandBlocksInvoiced,
	}
	if workspaceRepo != nil {
		if count, err := workspaceRepo.CountBillableSeats(ctx, billing.WorkspaceID); err == nil {
			summary.SeatUsage = int(count)
		}
	}
	return summary
}

func monthlyDueCentsForCustomerIO(plan, interval string) int {
	if plan == model.BillingPlanFounder {
		return 0
	}
	cents := PriceCentsForPlan(plan, interval)
	if strings.EqualFold(interval, "annual") {
		return cents / 12
	}
	return cents
}

func compactAttributes(attrs map[string]any) map[string]any {
	out := make(map[string]any, len(attrs))
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
		case *time.Time:
			if v == nil {
				continue
			}
		case nil:
			continue
		}
		out[key] = value
	}
	return out
}

func splitFullName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func customerIOEmailDomain(email string) string {
	parts := strings.Split(strings.TrimSpace(email), "@")
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[1])
}

func customerIOURLHost(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func customerIOStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func customerIOFirstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func trialDaysLeft(trialEndsAt *time.Time) int {
	if trialEndsAt == nil {
		return 0
	}
	remaining := time.Until(trialEndsAt.UTC())
	if remaining <= 0 {
		return 0
	}
	days := int(remaining / (24 * time.Hour))
	if remaining%(24*time.Hour) != 0 {
		days++
	}
	return days
}
