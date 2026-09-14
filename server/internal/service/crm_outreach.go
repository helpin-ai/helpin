package service

import (
	"context"
	"fmt"
	"html"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type outreachAccess interface {
	ResolveActor(context.Context, string, string) (*authorization.Actor, error)
	Can(*authorization.Actor, authorization.Permission) bool
	CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error)
}
type outreachMail interface {
	ValidateActionSender(context.Context, string, string, string) error
	SendActionEmail(context.Context, string, string, string, model.CRMPlaybookEmailAction) (*model.CRMEmailMessage, error)
	ReconcileActionEmail(context.Context, string, string, string, model.CRMPlaybookEmailAction) (bool, *string, error)
	LinkThreadDeal(context.Context, string, string, *string) error
}
type outreachTasks interface {
	Create(context.Context, model.CreateTaskRequest, string) (*model.TaskDetail, error)
}

// CRMOutreachService coordinates personalized email workflows and durable delivery.
type CRMOutreachService struct {
	repo     *repository.CRMOutreachRepository
	mail     outreachMail
	accounts *repository.CRMEmailRepository
	auth     outreachAccess
	tasks    outreachTasks
	appURL   string
	now      func() time.Time
}

// NewCRMOutreachService binds workflow persistence, authorization, native email and task services.
func NewCRMOutreachService(repo *repository.CRMOutreachRepository, email *CRMEmailService, accounts *repository.CRMEmailRepository, auth outreachAccess, tasks outreachTasks, appURL string) *CRMOutreachService {
	return &CRMOutreachService{repo: repo, mail: email, accounts: accounts, auth: auth, tasks: tasks, appURL: strings.TrimRight(appURL, "/"), now: time.Now}
}
func (s *CRMOutreachService) access(ctx context.Context, ws, user string) (context.Context, error) {
	if s.auth == nil {
		return ctx, fmt.Errorf("CRM authorization unavailable")
	}
	actor, err := s.auth.ResolveActor(ctx, ws, user)
	if err != nil {
		return ctx, err
	}
	allowed, err := s.auth.CanAccessModule(ctx, actor, model.ModuleCRM)
	if err != nil {
		return ctx, err
	}
	if !allowed || !s.auth.Can(actor, authorization.PermCRMEdit) {
		return ctx, fmt.Errorf("CRM edit access is required")
	}
	return authorization.WithActor(ctx, actor), nil
}

var outreachVariable = regexp.MustCompile(`\{\{\s*([a-z_]+)(?:\|([^{}]*))?\s*\}\}`)
var outreachVariables = map[string]bool{"first_name": true, "last_name": true, "full_name": true, "email": true, "company": true, "deal_name": true, "sender_email": true}

// RenderCRMEmail substitutes supported variables and rejects unresolved values.
func RenderCRMEmail(value string, values map[string]string, escape bool) (string, error) {
	var issue error
	result := outreachVariable.ReplaceAllStringFunc(value, func(token string) string {
		parts := outreachVariable.FindStringSubmatch(token)
		key := strings.TrimSpace(parts[1])
		if !outreachVariables[key] {
			issue = fmt.Errorf("unknown variable: %s", key)
			return token
		}
		replacement := strings.TrimSpace(values[key])
		if replacement == "" && strings.Contains(token, "|") {
			replacement = strings.TrimSpace(parts[2])
		}
		if replacement == "" {
			issue = fmt.Errorf("missing %s; add a fallback or update the contact", key)
			return token
		}
		if escape {
			return html.EscapeString(replacement)
		}
		return replacement
	})
	if strings.Contains(result, "{{") && issue == nil {
		issue = fmt.Errorf("invalid personalization variable")
	}
	return result, issue
}
func validateEmailContent(subject, body string) error {
	if len(subject) > 500 || len(body) > 200000 {
		return fmt.Errorf("email content is too long")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("email subject must be a single line")
	}
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(body, "")) == "" {
		return fmt.Errorf("add a subject and message")
	}
	values := map[string]string{}
	for key := range outreachVariables {
		values[key] = "Preview"
	}
	if _, err := RenderCRMEmail(subject, values, false); err != nil {
		return err
	}
	_, err := RenderCRMEmail(body, values, true)
	return err
}

