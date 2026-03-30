package worker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

var codexOptionSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

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

func codexHumanInputPause(params codexToolRequestUserInputParams, requestID json.RawMessage) (appmodel.ToolInvocation, json.RawMessage, *codexPendingRequest) {
	questions := make([]HumanInputQuestion, 0, len(params.Questions))
	questionIDs := make([]string, 0, len(params.Questions))
	for _, question := range params.Questions {
		questionIDs = append(questionIDs, strings.TrimSpace(question.ID))
		item := HumanInputQuestion{
			ID:   strings.TrimSpace(question.ID),
			Type: QuestionTypeSingleSelect,
			Text: codexQuestionPrompt(question),
		}
		usedValues := map[string]int{}
		for _, option := range question.Options {
			value := codexQuestionOptionValue(option.Label, usedValues)
			item.Options = append(item.Options, HumanInputOption{
				Value: value,
				Label: strings.TrimSpace(option.Label),
			})
		}
		if question.IsOther {
			item.Options = append(item.Options, HumanInputOption{
				Value:    codexQuestionOptionValue("other", usedValues),
				Label:    "Other",
				Freetext: true,
			})
		}
		if len(item.Options) == 0 {
			item.Options = append(item.Options, HumanInputOption{
				Value:    "reply",
				Label:    "Reply",
				Freetext: true,
			})
		}
		questions = append(questions, item)
	}

	request := HumanInputRequest{Questions: questions}
	payload, _ := json.Marshal(request)
	metadata, _ := json.Marshal(map[string]any{
		"runtime_kind":       "codex",
		"codex_request_kind": codexPendingRequestKindHumanInput,
		"codex_request_id":   codexRequestIDString(requestID),
		"codex_turn_id":      strings.TrimSpace(params.TurnID),
		"codex_item_id":      strings.TrimSpace(params.ItemID),
	})
	rawParams, _ := json.Marshal(params)
	return appmodel.ToolInvocation{
			ToolName:      ToolRequestHumanInput,
			Input:         payload,
			OutputSummary: "waiting for human input",
		},
		metadata,
		&codexPendingRequest{
			Kind:         codexPendingRequestKindHumanInput,
			RequestID:    codexRequestIDString(requestID),
			RequestIDRaw: append(json.RawMessage(nil), requestID...),
			TurnID:       strings.TrimSpace(params.TurnID),
			ItemID:       strings.TrimSpace(params.ItemID),
			QuestionIDs:  questionIDs,
			Payload:      rawParams,
		}
}

func codexHumanApprovalPause(kind string, title, summary string, requestID json.RawMessage, turnID, itemID string, payload json.RawMessage) (appmodel.ToolInvocation, json.RawMessage, *codexPendingRequest) {
	req := HumanApprovalRequest{
		Phase:   strings.TrimSpace(kind),
		Title:   strings.TrimSpace(title),
		Summary: strings.TrimSpace(summary),
	}
	input, _ := json.Marshal(req)
	metadata, _ := json.Marshal(map[string]any{
		"runtime_kind":       "codex",
		"codex_request_kind": strings.TrimSpace(kind),
		"codex_request_id":   codexRequestIDString(requestID),
		"codex_turn_id":      strings.TrimSpace(turnID),
		"codex_item_id":      strings.TrimSpace(itemID),
		"approval_kind":      strings.TrimSpace(kind),
	})
	return appmodel.ToolInvocation{
			ToolName:      ToolRequestHumanApproval,
			Input:         input,
			OutputSummary: "waiting for approval",
		},
		metadata,
		&codexPendingRequest{
			Kind:         strings.TrimSpace(kind),
			RequestID:    codexRequestIDString(requestID),
			RequestIDRaw: append(json.RawMessage(nil), requestID...),
			TurnID:       strings.TrimSpace(turnID),
			ItemID:       strings.TrimSpace(itemID),
			Payload:      append(json.RawMessage(nil), payload...),
		}
}

