package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	_defaultUsermavenEndpoint        = "https://events.usermaven.com/api/v1/s2s/event"
	_productAnalyticsOutboxLease     = 5 * time.Minute
	_productAnalyticsMaxAttempts     = 10
	_productAnalyticsOutboxBatchSize = 25
)

// ProductAnalyticsEvent is one canonical product event.
type ProductAnalyticsEvent = model.ProductAnalyticsEvent

// ProductAnalyticsService queues canonical product events without blocking product behavior.
type ProductAnalyticsService struct {
	repo   *repository.ProductAnalyticsOutboxRepository
	now    func() time.Time
	logger *slog.Logger
}

// NewProductAnalyticsService creates a canonical product analytics tracker.
func NewProductAnalyticsService(repo *repository.ProductAnalyticsOutboxRepository) *ProductAnalyticsService {
	return &ProductAnalyticsService{
		repo: repo, now: time.Now,
		logger: slog.Default().With("service", "product_analytics"),
	}
}

// Track durably queues one event. Analytics failures never fail the product operation.
func (s *ProductAnalyticsService) Track(ctx context.Context, event ProductAnalyticsEvent) {
	if s == nil || s.repo == nil || strings.TrimSpace(event.SemanticKey) == "" ||
		strings.TrimSpace(event.Name) == "" {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.now().UTC()
	}
	if event.Source == "" {
		event.Source = "server"
	}
	if event.UserID == "" {
		event.UserID = middleware.GetUserID(ctx)
	}
	_, err := s.repo.Enqueue(ctx, repository.ProductAnalyticsEventInput{
		SemanticKey: event.SemanticKey, UserID: event.UserID, WorkspaceID: event.WorkspaceID,
		AnonymousID: event.AnonymousID,
		EventName:   event.Name, Source: event.Source, OccurredAt: event.OccurredAt,
		Attributes: event.Attributes,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "queue product analytics event", "error", err,
			"event_name", event.Name, "workspace_id", event.WorkspaceID, "user_id", event.UserID)
	}
}

// UsermavenConfig configures server-side Usermaven event delivery.
type UsermavenConfig struct {
	APIKey      string
	ServerToken string
	Endpoint    string
	HTTPClient  *http.Client
}

// UsermavenDeliveryError describes a rejected server-side event delivery.
type UsermavenDeliveryError struct {
	StatusCode int
	RetryAfter time.Duration
	Message    string
	Err        error
}

func (e *UsermavenDeliveryError) Error() string {
	if e == nil {
		return "Usermaven delivery error"
	}
	if e.Err != nil {
		return fmt.Sprintf("Usermaven delivery: %v", e.Err)
	}
	return fmt.Sprintf("Usermaven delivery returned status %d: %s", e.StatusCode, e.Message)
}

func (e *UsermavenDeliveryError) Unwrap() error { return e.Err }

// UsermavenEvent is the resolved server-side event payload.
type UsermavenEvent struct {
	Name       string
	OccurredAt time.Time
	User       map[string]any
	Company    map[string]any
	Attributes map[string]any
}

// UsermavenClient sends canonical events through Usermaven's server-side endpoint.
type UsermavenClient struct {
	apiKey      string
	serverToken string
	endpoint    string
	httpClient  *http.Client
}

// NewUsermavenClient creates a server-side Usermaven HTTP client.
func NewUsermavenClient(cfg UsermavenConfig) *UsermavenClient {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = _defaultUsermavenEndpoint
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &UsermavenClient{
		apiKey: strings.TrimSpace(cfg.APIKey), serverToken: strings.TrimSpace(cfg.ServerToken), endpoint: endpoint, httpClient: httpClient,
	}
}

// Enabled reports whether server-side Usermaven delivery is configured.
func (c *UsermavenClient) Enabled() bool { return c != nil && c.apiKey != "" }

