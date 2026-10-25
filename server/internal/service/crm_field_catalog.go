package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	_crmFieldCatalogDefaultValues = 20
	_crmFieldCatalogMaxValues     = 100
	_crmFieldCatalogMaxValueQuery = 120
)

// _crmFieldDescriptions explains native fields whose meaning or values are
// not obvious from the name.
var _crmFieldDescriptions = map[string]string{
	"owner_member_id": "Workspace member ID of the owner; list_workspace_members resolves names.",
	"name":            "Full name for contacts; company name for companies.",
	"external_id":     "The company's ID in the customer's own product, as sent by the SDK.",
	"first_seen_at":   "First product event tied to the record within retained event history. Empty when no events match.",
	"last_seen_at":    "Most recent product event tied to the record. Empty when no events match.",
	"sessions_30d":    "Distinct product sessions in the last 30 days, capped at 500. Empty when no events match.",
	"portal_access":   "Customer portal decision; empty means the workspace default applies.",
}

// _crmFieldEnumValues lists allowed values for enum fields.
var _crmFieldEnumValues = map[string][]string{
	"lifecycle_stage": {
		model.CRMLifecycleSubscriber, model.CRMLifecycleLead, model.CRMLifecycleMarketingQualified,
		model.CRMLifecycleSalesQualified, model.CRMLifecycleOpportunity, model.CRMLifecycleCustomer,
		model.CRMLifecycleEvangelist,
	},
	"lead_status": {
		model.CRMLeadStatusNew, model.CRMLeadStatusOpen, model.CRMLeadStatusInProgress, model.CRMLeadStatusUnqualified,
	},
	"portal_access": {model.PortalAccessAllowed, model.PortalAccessBlocked},
}

// CRMFieldCatalogService describes the fields CRM list filters accept,
// including the custom property keys and values in use.
type CRMFieldCatalogService struct {
	companyRepo *repository.CRMCompanyRepository
	contacts    *CRMContactService
	behavior    *repository.CRMBehaviorProfileRepository
}

// NewCRMFieldCatalogService creates the service. behavior may be nil.
func NewCRMFieldCatalogService(
	companyRepo *repository.CRMCompanyRepository,
	contacts *CRMContactService,
	behavior *repository.CRMBehaviorProfileRepository,
) *CRMFieldCatalogService {
	return &CRMFieldCatalogService{companyRepo: companyRepo, contacts: contacts, behavior: behavior}
}

// CRMFieldCatalogInputError is a client-correctable catalog request error.
type CRMFieldCatalogInputError struct{ Message string }

func (e *CRMFieldCatalogInputError) Error() string { return e.Message }

