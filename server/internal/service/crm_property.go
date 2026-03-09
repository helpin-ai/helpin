package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var validFieldTypes = map[string]bool{
	model.CRMFieldText:        true,
	model.CRMFieldNumber:      true,
	model.CRMFieldDate:        true,
	model.CRMFieldSelect:      true,
	model.CRMFieldMultiSelect: true,
	model.CRMFieldBoolean:     true,
	model.CRMFieldURL:         true,
	model.CRMFieldEmail:       true,
	model.CRMFieldPhone:       true,
	model.CRMFieldCurrency:    true,
}

var validObjectTypes = map[string]bool{
	"contact": true,
	"company": true,
	"deal":    true,
}

var internalNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CRMPropertyService contains CRM property business logic.
type CRMPropertyService struct {
	propertyRepo *repository.CRMPropertyRepository
}

// NewCRMPropertyService creates a new CRMPropertyService.
func NewCRMPropertyService(propertyRepo *repository.CRMPropertyRepository) *CRMPropertyService {
	return &CRMPropertyService{propertyRepo: propertyRepo}
}

// ── Property Definitions ──

// ListDefinitions returns property definitions for a workspace and optional object type.
func (s *CRMPropertyService) ListDefinitions(ctx context.Context, workspaceID, objectType string) ([]model.CRMPropertyDefinition, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.propertyRepo.ListDefinitions(ctx, workspaceID, objectType)
}

// GetDefinition returns a property definition by ID.
func (s *CRMPropertyService) GetDefinition(ctx context.Context, id string) (*model.CRMPropertyDefinition, error) {
	def, err := s.propertyRepo.GetDefinitionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("property definition not found")
	}
	return def, nil
}

// CreateDefinition creates a property definition.
func (s *CRMPropertyService) CreateDefinition(ctx context.Context, req model.CreateCRMPropertyDefinitionRequest) (*model.CRMPropertyDefinition, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Label) == "" {
		return nil, fmt.Errorf("workspace_id and label are required")
	}
	if !validObjectTypes[req.ObjectType] {
		return nil, fmt.Errorf("invalid object_type: %s", req.ObjectType)
	}
	if req.FieldType == "" {
		req.FieldType = model.CRMFieldText
	}
	if !validFieldTypes[req.FieldType] {
		return nil, fmt.Errorf("invalid field_type: %s", req.FieldType)
	}

	internalName := strings.TrimSpace(req.InternalName)
	if internalName == "" {
		return nil, fmt.Errorf("internal_name is required")
	}
	if !internalNameRegex.MatchString(internalName) {
		return nil, fmt.Errorf("internal_name must be lowercase alphanumeric with underscores, starting with a letter")
	}

	// Check uniqueness
	existing, err := s.propertyRepo.GetDefinitionByInternalName(ctx, req.WorkspaceID, req.ObjectType, internalName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("property with internal_name '%s' already exists for this object type", internalName)
	}

	def := &model.CRMPropertyDefinition{
		WorkspaceID:  req.WorkspaceID,
		ObjectType:   req.ObjectType,
		InternalName: internalName,
		Label:        strings.TrimSpace(req.Label),
		FieldType:    req.FieldType,
		Options:      model.JSONB(req.Options),
		GroupName:    req.GroupName,
	}
	if req.IsRequired != nil {
		def.IsRequired = *req.IsRequired
	}
	if req.Position != nil {
		def.Position = *req.Position
	}

	if err := s.propertyRepo.CreateDefinition(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// UpdateDefinition updates a property definition.
func (s *CRMPropertyService) UpdateDefinition(ctx context.Context, id string, req model.UpdateCRMPropertyDefinitionRequest) (*model.CRMPropertyDefinition, error) {
	def, err := s.propertyRepo.GetDefinitionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("property definition not found")
	}
	if def.IsSystem {
		return nil, fmt.Errorf("cannot modify system property")
	}

	if req.Label != nil {
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			return nil, fmt.Errorf("label cannot be empty")
		}
		def.Label = label
	}
	if req.FieldType != nil {
		if !validFieldTypes[*req.FieldType] {
			return nil, fmt.Errorf("invalid field_type: %s", *req.FieldType)
		}
		def.FieldType = *req.FieldType
	}
	if req.Options != nil {
		def.Options = model.JSONB(req.Options)
	}
	if req.GroupName != nil {
		def.GroupName = req.GroupName
	}
	if req.IsRequired != nil {
		def.IsRequired = *req.IsRequired
	}
	if req.Position != nil {
		def.Position = *req.Position
	}

	if err := s.propertyRepo.UpdateDefinition(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// DeleteDefinition removes a property definition.
func (s *CRMPropertyService) DeleteDefinition(ctx context.Context, id string) error {
	def, err := s.propertyRepo.GetDefinitionByID(ctx, id)
	if err != nil {
		return err
	}
	if def == nil {
		return fmt.Errorf("property definition not found")
	}
	if def.IsSystem {
		return fmt.Errorf("cannot delete system property")
	}
	return s.propertyRepo.DeleteDefinition(ctx, id)
}

// ── Property Groups ──

// ListGroups returns property groups for a workspace and optional object type.
func (s *CRMPropertyService) ListGroups(ctx context.Context, workspaceID, objectType string) ([]model.CRMPropertyGroup, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.propertyRepo.ListGroups(ctx, workspaceID, objectType)
}

// CreateGroup creates a property group.
func (s *CRMPropertyService) CreateGroup(ctx context.Context, req model.CreateCRMPropertyGroupRequest) (*model.CRMPropertyGroup, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if !validObjectTypes[req.ObjectType] {
		return nil, fmt.Errorf("invalid object_type: %s", req.ObjectType)
	}

	group := &model.CRMPropertyGroup{
		WorkspaceID: req.WorkspaceID,
		ObjectType:  req.ObjectType,
		Name:        strings.TrimSpace(req.Name),
	}
	if req.Position != nil {
		group.Position = *req.Position
	}

	if err := s.propertyRepo.CreateGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

// UpdateGroup updates a property group.
func (s *CRMPropertyService) UpdateGroup(ctx context.Context, id string, req model.UpdateCRMPropertyGroupRequest) (*model.CRMPropertyGroup, error) {
	group, err := s.propertyRepo.GetGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, fmt.Errorf("property group not found")
	}
	if group.IsSystem {
		return nil, fmt.Errorf("cannot modify system group")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		group.Name = name
	}
	if req.Position != nil {
		group.Position = *req.Position
	}

	if err := s.propertyRepo.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

// DeleteGroup removes a property group.
func (s *CRMPropertyService) DeleteGroup(ctx context.Context, id string) error {
	group, err := s.propertyRepo.GetGroupByID(ctx, id)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("property group not found")
	}
	if group.IsSystem {
		return fmt.Errorf("cannot delete system group")
	}
	return s.propertyRepo.DeleteGroup(ctx, id)
}
