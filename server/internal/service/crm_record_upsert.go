package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _crmUpsertNoteSource = "crm_record_upsert"

// CRMRecordUpsertService creates or updates CRM companies and contacts in
// bounded batches matched on one stable key, with per-record results.
type CRMRecordUpsertService struct {
	companyRepo *repository.CRMCompanyRepository
	contactRepo *repository.CRMContactRepository
	companies   *CRMCompanyService
	contacts    *CRMContactService
	activities  *CRMActivityService
}

// NewCRMRecordUpsertService creates the CRM record upsert service.
func NewCRMRecordUpsertService(
	companyRepo *repository.CRMCompanyRepository,
	contactRepo *repository.CRMContactRepository,
	companies *CRMCompanyService,
	contacts *CRMContactService,
	activities *CRMActivityService,
) *CRMRecordUpsertService {
	return &CRMRecordUpsertService{
		companyRepo: companyRepo, contactRepo: contactRepo,
		companies: companies, contacts: contacts, activities: activities,
	}
}

// CRMUpsertInputMessage returns the client-safe message when err is a
// request or record validation error.
func CRMUpsertInputMessage(err error) (string, bool) {
	var inputErr *crmUpsertInputError
	if errors.As(err, &inputErr) {
		return inputErr.message, true
	}
	return "", false
}

// Upsert applies one batch. Request-level problems return an error; problems
// with individual records are reported in that record's result.
func (s *CRMRecordUpsertService) Upsert(ctx context.Context, req model.CRMRecordUpsertRequest) (*model.CRMRecordUpsertResponse, error) {
	if err := normalizeCRMUpsertRequest(&req); err != nil {
		return nil, err
	}
	response := &model.CRMRecordUpsertResponse{
		ObjectType: req.ObjectType,
		Counts:     map[string]int{},
		Results:    make([]model.CRMRecordUpsertResult, 0, len(req.Records)),
	}
	seen := make(map[string]string, len(req.Records))
	for index, record := range req.Records {
		ref := strings.TrimSpace(record.Ref)
		if ref == "" {
			ref = fmt.Sprintf("#%d", index)
		}
		result, err := s.upsertRecord(ctx, req, record, ref, seen)
		if err != nil {
			result = model.CRMRecordUpsertResult{Ref: ref, ID: result.ID, Action: model.CRMUpsertActionError, Error: crmUpsertPublicError(ctx, req, err)}
		}
		response.Counts[result.Action]++
		response.Results = append(response.Results, result)
	}
	return response, nil
}

func normalizeCRMUpsertRequest(req *model.CRMRecordUpsertRequest) error {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	switch req.ObjectType {
	case model.CRMUpsertObjectCompany, model.CRMUpsertObjectContact:
	default:
		return crmUpsertInvalid("object_type must be company or contact")
	}
	if req.Mode == "" {
		req.Mode = model.CRMUpsertModeUpsert
	}
	switch req.Mode {
	case model.CRMUpsertModeUpsert, model.CRMUpsertModeCreateOnly, model.CRMUpsertModeUpdateOnly:
	default:
		return crmUpsertInvalid("mode must be upsert, create_only, or update_only")
	}
	if req.MatchOn == "" {
		req.MatchOn = model.CRMUpsertMatchDomain
		if req.ObjectType == model.CRMUpsertObjectContact {
			req.MatchOn = model.CRMUpsertMatchEmail
		}
	}
	allowed := map[string][]string{
		model.CRMUpsertObjectCompany: {model.CRMUpsertMatchID, model.CRMUpsertMatchExternalID, model.CRMUpsertMatchDomain},
		model.CRMUpsertObjectContact: {model.CRMUpsertMatchID, model.CRMUpsertMatchEmail},
	}[req.ObjectType]
	if !containsString(allowed, req.MatchOn) {
		return crmUpsertInvalid("match_on for %s records must be one of %s", req.ObjectType, strings.Join(allowed, ", "))
	}
	if req.MatchOn == model.CRMUpsertMatchID && req.Mode == model.CRMUpsertModeCreateOnly {
		return crmUpsertInvalid("match_on id cannot create records; use update_only or upsert")
	}
	if len(req.Records) == 0 || len(req.Records) > _crmUpsertMaxRecords {
		return crmUpsertInvalid("records must contain between 1 and %d items", _crmUpsertMaxRecords)
	}
	return nil
}