// Track sends one event to Usermaven.
func (c *UsermavenClient) Track(ctx context.Context, event UsermavenEvent) error {
	if !c.Enabled() {
		return nil
	}
	payload := map[string]any{
		"api_key": c.apiKey, "event_type": event.Name,
		"timestamp": event.OccurredAt.UTC().UnixMilli(), "user": event.User,
		"event_attributes": event.Attributes,
	}
	if len(event.Company) > 0 {
		payload["company"] = event.Company
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Usermaven event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Usermaven request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.serverToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey+"."+c.serverToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &UsermavenDeliveryError{Err: err}
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return &UsermavenDeliveryError{
		StatusCode: resp.StatusCode, RetryAfter: parseAnalyticsRetryAfter(resp.Header.Get("Retry-After")),
		Message: strings.TrimSpace(string(responseBody)),
	}
}

// ProductAnalyticsOutboxWorker resolves event-time product context and delivers events.
type ProductAnalyticsOutboxWorker struct {
	repo             *repository.ProductAnalyticsOutboxRepository
	userRepo         *repository.UserRepository
	workspaceRepo    *repository.WorkspaceRepository
	organizationRepo *repository.OrganizationRepository
	billingRepo      workspaceBillingReader
	client           *UsermavenClient
	now              func() time.Time
}

// NewProductAnalyticsOutboxWorker creates a durable Usermaven delivery worker.
func NewProductAnalyticsOutboxWorker(
	repo *repository.ProductAnalyticsOutboxRepository,
	userRepo *repository.UserRepository,
	workspaceRepo *repository.WorkspaceRepository,
	organizationRepo *repository.OrganizationRepository,
	billingRepo workspaceBillingReader,
	client *UsermavenClient,
) *ProductAnalyticsOutboxWorker {
	return &ProductAnalyticsOutboxWorker{
		repo: repo, userRepo: userRepo, workspaceRepo: workspaceRepo,
		organizationRepo: organizationRepo,
		billingRepo:      billingRepo, client: client, now: time.Now,
	}
}

// ProcessDue claims and delivers one bounded batch.
func (w *ProductAnalyticsOutboxWorker) ProcessDue(ctx context.Context) error {
	if w == nil || w.repo == nil || w.client == nil || !w.client.Enabled() {
		return nil
	}
	rows, err := w.repo.ClaimDue(
		ctx, w.now().UTC(), _productAnalyticsOutboxLease, _productAnalyticsOutboxBatchSize,
	)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := w.deliver(ctx, &rows[i]); err != nil {
			slog.ErrorContext(ctx, "product analytics delivery state update failed",
				"error", err, "outbox_id", rows[i].ID)
		}
	}
	return nil
}

func (w *ProductAnalyticsOutboxWorker) deliver(
	ctx context.Context,
	row *model.ProductAnalyticsOutbox,
) error {
	token := analyticsClaimToken(row)
	var attributes map[string]any
	if err := json.Unmarshal(row.Attributes, &attributes); err != nil {
		_, markErr := w.repo.MarkFailed(ctx, row.ID, token, "decode attributes: "+err.Error())
		return markErr
	}
	if attributes == nil {
		attributes = make(map[string]any)
	}
	userID := analyticsID(row.UserID)
	workspaceID := analyticsID(row.WorkspaceID)
	workspace, err := w.workspace(ctx, workspaceID)
	if err != nil {
		return w.handleDeliveryError(ctx, row, token, err)
	}
	if userID == "" && workspace != nil {
		userID = workspace.OwnerID
	}
	user, err := w.userRepo.GetByID(ctx, userID)
	if err != nil {
		return w.handleDeliveryError(ctx, row, token, err)
	}
	if user == nil {
		_, markErr := w.repo.MarkFailed(ctx, row.ID, token, "analytics user not found")
		return markErr
	}
	attributes["event_id"] = row.SemanticKey
	attributes["source"] = row.Source
	company := map[string]any(nil)
	if workspace != nil {
		company = w.workspaceCompany(ctx, workspace)
		attributes["workspace_id"] = workspace.ID
		attributes["workspace_name"] = workspace.Name
		attributes["workspace_slug"] = workspace.Slug
		if workspace.OrganizationID != nil {
			attributes["organization_id"] = *workspace.OrganizationID
		}
		if role := w.workspaceRole(ctx, workspace.ID, user.ID); role != "" {
			attributes["workspace_role"] = role
		}
		if w.billingRepo != nil {
			if billing, billingErr := w.billingRepo.GetByWorkspaceID(ctx, workspace.ID); billingErr == nil && billing != nil {
				attributes["plan"] = billing.Plan
				attributes["billing_status"] = billing.Status
				attributes["billing_interval"] = billing.BillingInterval
			}
		}
	}
	firstName, lastName := analyticsNameParts(user.FullName)
	userTraits := map[string]any{
		"id": user.ID, "email": user.Email, "full_name": user.FullName,
		"first_name": firstName, "last_name": lastName,
		"created_at": user.CreatedAt.UTC().Format(time.RFC3339),
	}
	if anonymousID := analyticsID(row.AnonymousID); anonymousID != "" {
		userTraits["anonymous_id"] = anonymousID
	}
	deliveryErr := w.client.Track(ctx, UsermavenEvent{
		Name: row.EventName, OccurredAt: row.OccurredAt,
		User:    userTraits,
		Company: company, Attributes: attributes,
	})
	if deliveryErr != nil {
		return w.handleDeliveryError(ctx, row, token, deliveryErr)
	}
	_, err = w.repo.MarkDelivered(ctx, row.ID, token)
	return err
}

