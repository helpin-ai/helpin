package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrOutreachConflict signals a stale edit or lost execution lease.
var ErrOutreachConflict = errors.New("this item changed; refresh and try again")

// ErrOutreachRateLimit defers delivery when a mailbox reaches sequence sending capacity.
var ErrOutreachRateLimit = errors.New("mailbox sending limit reached")

// CRMOutreachRepository persists email workflows with workspace scoping and optimistic concurrency.
type CRMOutreachRepository struct{ db *gorm.DB }

// NewCRMOutreachRepository creates the workflow persistence boundary.
func NewCRMOutreachRepository(db *gorm.DB) *CRMOutreachRepository {
	return &CRMOutreachRepository{db: db}
}

// Templates lists templates visible to the requesting workspace member.
func (r *CRMOutreachRepository) Templates(ctx context.Context, ws, user string) ([]model.CRMEmailTemplate, error) {
	rows := []model.CRMEmailTemplate{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND (shared = ? OR owner_id = ?)", ws, true, user).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

// SaveTemplate creates or updates an owned template with version checking.
func (r *CRMOutreachRepository) SaveTemplate(ctx context.Context, row *model.CRMEmailTemplate, version int) error {
	if version == 0 {
		return r.db.WithContext(ctx).Create(row).Error
	}
	result := r.db.WithContext(ctx).Model(row).Where("workspace_id = ? AND owner_id = ? AND version = ?", row.WorkspaceID, row.OwnerID, version).Select("name", "subject", "body_html", "shared", "version", "updated_at").Updates(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOutreachConflict
	}
	return nil
}

// DeleteTemplate removes a template owned by the requester.
func (r *CRMOutreachRepository) DeleteTemplate(ctx context.Context, ws, user, id string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND owner_id = ? AND id = ?", ws, user, id).Delete(&model.CRMEmailTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Sequences lists sequence definitions in a workspace.
func (r *CRMOutreachRepository) Sequences(ctx context.Context, ws string) ([]model.CRMEmailSequence, error) {
	rows := []model.CRMEmailSequence{}
	err := r.db.WithContext(ctx).Where("workspace_id = ?", ws).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

// Sequence loads a workspace-scoped sequence.
func (r *CRMOutreachRepository) Sequence(ctx context.Context, ws, id string) (*model.CRMEmailSequence, error) {
	var row model.CRMEmailSequence
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", ws, id).First(&row).Error
	return &row, err
}

// SaveSequence saves a sequence with version checking and enrollment rule validation.
func (r *CRMOutreachRepository) SaveSequence(ctx context.Context, row *model.CRMEmailSequence, version int) error {
	if version == 0 {
		return r.db.WithContext(ctx).Create(row).Error
	}
	result := r.db.WithContext(ctx).Model(row).Where("workspace_id = ? AND owner_id = ? AND version = ?", row.WorkspaceID, row.OwnerID, version).Select("name", "status", "version", "steps", "timezone", "start_hour", "end_hour", "weekdays", "include_signature", "entry_stage_id", "entry_account_id", "entry_after", "entry_cursor_id", "entry_error", "updated_at").Updates(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOutreachConflict
	}
	return nil
}

// Contact loads a contact inside its workspace.
func (r *CRMOutreachRepository) Contact(ctx context.Context, ws, id string) (*model.CRMContact, error) {
	var row model.CRMContact
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", ws, id).First(&row).Error
	return &row, err
}

// CompanyName resolves the contact’s first linked workspace company.
func (r *CRMOutreachRepository) CompanyName(ctx context.Context, ws, contact string) (string, error) {
	var row struct{ Name string }
	err := r.db.WithContext(ctx).Table("crm_companies c").Select("c.name").Joins("JOIN crm_associations a ON (a.to_object_id = c.id AND a.to_object_type = 'company' AND a.from_object_type = 'contact' AND a.from_object_id = ?) OR (a.from_object_id = c.id AND a.from_object_type = 'company' AND a.to_object_type = 'contact' AND a.to_object_id = ?)", contact, contact).Where("c.workspace_id = ? AND a.workspace_id = ?", ws, ws).Order("a.created_at ASC").Limit(1).Scan(&row).Error
	return row.Name, err
}

// Deal loads a workspace-scoped deal.
func (r *CRMOutreachRepository) Deal(ctx context.Context, ws, id string) (*model.CRMDeal, error) {
	var row model.CRMDeal
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", ws, id).First(&row).Error
	return &row, err
}

// DealContacts lists contacts linked to a workspace deal.
func (r *CRMOutreachRepository) DealContacts(ctx context.Context, ws, deal string) ([]string, error) {
	rows := []model.CRMAssociation{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND ((from_object_type = 'deal' AND from_object_id = ? AND to_object_type = 'contact') OR (to_object_type = 'deal' AND to_object_id = ? AND from_object_type = 'contact'))", ws, deal, deal).Find(&rows).Error
	ids := []string{}
	for _, a := range rows {
		if a.FromObjectType == "contact" {
			ids = append(ids, a.FromObjectID)
		} else {
			ids = append(ids, a.ToObjectID)
		}
	}
	return ids, err
}

// DuplicateOrSuppressed reports duplicate enrollment or an existing opt-out.
func (r *CRMOutreachRepository) DuplicateOrSuppressed(ctx context.Context, ws, sequence, email string) (string, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailSuppression{}).Where("workspace_id = ? AND email = ?", ws, email).Count(&count).Error; err != nil {
		return "", err
	}
	if count > 0 {
		return "This contact has unsubscribed", nil
	}
	err := r.db.WithContext(ctx).Model(&model.CRMSequenceEnrollment{}).Where("workspace_id = ? AND sequence_id = ? AND email = ?", ws, sequence, email).Count(&count).Error
	if count > 0 {
		return "Already enrolled in this sequence", err
	}
	return "", err
}

// Enroll creates recipient snapshots after validating the published version.
func (r *CRMOutreachRepository) Enroll(ctx context.Context, rows []model.CRMSequenceEnrollment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			var seq model.CRMEmailSequence
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND status = 'active'", row.SequenceID, row.WorkspaceID).First(&seq).Error; err != nil {
				return err
			}
			if seq.Version != row.SequenceVersion {
				return ErrOutreachConflict
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("contact already enrolled or sequence unavailable: %w", err)
			}
		}
		return nil
	})
}