// Templates lists templates visible to the requesting workspace member.
func (s *CRMOutreachService) Templates(ctx context.Context, ws, user string) ([]model.CRMEmailTemplate, error) {
	return s.repo.Templates(ctx, ws, user)
}

// SaveTemplate creates or updates an owned template with version checking.
func (s *CRMOutreachService) SaveTemplate(ctx context.Context, ws, user, id string, row model.CRMEmailTemplate) (*model.CRMEmailTemplate, error) {
	if _, err := s.access(ctx, ws, user); err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Name) == "" || len(row.Name) > 150 {
		return nil, fmt.Errorf("add a template name (150 characters or fewer)")
	}
	if err := validateEmailContent(row.Subject, row.BodyHTML); err != nil {
		return nil, err
	}
	version := row.Version
	if id == "" {
		id = uuid.NewString()
		version = 0
	}
	row.ID = id
	row.WorkspaceID = ws
	row.OwnerID = user
	row.Version = version + 1
	row.UpdatedAt = s.now()
	if version == 0 {
		row.CreatedAt = s.now()
	}
	return &row, s.repo.SaveTemplate(ctx, &row, version)
}

// DeleteTemplate removes a template owned by the requester.
func (s *CRMOutreachService) DeleteTemplate(ctx context.Context, ws, user, id string) error {
	if _, err := s.access(ctx, ws, user); err != nil {
		return err
	}
	return s.repo.DeleteTemplate(ctx, ws, user, id)
}

// Sequences lists sequence definitions in a workspace.
func (s *CRMOutreachService) Sequences(ctx context.Context, ws string) ([]model.CRMEmailSequence, error) {
	return s.repo.Sequences(ctx, ws)
}

// SaveSequence saves a sequence with version checking and enrollment rule validation.
func (s *CRMOutreachService) SaveSequence(ctx context.Context, ws, user, id string, row model.CRMEmailSequence) (*model.CRMEmailSequence, error) {
	if _, err := s.access(ctx, ws, user); err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.Name) == "" || len(row.Name) > 150 {
		return nil, fmt.Errorf("add a sequence name (150 characters or fewer)")
	}
	if row.Status != "draft" && row.Status != "active" && row.Status != "paused" && row.Status != "archived" {
		return nil, fmt.Errorf("invalid sequence status")
	}
	if len(row.Steps) > 20 || row.Status == "active" && len(row.Steps) == 0 {
		return nil, fmt.Errorf("a sequence needs 1–20 steps")
	}
	if _, err := time.LoadLocation(row.Timezone); err != nil {
		return nil, fmt.Errorf("choose a valid timezone")
	}
	if row.StartHour < 0 || row.EndHour > 24 || row.StartHour >= row.EndHour {
		return nil, fmt.Errorf("choose a valid sending window")
	}
	for i, step := range row.Steps {
		if step.DelayDays < 0 || step.DelayDays > 365 {
			return nil, fmt.Errorf("step %d: delay must be 0–365 days", i+1)
		}
		if step.Kind == "email" {
			if step.Mode != "automatic" && step.Mode != "review" {
				return nil, fmt.Errorf("choose automatic or review for step %d", i+1)
			}
			if row.Status != "draft" {
				if err := validateEmailContent(step.Subject, step.BodyHTML); err != nil {
					return nil, fmt.Errorf("step %d: %w", i+1, err)
				}
			}
		} else if step.Kind == "task" {
			if row.Status != "draft" && (step.TaskName == "" || step.TeamID == "") {
				return nil, fmt.Errorf("step %d: add a task title and team", i+1)
			}
		} else {
			return nil, fmt.Errorf("unsupported step type")
		}
	}
	version := row.Version
	var previous *model.CRMEmailSequence
	if id != "" {
		var err error
		previous, err = s.repo.Sequence(ctx, ws, id)
		if err != nil {
			return nil, err
		}
		if previous.OwnerID != user {
			return nil, fmt.Errorf("only the sequence owner can edit it")
		}
	} else {
		id = uuid.NewString()
		version = 0
	}
	if row.EntryStageID != "" {
		if err := s.mail.ValidateActionSender(ctx, ws, row.EntryAccountID, user); err != nil {
			return nil, err
		}
		if err := s.repo.ValidateStage(ctx, ws, row.EntryStageID); err != nil {
			return nil, err
		}
		if previous == nil || previous.EntryStageID != row.EntryStageID || previous.EntryAccountID != row.EntryAccountID || (previous.Status != "active" && row.Status == "active") {
			now := s.now()
			row.EntryAfter = &now
			row.EntryCursorID = ""
			row.EntryError = ""
		} else {
			row.EntryAfter = previous.EntryAfter
			row.EntryCursorID = previous.EntryCursorID
			row.EntryError = previous.EntryError
		}
	} else {
		row.EntryAfter = nil
		row.EntryAccountID = ""
		row.EntryCursorID = ""
		row.EntryError = ""
	}
	row.ID = id
	row.OwnerID = user
	row.WorkspaceID = ws
	row.Version = version + 1
	row.UpdatedAt = s.now()
	if version == 0 {
		row.CreatedAt = s.now()
	}
	return &row, s.repo.SaveSequence(ctx, &row, version)
}