// crmUpsertTarget is the normalized input for one record.
type crmUpsertTarget struct {
	matchValue string
	fields     map[string]any
	properties map[string]any
	note       *model.CRMRecordUpsertNote
}

func (s *CRMRecordUpsertService) upsertRecord(
	ctx context.Context,
	req model.CRMRecordUpsertRequest,
	record model.CRMRecordUpsertInput,
	ref string,
	seen map[string]string,
) (model.CRMRecordUpsertResult, error) {
	result := model.CRMRecordUpsertResult{Ref: ref}
	target, err := normalizeCRMUpsertTarget(req, record)
	if err != nil {
		return result, err
	}
	matchKey := strings.ToLower(target.matchValue)
	if previous, duplicate := seen[matchKey]; duplicate {
		return result, crmUpsertInvalid("%s %q repeats record %s in this batch", req.MatchOn, target.matchValue, previous)
	}
	seen[matchKey] = ref
	if req.ObjectType == model.CRMUpsertObjectContact {
		return s.upsertContact(ctx, req, target, result)
	}
	return s.upsertCompany(ctx, req, target, result)
}

func normalizeCRMUpsertTarget(req model.CRMRecordUpsertRequest, record model.CRMRecordUpsertInput) (crmUpsertTarget, error) {
	fields, err := normalizeCRMUpsertFields(req.ObjectType, record.Fields)
	if err != nil {
		return crmUpsertTarget{}, err
	}
	properties, err := normalizeCRMUpsertCustomProperties(record.CustomProperties)
	if err != nil {
		return crmUpsertTarget{}, err
	}
	note, err := normalizeCRMUpsertNote(record.Note)
	if err != nil {
		return crmUpsertTarget{}, err
	}
	var matchValue string
	if req.MatchOn == model.CRMUpsertMatchID {
		matchValue = strings.TrimSpace(record.ID)
	} else if value, ok := fields[req.MatchOn].(string); ok {
		matchValue = value
	}
	if matchValue == "" {
		if req.MatchOn == model.CRMUpsertMatchID {
			return crmUpsertTarget{}, crmUpsertInvalid("id is required when match_on is id")
		}
		return crmUpsertTarget{}, crmUpsertInvalid("fields.%s is required when match_on is %s", req.MatchOn, req.MatchOn)
	}
	if len(fields) == 0 && len(properties) == 0 && note == nil {
		return crmUpsertTarget{}, crmUpsertInvalid("record has no fields, custom_properties, or note")
	}
	return crmUpsertTarget{matchValue: matchValue, fields: fields, properties: properties, note: note}, nil
}

func normalizeCRMUpsertNote(note *model.CRMRecordUpsertNote) (*model.CRMRecordUpsertNote, error) {
	if note == nil {
		return nil, nil
	}
	subject := strings.TrimSpace(note.Subject)
	body := strings.TrimSpace(note.Body)
	if body == "" {
		return nil, crmUpsertInvalid("note.body is required when note is given")
	}
	if len([]rune(subject)) > _crmUpsertMaxNoteSubject || len([]rune(body)) > _crmUpsertMaxNoteBody {
		return nil, crmUpsertInvalid("note subject or body is too long")
	}
	return &model.CRMRecordUpsertNote{Subject: subject, Body: body}, nil
}