// Enrollments lists paginated recipient activity without full email bodies.
func (r *CRMOutreachRepository) Enrollments(ctx context.Context, ws, sequence, contact, deal string, filters ...model.CRMSequenceEnrollmentFilter) ([]model.CRMSequenceEnrollment, error) {
	q := r.db.WithContext(ctx).Where("workspace_id = ?", ws)
	if sequence != "" {
		q = q.Where("sequence_id = ?", sequence)
	}
	if contact != "" {
		q = q.Where("contact_id = ?", contact)
	}
	if deal != "" {
		q = q.Where("deal_id = ?", deal)
	}
	rows := []model.CRMSequenceEnrollment{}
	offset := 0
	if len(filters) > 0 {
		f := filters[0]
		if f.Page > 1 {
			offset = (f.Page - 1) * 50
		}
		if f.Status != "" && f.Status != "all" {
			q = q.Where("status = ?", f.Status)
		}
		if f.Search != "" {
			search := "%" + strings.ToLower(f.Search) + "%"
			q = q.Where("lower(contact_name) LIKE ? OR lower(email) LIKE ? OR lower(sequence_name) LIKE ?", search, search, search)
		}
	}
	err := q.Omit("steps").Order("created_at DESC, id DESC").Offset(offset).Limit(50).Find(&rows).Error
	return rows, err
}

// Enrollment loads one workspace-scoped recipient snapshot.
func (r *CRMOutreachRepository) Enrollment(ctx context.Context, ws, id string) (*model.CRMSequenceEnrollment, error) {
	var row model.CRMSequenceEnrollment
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", ws, id).First(&row).Error
	return &row, err
}