// Preview personalizes and validates every selected recipient before enrollment.
func (s *CRMOutreachService) Preview(ctx context.Context, ws, user, sequence string, req model.CRMSequenceEnrollRequest) ([]model.CRMSequencePreview, error) {
	if _, err := s.access(ctx, ws, user); err != nil {
		return nil, err
	}
	seq, err := s.repo.Sequence(ctx, ws, sequence)
	if err != nil {
		return nil, err
	}
	if seq.Status != "active" {
		return nil, fmt.Errorf("publish this sequence before enrolling contacts")
	}
	if req.Version != seq.Version {
		return nil, repository.ErrOutreachConflict
	}
	if err = s.mail.ValidateActionSender(ctx, ws, req.AccountID, user); err != nil {
		return nil, err
	}
	account, err := s.accounts.GetAccountByIDForWorkspace(ctx, ws, req.AccountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("mailbox not found")
	}
	dealName := ""
	if req.DealID != "" {
		deal, err := s.repo.Deal(ctx, ws, req.DealID)
		if err != nil {
			return nil, err
		}
		dealName = deal.Name
		if len(req.ContactIDs) == 0 {
			req.ContactIDs, err = s.repo.DealContacts(ctx, ws, req.DealID)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(req.ContactIDs) == 0 || len(req.ContactIDs) > 100 {
		return nil, fmt.Errorf("select 1–100 contacts")
	}
	rows := []model.CRMSequencePreview{}
	seen := map[string]bool{}
	for _, id := range req.ContactIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		p := model.CRMSequencePreview{ContactID: id}
		contact, err := s.repo.Contact(ctx, ws, id)
		if err != nil {
			p.Error = "Contact unavailable"
			rows = append(rows, p)
			continue
		}
		p.ContactName = strings.TrimSpace(contact.FirstName + " " + stringValue(contact.LastName))
		p.Email = strings.ToLower(strings.TrimSpace(stringValue(contact.Email)))
		if _, err := mail.ParseAddress(p.Email); err != nil || p.Email == "" {
			p.Error = "Add a valid contact email"
		} else if contact.EmailStatus == model.CRMContactEmailStatusInvalid {
			p.Error = "This email is marked undeliverable"
		} else {
			p.Error, err = s.repo.DuplicateOrSuppressed(ctx, ws, sequence, p.Email)
			if err != nil {
				return nil, err
			}
		}
		company := ""
		needsCompany := false
		for _, step := range seq.Steps {
			if strings.Contains(step.Subject+step.BodyHTML+step.TaskName, "company") {
				needsCompany = true
			}
		}
		if needsCompany {
			company, err = s.repo.CompanyName(ctx, ws, id)
			if err != nil {
				return nil, err
			}
		}
		values := map[string]string{"first_name": contact.FirstName, "last_name": stringValue(contact.LastName), "full_name": p.ContactName, "email": p.Email, "company": company, "deal_name": dealName, "sender_email": account.EmailAddress}
		for _, step := range seq.Steps {
			var renderErr error
			step.Subject, renderErr = RenderCRMEmail(step.Subject, values, false)
			if renderErr != nil && p.Error == "" {
				p.Error = renderErr.Error()
			}
			step.BodyHTML, renderErr = RenderCRMEmail(step.BodyHTML, values, true)
			if renderErr != nil && p.Error == "" {
				p.Error = renderErr.Error()
			}
			step.TaskName, renderErr = RenderCRMEmail(step.TaskName, values, false)
			if renderErr != nil && p.Error == "" {
				p.Error = renderErr.Error()
			}
			if seq.IncludeSignature && step.Kind == "email" && account.Signature != "" {
				step.BodyHTML += "<p><br></p><div>" + strings.ReplaceAll(html.EscapeString(account.Signature), "\n", "<br>") + "</div>"
			}
			p.Steps = append(p.Steps, step)
		}
		rows = append(rows, p)
	}
	return rows, nil
}

// Enroll creates recipient snapshots after validating the published version.
func (s *CRMOutreachService) Enroll(ctx context.Context, ws, user, sequence string, req model.CRMSequenceEnrollRequest) ([]model.CRMSequenceEnrollment, error) {
	previews, err := s.Preview(ctx, ws, user, sequence, req)
	if err != nil {
		return nil, err
	}
	seq, err := s.repo.Sequence(ctx, ws, sequence)
	if err != nil {
		return nil, err
	}
	if seq.Version != req.Version {
		return nil, repository.ErrOutreachConflict
	}
	rows := []model.CRMSequenceEnrollment{}
	now := s.now()
	for _, p := range previews {
		if p.Error != "" {
			return nil, fmt.Errorf("%s: %s", p.ContactName, p.Error)
		}
		row := model.CRMSequenceEnrollment{ID: uuid.NewString(), WorkspaceID: ws, SequenceID: sequence, SequenceName: seq.Name, SequenceVersion: seq.Version, ContactID: p.ContactID, ContactName: p.ContactName, Email: p.Email, DealID: req.DealID, AccountID: req.AccountID, OwnerID: user, Status: "active", Steps: p.Steps, Timezone: seq.Timezone, StartHour: seq.StartHour, EndHour: seq.EndHour, Weekdays: seq.Weekdays, UnsubscribeToken: uuid.NewString() + uuid.NewString(), CreatedAt: now, UpdatedAt: now}
		for i := range row.Steps {
			if row.Steps[i].Kind == "email" {
				row.Steps[i].BodyHTML += `<p style="font-size:12px"><a href="` + html.EscapeString(s.appURL+"/email-preferences/"+row.UnsubscribeToken) + `">Stop these emails</a></p>`
			}
		}
		row.NextAt = NextCRMSequenceWindow(now.AddDate(0, 0, row.Steps[0].DelayDays), row)
		row.StepCount = len(row.Steps)
		rows = append(rows, row)
	}
	return rows, s.repo.Enroll(ctx, rows)
}

// NextCRMSequenceWindow finds the next allowed local sending time, including weekend and DST changes.
func NextCRMSequenceWindow(at time.Time, row model.CRMSequenceEnrollment) time.Time {
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		loc = time.UTC
	}
	at = at.In(loc)
	for range 8 {
		day := at.Weekday()
		start := time.Date(at.Year(), at.Month(), at.Day(), row.StartHour, 0, 0, 0, loc)
		end := time.Date(at.Year(), at.Month(), at.Day(), row.EndHour, 0, 0, 0, loc)
		if !row.Weekdays || day != time.Saturday && day != time.Sunday {
			if at.Before(start) {
				return start.UTC()
			}
			if at.Before(end) {
				return at.UTC()
			}
		}
		at = time.Date(at.Year(), at.Month(), at.Day()+1, row.StartHour, 0, 0, 0, loc)
	}
	return at.UTC()
}