func (s *CRMRecordUpsertService) upsertCompany(
	ctx context.Context,
	req model.CRMRecordUpsertRequest,
	target crmUpsertTarget,
	result model.CRMRecordUpsertResult,
) (model.CRMRecordUpsertResult, error) {
	existing, err := s.findCompany(ctx, req, target.matchValue)
	if err != nil {
		return result, err
	}
	if existing == nil {
		if req.Mode == model.CRMUpsertModeUpdateOnly {
			result.Action, result.Reason = model.CRMUpsertActionSkipped, "not_found"
			return result, nil
		}
		if _, ok := target.fields["name"]; !ok {
			return result, crmUpsertInvalid("fields.name is required to create a company")
		}
		var create model.CreateCRMCompanyRequest
		if err := remarshalCRMUpsert(crmUpsertCreatePayload(target), &create); err != nil {
			return result, err
		}
		create.WorkspaceID = req.WorkspaceID
		company, err := s.companies.Create(ctx, create)
		if err != nil {
			return result, err
		}
		result.ID, result.Action = company.ID, model.CRMUpsertActionCreated
		result.Changed = crmUpsertCreatedFields(target)
		return s.attachNote(ctx, req, target, result)
	}
	result.ID = existing.ID
	if req.Mode == model.CRMUpsertModeCreateOnly {
		result.Action, result.Reason = model.CRMUpsertActionSkipped, "exists"
		return result, nil
	}
	current, err := crmUpsertCurrentFields(existing)
	if err != nil {
		return result, err
	}
	plan := planCRMUpsertChanges(current, existing.CustomProperties, target.fields, target.properties, req.FillOnly)
	result.Changed, result.Kept = plan.changed, plan.kept
	result.Action = model.CRMUpsertActionUnchanged
	if len(plan.changed) > 0 {
		var update model.UpdateCRMCompanyRequest
		if err := remarshalCRMUpsert(plan.fields, &update); err != nil {
			return result, err
		}
		if plan.properties != nil {
			update.CustomProperties = plan.properties
		}
		if _, err := s.companies.Update(ctx, existing.ID, update); err != nil {
			return result, err
		}
		result.Action = model.CRMUpsertActionUpdated
	}
	return s.attachNote(ctx, req, target, result)
}

func (s *CRMRecordUpsertService) upsertContact(
	ctx context.Context,
	req model.CRMRecordUpsertRequest,
	target crmUpsertTarget,
	result model.CRMRecordUpsertResult,
) (model.CRMRecordUpsertResult, error) {
	existing, err := s.findContact(ctx, req, target.matchValue)
	if err != nil {
		return result, err
	}
	if existing == nil {
		if req.Mode == model.CRMUpsertModeUpdateOnly {
			result.Action, result.Reason = model.CRMUpsertActionSkipped, "not_found"
			return result, nil
		}
		if _, ok := target.fields["first_name"]; !ok {
			return result, crmUpsertInvalid("fields.first_name is required to create a contact")
		}
		var create model.CreateCRMContactRequest
		if err := remarshalCRMUpsert(crmUpsertCreatePayload(target), &create); err != nil {
			return result, err
		}
		create.WorkspaceID = req.WorkspaceID
		contact, err := s.contacts.Create(ctx, create)
		if err != nil {
			return result, err
		}
		result.ID, result.Action = contact.ID, model.CRMUpsertActionCreated
		result.Changed = crmUpsertCreatedFields(target)
		return s.attachNote(ctx, req, target, result)
	}
	result.ID = existing.ID
	if req.Mode == model.CRMUpsertModeCreateOnly {
		result.Action, result.Reason = model.CRMUpsertActionSkipped, "exists"
		return result, nil
	}
	current, err := crmUpsertCurrentFields(existing)
	if err != nil {
		return result, err
	}
	plan := planCRMUpsertChanges(current, existing.CustomProperties, target.fields, target.properties, req.FillOnly)
	result.Changed, result.Kept = plan.changed, plan.kept
	result.Action = model.CRMUpsertActionUnchanged
	if len(plan.changed) > 0 {
		var update model.UpdateCRMContactRequest
		if err := remarshalCRMUpsert(plan.fields, &update); err != nil {
			return result, err
		}
		if plan.properties != nil {
			update.CustomProperties = plan.properties
		}
		if _, err := s.contacts.UpdateWithActor(ctx, existing.ID, update, req.ActorUserID, ""); err != nil {
			return result, err
		}
		result.Action = model.CRMUpsertActionUpdated
	}
	return s.attachNote(ctx, req, target, result)
}

func (s *CRMRecordUpsertService) findCompany(ctx context.Context, req model.CRMRecordUpsertRequest, value string) (*model.CRMCompany, error) {
	var company *model.CRMCompany
	var err error
	switch req.MatchOn {
	case model.CRMUpsertMatchID:
		company, err = s.companyRepo.GetByID(ctx, value)
		if err == nil && (company == nil || company.WorkspaceID != req.WorkspaceID) {
			return nil, crmUpsertInvalid("company %q was not found", value)
		}
	case model.CRMUpsertMatchExternalID:
		company, err = s.companyRepo.GetByExternalID(ctx, req.WorkspaceID, value)
	default:
		company, err = s.companyRepo.GetByDomainVariants(ctx, req.WorkspaceID, crmDomainVariants(value))
	}
	return company, err
}

