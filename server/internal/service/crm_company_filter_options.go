package service

import (
	"context"
	"fmt"
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
	if len(values) > _crmFilterOptionsMaxValues {
		return values[:_crmFilterOptionsMaxValues], true
	}
	return values, false
}

type crmCustomPropertyStats struct {
	count  int
	types  map[string]bool
	values *crmFilterValueCounter
}

// buildCRMCompanyFilterOptions aggregates filter options from company rows.
func buildCRMCompanyFilterOptions(sources []model.CRMCompanyFilterSource) *model.CRMCompanyFilterOptions {
	industries := newCRMFilterValueCounter()
	properties := map[string]*crmCustomPropertyStats{}
	for _, source := range sources {
		if source.Industry != nil {
			industries.add(*source.Industry)
		}
		for key, raw := range source.CustomProperties {
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
		}
	}

	options := &model.CRMCompanyFilterOptions{ScannedCompanies: len(sources)}
	options.Industries, _ = industries.top()
	options.CustomProperties = make([]model.CRMCustomPropertySummary, 0, len(properties))
	for key, stats := range properties {
		values, more := stats.values.top()
		valueType := "mixed"
		if len(stats.types) == 1 {
			for only := range stats.types {
				valueType = only
			}
		}
		options.CustomProperties = append(options.CustomProperties, model.CRMCustomPropertySummary{
			Key: key, Count: stats.count, ValueType: valueType, Values: values, HasMoreValues: more,
		})
	}
	sort.Slice(options.CustomProperties, func(i, j int) bool {
		left, right := options.CustomProperties[i], options.CustomProperties[j]
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		return left.Key < right.Key
	})
	return options
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
