package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// supportImportLease hides a job from other workers while one page runs.
const supportImportLease = 5 * time.Minute

// supportImportPageSize is how many conversations one pass imports.
const supportImportPageSize = 25

// SupportImportService imports conversations from Intercom. A worker in the
// API processes each job one page at a time and saves its cursor, so an
// import survives restarts; a record per source conversation makes reruns
// skip what was already imported.
type SupportImportService struct {
	repo  *repository.SupportImportRepository
	inbox *SupportInboxService
	key   []byte
	now   func() time.Time
	// intercomBaseURLOverride replaces the Intercom API host for every
	// region (INTERCOM_API_BASE_URL).
	intercomBaseURLOverride string
}

func NewSupportImportService(repo *repository.SupportImportRepository, inbox *SupportInboxService, encryptionKey []byte) *SupportImportService {
	return &SupportImportService{repo: repo, inbox: inbox, key: encryptionKey, now: func() time.Time { return time.Now().UTC() }}
}

// SetIntercomBaseURL replaces the Intercom API host (INTERCOM_API_BASE_URL).
func (s *SupportImportService) SetIntercomBaseURL(baseURL string) *SupportImportService {
	s.intercomBaseURLOverride = baseURL
	return s
}

func invalidIntercomImport(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrIntercomImportInvalid, fmt.Sprintf(format, args...))
}

func (s *SupportImportService) client(token, region string) (*intercom.Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, invalidIntercomImport("an access token is required")
	}
	base, err := intercomBaseURL(region, s.intercomBaseURLOverride)
	if err != nil {
		return nil, err
	}
	return intercom.NewClient(token, base), nil
}

func parseImportSince(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return now.AddDate(-1, 0, 0), nil
	}
	since, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, invalidIntercomImport("since must be a date like 2025-01-31")
	}
	if since.After(now) {
		return time.Time{}, invalidIntercomImport("since cannot be in the future")
	}
	return since, nil
}

// Preview counts the conversations an import would bring in.
func (s *SupportImportService) Preview(ctx context.Context, req model.IntercomConversationImportRequest) (*model.IntercomConversationImportPreview, error) {
	since, err := parseImportSince(req.Since, s.now())
	if err != nil {
		return nil, err
	}
	client, err := s.client(req.AccessToken, req.Region)
	if err != nil {
		return nil, err
	}
	me, err := client.Me(ctx)
	if err != nil {
		return nil, intercomAccessError(err)
	}
	page, err := client.SearchConversations(ctx, since, "", 1)
	if err != nil {
		return nil, intercomAccessError(err)
	}
	return &model.IntercomConversationImportPreview{WorkspaceName: me.App.Name, Conversations: page.TotalCount}, nil
}

type storedIntercomImport struct {
	AccessToken string `json:"access_token"`
	Region      string `json:"region"`
}

func supportImportAAD(jobID string) []byte { return []byte("support-import:" + jobID) }