func (s *CRMRecordUpsertService) findContact(ctx context.Context, req model.CRMRecordUpsertRequest, value string) (*model.CRMContact, error) {
	if req.MatchOn == model.CRMUpsertMatchID {
		contact, err := s.contactRepo.GetByID(ctx, value)
		if err == nil && (contact == nil || contact.WorkspaceID != req.WorkspaceID) {
			return nil, crmUpsertInvalid("contact %q was not found", value)
		}
		return contact, err
	}
	return s.contactRepo.GetByEmail(ctx, req.WorkspaceID, value)
}

// attachNote logs the record's note once. A note whose subject and body were
// already logged on this record by the upsert is not repeated, so reruns of
// the same research batch stay quiet.
func (s *CRMRecordUpsertService) attachNote(
	ctx context.Context,
	req model.CRMRecordUpsertRequest,
	target crmUpsertTarget,
	result model.CRMRecordUpsertResult,
) (model.CRMRecordUpsertResult, error) {
	if target.note == nil || result.ID == "" {
		return result, nil
	}
	digest := sha256.Sum256([]byte(target.note.Subject + "\n" + target.note.Body))
	hash := hex.EncodeToString(digest[:])
	filters := model.CRMActivityListFilters{}
	create := model.CreateCRMActivityRequest{
		WorkspaceID: req.WorkspaceID, ActivityType: "note", Body: &target.note.Body,
		Metadata: map[string]interface{}{"source": _crmUpsertNoteSource, "content_hash": hash},
	}
	if target.note.Subject != "" {
		create.Subject = &target.note.Subject
	}
	if req.ObjectType == model.CRMUpsertObjectContact {
		filters.ContactID, create.ContactID = &result.ID, &result.ID
	} else {
		filters.CompanyID, create.CompanyID = &result.ID, &result.ID
	}
	if result.Action != model.CRMUpsertActionCreated {
		activities, _, err := s.activities.List(ctx, req.WorkspaceID, filters, model.PMPagination{Page: 1, PerPage: 200})
		if err != nil {
			return result, err
		}
		for _, activity := range activities {
			if activity.Metadata["content_hash"] == hash && activity.Metadata["source"] == _crmUpsertNoteSource {
				return result, nil
			}
		}
	}
	if _, err := s.activities.Create(ctx, create); err != nil {
		return result, err
	}
	result.NoteAdded = true
	if result.Action == model.CRMUpsertActionUnchanged {
		result.Action = model.CRMUpsertActionUpdated
	}
	return result, nil
}

func crmUpsertCreatePayload(target crmUpsertTarget) map[string]any {
	payload := make(map[string]any, len(target.fields)+1)
	for name, value := range target.fields {
		payload[name] = value
	}
	properties := make(map[string]any, len(target.properties))
	for key, value := range target.properties {
		if value != nil {
			properties[key] = value
		}
	}
	if len(properties) > 0 {
		payload["custom_properties"] = properties
	}
	return payload
}

func crmUpsertCreatedFields(target crmUpsertTarget) []string {
	changed := sortedCRMUpsertKeys(target.fields)
	for _, key := range sortedCRMUpsertKeys(target.properties) {
		if target.properties[key] != nil {
			changed = append(changed, "custom_properties."+key)
		}
	}
	return changed
}

// crmUpsertCurrentFields reads a record's native fields keyed by JSON name.
func crmUpsertCurrentFields(record any) (map[string]any, error) {
	current := map[string]any{}
	if err := remarshalCRMUpsert(record, &current); err != nil {
		return nil, err
	}
	return current, nil
}

func remarshalCRMUpsert(input, output any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode CRM upsert values: %w", err)
	}
	if err := json.Unmarshal(encoded, output); err != nil {
		return fmt.Errorf("decode CRM upsert values: %w", err)
	}
	return nil
}

// crmUpsertPublicError keeps internal failures out of tool results.
func crmUpsertPublicError(ctx context.Context, req model.CRMRecordUpsertRequest, err error) string {
	if message, ok := CRMUpsertInputMessage(err); ok {
		return message
	}
	slog.ErrorContext(ctx, "CRM record upsert failed", "error", err,
		"workspace_id", req.WorkspaceID, "object_type", req.ObjectType)
	return "the record could not be saved"
}
