package crmsignal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmtext"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	defaultTaskQueue                = "automation-default"
	signalDetectionWorkflowName     = "SignalDetectionWorkflow"
	threadSignalContextMessageLimit = 3
	maxSignalBodyChars              = 3000
	maxThreadContextChars           = 1500
)

// StartWorkflow defines the workflow-start behavior required by ingestion.
type StartWorkflow interface {
	StartEmailSignalDetection(ctx context.Context, messageID string, payloads []model.SignalSourcePayload) error
}

// TemporalStarter starts signal detection workflows in Temporal.
type TemporalStarter struct {
	client    tclient.Client
	taskQueue string
}

// NewTemporalStarter creates a Temporal-backed signal workflow starter.
func NewTemporalStarter(client tclient.Client, taskQueue string) *TemporalStarter {
	if taskQueue == "" {
		taskQueue = defaultTaskQueue
	}
	return &TemporalStarter{client: client, taskQueue: taskQueue}
}

// StartEmailSignalDetection starts the per-message signal detection workflow.
func (s *TemporalStarter) StartEmailSignalDetection(ctx context.Context, messageID string, payloads []model.SignalSourcePayload) error {
	return s.StartSignalDetection(ctx, "email-"+messageID, payloads)
}

// StartSignalDetection starts a source-keyed signal workflow.
func (s *TemporalStarter) StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error {
	if s == nil || s.client == nil || sourceKey == "" || len(payloads) == 0 {
		return nil
	}

	_, err := s.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:                    "crm-signal-" + sourceKey,
		TaskQueue:             s.taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, signalDetectionWorkflowName, payloads)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start signal detection workflow: %w", err)
	}
	return nil
}

// WorkflowIDForMessage returns the deterministic workflow ID for one message.
func WorkflowIDForMessage(messageID string) string {
	return "crm-signal-email-" + messageID
}

// IngestionResult summarizes the enqueue decision for a stored CRM email.
type IngestionResult struct {
	Started      bool
	SkipReason   string
	PayloadCount int
	WorkflowID   string
}

// IngestionService decides whether a stored CRM email should produce a signal
// detection workflow and starts that workflow when eligible.
type IngestionService struct {
	emailRepo *repository.CRMEmailRepository
	starter   StartWorkflow
}

// NewIngestionService creates a CRM signal ingestion service.
func NewIngestionService(emailRepo *repository.CRMEmailRepository, starter StartWorkflow) *IngestionService {
	return &IngestionService{
		emailRepo: emailRepo,
		starter:   starter,
	}
}

// EnqueueEmailMessage evaluates a stored CRM email message and starts signal
// detection when it is eligible.
func (s *IngestionService) EnqueueEmailMessage(ctx context.Context, messageID string) (*IngestionResult, error) {
	if s == nil || s.emailRepo == nil || messageID == "" {
		return &IngestionResult{SkipReason: "not_configured"}, nil
	}

	message, err := s.emailRepo.GetMessageByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("load message for signal ingestion: %w", err)
	}
	if message == nil {
		return &IngestionResult{SkipReason: "message_not_found"}, nil
	}

	payload, skipReason, err := s.buildEmailPayload(ctx, message)
	if err != nil {
		return nil, err
	}
	if skipReason != "" {
		slog.InfoContext(ctx, "crm buyer signal ingestion skipped", "workspace_id", message.WorkspaceID, "message_id", message.ID, "thread_id", stringValue(message.ThreadID), "skip_reason", skipReason, "contact_count", len(message.ContactIDs), "has_deal", message.DealID != nil)
		return &IngestionResult{SkipReason: skipReason}, nil
	}
	if payload == nil {
		return &IngestionResult{SkipReason: "empty_payload"}, nil
	}
	if s.starter == nil {
		return &IngestionResult{SkipReason: "starter_not_configured"}, nil
	}

	payloads := []model.SignalSourcePayload{*payload}
	if err := s.starter.StartEmailSignalDetection(ctx, message.ID, payloads); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "crm buyer signal ingestion enqueued", "workspace_id", message.WorkspaceID, "message_id", message.ID, "thread_id", stringValue(message.ThreadID), "workflow_id", WorkflowIDForMessage(message.ID), "payloads", len(payloads))
	return &IngestionResult{
		Started:      true,
		PayloadCount: len(payloads),
		WorkflowID:   WorkflowIDForMessage(message.ID),
	}, nil
}

