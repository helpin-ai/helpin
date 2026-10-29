package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/customfields"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrCustomFieldInvalid wraps a message about a field or value that is safe
// to show.
var ErrCustomFieldInvalid = errors.New("invalid custom field")

// ErrCustomFieldExists means the key is already defined for the object.
var ErrCustomFieldExists = errors.New("a field with this key already exists")

// ErrCustomFieldRecordNotFound means the record to write values on does not
// exist in the workspace.
var ErrCustomFieldRecordNotFound = errors.New("record not found")

// customFieldUsageSample bounds how many recent records the field list scans
// to count usage and values that do not fit.
const customFieldUsageSample = 5000

// customFieldReservedKeys hold internal data and cannot be defined.
var customFieldReservedKeys = []string{_crmUpsertReservedPropertyNS}

var customFieldObjects = []string{model.CustomFieldObjectConversation, model.CustomFieldObjectCRMContact, model.CustomFieldObjectCRMCompany}

// CustomFieldService manages custom field definitions and validates the
// values people enter.
type CustomFieldService struct {
	repo *repository.CustomFieldRepository
	now  func() time.Time
}

// NewCustomFieldService creates the service.
func NewCustomFieldService(repo *repository.CustomFieldRepository) *CustomFieldService {
	return &CustomFieldService{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func invalidCustomField(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCustomFieldInvalid, fmt.Sprintf(format, args...))
}

func requireCustomFieldObject(objectType string) error {
	if !slices.Contains(customFieldObjects, objectType) {
		return invalidCustomField("unknown object type %q", objectType)
	}
	return nil
}

// List returns an object type's fields with labels, and usage counts from
// recent records when withUsage is set.
func (s *CustomFieldService) List(ctx context.Context, workspaceID, objectType string, includeArchived, withUsage bool) ([]model.CustomFieldDefinition, error) {
	if err := requireCustomFieldObject(objectType); err != nil {
		return nil, err
	}
	definitions, err := s.repo.ListDefinitions(ctx, workspaceID, objectType, includeArchived)
	if err != nil {
		return nil, err
	}
	if definitions == nil {
		definitions = []model.CustomFieldDefinition{}
	}
	for i := range definitions {
		definitions[i].DisplayLabel = customfields.DisplayLabel(definitions[i].Definition())
	}
	if !withUsage || len(definitions) == 0 {
		return definitions, nil
	}
	sample, err := s.repo.SampleValues(ctx, workspaceID, objectType, customFieldUsageSample)
	if err != nil {
		return nil, err
	}
	for i := range definitions {
		definitions[i].UsageSampled = len(sample) == customFieldUsageSample
		for _, values := range sample {
			value, ok := values[definitions[i].Key]
			if !ok || value == nil {
				continue
			}
			definitions[i].UsageCount++
			if !customfields.Fits(definitions[i].Definition(), value) {
				definitions[i].MismatchCount++
			}
		}
	}
	return definitions, nil
}

// Active returns an object type's active fields by key.
func (s *CustomFieldService) Active(ctx context.Context, workspaceID, objectType string) (map[string]model.CustomFieldDefinition, error) {
	definitions, err := s.repo.ListDefinitions(ctx, workspaceID, objectType, false)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]model.CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		definition.DisplayLabel = customfields.DisplayLabel(definition.Definition())
		byKey[definition.Key] = definition
	}
	return byKey, nil
}

func (s *CustomFieldService) normalize(req model.CustomFieldDefinitionRequest, definition *model.CustomFieldDefinition) error {
	label := strings.Join(strings.Fields(req.Label), " ")
	if utf8.RuneCountInString(label) > 80 {
		return invalidCustomField("labels can be at most 80 characters")
	}
	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > 300 {
		return invalidCustomField("descriptions can be at most 300 characters")
	}
	if !customfields.ValidType(req.FieldType) {
		return invalidCustomField("choose a field type")
	}
	options := model.CustomFieldOptions{}
	if req.FieldType == customfields.TypeSelect || req.FieldType == customfields.TypeMultiSelect {
		normalized, err := customfields.NormalizeOptions(req.Options)
		if err != nil {
			return invalidCustomField("%s", strings.TrimPrefix(err.Error(), customfields.ErrInvalid.Error()+": "))
		}
		if len(normalized) == 0 {
			return invalidCustomField("add at least one option")
		}
		options = normalized
	}
	definition.Label, definition.Description, definition.FieldType, definition.Options = label, description, req.FieldType, options
	return nil
}

