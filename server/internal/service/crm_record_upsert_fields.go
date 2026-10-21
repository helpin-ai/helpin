package service

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	_crmUpsertMaxRecords         = 100
	_crmUpsertMaxCustomKeys      = 50
	_crmUpsertMaxCustomString    = 2000
	_crmUpsertMaxCustomArray     = 50
	_crmUpsertMaxNoteSubject     = 500
	_crmUpsertMaxNoteBody        = 20000
	_crmUpsertReservedPropertyNS = "agent_enrichment"
)

type crmUpsertFieldKind int

const (
	crmUpsertText crmUpsertFieldKind = iota
	crmUpsertURL
	crmUpsertDomain
	crmUpsertEmail
	crmUpsertInt
	crmUpsertFloat
	crmUpsertEnum
)

type crmUpsertFieldSpec struct {
	kind   crmUpsertFieldKind
	maxLen int
	enum   []string
}

// _crmCompanyUpsertFields lists the native company fields the upsert may write.
// Keys match the JSON tags on the company create and update requests.
var _crmCompanyUpsertFields = map[string]crmUpsertFieldSpec{
	"name":           {kind: crmUpsertText, maxLen: 500},
	"domain":         {kind: crmUpsertDomain},
	"external_id":    {kind: crmUpsertText, maxLen: 255},
	"industry":       {kind: crmUpsertText, maxLen: 200},
	"employee_count": {kind: crmUpsertInt},
	"annual_revenue": {kind: crmUpsertFloat},
	"description":    {kind: crmUpsertText, maxLen: 10000},
	"logo_url":       {kind: crmUpsertURL},
	"linkedin_url":   {kind: crmUpsertURL},
	"headquarters":   {kind: crmUpsertText, maxLen: 500},
}

// _crmContactUpsertFields lists the native contact fields the upsert may write.
var _crmContactUpsertFields = map[string]crmUpsertFieldSpec{
	"first_name":       {kind: crmUpsertText, maxLen: 200},
	"last_name":        {kind: crmUpsertText, maxLen: 200},
	"email":            {kind: crmUpsertEmail},
	"phone":            {kind: crmUpsertText, maxLen: 50},
	"job_title":        {kind: crmUpsertText, maxLen: 300},
	"description":      {kind: crmUpsertText, maxLen: 10000},
	"linkedin_url":     {kind: crmUpsertURL},
	"primary_location": {kind: crmUpsertText, maxLen: 500},
	"country_code":     {kind: crmUpsertText, maxLen: 2},
	"country_name":     {kind: crmUpsertText, maxLen: 200},
	"source":           {kind: crmUpsertText, maxLen: 100},
	"lifecycle_stage": {kind: crmUpsertEnum, enum: []string{
		model.CRMLifecycleSubscriber, model.CRMLifecycleLead, model.CRMLifecycleMarketingQualified,
		model.CRMLifecycleSalesQualified, model.CRMLifecycleOpportunity, model.CRMLifecycleCustomer,
		model.CRMLifecycleEvangelist,
	}},
	"lead_status": {kind: crmUpsertEnum, enum: []string{
		model.CRMLeadStatusNew, model.CRMLeadStatusOpen, model.CRMLeadStatusInProgress, model.CRMLeadStatusUnqualified,
	}},
}