// Start queues an import. One import per source runs at a time.
func (s *SupportImportService) Start(ctx context.Context, workspaceID, userID string, req model.IntercomConversationImportRequest) (*model.SupportImportJob, error) {
	if len(s.key) != 32 {
		return nil, invalidIntercomImport("imports need PM_IMPORT_ENCRYPTION_KEY or CRM_ENCRYPTION_KEY on the server")
	}
	since, err := parseImportSince(req.Since, s.now())
	if err != nil {
		return nil, err
	}
	var mailboxID *string
	if req.MailboxID != nil && strings.TrimSpace(*req.MailboxID) != "" && strings.TrimSpace(*req.MailboxID) != "shared" {
		id := strings.TrimSpace(*req.MailboxID)
		mailbox, err := s.inbox.mailboxRepo.GetByID(ctx, workspaceID, id)
		if err != nil || mailbox == nil || !mailbox.Active {
			return nil, invalidIntercomImport("the inbox was not found")
		}
		mailboxID = &id
	}
	client, err := s.client(req.AccessToken, req.Region)
	if err != nil {
		return nil, err
	}
	if _, err := client.Me(ctx); err != nil {
		return nil, intercomAccessError(err)
	}
	if active, err := s.repo.HasActiveJob(ctx, workspaceID, model.SupportImportSourceIntercom); err != nil {
		return nil, err
	} else if active {
		return nil, invalidIntercomImport("an Intercom import is already running")
	}
	job := &model.SupportImportJob{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Source: model.SupportImportSourceIntercom, Status: model.SupportImportStatusPending,
		Since: since, MailboxID: mailboxID, Failures: json.RawMessage("[]"), StartedBy: userID,
	}
	payload, err := json.Marshal(storedIntercomImport{AccessToken: strings.TrimSpace(req.AccessToken), Region: req.Region})
	if err != nil {
		return nil, err
	}
	if job.PayloadEncrypted, err = appcrypto.EncryptStringWithAAD(string(payload), s.key, supportImportAAD(job.ID)); err != nil {
		return nil, fmt.Errorf("encrypt import token: %w", err)
	}
	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// ListJobs returns the workspace's recent imports.
func (s *SupportImportService) ListJobs(ctx context.Context, workspaceID string) ([]model.SupportImportJob, error) {
	jobs, err := s.repo.ListJobs(ctx, workspaceID)
	if jobs == nil {
		jobs = []model.SupportImportJob{}
	}
	return jobs, err
}

// Cancel stops an import; conversations already imported stay.
func (s *SupportImportService) Cancel(ctx context.Context, workspaceID, jobID string) error {
	return s.repo.CancelJob(ctx, workspaceID, jobID, s.now())
}

// supportImportMaxAttempts is how many times in a row a page may fail
// before the import stops.
const supportImportMaxAttempts = 8

// ProcessNext imports one page of the job that waited longest. It reports
// whether there was a job to work on.
func (s *SupportImportService) ProcessNext(ctx context.Context) (bool, error) {
	now := s.now()
	token := uuid.NewString()
	job, err := s.repo.ClaimJob(ctx, now, now.Add(supportImportLease), token)
	if err != nil || job == nil {
		return false, err
	}
	finish := func(status, message string) error {
		_, err := s.repo.FinishJob(context.WithoutCancel(ctx), job.ID, token, status, message, s.now())
		return err
	}
	raw, err := appcrypto.DecryptStringWithAAD(job.PayloadEncrypted, s.key, supportImportAAD(job.ID))
	if err != nil {
		return true, finish(model.SupportImportStatusFailed, "The stored access token could not be read.")
	}
	var stored storedIntercomImport
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return true, finish(model.SupportImportStatusFailed, "The stored import settings are invalid.")
	}
	client, err := s.client(stored.AccessToken, stored.Region)
	if err != nil {
		return true, finish(model.SupportImportStatusFailed, err.Error())
	}
	page, err := client.SearchConversations(ctx, job.Since, job.Cursor, supportImportPageSize)
	if err != nil {
		if ctx.Err() != nil {
			return true, nil // The lease returns the job after a restart.
		}
		return true, s.pageFailed(ctx, job, token, err)
	}
	ids := make([]string, 0, len(page.Conversations))
	for _, ref := range page.Conversations {
		ids = append(ids, ref.ID)
	}
	done, err := s.repo.ImportedIDs(ctx, job.WorkspaceID, model.SupportImportSourceIntercom, ids)
	if err != nil {
		return true, err
	}
	var failures []model.SupportImportFailure
	if err := json.Unmarshal(job.Failures, &failures); err != nil {
		failures = nil
	}
	importer := newIntercomConversationImporter(s, client, *job)
	imported, skipped, failed := 0, 0, 0
	renewAt := s.now().Add(supportImportLease / 2)
	for _, ref := range page.Conversations {
		if ctx.Err() != nil {
			return true, nil
		}
		if s.now().After(renewAt) {
			// Rate limits can stretch a page; keep the lease, and stop when the
			// import was cancelled or another worker took it over.
			held, err := s.repo.ExtendLease(ctx, job.ID, token, s.now().Add(supportImportLease))
			if err != nil || !held {
				return true, err
			}
			renewAt = s.now().Add(supportImportLease / 2)
		}
		if jobID, ok := done[ref.ID]; ok {
			// A resumed job finds its own conversations already stored.
			if jobID == job.ID {
				imported++
			} else {
				skipped++
			}
			continue
		}
		switch err := importer.importConversation(ctx, ref.ID); {
		case err == nil:
			imported++
		case errors.Is(err, repository.ErrSupportAlreadyImported):
			skipped++
		case ctx.Err() != nil:
			return true, nil
		default:
			failed++
			if len(failures) < model.SupportImportFailureLimit {
				failures = append(failures, model.SupportImportFailure{ExternalID: ref.ID, Error: truncateWebhookText(err.Error(), 300)})
			}
			slog.WarnContext(ctx, "intercom conversation import failed", "job_id", job.ID, "conversation", ref.ID, "error", err)
		}
	}
	failuresJSON, err := json.Marshal(failures)
	if err != nil || failures == nil {
		failuresJSON = json.RawMessage("[]")
	}
	fields := map[string]any{
		"total": max(page.TotalCount, job.Imported+job.Skipped+job.Failed+imported+skipped+failed), "imported": job.Imported + imported,
		"skipped": job.Skipped + skipped, "failed": job.Failed + failed, "failures": string(failuresJSON), "cursor": page.Next,
		"error": "", "attempts": 0,
	}
	if page.Next != "" {
		fields["locked_until"] = nil
	}
	// Counts are saved even when the import was cancelled during the page.
	// The last page keeps its lease until the job is finished.
	held, err := s.repo.SaveLeased(ctx, job.ID, token, fields)
	if err != nil || !held || page.Next != "" {
		return true, err
	}
	return true, finish(model.SupportImportStatusDone, "")
}

