package service

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Record only actual CC additions; retries and already-seen recipients should
// not create repeated timeline events. BCC fields are deliberately excluded.
func inboundCCAddedSystemMessage(conv *model.SupportConversation, payload model.PostmarkInboundPayload, added []string) *model.SupportMessage {
	names := make(map[string]string)
	for _, raw := range []string{payload.From, payload.To, payload.Cc} {
		if addresses, err := mail.ParseAddressList(raw); err == nil {
			for _, address := range addresses {
				names[strings.ToLower(address.Address)] = address.Name
			}
		}
	}
	addresses := append([]model.PostmarkAddress{payload.FromFull}, payload.ToFull...)
	addresses = append(addresses, payload.CcFull...)
	for _, address := range addresses {
		if strings.TrimSpace(address.Name) != "" {
			names[strings.ToLower(strings.TrimSpace(address.Email))] = address.Name
		}
	}
	changes := make([]string, 0, len(added))
	for _, address := range added {
		label := address
		if name := strings.Join(strings.Fields(names[address]), " "); name != "" && !strings.EqualFold(name, address) {
			label = fmt.Sprintf("%s (%s)", name, address)
		}
		changes = append(changes, label+" was added to CC.")
	}
	return &model.SupportMessage{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		SenderType: "system", MessageType: "system", IsInternal: true,
		SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventEmailRecipientsUpdated),
		Content:         strings.Join(changes, " "), Metadata: "{}",
	}
}
