package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// _crmFilterOptionsScanLimit bounds how many companies are read to build
	// filter options; the most recently updated ones are used.
	_crmFilterOptionsScanLimit = 20000
	// _crmFilterOptionsMaxValues bounds the values returned per field.
	_crmFilterOptionsMaxValues = 50
	// _crmFilterOptionMaxLength skips long free-text values as filter options.
	_crmFilterOptionMaxLength = 120
)

// FilterOptions returns the industries and custom property keys in use on the
// workspace's companies, with their most common values, for list columns and
// filters.
func (s *CRMCompanyService) FilterOptions(ctx context.Context, workspaceID string) (*model.CRMCompanyFilterOptions, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	sources, err := s.companyRepo.ListFilterSources(ctx, workspaceID, _crmFilterOptionsScanLimit+1)
	if err != nil {
		return nil, err
	}
	truncated := len(sources) > _crmFilterOptionsScanLimit
	if truncated {
		sources = sources[:_crmFilterOptionsScanLimit]
	}
	options := buildCRMCompanyFilterOptions(sources)
	options.Truncated = truncated
	return options, nil
}

// FilterOptions returns the custom property keys in use on the workspace's
// contacts, with their most common values, for list columns and filters.
func (s *CRMContactService) FilterOptions(ctx context.Context, workspaceID string) (*model.CRMContactFilterOptions, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	sources, truncated, err := s.customPropertySources(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return &model.CRMContactFilterOptions{
		CustomProperties: buildCRMCustomPropertySummaries(sources),
		ScannedContacts:  len(sources), Truncated: truncated,
	}, nil
}

// customPropertySources reads the custom properties of the most recently
// updated contacts, subject to the same entitlement as listing contacts.
func (s *CRMContactService) customPropertySources(ctx context.Context, workspaceID string) ([]model.JSONB, bool, error) {
	if err := s.requireContactViewEntitlement(ctx, workspaceID); err != nil {
		return nil, false, err
	}
	sources, err := s.contactRepo.ListFilterSources(ctx, workspaceID, _crmFilterOptionsScanLimit+1)
	if err != nil {
		return nil, false, err
	}
	truncated := len(sources) > _crmFilterOptionsScanLimit
	if truncated {
		sources = sources[:_crmFilterOptionsScanLimit]
	}
	return sources, truncated, nil
}

type crmFilterValueCounter struct {
	counts map[string]int
	// display keeps the first spelling seen for case-insensitively equal values.
	display map[string]string
}

func newCRMFilterValueCounter() *crmFilterValueCounter {
	return &crmFilterValueCounter{counts: map[string]int{}, display: map[string]string{}}
}

func (c *crmFilterValueCounter) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > _crmFilterOptionMaxLength {
		return
	}
	normalized := strings.ToLower(value)
	if _, ok := c.display[normalized]; !ok {
		c.display[normalized] = value
	}
	c.counts[normalized]++
}

// top returns the most common values, most frequent first, and whether more
// values exist.
func (c *crmFilterValueCounter) top() ([]model.CRMFilterValueCount, bool) {
	return c.topN(_crmFilterOptionsMaxValues)
}