// Create defines a new field. Its key cannot change later.
func (s *CustomFieldService) Create(ctx context.Context, workspaceID, actorID, objectType string, req model.CustomFieldDefinitionRequest) (*model.CustomFieldDefinition, error) {
	if err := requireCustomFieldObject(objectType); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.Key)
	if !customfields.ValidKey(key) {
		return nil, invalidCustomField("keys use lowercase letters, digits and underscores, start with a letter, and have at most 64 characters")
	}
	if slices.Contains(customFieldReservedKeys, key) {
		return nil, invalidCustomField("%q is reserved", key)
	}
	existing, err := s.repo.ListDefinitions(ctx, workspaceID, objectType, true)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(existing, func(definition model.CustomFieldDefinition) bool { return definition.Key == key }) {
		return nil, ErrCustomFieldExists
	}
	if len(existing) >= model.CustomFieldDefinitionLimit {
		return nil, invalidCustomField("an object can have up to %d fields", model.CustomFieldDefinitionLimit)
	}
	definition := &model.CustomFieldDefinition{ID: uuid.NewString(), WorkspaceID: workspaceID, ObjectType: objectType, Key: key}
	if strings.TrimSpace(actorID) != "" {
		definition.CreatedByID = &actorID
	}
	if err := s.normalize(req, definition); err != nil {
		return nil, err
	}
	if err := s.repo.CreateDefinition(ctx, definition); errors.Is(err, repository.ErrCustomFieldKeyTaken) {
		return nil, ErrCustomFieldExists
	} else if err != nil {
		return nil, err
	}
	definition.DisplayLabel = customfields.DisplayLabel(definition.Definition())
	return definition, nil
}

// field returns a field of the object type, or not found.
func (s *CustomFieldService) field(ctx context.Context, workspaceID, objectType, id string) (*model.CustomFieldDefinition, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, repository.ErrCustomFieldNotFound
	}
	definition, err := s.repo.GetDefinition(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if definition.ObjectType != objectType {
		return nil, repository.ErrCustomFieldNotFound
	}
	return definition, nil
}

// Update changes a field's label, description, type and options. Stored
// values never change; values that no longer fit count as empty.
func (s *CustomFieldService) Update(ctx context.Context, workspaceID, objectType, id string, req model.CustomFieldDefinitionRequest) (*model.CustomFieldDefinition, error) {
	definition, err := s.field(ctx, workspaceID, objectType, id)
	if err != nil {
		return nil, err
	}
	if err := s.normalize(req, definition); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateDefinition(ctx, definition); err != nil {
		return nil, err
	}
	definition.DisplayLabel = customfields.DisplayLabel(definition.Definition())
	return definition, nil
}

// SetArchived archives or restores a field; values stay on records.
func (s *CustomFieldService) SetArchived(ctx context.Context, workspaceID, objectType, id string, archived bool) error {
	if _, err := s.field(ctx, workspaceID, objectType, id); err != nil {
		return err
	}
	var at *time.Time
	if archived {
		now := s.now()
		at = &now
	}
	return s.repo.SetArchived(ctx, workspaceID, id, at)
}

// Reorder sets the display order of an object type's fields.
func (s *CustomFieldService) Reorder(ctx context.Context, workspaceID, objectType string, ids []string) ([]model.CustomFieldDefinition, error) {
	if err := requireCustomFieldObject(objectType); err != nil {
		return nil, err
	}
	if err := s.repo.ReorderDefinitions(ctx, workspaceID, objectType, ids); err != nil {
		return nil, err
	}
	return s.List(ctx, workspaceID, objectType, true, false)
}

// ValidateValues checks values a person entered for defined, active fields
// and returns them in canonical form; a nil value clears the key.
func (s *CustomFieldService) ValidateValues(ctx context.Context, workspaceID, objectType string, values map[string]any) (map[string]any, error) {
	active, err := s.Active(ctx, workspaceID, objectType)
	if err != nil {
		return nil, err
	}
	return validateCustomFieldValues(active, values)
}

func validateCustomFieldValues(active map[string]model.CustomFieldDefinition, values map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(values))
	for key, raw := range values {
		definition, ok := active[key]
		if !ok {
			return nil, invalidCustomField("%q is not a field", key)
		}
		value, err := customfields.Validate(definition.Definition(), raw)
		if err != nil {
			return nil, invalidCustomField("%s", strings.TrimPrefix(err.Error(), customfields.ErrInvalid.Error()+": "))
		}
		normalized[key] = value
	}
	return normalized, nil
}

// DateKeys returns the keys of objectType defined as active date fields when
// group filters on a custom property; nil otherwise. A nil service has none.
func (s *CustomFieldService) DateKeys(ctx context.Context, workspaceID, objectType string, group *model.QueryFilterGroup) (map[string]bool, error) {
	if s == nil || group == nil || !slices.ContainsFunc(group.Rules, func(rule model.QueryFilterRule) bool {
		return strings.HasPrefix(rule.Field, repository.CRMCustomPropertyFilterPrefix)
	}) {
		return nil, nil
	}
	active, err := s.Active(ctx, workspaceID, objectType)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for key, definition := range active {
		if definition.FieldType == customfields.TypeDate {
			keys[key] = true
		}
	}
	return keys, nil
}

// SetValues validates values for defined, active fields and writes them on
// one record key by key.
func (s *CustomFieldService) SetValues(ctx context.Context, workspaceID, objectType, recordID string, values map[string]any) error {
	if err := requireCustomFieldObject(objectType); err != nil {
		return err
	}
	if len(values) == 0 {
		return invalidCustomField("no values to save")
	}
	normalized, err := s.ValidateValues(ctx, workspaceID, objectType, values)
	if err != nil {
		return err
	}
	found, err := s.repo.SetValues(ctx, workspaceID, objectType, recordID, normalized)
	if err != nil {
		return err
	}
	if !found {
		return ErrCustomFieldRecordNotFound
	}
	return nil
}
