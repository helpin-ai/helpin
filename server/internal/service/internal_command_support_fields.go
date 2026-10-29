package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/customfields"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Request types and conversation fields for agents and integrations: the
// support AI reads them with a conversation, and automations can fill them.

func (s *InternalCommandService) registerSupportFieldCommands() {
	s.register(InternalCommandDefinition{
		Name: "support.list_request_types", Module: "support", SupportedTargetTypes: []string{"workspace", "conversation", "support_conversation"},
		Tool: mustCommandToolMetadata("support.list_request_types"), Execute: s.executeSupportListRequestTypes,
	})
	s.register(InternalCommandDefinition{
		Name: "support.set_conversation_fields", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes,
		Tool: mustCommandToolMetadata("support.set_conversation_fields"), Execute: s.executeSupportSetConversationFields,
	})
}

func (s *InternalCommandService) executeSupportListRequestTypes(ctx context.Context, meta model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
	if err := s.requireSupportOperationalServices(false); err != nil {
		return nil, err
	}
	inbox := s.supportInboxService
	if inbox.requestTypes == nil {
		return nil, errCommandInput("request types are unavailable")
	}
	types, err := inbox.requestTypes.List(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	active, err := inbox.customFields.Active(ctx, meta.WorkspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(types))
	for _, requestType := range types {
		fields := make([]map[string]any, 0, len(requestType.Fields))
		for _, field := range requestType.Fields {
			definition, ok := active[field.Key]
			if !ok {
				continue
			}
			fields = append(fields, map[string]any{
				"key": field.Key, "label": definition.DisplayLabel, "type": definition.FieldType, "options": definition.Options,
				"required": field.Required, "customer_visible": field.CustomerVisible,
			})
		}
		items = append(items, map[string]any{"id": requestType.ID, "name": requestType.Name, "description": requestType.Description, "enabled": requestType.Enabled, "fields": fields})
	}
	all := make([]map[string]any, 0, len(active))
	for _, definition := range active {
		all = append(all, map[string]any{"key": definition.Key, "label": definition.DisplayLabel, "type": definition.FieldType, "options": definition.Options})
	}
	return mustJSON(map[string]any{"request_types": items, "fields": all}), nil
}

func (s *InternalCommandService) executeSupportSetConversationFields(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ConversationID string         `json:"conversation_id"`
		RequestTypeID  *string        `json:"request_type_id"`
		Fields         map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	// A null request_type_id clears the type; leaving it out keeps it.
	var present map[string]json.RawMessage
	if err := json.Unmarshal(input, &present); err != nil {
		return nil, errCommandInput("invalid input")
	}
	_, setsType := present["request_type_id"]
	if !setsType && len(req.Fields) == 0 {
		return nil, errCommandInput("give request_type_id, fields, or both")
	}
	id, scoped, err := s.supportWritableConversation(ctx, meta, req.ConversationID)
	if err != nil {
		return nil, err
	}
	var conversation *model.SupportConversation
	if setsType {
		if req.RequestTypeID != nil {
			trimmed := strings.TrimSpace(*req.RequestTypeID)
			req.RequestTypeID = &trimmed
		}
		if conversation, err = s.supportInboxService.SetConversationRequestType(scoped, meta.WorkspaceID, id, req.RequestTypeID, fallbackActor(meta)); err != nil {
			return nil, supportFieldCommandError(err)
		}
	}
	if len(req.Fields) > 0 {
		if conversation, err = s.supportInboxService.SetConversationFields(scoped, meta.WorkspaceID, id, req.Fields, fallbackActor(meta)); err != nil {
			return nil, supportFieldCommandError(err)
		}
	}
	result := map[string]any{"conversation_id": conversation.ID}
	for key, value := range s.supportConversationFieldContext(ctx, conversation) {
		result[key] = value
	}
	return mustJSON(result), nil
}

func supportFieldCommandError(err error) error {
	switch {
	case errors.Is(err, ErrCustomFieldInvalid):
		return errCommandInput("%s", strings.TrimPrefix(err.Error(), ErrCustomFieldInvalid.Error()+": "))
	case errors.Is(err, ErrSupportRequestTypeInvalid):
		return errCommandInput("%s", strings.TrimPrefix(err.Error(), ErrSupportRequestTypeInvalid.Error()+": "))
	case errors.Is(err, repository.ErrSupportRequestTypeNotFound):
		return errCommandNotFound("request type")
	}
	return supportCommandError(err)
}

// supportConversationFieldContext names a conversation's request type and
// its defined field values, formatted, for agents. Values that do not fit
// their field are left out.
func (s *InternalCommandService) supportConversationFieldContext(ctx context.Context, conversation *model.SupportConversation) map[string]any {
	inbox := s.supportInboxService
	result := map[string]any{"request_type": nil, "fields": []map[string]any{}}
	if inbox == nil || inbox.customFields == nil || conversation == nil {
		return result
	}
	visible := map[string]bool{}
	if conversation.RequestTypeID != nil && inbox.requestTypes != nil {
		if requestType, err := inbox.requestTypes.Get(ctx, conversation.WorkspaceID, *conversation.RequestTypeID); err == nil {
			result["request_type"] = map[string]any{"id": requestType.ID, "name": requestType.Name}
			for _, field := range requestType.Fields {
				visible[field.Key] = field.CustomerVisible
			}
		}
	}
	if len(conversation.CustomFields) == 0 {
		return result
	}
	definitions, err := inbox.customFields.List(ctx, conversation.WorkspaceID, model.CustomFieldObjectConversation, false, false)
	if err != nil {
		return result
	}
	fields := []map[string]any{}
	for _, definition := range definitions {
		value, ok := conversation.CustomFields[definition.Key]
		if !ok {
			continue
		}
		display := customfields.Display(definition.Definition(), value)
		if display == "" {
			continue
		}
		fields = append(fields, map[string]any{"key": definition.Key, "label": definition.DisplayLabel, "value": value, "display": display, "customer_visible": visible[definition.Key]})
	}
	result["fields"] = fields
	return result
}