// topN returns up to limit of the most common values and whether more exist.
func (c *crmFilterValueCounter) topN(limit int) ([]model.CRMFilterValueCount, bool) {
	values := make([]model.CRMFilterValueCount, 0, len(c.counts))
	for normalized, count := range c.counts {
		values = append(values, model.CRMFilterValueCount{Value: c.display[normalized], Count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count != values[j].Count {
			return values[i].Count > values[j].Count
		}
		return strings.ToLower(values[i].Value) < strings.ToLower(values[j].Value)
	})
	if len(values) > limit {
		return values[:limit], true
	}
	return values, false
}

type crmCustomPropertyStats struct {
	count        int
	types        map[string]bool
	values       *crmFilterValueCounter
	numericCount int
	numericMin   *float64
	numericMax   *float64
}

func (s *crmCustomPropertyStats) addNumber(number float64) {
	s.numericCount++
	if s.numericMin == nil || number < *s.numericMin {
		s.numericMin = &number
	}
	if s.numericMax == nil || number > *s.numericMax {
		s.numericMax = &number
	}
}

// buildCRMCompanyFilterOptions aggregates filter options from company rows.
func buildCRMCompanyFilterOptions(sources []model.CRMCompanyFilterSource) *model.CRMCompanyFilterOptions {
	industries := newCRMFilterValueCounter()
	properties := make([]model.JSONB, 0, len(sources))
	for _, source := range sources {
		if source.Industry != nil {
			industries.add(*source.Industry)
		}
		properties = append(properties, source.CustomProperties)
	}
	options := &model.CRMCompanyFilterOptions{ScannedCompanies: len(sources)}
	options.Industries, _ = industries.top()
	options.CustomProperties = buildCRMCustomPropertySummaries(properties)
	return options
}

// buildCRMCustomPropertySummaries aggregates the custom property keys in use
// across records, most used first.
func buildCRMCustomPropertySummaries(records []model.JSONB) []model.CRMCustomPropertySummary {
	properties := map[string]*crmCustomPropertyStats{}
	for _, record := range records {
		for key, raw := range record {
			if key == _crmUpsertReservedPropertyNS {
				continue
			}
			valueType, values := crmCustomPropertyFilterValues(raw)
			if valueType == "" {
				continue
			}
			stats := properties[key]
			if stats == nil {
				stats = &crmCustomPropertyStats{types: map[string]bool{}, values: newCRMFilterValueCounter()}
				properties[key] = stats
			}
			stats.count++
			stats.types[valueType] = true
			for _, value := range values {
				stats.values.add(value)
			}
			if number, ok := crmCustomPropertyNumber(raw); ok {
				stats.addNumber(number)
			}
		}
	}
	summaries := make([]model.CRMCustomPropertySummary, 0, len(properties))
	for key, stats := range properties {
		values, more := stats.values.top()
		summaries = append(summaries, model.CRMCustomPropertySummary{
			Key: key, Count: stats.count, ValueType: stats.valueType(), Values: values, HasMoreValues: more,
			NumericCount: stats.numericCount, NumericMin: stats.numericMin, NumericMax: stats.numericMax,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		return summaries[i].Key < summaries[j].Key
	})
	return summaries
}

func (s *crmCustomPropertyStats) valueType() string {
	if len(s.types) != 1 {
		return "mixed"
	}
	for only := range s.types {
		return only
	}
	return "mixed"
}

// crmCustomPropertyFilterValues returns a value's type and its filterable text
// values, matching how the list filter reads them (JSON ->> text, or list
// elements). Empty values and nested objects return an empty type.
func crmCustomPropertyFilterValues(raw any) (string, []string) {
	switch value := raw.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return "", nil
		}
		return "text", []string{value}
	case float64:
		return "number", []string{strconv.FormatFloat(value, 'f', -1, 64)}
	case bool:
		return "boolean", []string{strconv.FormatBool(value)}
	case []any:
		var items []string
		for _, item := range value {
			switch element := item.(type) {
			case string:
				items = append(items, element)
			case float64:
				items = append(items, strconv.FormatFloat(element, 'f', -1, 64))
			}
		}
		if len(items) == 0 {
			return "", nil
		}
		return "list", items
	default:
		return "", nil
	}
}

// _crmNumericText matches the strings the list filter compares as numbers.
var _crmNumericText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// crmCustomPropertyNumber reports a value numeric filters can compare, using
// the same rule as the list filter's numeric expression.
func crmCustomPropertyNumber(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case string:
		text := strings.TrimSpace(value)
		if !_crmNumericText.MatchString(text) {
			return 0, false
		}
		number, err := strconv.ParseFloat(text, 64)
		return number, err == nil
	default:
		return 0, false
	}
}
