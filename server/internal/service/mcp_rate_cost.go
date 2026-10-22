package service

import "encoding/json"

// _bulkRecordsPerRateUnit is how many upserted records count as one write
// against the expensive rate limit.
const _bulkRecordsPerRateUnit = 10

// MCPToolRateCost returns how many rate-limit units one call of a tool costs.
// Bulk tools are weighted by batch size so a 100-record upsert cannot cost the
// same as a single update; every other call costs one unit.
func MCPToolRateCost(toolName string, arguments json.RawMessage) int64 {
	if toolName != "upsert_crm_records" || len(arguments) == 0 {
		return 1
	}
	var input struct {
		Records []json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(arguments, &input); err != nil || len(input.Records) == 0 {
		return 1
	}
	return int64((len(input.Records) + _bulkRecordsPerRateUnit - 1) / _bulkRecordsPerRateUnit)
}
