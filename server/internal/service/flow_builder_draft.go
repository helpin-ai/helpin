package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Omitted settings survive conversational edits. Explicit null removes a config
// filter; changing trigger or action type starts that config from scratch.
func decodeFlowDraft(input json.RawMessage, previous *model.FlowBuilderDraft) (model.FlowBuilderDraft, error) {
	var draft model.FlowBuilderDraft
	if previous != nil {
		raw, err := json.Marshal(previous)
		if err != nil {
			return draft, err
		}
		if err := json.Unmarshal(raw, &draft); err != nil {
			return draft, err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return draft, fmt.Errorf("invalid flow draft: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return draft, fmt.Errorf("provide exactly one flow draft")
	}
	var supplied map[string]json.RawMessage
	if err := json.Unmarshal(input, &supplied); err != nil {
		return draft, err
	}
	merge := func(key string, old, next json.RawMessage) (json.RawMessage, error) {
		if _, ok := supplied[key]; !ok {
			return old, nil
		}
		var fields, patch map[string]json.RawMessage
		if err := json.Unmarshal(old, &fields); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(next, &patch); err != nil {
			return nil, err
		}
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
		for k, v := range patch {
			if string(v) == "null" {
				delete(fields, k)
			} else {
				fields[k] = v
			}
		}
		return json.Marshal(fields)
	}
	if previous != nil {
		var err error
		if draft.TriggerType == previous.TriggerType {
			draft.TriggerConfig, err = merge("trigger_config", previous.TriggerConfig, draft.TriggerConfig)
			if err != nil {
				return draft, err
			}
		}
		if draft.ActionType == previous.ActionType {
			draft.ActionConfig, err = merge("action_config", previous.ActionConfig, draft.ActionConfig)
			if err != nil {
				return draft, err
			}
		}
		// A new proposal must supply its own explanation after a user asks for changes.
		if _, ok := supplied["summary"]; !ok {
			draft.Summary = ""
		}
	}
	// semantic_condition is also exposed as a convenient top-level setting.
	// Keep its stored trigger representation in sync when either form changes.
	var trigger map[string]json.RawMessage
	if len(draft.TriggerConfig) > 0 {
		if err := json.Unmarshal(draft.TriggerConfig, &trigger); err != nil {
			return draft, err
		}
		if trigger == nil {
			trigger = map[string]json.RawMessage{}
		}
		if _, explicit := supplied["semantic_condition"]; explicit {
			if draft.SemanticCondition == "" {
				delete(trigger, "semantic_condition")
			} else {
				encoded, err := json.Marshal(model.SemanticFlowCondition{Text: draft.SemanticCondition})
				if err != nil {
					return draft, err
				}
				trigger["semantic_condition"] = encoded
			}
		} else if raw, explicit := supplied["trigger_config"]; explicit {
			var patch map[string]json.RawMessage
			if err := json.Unmarshal(raw, &patch); err != nil {
				return draft, err
			}
			if condition, ok := patch["semantic_condition"]; ok {
				draft.SemanticCondition = ""
				if string(condition) == "null" {
					delete(trigger, "semantic_condition")
				} else {
					var value model.SemanticFlowCondition
					if err := json.Unmarshal(condition, &value); err != nil {
						return draft, err
					}
					draft.SemanticCondition = value.Text
				}
			}
		}
		var err error
		draft.TriggerConfig, err = json.Marshal(trigger)
		if err != nil {
			return draft, err
		}
	}
	return draft, nil
}
