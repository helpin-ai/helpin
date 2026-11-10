package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/helpin-ai/helpin/server/internal/customfields"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportFieldsUnavailable means request types and conversation fields
// are not configured on this server.
var ErrSupportFieldsUnavailable = errors.New("conversation fields are not available")

// SetRequestTypes wires request types and conversation fields.
func (s *SupportInboxService) SetRequestTypes(types *SupportRequestTypeService, fields *CustomFieldService) *SupportInboxService {
	s.requestTypes = types
	s.customFields = fields
	return s
}

// SetConversationRequestType sets or clears a conversation's request type.
// Field values stay; the SLA is checked again for the new type.
func (s *SupportInboxService) SetConversationRequestType(ctx context.Context, workspaceID, conversationID string, requestTypeID *string, actorID string) (*model.SupportConversation, error) {
	if s.requestTypes == nil {
		return nil, ErrSupportFieldsUnavailable
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if requestTypeID != nil && *requestTypeID == "" {
		requestTypeID = nil
	}
	if sameOptionalString(conversation.RequestTypeID, requestTypeID) {
		return conversation, nil
	}
	var next *model.SupportRequestType
	if requestTypeID != nil {
		if next, err = s.requestTypes.Get(ctx, workspaceID, *requestTypeID); err != nil {
			return nil, err
		}
		if !next.Enabled {
			return nil, invalidRequestType("%s is turned off", next.Name)
		}
	}
	previousName := ""
	if conversation.RequestTypeID != nil {
		if previous, err := s.requestTypes.Get(ctx, workspaceID, *conversation.RequestTypeID); err == nil {
			previousName = previous.Name
		}
	}
	if err := s.conversationRepo.SetRequestType(ctx, workspaceID, conversation.ID, requestTypeID); err != nil {
		return nil, err
	}
	conversation.RequestTypeID = requestTypeID
	nextName := ""
	if next != nil {
		nextName = next.Name
	}
	s.teammateSystemEvent(ctx, conversation, actorID, model.SystemEventRequestTypeChanged, func(actor string) string {
		switch {
		case nextName == "":
			return fmt.Sprintf("%s cleared the request type.", actor)
		case previousName == "":
			return fmt.Sprintf("%s set the request type to %s.", actor, nextName)
		}
		return fmt.Sprintf("%s changed the request type from %s to %s.", actor, previousName, nextName)
	}, map[string]any{"previous_request_type": previousName, "request_type": nextName})
	s.publishConversationUpdated(conversation, actorID)
	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversation.ID, &actorID, "updated", strPtr("request_type"), &previousName, &nextName, nil)
	}
	return conversation, nil
}

// SetConversationFields writes conversation field values key by key; a nil
// value clears the field. Each change is noted on the timeline.
func (s *SupportInboxService) SetConversationFields(ctx context.Context, workspaceID, conversationID string, values map[string]any, actorID string) (*model.SupportConversation, error) {
	if s.customFields == nil {
		return nil, ErrSupportFieldsUnavailable
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if len(values) == 0 {
		return nil, invalidCustomField("no values to save")
	}
	active, err := s.customFields.Active(ctx, workspaceID, model.CustomFieldObjectConversation)
	if err != nil {
		return nil, err
	}
	normalized, err := validateCustomFieldValues(active, values)
	if err != nil {
		return nil, err
	}
	changed := map[string]any{}
	for key, value := range normalized {
		if !reflect.DeepEqual(comparableFieldValue(conversation.CustomFields[key]), comparableFieldValue(value)) {
			changed[key] = value
		}
	}
	if len(changed) == 0 {
		return conversation, nil
	}
	if err := s.customFields.SetValues(ctx, workspaceID, model.CustomFieldObjectConversation, conversation.ID, changed); err != nil {
		return nil, err
	}
	if conversation.CustomFields == nil {
		conversation.CustomFields = model.JSONB{}
	}
	keys := make([]string, 0, len(changed))
	for key := range changed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		definition := active[key].Definition()
		label, value := customfields.DisplayLabel(definition), changed[key]
		if value == nil {
			delete(conversation.CustomFields, key)
		} else {
			conversation.CustomFields[key] = value
		}
		s.teammateSystemEvent(ctx, conversation, actorID, model.SystemEventFieldChanged, func(actor string) string {
			if value == nil {
				return fmt.Sprintf("%s cleared %s.", actor, label)
			}
			return fmt.Sprintf("%s set %s to %s.", actor, label, customfields.Display(definition, value))
		}, map[string]any{"field": key, "value": value})
		if s.activitySvc != nil {
			display := customfields.Display(definition, value)
			_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversation.ID, &actorID, "updated", strPtr("field:"+key), nil, &display, nil)
		}
	}
	s.publishConversationUpdated(conversation, actorID)
	return conversation, nil
}

// comparableFieldValue lets stored JSON ([]any, float64) compare with
// validated values ([]string, float64).
func comparableFieldValue(value any) any {
	if values, ok := value.([]any); ok {
		result := make([]string, 0, len(values))
		for _, item := range values {
			text, _ := item.(string)
			result = append(result, text)
		}
		return result
	}
	return value
}
