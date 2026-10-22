package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPToolRateCost(t *testing.T) {
	records := func(count int) json.RawMessage {
		return json.RawMessage(`{"object_type":"company","records":[` + strings.TrimSuffix(strings.Repeat(`{"fields":{"name":"x"}},`, count), ",") + `]}`)
	}
	tests := []struct {
		name string
		tool string
		args json.RawMessage
		want int64
	}{
		{name: "single record", tool: "upsert_crm_records", args: records(1), want: 1},
		{name: "ten records", tool: "upsert_crm_records", args: records(10), want: 1},
		{name: "eleven records", tool: "upsert_crm_records", args: records(11), want: 2},
		{name: "full batch", tool: "upsert_crm_records", args: records(100), want: 10},
		{name: "malformed arguments", tool: "upsert_crm_records", args: json.RawMessage(`{`), want: 1},
		{name: "no arguments", tool: "upsert_crm_records", want: 1},
		{name: "other tool", tool: "update_crm_company", args: records(100), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MCPToolRateCost(tt.tool, tt.args); got != tt.want {
				t.Fatalf("MCPToolRateCost() = %d, want %d", got, tt.want)
			}
		})
	}
}
