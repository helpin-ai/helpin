package eventcatalog

import (
	_ "embed"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

//go:embed commercial_events.json
var commercialEventCatalogJSON []byte

func TestGeneratedCommercialEventCatalogMatchesSource(t *testing.T) {
	var source struct {
		ServerOnlyEventNames    []string `json:"server_only_event_names"`
		ServerOnlyCompanyFields []string `json:"server_only_company_fields"`
	}
	if err := json.Unmarshal(commercialEventCatalogJSON, &source); err != nil {
		t.Fatalf("parse commercial event catalog: %v", err)
	}
	generatedEvents := mapKeys(ServerOnlyEventNames)
	generatedFields := mapKeys(ServerOnlyCompanyFields)
	sort.Strings(source.ServerOnlyEventNames)
	sort.Strings(source.ServerOnlyCompanyFields)
	if !reflect.DeepEqual(generatedEvents, source.ServerOnlyEventNames) {
		t.Fatalf("generated server-only events = %v, source = %v", generatedEvents, source.ServerOnlyEventNames)
	}
	if !reflect.DeepEqual(generatedFields, source.ServerOnlyCompanyFields) {
		t.Fatalf("generated server-only company fields = %v, source = %v", generatedFields, source.ServerOnlyCompanyFields)
	}
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