func codexPendingRequestResponseID(pending *codexPendingRequest) json.RawMessage {
	if pending == nil {
		return nil
	}
	if raw := strings.TrimSpace(string(pending.RequestIDRaw)); raw != "" && raw != "null" {
		return append(json.RawMessage(nil), pending.RequestIDRaw...)
	}
	if strings.TrimSpace(pending.RequestID) == "" {
		return nil
	}
	encoded, err := json.Marshal(strings.TrimSpace(pending.RequestID))
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

func codexQuestionPrompt(question codexToolRequestInputQuestion) string {
	header := strings.TrimSpace(question.Header)
	prompt := strings.TrimSpace(question.Question)
	switch {
	case header != "" && prompt != "" && !strings.EqualFold(header, prompt):
		return header + ": " + prompt
	case prompt != "":
		return prompt
	default:
		return firstNonEmptyText(header, strings.TrimSpace(question.ID), "Question")
	}
}

func codexQuestionOptionValue(label string, used map[string]int) string {
	base := strings.ToLower(strings.TrimSpace(label))
	base = codexOptionSlugSanitizer.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		base = "option"
	}
	if used == nil {
		return base
	}
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, used[base])
}

func codexCommandApprovalSummary(params codexCommandExecutionRequestApprovalParams) string {
	lines := make([]string, 0, 4)
	if params.Command != nil && strings.TrimSpace(*params.Command) != "" {
		lines = append(lines, "Command: "+strings.TrimSpace(*params.Command))
	}
	if params.Cwd != nil && strings.TrimSpace(*params.Cwd) != "" {
		lines = append(lines, "Working directory: "+strings.TrimSpace(*params.Cwd))
	}
	if params.Reason != nil && strings.TrimSpace(*params.Reason) != "" {
		lines = append(lines, "Reason: "+strings.TrimSpace(*params.Reason))
	}
	if len(lines) == 0 {
		lines = append(lines, "Codex requested approval before executing a command.")
	}
	return strings.Join(lines, "\n")
}

func codexFileChangeApprovalSummary(params codexFileChangeRequestApprovalParams, diffPreview string) string {
	lines := make([]string, 0, 3)
	if params.Reason != nil && strings.TrimSpace(*params.Reason) != "" {
		lines = append(lines, "Reason: "+strings.TrimSpace(*params.Reason))
	}
	if params.GrantRoot != nil && strings.TrimSpace(*params.GrantRoot) != "" {
		lines = append(lines, "Grant root: "+strings.TrimSpace(*params.GrantRoot))
	}
	if snippet := strings.TrimSpace(diffPreview); snippet != "" {
		lines = append(lines, "Diff preview:\n"+truncate(snippet, 4000))
	}
	if len(lines) == 0 {
		lines = append(lines, "Codex requested approval before applying file changes.")
	}
	return strings.Join(lines, "\n")
}

func codexPermissionsApprovalSummary(params codexPermissionsRequestApprovalParams) string {
	lines := make([]string, 0, 4)
	if params.Reason != nil && strings.TrimSpace(*params.Reason) != "" {
		lines = append(lines, "Reason: "+strings.TrimSpace(*params.Reason))
	}
	if params.Permissions.Network != nil {
		enabled := false
		if params.Permissions.Network.Enabled != nil {
			enabled = *params.Permissions.Network.Enabled
		}
		lines = append(lines, fmt.Sprintf("Network access requested: %t", enabled))
	}
	if params.Permissions.FileSystem != nil {
		if len(params.Permissions.FileSystem.Read) > 0 {
			lines = append(lines, "Read access: "+strings.Join(params.Permissions.FileSystem.Read, ", "))
		}
		if len(params.Permissions.FileSystem.Write) > 0 {
			lines = append(lines, "Write access: "+strings.Join(params.Permissions.FileSystem.Write, ", "))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "Codex requested additional permissions.")
	}
	return strings.Join(lines, "\n")
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