// Catalog returns every filterable field for objectType: native fields with
// types, operators, and enum values, plus the custom property keys in use.
func (s *CRMFieldCatalogService) Catalog(ctx context.Context, workspaceID, objectType string) (*model.CRMFieldCatalog, error) {
	definitions, err := crmFieldDefinitions(objectType)
	if err != nil {
		return nil, err
	}
	sources, truncated, err := s.customPropertySources(ctx, workspaceID, objectType)
	if err != nil {
		return nil, err
	}
	catalog := &model.CRMFieldCatalog{
		ObjectType: objectType, Fields: crmNativeFieldDescriptors(definitions),
		ScannedRecords: len(sources), Truncated: truncated,
	}
	for _, summary := range buildCRMCustomPropertySummaries(sources) {
		catalog.CustomProperties = append(catalog.CustomProperties, model.CRMCustomPropertyDescriptor{
			CRMCustomPropertySummary: summary,
			Field:                    repository.CRMCustomPropertyFilterPrefix + summary.Key,
			Operators:                crmCustomPropertyOperatorsFor(summary.ValueType, summary.NumericCount),
		})
	}
	if catalog.CustomProperties == nil {
		catalog.CustomProperties = []model.CRMCustomPropertyDescriptor{}
	}
	if s.behavior != nil {
		catalog.Behavior, err = s.behavior.Coverage(ctx, workspaceID, objectType)
		if err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

// PropertyValues returns the values in use for custom_properties.<key>,
// optionally narrowed to values containing valueQuery.
func (s *CRMFieldCatalogService) PropertyValues(
	ctx context.Context, workspaceID string, req model.CRMFieldCatalogRequest,
) (*model.CRMCustomPropertyValues, error) {
	if _, err := crmFieldDefinitions(req.ObjectType); err != nil {
		return nil, err
	}
	key := strings.TrimPrefix(strings.TrimSpace(req.Key), repository.CRMCustomPropertyFilterPrefix)
	if !repository.ValidCRMCustomPropertyKey(key) {
		return nil, &CRMFieldCatalogInputError{Message: "key must be a custom property key: lowercase letters, digits, and underscores, starting with a letter"}
	}
	// The reserved enrichment namespace is internal; the catalog never lists it.
	if key == _crmUpsertReservedPropertyNS {
		return nil, &CRMFieldCatalogInputError{Message: fmt.Sprintf("%s is reserved and cannot be described", key)}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = _crmFieldCatalogDefaultValues
	}
	if limit > _crmFieldCatalogMaxValues {
		return nil, &CRMFieldCatalogInputError{Message: fmt.Sprintf("limit must be between 1 and %d", _crmFieldCatalogMaxValues)}
	}
	valueQuery := strings.TrimSpace(req.ValueQuery)
	if len([]rune(valueQuery)) > _crmFieldCatalogMaxValueQuery {
		return nil, &CRMFieldCatalogInputError{Message: fmt.Sprintf("value_query must be at most %d characters", _crmFieldCatalogMaxValueQuery)}
	}
	sources, truncated, err := s.customPropertySources(ctx, workspaceID, req.ObjectType)
	if err != nil {
		return nil, err
	}
	result := &model.CRMCustomPropertyValues{
		ObjectType: req.ObjectType, Key: key, Field: repository.CRMCustomPropertyFilterPrefix + key,
		ValueQuery: valueQuery, ScannedRecords: len(sources), Truncated: truncated,
		Values: []model.CRMFilterValueCount{},
	}
	stats := &crmCustomPropertyStats{types: map[string]bool{}, values: newCRMFilterValueCounter()}
	needle := strings.ToLower(valueQuery)
	for _, record := range sources {
		valueType, values := crmCustomPropertyFilterValues(record[key])
		if valueType == "" {
			continue
		}
		stats.count++
		stats.types[valueType] = true
		for _, value := range values {
			if needle == "" || strings.Contains(strings.ToLower(value), needle) {
				stats.values.add(value)
			}
		}
		if number, ok := crmCustomPropertyNumber(record[key]); ok {
			stats.addNumber(number)
		}
	}
	if stats.count == 0 {
		return result, nil
	}
	result.Found, result.Count, result.ValueType = true, stats.count, stats.valueType()
	result.Values, result.HasMore = stats.values.topN(limit)
	result.Operators = crmCustomPropertyOperatorsFor(result.ValueType, stats.numericCount)
	return result, nil
}

// crmCustomPropertyOperatorsFor narrows the operators a custom property
// accepts to those that can match its values: list elements match only
// is_any_of and is_none_of, and numeric comparisons need numeric values.
func crmCustomPropertyOperatorsFor(valueType string, numericCount int) []model.QueryFilterOperator {
	listOperators := []model.QueryFilterOperator{
		model.QueryFilterOpIsAnyOf, model.QueryFilterOpIsNoneOf, model.QueryFilterOpIsEmpty, model.QueryFilterOpIsNotEmpty,
	}
	if valueType == "list" {
		return listOperators
	}
	var result []model.QueryFilterOperator
	for _, operator := range repository.CRMCustomPropertyOperators() {
		numeric := operator == model.QueryFilterOpGT || operator == model.QueryFilterOpGTE ||
			operator == model.QueryFilterOpLT || operator == model.QueryFilterOpLTE || operator == model.QueryFilterOpBetween
		if numeric && numericCount == 0 {
			continue
		}
		result = append(result, operator)
	}
	return result
}

func (s *CRMFieldCatalogService) customPropertySources(ctx context.Context, workspaceID, objectType string) ([]model.JSONB, bool, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, false, fmt.Errorf("workspace_id is required")
	}
	if objectType == model.CRMFieldObjectContact {
		return s.contacts.customPropertySources(ctx, workspaceID)
	}
	records, err := s.companyRepo.ListFilterSources(ctx, workspaceID, _crmFilterOptionsScanLimit+1)
	if err != nil {
		return nil, false, err
	}
	sources := make([]model.JSONB, 0, len(records))
	for _, record := range records {
		sources = append(sources, record.CustomProperties)
	}
	truncated := len(sources) > _crmFilterOptionsScanLimit
	if truncated {
		sources = sources[:_crmFilterOptionsScanLimit]
	}
	return sources, truncated, nil
}

func crmFieldDefinitions(objectType string) (querybuilder.Definitions, error) {
	definitions, ok := repository.CRMFilterFieldDefinitions(objectType)
	if !ok {
		return nil, &CRMFieldCatalogInputError{Message: "object_type must be contact or company"}
	}
	return definitions, nil
}

func crmNativeFieldDescriptors(definitions querybuilder.Definitions) []model.CRMFieldDescriptor {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]model.CRMFieldDescriptor, 0, len(names))
	for _, name := range names {
		definition := definitions[name]
		fields = append(fields, model.CRMFieldDescriptor{
			Field: name, Type: string(definition.Type), Operators: definition.AllowedOperators(),
			Values: _crmFieldEnumValues[name], Description: _crmFieldDescriptions[name],
		})
	}
	return fields
}