// Deliveries lists delivery intents for a workspace recipient.
func (r *CRMOutreachRepository) Deliveries(ctx context.Context, ws, id string) ([]model.CRMSequenceDelivery, error) {
	rows := []model.CRMSequenceDelivery{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND enrollment_id = ?", ws, id).Order("step_index").Find(&rows).Error
	return rows, err
}

// Claim claims one due recipient using an exclusive lease token.
func (r *CRMOutreachRepository) Claim(ctx context.Context, now time.Time) (*model.CRMSequenceEnrollment, error) {
	var row model.CRMSequenceEnrollment
	err := r.db.WithContext(ctx).Where("status IN ? AND next_at <= ? AND (lease_until IS NULL OR lease_until < ?)", []string{"active", "waiting_task", "sending"}, now, now).Order("next_at").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	until := now.Add(5 * time.Minute)
	token := uuid.NewString()
	result := r.db.WithContext(ctx).Model(&model.CRMSequenceEnrollment{}).Where("id = ? AND status = ? AND (lease_until IS NULL OR lease_until < ?)", row.ID, row.Status, now).Updates(map[string]any{"lease_until": until, "lease_token": token})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	row.LeaseUntil = &until
	row.LeaseToken = token
	return &row, nil
}

// Finish advances a recipient only while the caller still owns its lease.
func (r *CRMOutreachRepository) Finish(ctx context.Context, row *model.CRMSequenceEnrollment, status, reason string, next time.Time, index int) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSequenceEnrollment{}).Where("id = ? AND lease_token = ?", row.ID, row.LeaseToken).Updates(map[string]any{"status": status, "error": reason, "next_at": next, "step_index": index, "lease_until": nil, "lease_token": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOutreachConflict
	}
	return nil
}

// Control applies an owner-authorized recipient action.
func (r *CRMOutreachRepository) Control(ctx context.Context, ws, id, user, status string, next time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSequenceEnrollment{}).Where("workspace_id = ? AND id = ? AND owner_id = ? AND status NOT IN ?", ws, id, user, []string{"completed", "stopped", "replied", "unsubscribed", "bounced", "deal_closed"}).Updates(map[string]any{"status": status, "next_at": next, "lease_token": "", "lease_until": nil, "error": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOutreachConflict
	}
	return nil
}

// PrepareDelivery journals a step once and enforces mailbox sending capacity.
func (r *CRMOutreachRepository) PrepareDelivery(ctx context.Context, row *model.CRMSequenceEnrollment, d *model.CRMSequenceDelivery, now time.Time) (bool, error) {
	fresh := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.CRMSequenceDelivery
		err := tx.Where("enrollment_id = ? AND step_index = ?", row.ID, row.StepIndex).First(&existing).Error
		if err == nil {
			*d = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current model.CRMSequenceEnrollment
		if err := tx.Where("id = ? AND lease_token = ? AND status = 'active'", row.ID, row.LeaseToken).First(&current).Error; err != nil {
			return err
		}
		if d.Kind == "email" {
			var account model.CRMEmailAccount
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", row.AccountID).First(&account).Error; err != nil {
				return err
			}
			var minute, day int64
			if err := tx.Model(d).Where("account_id = ? AND kind = 'email' AND created_at > ?", row.AccountID, now.Add(-time.Minute)).Count(&minute).Error; err != nil {
				return err
			}
			if err := tx.Model(d).Where("account_id = ? AND kind = 'email' AND created_at > ?", row.AccountID, now.Add(-24*time.Hour)).Count(&day).Error; err != nil {
				return err
			}
			if minute > 0 || day >= 100 {
				return ErrOutreachRateLimit
			}
		}
		if err := tx.Create(d).Error; err != nil {
			return err
		}
		fresh = true
		return nil
	})
	return fresh, err
}

// SaveDelivery persists the observed outcome of a delivery intent.
func (r *CRMOutreachRepository) SaveDelivery(ctx context.Context, d *model.CRMSequenceDelivery) error {
	return r.db.WithContext(ctx).Model(d).Select("status", "result_id", "error", "updated_at").Updates(d).Error
}

