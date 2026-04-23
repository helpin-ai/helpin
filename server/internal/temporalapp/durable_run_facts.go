package temporalapp

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func buildDurableRunFacts(state *resolvedRunState, input planningRunInput) map[string]string {
	facts := map[string]string{}
	if state == nil || state.run == nil {
		return facts
	}

	collectStructIDFacts(facts, "run", state.run)
	collectStructIDFacts(facts, "agent", state.agent)
	collectStructIDFacts(facts, "task", state.task)
	collectStructIDFacts(facts, "story", state.task) // backward-compat alias for "task"
	collectStructIDFacts(facts, "epic", state.epic)
	collectStructIDFacts(facts, "conversation", state.conversation)
	collectStructIDFacts(facts, "delivery_target", state.deliveryTarget)
	collectStructIDFacts(facts, "repository", state.repository)
	collectStructIDFacts(facts, "integration", state.integration)
	collectStructIDFacts(facts, "team_default", state.teamDefault)
	collectStructIDFacts(facts, "planning_input", input)
	collectJSONIDFacts(facts, "", state.run.Input)

	setFact(facts, "workspace_id", state.run.WorkspaceID)
	setFact(facts, "run_id", state.run.ID)
	setFact(facts, "agent_id", state.run.AgentID)
	setFact(facts, "target_type", state.run.TargetType)
	setFact(facts, "target_id", state.run.TargetID)
	setFact(facts, "task_id", firstNonEmptyString(derefString(state.run.TaskID), structID(state.task)))
	setFact(facts, "story_id", firstNonEmptyString(derefString(state.run.TaskID), structID(state.task)))
	setFact(facts, "conversation_id", firstNonEmptyString(derefString(state.run.ConversationID), structID(state.conversation)))
	setFact(facts, "epic_id", firstNonEmptyString(structID(state.epic), derefString(epicIDOfTask(state.task))))
	setFact(facts, "plan_document_id", firstNonEmptyString(input.PlanDocumentID, derefString(planDocumentIDOfTask(state.task))))
	setFact(facts, "spec_document_id", firstNonEmptyString(input.SpecDocumentID, derefString(specDocumentIDOfEpic(state.epic))))
	setFact(facts, "spec_version_id", input.SpecVersionID)
	setFact(facts, "approved_spec_version_id", derefString(approvedSpecVersionIDOfEpic(state.epic)))
	setFact(facts, "repository_id", firstNonEmptyString(derefString(state.run.RepositoryID), repositoryIDOfDeliveryTarget(state.deliveryTarget), structID(state.repository)))
	setFact(facts, "delivery_target_id", firstNonEmptyString(derefString(state.run.DeliveryTargetID), structID(state.deliveryTarget)))
	setFact(facts, "repo_full_name", repoFullName(state))
	setFact(facts, "base_branch", derefString(state.run.BaseBranch))
	setFact(facts, "working_branch", derefString(state.run.WorkingBranch))
	setFact(facts, "branch_sync_status", strings.TrimSpace(state.branchSync.Status))
	setFact(facts, "branch_sync_base_branch", strings.TrimSpace(state.branchSync.BaseBranch))
	setFact(facts, "branch_sync_working_branch", strings.TrimSpace(state.branchSync.WorkingBranch))
	if len(state.branchSync.ConflictFiles) > 0 {
		setFact(facts, "branch_sync_conflict_files", strings.Join(state.branchSync.ConflictFiles, ","))
	}

	targetType := sanitizeFactKey(state.run.TargetType)
	if targetType != "" && strings.TrimSpace(state.run.TargetID) != "" {
		setFact(facts, targetType+"_id", state.run.TargetID)
	}

	return facts
}

func collectStructIDFacts(facts map[string]string, prefix string, value any) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for idx := 0; idx < rv.NumField(); idx++ {
		field := rt.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}

		key := ""
		switch {
		case name == "id":
			key = prefix + "_id"
		case strings.HasSuffix(name, "_id"), strings.HasSuffix(name, "_ids"):
			key = prefix + "_" + name
		default:
			continue
		}

		if list, ok := reflectedStringSlice(rv.Field(idx)); ok && len(list) > 0 {
			setFact(facts, key, strings.Join(list, ", "))
			continue
		}
		if value, ok := reflectedString(rv.Field(idx)); ok {
			setFact(facts, key, value)
		}
	}
}

func collectJSONIDFacts(facts map[string]string, prefix string, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	collectJSONIDFactsValue(facts, prefix, payload)
}

func collectJSONIDFactsValue(facts map[string]string, prefix string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			key = sanitizeFactKey(key)
			if key == "" {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "_" + key
			}
			if strings.HasSuffix(key, "_id") {
				if stringValue, ok := child.(string); ok {
					setFact(facts, path, stringValue)
				}
			} else if strings.HasSuffix(key, "_ids") {
				if list := jsonStringSlice(child); len(list) > 0 {
					setFact(facts, path, strings.Join(list, ", "))
				}
			}
			collectJSONIDFactsValue(facts, path, child)
		}
	case []any:
		for _, child := range typed {
			collectJSONIDFactsValue(facts, prefix, child)
		}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return sanitizeFactKey(field.Name)
	}
	name := strings.TrimSpace(strings.Split(tag, ",")[0])
	if name == "" {
		return sanitizeFactKey(field.Name)
	}
	return name
}

func reflectedString(value reflect.Value) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return "", false
	}
	text := strings.TrimSpace(value.String())
	if text == "" {
		return "", false
	}
	return text, true
}

func reflectedStringSlice(value reflect.Value) ([]string, bool) {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return nil, false
	}
	items := make([]string, 0, value.Len())
	for idx := 0; idx < value.Len(); idx++ {
		item, ok := reflectedString(value.Index(idx))
		if ok {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

func jsonStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func sanitizeFactKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func setFact(facts map[string]string, key, value string) {
	key = sanitizeFactKey(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	facts[key] = value
}

func structID(value any) string {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	field := rv.FieldByName("ID")
	if !field.IsValid() {
		return ""
	}
	id, ok := reflectedString(field)
	if !ok {
		return ""
	}
	return id
}

func epicIDOfTask(task *model.PMTask) *string {
	if task == nil {
		return nil
	}
	return task.EpicID
}

func planDocumentIDOfTask(task *model.PMTask) *string {
	if task == nil {
		return nil
	}
	return task.PlanDocumentID
}

func specDocumentIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.SpecDocumentID
}

func approvedSpecVersionIDOfEpic(epic *model.PMEpic) *string {
	if epic == nil {
		return nil
	}
	return epic.ApprovedSpecVersionID
}

func repositoryIDOfDeliveryTarget(target *model.TaskDeliveryTarget) string {
	if target == nil {
		return ""
	}
	return derefString(target.RepositoryID)
}

func repoFullName(state *resolvedRunState) string {
	if state.run != nil && state.run.RepoFullName != nil && *state.run.RepoFullName != "" {
		return *state.run.RepoFullName
	}
	if state.deliveryTarget != nil && state.deliveryTarget.RepoFullName != nil {
		return *state.deliveryTarget.RepoFullName
	}
	if state.repository != nil {
		return state.repository.FullName
	}
	return ""
}
