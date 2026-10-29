package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/customfields"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrSupportRequestTypeInvalid wraps a message about a request type or a
// submission that is safe to show.
var ErrSupportRequestTypeInvalid = errors.New("invalid request type")

func invalidRequestType(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSupportRequestTypeInvalid, fmt.Sprintf(format, args...))
}

// SupportRequestTypeService manages request types and checks the field
// values submitted with them.
type SupportRequestTypeService struct {
	repo      *repository.SupportRequestTypeRepository
	fields    *CustomFieldService
	mailboxes *repository.SupportMailboxRepository
}

// NewSupportRequestTypeService creates the service.
func NewSupportRequestTypeService(repo *repository.SupportRequestTypeRepository, fields *CustomFieldService, mailboxes *repository.SupportMailboxRepository) *SupportRequestTypeService {
	return &SupportRequestTypeService{repo: repo, fields: fields, mailboxes: mailboxes}
}

// List returns the workspace's request types in order with how many
// conversations carry each.
func (s *SupportRequestTypeService) List(ctx context.Context, workspaceID string) ([]model.SupportRequestType, error) {
	types, err := s.repo.List(ctx, workspaceID, false)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountConversations(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range types {
		types[i].ConversationCount = counts[types[i].ID]
	}
	if types == nil {
		types = []model.SupportRequestType{}
	}
	return types, nil
}

// Get returns one request type.
func (s *SupportRequestTypeService) Get(ctx context.Context, workspaceID, id string) (*model.SupportRequestType, error) {
	return s.repo.Get(ctx, workspaceID, id)
}

func (s *SupportRequestTypeService) normalize(ctx context.Context, workspaceID string, req model.SupportRequestTypeRequest, requestType *model.SupportRequestType) error {
	name := strings.Join(strings.Fields(req.Name), " ")
	if name == "" || utf8.RuneCountInString(name) > 60 {
		return invalidRequestType("name must be 1 to 60 characters")
	}
	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > 300 {
		return invalidRequestType("descriptions can be at most 300 characters")
	}
	if req.DefaultPriority != nil && *req.DefaultPriority != "" && !model.IsValidSupportConversationPriority(*req.DefaultPriority) {
		return ErrSupportInvalidPriority
	}
	var priority *string
	if req.DefaultPriority != nil && *req.DefaultPriority != "" {
		priority = req.DefaultPriority
	}
	var mailboxID *string
	if req.MailboxID != nil && *req.MailboxID != "" {
		if _, err := uuid.Parse(*req.MailboxID); err != nil {
			return invalidRequestType("unknown inbox")
		}
		mailbox, err := s.mailboxes.GetByID(ctx, workspaceID, *req.MailboxID)
		if err != nil {
			return err
		}
		if mailbox == nil || !mailbox.Active {
			return invalidRequestType("unknown inbox")
		}
		mailboxID = req.MailboxID
	}
	if len(req.Fields) > model.SupportRequestTypeFieldLimit {
		return invalidRequestType("a request type can have up to %d fields", model.SupportRequestTypeFieldLimit)
	}
	active, err := s.fields.Active(ctx, workspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return err
	}
	fields := model.SupportRequestTypeFields{}
	seen := map[string]bool{}
	for _, field := range req.Fields {
		if _, ok := active[field.Key]; !ok {
			return invalidRequestType("%q is not a conversation field", field.Key)
		}
		if seen[field.Key] {
			return invalidRequestType("%q is listed twice", field.Key)
		}
		seen[field.Key] = true
		help := strings.TrimSpace(field.HelpText)
		if utf8.RuneCountInString(help) > 300 {
			return invalidRequestType("help text can be at most 300 characters")
		}
		// Customers cannot fill in internal fields, so those are never required of them.
		fields = append(fields, model.SupportRequestTypeField{Key: field.Key, Required: field.Required && field.CustomerVisible, CustomerVisible: field.CustomerVisible, HelpText: help})
	}
	requestType.Name, requestType.Description, requestType.Fields = name, description, fields
	requestType.DefaultPriority, requestType.MailboxID, requestType.Enabled = priority, mailboxID, req.Enabled
	return nil
}

// Create adds a request type at the end of the list.
func (s *SupportRequestTypeService) Create(ctx context.Context, workspaceID string, req model.SupportRequestTypeRequest) (*model.SupportRequestType, error) {
	count, err := s.repo.Count(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if count >= model.SupportRequestTypeLimit {
		return nil, invalidRequestType("a workspace can have up to %d request types", model.SupportRequestTypeLimit)
	}
	requestType := &model.SupportRequestType{WorkspaceID: workspaceID}
	if err := s.normalize(ctx, workspaceID, req, requestType); err != nil {
		return nil, err
	}
	if taken, err := s.repo.NameTaken(ctx, workspaceID, requestType.Name, ""); err != nil {
		return nil, err
	} else if taken {
		return nil, repository.ErrSupportRequestTypeNameTaken
	}
	if err := s.repo.Create(ctx, requestType); err != nil {
		return nil, err
	}
	return requestType, nil
}

// Update replaces a request type's settings. Conversations keep their type
// and field values.
func (s *SupportRequestTypeService) Update(ctx context.Context, workspaceID, id string, req model.SupportRequestTypeRequest) (*model.SupportRequestType, error) {
	requestType, err := s.repo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if err := s.normalize(ctx, workspaceID, req, requestType); err != nil {
		return nil, err
	}
	if taken, err := s.repo.NameTaken(ctx, workspaceID, requestType.Name, id); err != nil {
		return nil, err
	} else if taken {
		return nil, repository.ErrSupportRequestTypeNameTaken
	}
	if err := s.repo.Update(ctx, requestType); err != nil {
		return nil, err
	}
	return requestType, nil
}

// Delete removes a request type no conversation carries.
func (s *SupportRequestTypeService) Delete(ctx context.Context, workspaceID, id string) error {
	if _, err := s.repo.Get(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, workspaceID, id)
}

// Reorder sets the order customers see request types in.
func (s *SupportRequestTypeService) Reorder(ctx context.Context, workspaceID string, ids []string) ([]model.SupportRequestType, error) {
	if err := s.repo.Reorder(ctx, workspaceID, ids); err != nil {
		return nil, err
	}
	return s.List(ctx, workspaceID)
}

// PortalRequestTypeField is a customer-visible field on the portal form.
type PortalRequestTypeField struct {
	Key       string                   `json:"key"`
	Label     string                   `json:"label"`
	FieldType string                   `json:"field_type"`
	Options   model.CustomFieldOptions `json:"options"`
	Required  bool                     `json:"required"`
	HelpText  string                   `json:"help_text"`
}

// PortalRequestType is an enabled request type as customers see it.
type PortalRequestType struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Fields      []PortalRequestTypeField `json:"fields"`
}

// ForPortal returns the enabled request types with their customer-visible
// fields; archived fields are left out.
func (s *SupportRequestTypeService) ForPortal(ctx context.Context, workspaceID string) ([]PortalRequestType, error) {
	types, err := s.repo.List(ctx, workspaceID, true)
	if err != nil {
		return nil, err
	}
	result := make([]PortalRequestType, 0, len(types))
	if len(types) == 0 {
		return result, nil
	}
	active, err := s.fields.Active(ctx, workspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return nil, err
	}
	for _, requestType := range types {
		portal := PortalRequestType{ID: requestType.ID, Name: requestType.Name, Description: requestType.Description, Fields: []PortalRequestTypeField{}}
		for _, field := range requestType.Fields {
			definition, ok := active[field.Key]
			if !ok || !field.CustomerVisible {
				continue
			}
			// The field's description is written for teammates, so only the
			// type's own help text reaches customers.
			portal.Fields = append(portal.Fields, PortalRequestTypeField{
				Key: field.Key, Label: definition.DisplayLabel, FieldType: definition.FieldType,
				Options: definition.Options, Required: field.Required, HelpText: field.HelpText,
			})
		}
		result = append(result, portal)
	}
	return result, nil
}

// PortalFieldValue is a customer-visible field value on a portal request.
type PortalFieldValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// CustomerView returns a conversation's request type name and the values
// of the fields that type shows customers, in the type's order. A nil
// service, or a conversation without a type, has none.
func (s *SupportRequestTypeService) CustomerView(ctx context.Context, workspaceID string, conversation *model.SupportConversation) (*string, []PortalFieldValue, error) {
	values := []PortalFieldValue{}
	if s == nil || conversation == nil || conversation.RequestTypeID == nil {
		return nil, values, nil
	}
	requestType, err := s.repo.Get(ctx, workspaceID, *conversation.RequestTypeID)
	if errors.Is(err, repository.ErrSupportRequestTypeNotFound) {
		return nil, values, nil
	}
	if err != nil {
		return nil, nil, err
	}
	active, err := s.fields.Active(ctx, workspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return nil, nil, err
	}
	for _, field := range requestType.Fields {
		definition, ok := active[field.Key]
		if !ok || !field.CustomerVisible {
			continue
		}
		if display := customfields.Display(definition.Definition(), conversation.CustomFields[field.Key]); display != "" {
			values = append(values, PortalFieldValue{Label: definition.DisplayLabel, Value: display})
		}
	}
	return &requestType.Name, values, nil
}

// ValidateSubmission checks the type and field values a customer submits
// with a new request. Only the type's customer-visible fields are accepted,
// required ones must be filled in, and values are returned in canonical
// form. An empty typeID is allowed only while no type is enabled.
func (s *SupportRequestTypeService) ValidateSubmission(ctx context.Context, workspaceID, typeID string, values map[string]any) (*model.SupportRequestType, map[string]any, error) {
	if s == nil {
		if typeID != "" || len(values) > 0 {
			return nil, nil, invalidRequestType("request types are not available")
		}
		return nil, nil, nil
	}
	if typeID == "" {
		enabled, err := s.repo.List(ctx, workspaceID, true)
		if err != nil {
			return nil, nil, err
		}
		if len(enabled) > 0 {
			return nil, nil, invalidRequestType("choose what your request is about")
		}
		if len(values) > 0 {
			return nil, nil, invalidRequestType("fields need a request type")
		}
		return nil, nil, nil
	}
	requestType, err := s.repo.Get(ctx, workspaceID, typeID)
	if errors.Is(err, repository.ErrSupportRequestTypeNotFound) || (err == nil && !requestType.Enabled) {
		return nil, nil, invalidRequestType("this request type is no longer available")
	}
	if err != nil {
		return nil, nil, err
	}
	active, err := s.fields.Active(ctx, workspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return nil, nil, err
	}
	visible := map[string]model.SupportRequestTypeField{}
	for _, field := range requestType.Fields {
		if _, ok := active[field.Key]; ok && field.CustomerVisible {
			visible[field.Key] = field
		}
	}
	for key := range values {
		if _, ok := visible[key]; !ok {
			return nil, nil, invalidRequestType("%q is not a field of this request type", key)
		}
	}
	normalized, err := validateCustomFieldValues(active, values)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrSupportRequestTypeInvalid, strings.TrimPrefix(err.Error(), ErrCustomFieldInvalid.Error()+": "))
	}
	for key, field := range visible {
		if field.Required && normalized[key] == nil {
			return nil, nil, invalidRequestType("%s is required", customfields.DisplayLabel(active[key].Definition()))
		}
	}
	for key, value := range normalized {
		if value == nil {
			delete(normalized, key)
		}
	}
	return requestType, normalized, nil
}
