package model

// SupportMessageTypeEmailNotice preserves an informational incoming email in
// the timeline without projecting it as a customer reply, unread work or an
// answer. Inbox projections deliberately count only message_type=reply.
const SupportMessageTypeEmailNotice = "email_notice"

// IsSupportEmailNotice reports whether a message is an informational email.
func IsSupportEmailNotice(message *SupportMessage) bool {
	return message != nil && message.MessageType == SupportMessageTypeEmailNotice
}