// StopReason checks suppression, contact validity, replies and deal closure.
func (r *CRMOutreachRepository) StopReason(ctx context.Context, row *model.CRMSequenceEnrollment) (string, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailSuppression{}).Where("workspace_id = ? AND email = ?", row.WorkspaceID, row.Email).Count(&n).Error; err != nil {
		return "", err
	}
	if n > 0 {
		return "unsubscribed", nil
	}
	contact, err := r.Contact(ctx, row.WorkspaceID, row.ContactID)
	if err != nil {
		return "", err
	}
	if contact.EmailStatus == model.CRMContactEmailStatusInvalid {
		return "bounced", nil
	}
	if err := r.db.WithContext(ctx).Model(&model.CRMEmailMessage{}).Where("workspace_id = ? AND email_account_id = ? AND direction = 'inbound' AND lower(from_address) = ? AND sent_at >= ?", row.WorkspaceID, row.AccountID, row.Email, row.CreatedAt).Count(&n).Error; err != nil {
		return "", err
	}
	if n > 0 {
		return "replied", nil
	}
	if row.DealID != "" {
		if err := r.db.WithContext(ctx).Table("crm_deals d").Joins("JOIN crm_pipeline_stages s ON s.id = d.stage_id").Where("d.workspace_id = ? AND d.id = ? AND s.stage_type IN ?", row.WorkspaceID, row.DealID, []string{"won", "lost"}).Count(&n).Error; err != nil {
			return "", err
		}
		if n > 0 {
			return "deal_closed", nil
		}
	}
	return "", nil
}

// Unsubscribe confirms or applies a recipient’s workspace opt-out.
func (r *CRMOutreachRepository) Unsubscribe(ctx context.Context, token string, apply bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.CRMSequenceEnrollment
		if err := tx.Where("unsubscribe_token = ?", token).First(&row).Error; err != nil {
			return err
		}
		if !apply {
			return nil
		}
		suppression := model.CRMEmailSuppression{WorkspaceID: row.WorkspaceID, Email: row.Email, CreatedAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&suppression).Error; err != nil {
			return err
		}
		return tx.Model(&model.CRMSequenceEnrollment{}).Where("workspace_id = ? AND email = ? AND status NOT IN ?", row.WorkspaceID, row.Email, []string{"completed", "stopped", "replied"}).Updates(map[string]any{"status": "unsubscribed", "lease_token": "", "lease_until": nil}).Error
	})
}

// ValidateStage requires an open deal stage in the workspace.
func (r *CRMOutreachRepository) ValidateStage(ctx context.Context, ws, id string) error {
	var n int64
	err := r.db.WithContext(ctx).Table("crm_pipeline_stages s").Joins("JOIN crm_pipelines p ON p.id = s.pipeline_id").Where("p.workspace_id = ? AND s.id = ? AND s.stage_type NOT IN ?", ws, id, []string{"won", "lost"}).Count(&n).Error
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("choose an open stage in this workspace")
	}
	return nil
}

// ApproveStep atomically saves reviewed content and releases it for delivery.
func (r *CRMOutreachRepository) ApproveStep(ctx context.Context, row *model.CRMSequenceEnrollment, now time.Time) error {
	result := r.db.WithContext(ctx).Model(row).Where("workspace_id = ? AND owner_id = ? AND status = 'needs_review' AND step_index = ?", row.WorkspaceID, row.OwnerID, row.StepIndex).Updates(map[string]any{"steps": mustOutreachStepsJSON(row.Steps), "status": "active", "next_at": now, "error": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrOutreachConflict
	}
	return nil
}
func mustOutreachStepsJSON(steps []model.CRMSequenceStep) string {
	data, _ := json.Marshal(steps)
	return string(data)
}

// ResolveDelivery atomically records confirmed delivery and advances progress.
func (r *CRMOutreachRepository) ResolveDelivery(ctx context.Context, row *model.CRMSequenceEnrollment, d *model.CRMSequenceDelivery, next time.Time, index int, status string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(row).Where("status = 'uncertain' AND owner_id = ?", row.OwnerID).Updates(map[string]any{"status": status, "step_index": index, "next_at": next, "error": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrOutreachConflict
		}
		return tx.Model(d).Select("status", "result_id", "error", "updated_at").Updates(d).Error
	})
}

