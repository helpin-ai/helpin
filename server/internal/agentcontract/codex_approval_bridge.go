package agentcontract

import (
	"encoding/json"
	"fmt"
	"strings"
)

type codexStructuredAnswerEnvelope struct {
	Answers []codexStructuredAnswerEntry `json:"answers"`
}

type codexStructuredAnswerEntry struct {
	QuestionID    string `json:"question_id"`
	Question      string `json:"question"`
	SelectedValue string `json:"selected_value"`
	SelectedLabel string `json:"selected_label"`
	Freetext      string `json:"freetext"`
}

func codexRequestIDString(id json.RawMessage) string {
	trimmed := strings.TrimSpace(string(id))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(id, &decoded); err == nil {
		switch typed := decoded.(type) {
		case string:
			return strings.TrimSpace(typed)
		case float64:
			if typed == float64(int64(typed)) {
				return fmt.Sprintf("%d", int64(typed))
			}
			return strings.TrimSpace(fmt.Sprintf("%v", typed))
		}
	}
	return strings.Trim(trimmed, `"`)
}

func threadIDFromApprovalPayload(payload json.RawMessage) string {
	var envelope struct {
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return strings.TrimSpace(envelope.ThreadID)
}

func approvalIDFromApprovalPayload(payload json.RawMessage) *string {
	var envelope struct {
		ApprovalID *string `json:"approvalId,omitempty"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil
	}
	if envelope.ApprovalID == nil {
		return nil
	}
	value := strings.TrimSpace(*envelope.ApprovalID)
	if value == "" {
		return nil
	}
	return &value
}

func codexParseUserInputResponse(pending *codexPendingRequest, content string) (codexToolRequestUserInputResponse, error) {
	var params codexToolRequestUserInputParams
	if pending == nil || len(pending.Payload) == 0 {
		return codexToolRequestUserInputResponse{}, fmt.Errorf("missing pending request payload")
	}
	if err := json.Unmarshal(pending.Payload, &params); err != nil {
		return codexToolRequestUserInputResponse{}, fmt.Errorf("parse pending human input payload: %w", err)
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return codexToolRequestUserInputResponse{}, fmt.Errorf("human input reply is empty")
	}

	var envelope codexStructuredAnswerEnvelope
	for _, candidate := range []string{
		strings.TrimSpace(trimJSONFences(trimmed)),
		strings.TrimSpace(extractJSONObject(trimmed)),
	} {
		if candidate == "" {
			continue
		}
		if err := json.Unmarshal([]byte(candidate), &envelope); err == nil && len(envelope.Answers) > 0 {
			break
		}
	}

	answerMap := make(map[string]codexStructuredAnswerEntry, len(envelope.Answers))
	for _, answer := range envelope.Answers {
		answer.QuestionID = strings.TrimSpace(answer.QuestionID)
		if answer.QuestionID == "" {
			continue
		}
		answerMap[answer.QuestionID] = answer
	}

	response := codexToolRequestUserInputResponse{
		Answers: make(map[string]codexToolRequestUserInputAnswer, len(params.Questions)),
	}

	if len(answerMap) == 0 && len(params.Questions) == 1 {
		questionID := strings.TrimSpace(params.Questions[0].ID)
		if questionID == "" {
			return codexToolRequestUserInputResponse{}, fmt.Errorf("question id is required")
		}
		response.Answers[questionID] = codexToolRequestUserInputAnswer{Answers: []string{trimmed}}
		return response, nil
	}

	for _, question := range params.Questions {
		answer, ok := answerMap[strings.TrimSpace(question.ID)]
		if !ok {
			continue
		}
		value := strings.TrimSpace(answer.Freetext)
		if value == "" {
			value = strings.TrimSpace(answer.SelectedLabel)
		}
		if value == "" {
			value = strings.TrimSpace(answer.SelectedValue)
		}
		if value == "" {
			return codexToolRequestUserInputResponse{}, fmt.Errorf("question %q is missing an answer", strings.TrimSpace(question.ID))
		}
		response.Answers[strings.TrimSpace(question.ID)] = codexToolRequestUserInputAnswer{
			Answers: []string{value},
		}
	}
	if len(response.Answers) == 0 {
		return codexToolRequestUserInputResponse{}, fmt.Errorf("failed to parse structured answers from reply")
	}
	return response, nil
}

func BuildCodexUserInputResponseFromPayload(requestPayload json.RawMessage, content string) (json.RawMessage, error) {
	response, err := codexParseUserInputResponse(&codexPendingRequest{
		Kind:    codexPendingRequestKindHumanInput,
		Payload: append(json.RawMessage(nil), requestPayload...),
	}, content)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal codex user input response: %w", err)
	}
	return raw, nil
}

func codexApprovalResponse(pending *codexPendingRequest, approved bool, requestChanges bool) (any, error) {
	if pending == nil {
		return nil, fmt.Errorf("missing pending approval state")
	}
	switch pending.Kind {
	case codexPendingRequestKindCommandApproval:
		decision := "decline"
		if requestChanges {
			decision = "cancel"
		} else if approved {
			decision = "accept"
		}
		return map[string]any{"decision": decision}, nil
	case codexPendingRequestKindFileApproval:
		decision := "decline"
		if requestChanges {
			decision = "cancel"
		} else if approved {
			decision = "accept"
		}
		return map[string]any{"decision": decision}, nil
	case codexPendingRequestKindPermissions:
		if !approved {
			return map[string]any{
				"permissions": codexGrantedPermissionProfile{},
				"scope":       "turn",
			}, nil
		}
		var params codexPermissionsRequestApprovalParams
		if err := json.Unmarshal(pending.Payload, &params); err != nil {
			return nil, fmt.Errorf("parse pending permissions payload: %w", err)
		}
		return map[string]any{
			"permissions": codexGrantedPermissionProfile{
				Network:    params.Permissions.Network,
				FileSystem: params.Permissions.FileSystem,
			},
			"scope": "turn",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported pending approval kind %q", pending.Kind)
	}
}

func BuildCodexApprovalResponseFromPayload(kind string, requestPayload json.RawMessage, approved bool, requestChanges bool) (json.RawMessage, error) {
	response, err := codexApprovalResponse(&codexPendingRequest{
		Kind:    strings.TrimSpace(kind),
		Payload: append(json.RawMessage(nil), requestPayload...),
	}, approved, requestChanges)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal codex approval response: %w", err)
	}
	return raw, nil
}