func (w *ProductAnalyticsOutboxWorker) workspace(
	ctx context.Context,
	workspaceID string,
) (*model.Workspace, error) {
	if workspaceID == "" {
		return nil, nil
	}
	return w.workspaceRepo.GetByID(ctx, workspaceID)
}

func (w *ProductAnalyticsOutboxWorker) workspaceCompany(
	ctx context.Context,
	workspace *model.Workspace,
) map[string]any {
	if workspace.OrganizationID != nil && w.organizationRepo != nil {
		if organization, err := w.organizationRepo.GetByID(ctx, *workspace.OrganizationID); err == nil && organization != nil {
			return map[string]any{
				"id": organization.ID, "name": organization.Name,
				"created_at":        organization.CreatedAt.UTC().Format(time.RFC3339),
				"organization_slug": organization.Slug,
			}
		}
	}
	company := map[string]any{
		"id": workspace.ID, "name": workspace.Name,
		"created_at":     workspace.CreatedAt.UTC().Format(time.RFC3339),
		"workspace_slug": workspace.Slug,
	}
	if workspace.OrganizationID != nil {
		company["organization_id"] = *workspace.OrganizationID
	}
	if w.billingRepo != nil {
		if billing, err := w.billingRepo.GetByWorkspaceID(ctx, workspace.ID); err == nil && billing != nil {
			company["plan"] = billing.Plan
			company["billing_status"] = billing.Status
			company["billing_interval"] = billing.BillingInterval
		}
	}
	return company
}

func (w *ProductAnalyticsOutboxWorker) workspaceRole(
	ctx context.Context,
	workspaceID, userID string,
) string {
	members, err := w.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return ""
	}
	for _, member := range members {
		if member.UserID == userID {
			return member.Role
		}
	}
	return ""
}

func (w *ProductAnalyticsOutboxWorker) handleDeliveryError(
	ctx context.Context,
	row *model.ProductAnalyticsOutbox,
	token string,
	deliveryErr error,
) error {
	retry, retryAfter := productAnalyticsRetry(deliveryErr)
	if row.Attempts >= _productAnalyticsMaxAttempts {
		retry = false
	}
	if !retry {
		_, err := w.repo.MarkFailed(ctx, row.ID, token, deliveryErr.Error())
		return err
	}
	if retryAfter <= 0 {
		shift := row.Attempts - 1
		if shift < 0 {
			shift = 0
		}
		if shift > 8 {
			shift = 8
		}
		retryAfter = time.Second * time.Duration(1<<shift)
		retryAfter += time.Duration(rand.Int63n(int64(retryAfter/4 + 1)))
	}
	if retryAfter > time.Hour {
		retryAfter = time.Hour
	}
	_, err := w.repo.ScheduleRetry(
		ctx, row.ID, token, w.now().UTC().Add(retryAfter), deliveryErr.Error(),
	)
	return err
}

// Run polls until cancellation.
func (w *ProductAnalyticsOutboxWorker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if err := w.ProcessDue(ctx); err != nil {
		slog.ErrorContext(ctx, "product analytics outbox poll failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.ProcessDue(ctx); err != nil {
				slog.ErrorContext(ctx, "product analytics outbox poll failed", "error", err)
			}
		}
	}
}

func productAnalyticsRetry(err error) (bool, time.Duration) {
	var deliveryErr *UsermavenDeliveryError
	if !errors.As(err, &deliveryErr) {
		return true, 0
	}
	status := deliveryErr.StatusCode
	return status == 0 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests ||
		status >= http.StatusInternalServerError, deliveryErr.RetryAfter
}

func parseAnalyticsRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		return time.Until(retryAt)
	}
	return 0
}

func analyticsNameParts(fullName string) (string, string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func analyticsID(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func analyticsClaimToken(row *model.ProductAnalyticsOutbox) string {
	if row.ClaimToken == nil {
		return ""
	}
	return *row.ClaimToken
}

type workspaceBillingReader interface {
	GetByWorkspaceID(context.Context, string) (*model.WorkspaceBilling, error)
}
