package service

func supportAnswerJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"content", "can_answer", "source_doc_ids", "confidence", "claims", "evidence_coverage",
		},
		"properties": map[string]any{
			"content":    map[string]any{"type": "string"},
			"can_answer": map[string]any{"type": "boolean"},
			"source_doc_ids": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"confidence": map[string]any{
				"type":    "number",
				"minimum": 0,
				"maximum": 1,
			},
			"claims": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"text", "evidence_ids"},
					"properties": map[string]any{
						"text": map[string]any{"type": "string"},
						"evidence_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
				},
			},
			"evidence_coverage": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           map[string]any{},
			},
		},
	}
}