func (s *IngestionService) buildEmailPayload(ctx context.Context, message *model.CRMEmailMessage) (*model.SignalSourcePayload, string, error) {
	body := preferredBody(message.BodyText, message.BodyHTML)
	if body == "" {
		return nil, "empty_body", nil
	}
	if len(message.ContactIDs) == 0 && message.DealID == nil {
		return nil, "no_contact_or_deal", nil
	}
	if len(message.ContactIDs) > 1 && message.DealID == nil {
		return nil, "multi_contact_without_deal", nil
	}

	var contactID *string
	if len(message.ContactIDs) == 1 {
		contactID = &message.ContactIDs[0]
	}

	var thread *model.CRMEmailThread
	if message.ThreadID != nil && *message.ThreadID != "" {
		loaded, err := s.emailRepo.GetThreadByID(ctx, *message.ThreadID)
		if err != nil {
			return nil, "", fmt.Errorf("load thread for signal ingestion: %w", err)
		}
		thread = loaded
	}

	threadContext := ""
	if message.ThreadID != nil && *message.ThreadID != "" {
		recent, err := s.emailRepo.ListRecentMessagesForThread(ctx, *message.ThreadID, message.ID, message.SentAt, threadSignalContextMessageLimit)
		if err != nil {
			return nil, "", fmt.Errorf("load thread context for signal ingestion: %w", err)
		}
		threadContext = formatThreadContext(recent)
	}

	payload := &model.SignalSourcePayload{
		SourceType:             model.CRMSignalSourceEmail,
		SourceID:               message.ID,
		SourceThreadID:         message.ThreadID,
		SourceThreadExternalID: threadExternalID(thread),
		WorkspaceID:            message.WorkspaceID,
		ContactID:              contactID,
		DealID:                 message.DealID,
		Subject:                preferredSubject(message.Subject, thread),
		Body:                   body,
		Participants:           emailParticipants(message),
		Direction:              message.Direction,
		OccurredAt:             message.SentAt,
		ThreadContext:          threadContext,
	}
	return payload, "", nil
}

func preferredBody(bodyText, bodyHTML *string) string {
	return crmtext.PreferredBody(stringValue(bodyText), stringValue(bodyHTML), maxSignalBodyChars)
}

func preferredSubject(messageSubject string, thread *model.CRMEmailThread) string {
	subject := strings.TrimSpace(messageSubject)
	if subject != "" {
		return subject
	}
	if thread == nil {
		return ""
	}
	return strings.TrimSpace(thread.Subject)
}

func threadExternalID(thread *model.CRMEmailThread) *string {
	if thread == nil {
		return nil
	}
	value := strings.TrimSpace(thread.ThreadExternalID)
	if value == "" {
		return nil
	}
	return &value
}

func emailParticipants(message *model.CRMEmailMessage) []model.SignalParticipant {
	participants := make([]model.SignalParticipant, 0, 1+len(message.ContactIDs))
	if email := crmemail.NormalizeEmailAddress(message.FromAddress); email != "" {
		participants = append(participants, model.SignalParticipant{
			Email: email,
			Name:  strings.TrimSpace(stringValue(message.FromName)),
			Role:  model.CRMEmailParticipantRoleFrom,
		})
	}
	for _, email := range crmemail.ParseAddressJSONArray(message.ToAddresses) {
		participants = append(participants, model.SignalParticipant{Email: email, Role: model.CRMEmailParticipantRoleTo})
	}
	for _, email := range crmemail.ParseAddressJSONArray(message.CCAddresses) {
		participants = append(participants, model.SignalParticipant{Email: email, Role: model.CRMEmailParticipantRoleCC})
	}
	return participants
}

func formatThreadContext(messages []model.CRMEmailMessage) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, message := range messages {
		body := preferredBody(message.BodyText, message.BodyHTML)
		if body == "" {
			continue
		}
		line := fmt.Sprintf("[%s] %s %s: %s\n", message.SentAt.UTC().Format(time.RFC3339), message.Direction, crmemail.NormalizeEmailAddress(message.FromAddress), body)
		if builder.Len()+len(line) > maxThreadContextChars {
			remaining := maxThreadContextChars - builder.Len()
			if remaining > 0 {
				builder.WriteString(line[:remaining])
			}
			break
		}
		builder.WriteString(line)
	}
	return strings.TrimSpace(builder.String())
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
