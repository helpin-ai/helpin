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

// SupportCoverageDigestService handles weekly digest generation
// and delivery. Separated from core coverage to isolate email
// and user-lookup dependencies.
type SupportCoverageDigestService struct {
	coverageRepo  *repository.SupportCoverageRepository
	workspaceRepo *repository.WorkspaceRepository
	emailClient   EmailSender
	appBaseURL    string
	logger        *slog.Logger
}

// EmailSender is the interface for sending emails.
type EmailSender interface {
	SendEmail(to, subject, htmlBody, textBody string) error
}

// NewSupportCoverageDigestService creates a new digest service.
func NewSupportCoverageDigestService(
	coverageRepo *repository.SupportCoverageRepository,
	workspaceRepo *repository.WorkspaceRepository,
	emailClient EmailSender,
	appBaseURL string,
) *SupportCoverageDigestService {
	return &SupportCoverageDigestService{
		coverageRepo:  coverageRepo,
		workspaceRepo: workspaceRepo,
		emailClient:   emailClient,
		appBaseURL:    appBaseURL,
		logger:        slog.Default().With("service", "support_coverage_digest"),
	}
}

// SendWeeklyDigestForWorkspace generates and sends the weekly digest
// for a workspace. Skips if no meaningful gaps exist or digest was
// already sent this week.
func (s *SupportCoverageDigestService) SendWeeklyDigestForWorkspace(ctx context.Context, workspaceID string, weekStart time.Time) error {
	// Check if there are gaps worth reporting.
	summary, err := s.coverageRepo.GetSummary(ctx, workspaceID)
	if err != nil {
		return err
	}
	if summary.TotalOpenGaps == 0 && summary.NewGapsThisWeek == 0 {
		return nil // Nothing to report.
	}

	// Get workspace for name/slug.
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || ws == nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	// Get top gaps for the digest.
	topGaps, _, err := s.coverageRepo.ListGaps(ctx, workspaceID, model.SupportCoverageGapFilter{
		Status:  model.SupportCoverageGapStatusOpen,
		PerPage: 5,
	})
	if err != nil {
		return err
	}

	// Get recipients: workspace admins and owners.
	recipients, err := s.getDigestRecipients(ctx, workspaceID)
	if err != nil {
		return err
	}

	for _, recipient := range recipients {
		// Check if already sent this week.
		existing, err := s.coverageRepo.GetDigestDelivery(ctx, workspaceID, weekStart, recipient.ID)
		if err != nil {
			s.logger.Warn("check digest delivery failed", "error", err)
			continue
		}
		if existing != nil {
			continue // Already sent.
		}

		// Build and send email.
		subject := fmt.Sprintf("Docs coverage: %d support gaps found this week", summary.NewGapsThisWeek)
		htmlBody := s.buildDigestHTML(ws, summary, topGaps)
		textBody := s.buildDigestText(ws, summary, topGaps)

		if s.emailClient != nil {
			if err := s.emailClient.SendEmail(recipient.Email, subject, htmlBody, textBody); err != nil {
				s.logger.Warn("send digest email failed",
					"workspace_id", workspaceID, "recipient", recipient.Email, "error", err)
				continue
			}
		} else {
			s.logger.Info("digest email (no email client)",
				"workspace_id", workspaceID, "recipient", recipient.Email,
				"subject", subject)
		}

		// Record delivery to prevent duplicates.
		_ = s.coverageRepo.CreateDigestDelivery(ctx, &model.SupportCoverageDigestDelivery{
			WorkspaceID:     workspaceID,
			WeekStart:       weekStart,
			RecipientUserID: recipient.ID,
			SentAt:          time.Now(),
		})
	}

	return nil
}

type digestRecipient struct {
	ID    string
	Email string
}

func (s *SupportCoverageDigestService) getDigestRecipients(ctx context.Context, workspaceID string) ([]digestRecipient, error) {
	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var recipients []digestRecipient
	for _, m := range members {
		if m.Role == "admin" || m.Role == "owner" {
			recipients = append(recipients, digestRecipient{
				ID:    m.UserID,
				Email: m.Email,
			})
		}
	}
	return recipients, nil
}

func (s *SupportCoverageDigestService) buildDigestHTML(ws *model.Workspace, summary *model.SupportCoverageSummary, gaps []model.SupportCoverageGapListItem) string {
	var b strings.Builder
	b.WriteString("<h2>Docs Coverage Weekly Digest</h2>")
	b.WriteString(fmt.Sprintf("<p>Workspace: <strong>%s</strong></p>", ws.Name))
	b.WriteString("<table>")
	b.WriteString(fmt.Sprintf("<tr><td>New gaps this week</td><td><strong>%d</strong></td></tr>", summary.NewGapsThisWeek))
	b.WriteString(fmt.Sprintf("<tr><td>Total open gaps</td><td><strong>%d</strong></td></tr>", summary.TotalOpenGaps))
	b.WriteString(fmt.Sprintf("<tr><td>Gaps fixed this week</td><td><strong>%d</strong></td></tr>", summary.GapsFixedThisWeek))
	b.WriteString("</table>")

	if len(gaps) > 0 {
		b.WriteString("<h3>Top Gaps</h3><ul>")
		for _, g := range gaps {
			link := fmt.Sprintf("%s/w/%s/support/coverage?gap=%s", s.appBaseURL, ws.Slug, g.ID)
			b.WriteString(fmt.Sprintf("<li><a href=\"%s\">%s</a> — %s (%d conversations)</li>",
				link, g.Title, g.V1GapType, g.EvidenceCount))
		}
		b.WriteString("</ul>")
	}

	return b.String()
}

func (s *SupportCoverageDigestService) buildDigestText(ws *model.Workspace, summary *model.SupportCoverageSummary, gaps []model.SupportCoverageGapListItem) string {
	var b strings.Builder
	b.WriteString("Docs Coverage Weekly Digest\n\n")
	b.WriteString(fmt.Sprintf("Workspace: %s\n", ws.Name))
	b.WriteString(fmt.Sprintf("New gaps this week: %d\n", summary.NewGapsThisWeek))
	b.WriteString(fmt.Sprintf("Total open gaps: %d\n", summary.TotalOpenGaps))
	b.WriteString(fmt.Sprintf("Gaps fixed this week: %d\n\n", summary.GapsFixedThisWeek))

	if len(gaps) > 0 {
		b.WriteString("Top Gaps:\n")
		for _, g := range gaps {
			b.WriteString(fmt.Sprintf("- %s (%s, %d conversations)\n", g.Title, g.V1GapType, g.EvidenceCount))
		}
	}

	return b.String()
}