// NativeTask finds the task created by a stable sequence delivery identifier.
func (r *CRMOutreachRepository) NativeTask(ctx context.Context, ws, externalID string) (string, string, error) {
	var row struct {
		ID        string
		StateType string
	}
	err := r.db.WithContext(ctx).Table("pm_tasks t").Select("t.id, s.state_type").Joins("JOIN pm_workflow_states s ON s.id = t.workflow_state_id").Where("t.workspace_id = ? AND t.external_id = ?", ws, externalID).Limit(1).Scan(&row).Error
	return row.ID, row.StateType, err
}

// LinkTask links a native follow-up task to its CRM record.
func (r *CRMOutreachRepository) LinkTask(ctx context.Context, ws, task, contact, deal string) error {
	target, kind := contact, "contact"
	if deal != "" {
		target, kind = deal, "deal"
	}
	var n int64
	q := r.db.WithContext(ctx).Model(&model.CRMAssociation{}).Where("workspace_id = ? AND from_object_type = 'task' AND from_object_id = ? AND to_object_type = ? AND to_object_id = ?", ws, task, kind, target)
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&model.CRMAssociation{ID: uuid.NewString(), WorkspaceID: ws, FromObjectType: "task", FromObjectID: task, ToObjectType: kind, ToObjectID: target}).Error
}

// EntrySequences lists active rules ordered by their event cursor.
func (r *CRMOutreachRepository) EntrySequences(ctx context.Context) ([]model.CRMEmailSequence, error) {
	rows := []model.CRMEmailSequence{}
	err := r.db.WithContext(ctx).Where("status = 'active' AND entry_stage_id <> '' AND entry_after IS NOT NULL").Order("entry_after ASC").Find(&rows).Error
	return rows, err
}

// EntryEvents reads the next bounded batch of stage changes after a rule cursor.
func (r *CRMOutreachRepository) EntryEvents(ctx context.Context, seq model.CRMEmailSequence) ([]model.PMActivityLog, error) {
	rows := []model.PMActivityLog{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND entity_type = 'deal' AND event_type = 'deal.stage_changed' AND new_value = ? AND (created_at > ? OR (created_at = ? AND id > ?))", seq.WorkspaceID, seq.EntryStageID, seq.EntryAfter, seq.EntryAfter, seq.EntryCursorID).Order("created_at, id").Limit(25).Find(&rows).Error
	return rows, err
}

// PrimaryDealContact resolves the deal’s explicitly designated primary contact.
func (r *CRMOutreachRepository) PrimaryDealContact(ctx context.Context, ws, deal string) (string, error) {
	var row model.CRMAssociation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND association_label IN ? AND ((from_object_type = 'deal' AND from_object_id = ? AND to_object_type = 'contact') OR (to_object_type = 'deal' AND to_object_id = ? AND from_object_type = 'contact'))", ws, []string{"deal_primary_contact", "deal_customer"}, deal, deal).First(&row).Error
	if err != nil {
		return "", err
	}
	if row.FromObjectType == "contact" {
		return row.FromObjectID, nil
	}
	return row.ToObjectID, nil
}

// AdvanceEntry advances a stage rule cursor while its version is unchanged.
func (r *CRMOutreachRepository) AdvanceEntry(ctx context.Context, seq model.CRMEmailSequence, event model.PMActivityLog, issue string) error {
	return r.db.WithContext(ctx).Model(&model.CRMEmailSequence{}).Where("id = ? AND version = ?", seq.ID, seq.Version).Updates(map[string]any{"entry_after": event.CreatedAt, "entry_cursor_id": event.ID, "entry_error": issue}).Error
}

// Template loads one template after checking its visibility.
func (r *CRMOutreachRepository) Template(ctx context.Context, ws, user, id string) (*model.CRMEmailTemplate, error) {
	var row model.CRMEmailTemplate
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND (shared = ? OR owner_id = ?)", ws, id, true, user).First(&row).Error
	return &row, err
}

// ContactByEmail finds an exact workspace contact email without creating a contact.
func (r *CRMOutreachRepository) ContactByEmail(ctx context.Context, ws, email string) (*model.CRMContact, error) {
	var row model.CRMContact
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND lower(email) = ?", ws, strings.ToLower(email)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