// Enrollments lists paginated recipient activity without full email bodies.
func (s *CRMOutreachService) Enrollments(ctx context.Context, ws, seq, contact, deal string, filters ...model.CRMSequenceEnrollmentFilter) ([]model.CRMSequenceEnrollment, error) {
	return s.repo.Enrollments(ctx, ws, seq, contact, deal, filters...)
}

// Detail loads a recipient snapshot and its delivery history.
func (s *CRMOutreachService) Detail(ctx context.Context, ws, id string) (*model.CRMSequenceEnrollment, []model.CRMSequenceDelivery, error) {
	row, err := s.repo.Enrollment(ctx, ws, id)
	if err != nil {
		return nil, nil, err
	}
	events, err := s.repo.Deliveries(ctx, ws, id)
	return row, events, err
}

// Control applies an owner-authorized recipient action.
func (s *CRMOutreachService) Control(ctx context.Context, ws, user, id, action string, review ...string) error {
	if _, err := s.access(ctx, ws, user); err != nil {
		return err
	}
	row, err := s.repo.Enrollment(ctx, ws, id)
	if err != nil {
		return err
	}
	if row.OwnerID != user {
		return fmt.Errorf("only the sending mailbox owner can manage this recipient")
	}
	status := ""
	switch action {
	case "pause":
		status = "paused"
	case "stop":
		status = "stopped"
	case "resume":
		if row.Status != "paused" && row.Status != "failed" {
			return fmt.Errorf("this recipient cannot be resumed")
		}
		status = "active"
	case "approve":
		if row.Status != "needs_review" {
			return fmt.Errorf("this email is not awaiting review")
		}
		status = "active"
		if row.StepIndex < 0 || row.StepIndex >= len(row.Steps) {
			return fmt.Errorf("invalid review step")
		}
		if len(review) == 2 {
			if err := validateEmailContent(review[0], review[1]); err != nil {
				return err
			}
			if strings.Contains(review[0], "{{") || strings.Contains(review[1], "{{") || strings.ContainsAny(review[0], "\r\n") {
				return fmt.Errorf("resolve personalization before approving")
			}
			// Keep the opt-out link even when the reviewed body is replaced.
			body := review[1]
			if !strings.Contains(body, row.UnsubscribeToken) {
				body += `<p><a href="` + html.EscapeString(strings.TrimRight(s.appURL, "/")+"/email-preferences/"+row.UnsubscribeToken) + `">Unsubscribe</a></p>`
			}
			row.Steps[row.StepIndex].Subject = review[0]
			row.Steps[row.StepIndex].BodyHTML = body
		}
		row.Steps[row.StepIndex].Mode = "automatic"
		if err := s.mail.ValidateActionSender(ctx, ws, row.AccountID, user); err != nil {
			return err
		}
		return s.repo.ApproveStep(ctx, row, s.now())
	case "check_delivery":
		return s.checkDelivery(ctx, row)
	default:
		return fmt.Errorf("unknown recipient action")
	}
	if status == "active" {
		if err := s.mail.ValidateActionSender(ctx, ws, row.AccountID, user); err != nil {
			return err
		}
	}
	return s.repo.Control(ctx, ws, id, user, status, s.now())
}