// CRMUpsertFieldNames returns the sorted native field names for one object type.
func CRMUpsertFieldNames(objectType string) []string {
	specs := crmUpsertFieldSpecs(objectType)
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// crmUpsertInputError is a client-correctable problem with one record.
type crmUpsertInputError struct {
	message string
}

func (e *crmUpsertInputError) Error() string { return e.message }

func crmUpsertInvalid(format string, args ...any) error {
	return &crmUpsertInputError{message: fmt.Sprintf(format, args...)}
}

func crmUpsertFieldSpecs(objectType string) map[string]crmUpsertFieldSpec {
	if objectType == model.CRMUpsertObjectContact {
		return _crmContactUpsertFields
	}
	return _crmCompanyUpsertFields
}

// normalizeCRMUpsertFields validates and canonicalizes native field values.
func normalizeCRMUpsertFields(objectType string, fields map[string]any) (map[string]any, error) {
	specs := crmUpsertFieldSpecs(objectType)
	normalized := make(map[string]any, len(fields))
	for name, raw := range fields {
		spec, ok := specs[name]
		if !ok {
			return nil, crmUpsertInvalid("field %q is not supported for %s records", name, objectType)
		}
		if raw == nil {
			return nil, crmUpsertInvalid("field %q is null; omit fields you do not want to change", name)
		}
		value, err := normalizeCRMUpsertValue(spec, raw)
		if err != nil {
			return nil, crmUpsertInvalid("field %q %v", name, err)
		}
		if value == nil {
			continue
		}
		normalized[name] = value
	}
	return normalized, nil
}

func normalizeCRMUpsertValue(spec crmUpsertFieldSpec, raw any) (any, error) {
	switch spec.kind {
	case crmUpsertInt:
		number, ok := raw.(float64)
		if !ok || number != math.Trunc(number) || number < 0 || number > math.MaxInt32 {
			return nil, fmt.Errorf("must be a non-negative whole number")
		}
		return number, nil
	case crmUpsertFloat:
		number, ok := raw.(float64)
		if !ok || number < 0 || math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, fmt.Errorf("must be a non-negative number")
		}
		return number, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("must be a string")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	switch spec.kind {
	case crmUpsertURL:
		return validateHTTPURL(text)
	case crmUpsertDomain:
		domain := normalizeCRMDomain(&text)
		if domain == "" || strings.ContainsAny(domain, " @") || !strings.Contains(domain, ".") {
			return nil, fmt.Errorf("must be a domain such as example.com")
		}
		return domain, nil
	case crmUpsertEmail:
		email := strings.ToLower(text)
		if at := strings.LastIndex(email, "@"); at < 1 || at == len(email)-1 || strings.ContainsAny(email, " ") {
			return nil, fmt.Errorf("must be an email address")
		}
		return email, nil
	case crmUpsertEnum:
		for _, allowed := range spec.enum {
			if text == allowed {
				return text, nil
			}
		}
		return nil, fmt.Errorf("must be one of %s", strings.Join(spec.enum, ", "))
	}
	if spec.maxLen > 0 && len([]rune(text)) > spec.maxLen {
		return nil, fmt.Errorf("must be at most %d characters", spec.maxLen)
	}
	return text, nil
}

// normalizeCRMUpsertCustomProperties validates custom property keys and values.
// A nil value asks to remove the key.
func normalizeCRMUpsertCustomProperties(properties map[string]any) (map[string]any, error) {
	if len(properties) > _crmUpsertMaxCustomKeys {
		return nil, crmUpsertInvalid("at most %d custom properties per record", _crmUpsertMaxCustomKeys)
	}
	normalized := make(map[string]any, len(properties))
	for key, raw := range properties {
		if !repository.ValidCRMCustomPropertyKey(key) {
			return nil, crmUpsertInvalid("custom property %q must be snake_case: lowercase letters, digits, and underscores", key)
		}
		if key == _crmUpsertReservedPropertyNS {
			return nil, crmUpsertInvalid("custom property %q is reserved", key)
		}
		value, err := normalizeCRMUpsertCustomValue(raw)
		if err != nil {
			return nil, crmUpsertInvalid("custom property %q %v", key, err)
		}
		normalized[key] = value
	}
	return normalized, nil
}

func normalizeCRMUpsertCustomValue(raw any) (any, error) {
	switch value := raw.(type) {
	case nil, bool, float64:
		return value, nil
	case string:
		value = strings.TrimSpace(value)
		if len([]rune(value)) > _crmUpsertMaxCustomString {
			return nil, fmt.Errorf("must be at most %d characters", _crmUpsertMaxCustomString)
		}
		return value, nil
	case []any:
		if len(value) > _crmUpsertMaxCustomArray {
			return nil, fmt.Errorf("must have at most %d items", _crmUpsertMaxCustomArray)
		}
		items := make([]any, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok || len([]rune(text)) > 500 {
				return nil, fmt.Errorf("lists may only contain strings of at most 500 characters")
			}
			items = append(items, strings.TrimSpace(text))
		}
		return items, nil
	default:
		return nil, fmt.Errorf("must be a string, number, boolean, list of strings, or null")
	}
}

// crmUpsertPlan is the computed change set for one existing record.
type crmUpsertPlan struct {
	fields     map[string]any
	properties model.JSONB
	changed    []string
	kept       []string
}

// planCRMUpsertChanges compares desired values with the current record.
// current holds the record's native fields keyed by JSON name.
func planCRMUpsertChanges(
	current map[string]any,
	currentProperties model.JSONB,
	fields, properties map[string]any,
	fillOnly bool,
) crmUpsertPlan {
	plan := crmUpsertPlan{fields: map[string]any{}}
	for _, name := range sortedCRMUpsertKeys(fields) {
		desired := fields[name]
		existing := current[name]
		switch {
		case crmUpsertValuesEqual(existing, desired):
		case fillOnly && !crmUpsertEmpty(existing):
			plan.kept = append(plan.kept, name)
		default:
			plan.fields[name] = desired
			plan.changed = append(plan.changed, name)
		}
	}
	merged := cloneJSONB(currentProperties)
	propertiesChanged := false
	for _, key := range sortedCRMUpsertKeys(properties) {
		desired := properties[key]
		existing, present := merged[key]
		label := "custom_properties." + key
		switch {
		case desired == nil && !present:
		case crmUpsertValuesEqual(existing, desired) && present:
		case fillOnly && present && !crmUpsertEmpty(existing):
			plan.kept = append(plan.kept, label)
		case desired == nil && fillOnly:
		case desired == nil:
			delete(merged, key)
			propertiesChanged = true
			plan.changed = append(plan.changed, label)
		default:
			merged[key] = desired
			propertiesChanged = true
			plan.changed = append(plan.changed, label)
		}
	}
	if propertiesChanged {
		plan.properties = merged
	}
	return plan
}

func crmUpsertEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func crmUpsertValuesEqual(left, right any) bool {
	if leftText, ok := left.(string); ok {
		rightText, ok := right.(string)
		return ok && strings.TrimSpace(leftText) == strings.TrimSpace(rightText)
	}
	return reflect.DeepEqual(left, right)
}

func sortedCRMUpsertKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// crmDomainVariants returns stored spellings that refer to the same domain.
func crmDomainVariants(domain string) []string {
	variants := make([]string, 0, 12)
	for _, prefix := range []string{"", "www.", "http://", "https://", "http://www.", "https://www."} {
		variants = append(variants, prefix+domain, prefix+domain+"/")
	}
	return variants
}