// pageFailed retries a page that could not be listed, with growing delays,
// and stops the import when retrying cannot help.
func (s *SupportImportService) pageFailed(ctx context.Context, job *model.SupportImportJob, token string, err error) error {
	switch {
	case errors.Is(err, intercom.ErrUnauthorized):
		return s.finishFailed(ctx, job, token, "Intercom rejected the access token.")
	case errors.Is(err, intercom.ErrForbidden):
		return s.finishFailed(ctx, job, token, "The access token cannot read conversations.")
	}
	var status *intercom.StatusError
	if errors.As(err, &status) && status.Status >= 400 && status.Status < 500 && status.Status != 429 {
		return s.finishFailed(ctx, job, token, "Intercom refused the request: "+truncateWebhookText(status.Message, 200))
	}
	attempts := job.Attempts + 1
	if attempts >= supportImportMaxAttempts {
		return s.finishFailed(ctx, job, token, "Intercom could not be reached; the import stopped after several tries.")
	}
	delay := time.Duration(1<<(attempts-1)) * time.Minute
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	retryAt := s.now().Add(delay)
	slog.WarnContext(ctx, "intercom conversation page failed", "job_id", job.ID, "attempt", attempts, "error", err)
	_, saveErr := s.repo.SaveLeased(ctx, job.ID, token, map[string]any{
		"locked_until": retryAt, "attempts": attempts, "error": "Intercom could not be reached; retrying.",
	})
	return saveErr
}

func (s *SupportImportService) finishFailed(ctx context.Context, job *model.SupportImportJob, token, message string) error {
	_, err := s.repo.FinishJob(context.WithoutCancel(ctx), job.ID, token, model.SupportImportStatusFailed, message, s.now())
	return err
}

// Run processes imports until ctx is cancelled.
func (s *SupportImportService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for i := 0; i < 40 && ctx.Err() == nil; i++ {
			worked, err := s.ProcessNext(ctx)
			if err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "process support import", "error", err)
			}
			if err != nil || !worked {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