// Unsubscribe confirms or applies a recipient’s workspace opt-out.
func (s *CRMOutreachService) Unsubscribe(ctx context.Context, token string, apply bool) error {
	return s.repo.Unsubscribe(ctx, token, apply)
}

// RenderTemplate uses the same workspace-scoped contact data and renderer as sequences.
func (s *CRMOutreachService) RenderTemplate(ctx context.Context, ws, user, id, email, accountID, dealID string) (*model.CRMEmailTemplate, error) {
	if _, err := s.access(ctx, ws, user); err != nil {
		return nil, err
	}
	if err := s.mail.ValidateActionSender(ctx, ws, accountID, user); err != nil {
		return nil, err
	}
	row, err := s.repo.Template(ctx, ws, user, id)
	if err != nil {
		return nil, err
	}
	account, err := s.accounts.GetAccountByIDForWorkspace(ctx, ws, accountID)
	if err != nil {
		return nil, err
	}
	values := map[string]string{"email": email, "sender_email": account.EmailAddress}
	contact, err := s.repo.ContactByEmail(ctx, ws, email)
	if err != nil {
		return nil, err
	}
	if contact != nil {
		values["first_name"] = contact.FirstName
		values["last_name"] = stringValue(contact.LastName)
		values["full_name"] = strings.TrimSpace(contact.FirstName + " " + stringValue(contact.LastName))
		if strings.Contains(row.Subject+row.BodyHTML, "company") {
			values["company"], err = s.repo.CompanyName(ctx, ws, contact.ID)
			if err != nil {
				return nil, err
			}
		}
	}
	if dealID != "" {
		deal, err := s.repo.Deal(ctx, ws, dealID)
		if err != nil {
			return nil, err
		}
		values["deal_name"] = deal.Name
	}
	row.Subject, err = RenderCRMEmail(row.Subject, values, false)
	if err != nil {
		return nil, err
	}
	row.BodyHTML, err = RenderCRMEmail(row.BodyHTML, values, true)
	return row, err
}
