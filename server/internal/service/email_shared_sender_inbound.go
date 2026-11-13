package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	// sharedSenderCopyDelay holds back mail that reached Helpin only through the
	// shared sender. Customers who write to it often also wrote to the reply
	// address; that copy goes first and this one is then seen as a duplicate.
	sharedSenderCopyDelay = time.Minute
	// sharedSenderCopyWindow is how recent the same text from the same sender
	// must be to count as one message sent separately to two addresses.
	sharedSenderCopyWindow = 30 * time.Minute
	// sharedSenderNoticeInterval allows one undelivered notice per sender.
	sharedSenderNoticeInterval  = 24 * time.Hour
	sharedSenderNoticeKeyPrefix = "email_shared_sender_notice:"
	sharedSenderHistoryLimit    = 50
)

// processUnaddressedInbound handles email with no mailbox hash and no route,
// mostly mail to the shared sender (support@helpin.email). Many workspaces send
// from that address, so delivery never guesses between them:
//  1. A reply to an email Helpin sent joins that email's conversation.
//  2. A new email from someone exactly one workspace emailed from the shared
//     sender goes to that workspace: into the thread whose subject it carries,
//     otherwise into a new conversation in the same inbox.
//  3. Anything else is not delivered. The job fails for platform admins to
//     review, and a person who wrote is told how to reach the business.
func (s *EmailFallbackService) processUnaddressedInbound(ctx context.Context, payload model.PostmarkInboundPayload, rawPayload string) error {
	shared := s.sharedSenderRecipient(payload)
	conv, err := s.conversationRepliedTo(ctx, payload)
	if err != nil {
		return err
	}
	if conv != nil {
		s.logger.InfoContext(ctx, "postmark inbound matched to conversation by reply headers",
			"message_id", strings.TrimSpace(payload.MessageID),
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conv.ID,
		)
		return s.addSharedSenderEmailToConversation(ctx, conv, payload, rawPayload)
	}
	if shared == "" {
		s.logger.WarnContext(ctx, "postmark inbound missing mailbox hash",
			"message_id", strings.TrimSpace(payload.MessageID),
			"original_recipient", strings.TrimSpace(payload.OriginalRecipient),
			"to", strings.TrimSpace(payload.To),
		)
		return &repository.InboundUndeliverableError{Reason: fmt.Sprintf(
			"Sent to %s, an address Helpin doesn't use, and not a reply to an email Helpin sent.",
			inboundRecipientAddress(payload))}
	}
	reason := inboundEmailAutomaticReason(payload)
	if sender := inboundEmailAddress(payload); reason == "" && (isSharedSenderAddress(shared, sender) || s.isHelpinInboxDomain(emailDomain(sender))) {
		reason = "helpin_delivery_check"
	}
	if reason != "" {
		// Not from a person and not a reply: nobody is waiting for an answer.
		s.logger.InfoContext(ctx, "postmark inbound automatic email to shared sender ignored",
			"message_id", strings.TrimSpace(payload.MessageID),
			"reason", reason,
		)
		return nil
	}

	delivered, workspaces, err := s.deliverSharedSenderEmailByHistory(ctx, shared, payload, rawPayload)
	if err != nil || delivered {
		return err
	}
	reason = fmt.Sprintf("Sent to %s and not a reply to an email Helpin sent.", shared)
	if workspaces == 0 {
		reason += " No workspace has emailed this sender from that address."
	} else {
		reason += fmt.Sprintf(" %d workspaces have emailed this sender from that address, so Helpin didn't pick one.", workspaces)
	}
	reason += " " + s.sendSharedSenderNotice(ctx, shared, payload)
	s.logger.WarnContext(ctx, "postmark inbound to shared sender not delivered",
		"message_id", strings.TrimSpace(payload.MessageID),
		"workspaces", workspaces,
	)
	return &repository.InboundUndeliverableError{Reason: reason}
}

// conversationRepliedTo finds the conversation of the Helpin email this one
// replies to, in any workspace. Helpin's Message-IDs embed the email log ID
// (<helpin-<log id>@reply domain>), so only IDs Helpin generated can match.
func (s *EmailFallbackService) conversationRepliedTo(ctx context.Context, payload model.PostmarkInboundPayload) (*model.SupportConversation, error) {
	references := map[string]string{}
	var ids []string
	for _, reference := range inboundThreadReferences(payload) {
		id := helpinEmailLogIDFromMessageID(reference)
		if id == "" {
			continue
		}
		if _, seen := references[id]; !seen {
			ids = append(ids, id)
		}
		references[id] = reference
	}
	if len(ids) == 0 {
		return nil, nil
	}
	logs, err := s.emailLogRepo.ListOutboundByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var matched []model.SupportEmailLog
	for _, log := range logs {
		if strings.EqualFold(strings.TrimSpace(log.RFCMessageID), references[log.ID]) {
			matched = append(matched, log)
		}
	}
	target := newestEmailLogInOneWorkspace(matched)
	if target == nil && len(matched) > 0 {
		// A thread forwarded between businesses names several workspaces; only
		// the email directly replied to decides.
		parents := parseRFCMessageIDList(inboundHeaderValue(payload.Headers, "In-Reply-To"))
		var direct []model.SupportEmailLog
		for _, log := range matched {
			for _, parent := range parents {
				if strings.EqualFold(parent, log.RFCMessageID) {
					direct = append(direct, log)
					break
				}
			}
		}
		target = newestEmailLogInOneWorkspace(direct)
		if target == nil {
			s.logger.WarnContext(ctx, "postmark inbound replies to emails from several workspaces",
				"message_id", strings.TrimSpace(payload.MessageID),
			)
		}
	}
	if target == nil {
		return nil, nil
	}
	return s.findConversationByID(ctx, target.ConversationID)
}

// deliverSharedSenderEmailByHistory delivers a new email when exactly one
// workspace has emailed its sender from the shared sender; that is where they
// got the address. It returns how many workspaces have.
func (s *EmailFallbackService) deliverSharedSenderEmailByHistory(ctx context.Context, shared string, payload model.PostmarkInboundPayload, rawPayload string) (bool, int, error) {
	sender := inboundEmailAddress(payload)
	workspaces, err := s.emailLogRepo.OutboundWorkspacesFromSender(ctx, shared, sender)
	if err != nil || len(workspaces) != 1 {
		return false, len(workspaces), err
	}
	workspaceID := workspaces[0]
	logs, err := s.emailLogRepo.ListRecentOutboundFromSender(ctx, workspaceID, shared, sender, sharedSenderHistoryLimit)
	if err != nil || len(logs) == 0 {
		return false, 1, err
	}

	// A copy sent to the workspace's own address delivers the email itself.
	ownAddress, err := s.workspaceAddressAmongRecipients(ctx, workspaceID, shared, payload)
	if err != nil {
		return false, 1, err
	}
	if ownAddress != "" {
		s.logger.InfoContext(ctx, "postmark inbound to shared sender left to the workspace address copy",
			"message_id", strings.TrimSpace(payload.MessageID),
			"workspace_id", workspaceID,
		)
		return true, 1, nil
	}

	// The subject names the thread when they wrote "Re: <our subject>" by hand.
	subject := normalizedEmailThreadSubject(payload.Subject)
	for _, log := range logs {
		if subject == "" || subject != normalizedEmailThreadSubject(log.Subject) {
			continue
		}
		conv, err := s.findConversationByID(ctx, log.ConversationID)
		if err != nil {
			return false, 1, err
		}
		if conv != nil && conv.AnonymizedAt == nil && !isEmailFallbackInboundTerminalStatus(conv.Status) {
			s.logger.InfoContext(ctx, "postmark inbound to shared sender matched to conversation by subject",
				"message_id", strings.TrimSpace(payload.MessageID),
				"workspace_id", workspaceID,
				"conversation_id", conv.ID,
			)
			return true, 1, s.addSharedSenderEmailToConversation(ctx, conv, payload, rawPayload)
		}
		break
	}

	if copied, err := s.emailLogRepo.HasInboundCopy(ctx, s.sharedSenderCopyQuery(workspaceID, "", payload)); err != nil || copied {
		return copied, 1, err
	}
	var mailboxID *string
	latest, err := s.findConversationByID(ctx, logs[0].ConversationID)
	if err != nil {
		return false, 1, err
	}
	if latest != nil {
		mailboxID = latest.MailboxID
	}
	s.logger.InfoContext(ctx, "postmark inbound to shared sender opened a conversation",
		"message_id", strings.TrimSpace(payload.MessageID),
		"workspace_id", workspaceID,
	)
	// An unsaved route: the conversation lands where a route to this inbox
	// would put it, without crediting any real route with the email.
	route := &model.SupportEmailRoute{WorkspaceID: workspaceID, MailboxID: mailboxID, InboundAddress: shared}
	return true, 1, s.createInboundConversationFromRoute(ctx, route, payload, rawPayload)
}

// addSharedSenderEmailToConversation adds the email unless the same message
// already reached the conversation through another address.
func (s *EmailFallbackService) addSharedSenderEmailToConversation(ctx context.Context, conv *model.SupportConversation, payload model.PostmarkInboundPayload, rawPayload string) error {
	copied, err := s.emailLogRepo.HasInboundCopy(ctx, s.sharedSenderCopyQuery(conv.WorkspaceID, conv.ID, payload))
	if err != nil {
		return err
	}
	if copied {
		s.logger.InfoContext(ctx, "postmark inbound copy already in conversation",
			"message_id", strings.TrimSpace(payload.MessageID),
			"conversation_id", conv.ID,
		)
		return nil
	}
	return s.processInboundConversationReply(ctx, conv, nil, payload, rawPayload)
}

func (s *EmailFallbackService) sharedSenderCopyQuery(workspaceID, conversationID string, payload model.PostmarkInboundPayload) repository.InboundCopyQuery {
	text := inboundPayloadProjection(payload).VisibleText
	if len(text) > 50_000 {
		text = text[:50_000]
	}
	return repository.InboundCopyQuery{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		RFCMessageID:   inboundHeaderMessageID(payload),
		FromEmail:      inboundEmailAddress(payload),
		StrippedText:   text,
		Since:          s.now().Add(-sharedSenderCopyWindow),
	}
}

// workspaceAddressAmongRecipients returns a recipient that is one of the
// workspace's own support addresses, other than the shared sender.
func (s *EmailFallbackService) workspaceAddressAmongRecipients(ctx context.Context, workspaceID, shared string, payload model.PostmarkInboundPayload) (string, error) {
	addresses, err := s.inboundSupportAddresses(ctx, workspaceID, nil, payload)
	if err != nil {
		return "", err
	}
	own := map[string]struct{}{}
	for _, address := range addresses {
		if !isSharedSenderAddress(shared, address) {
			own[strings.ToLower(address)] = struct{}{}
		}
	}
	for _, recipient := range inboundRecipientAddresses(payload) {
		if _, ok := own[strings.ToLower(recipient)]; ok {
			return recipient, nil
		}
	}
	return "", nil
}

// sendSharedSenderNotice tells a person whose email was not delivered how to
// reach the business, at most once a day, and only when their own domain
// vouches for the address so a forged sender never receives it. It returns a
// sentence for the admin queue saying what happened.
func (s *EmailFallbackService) sendSharedSenderNotice(ctx context.Context, shared string, payload model.PostmarkInboundPayload) string {
	sender := strings.ToLower(inboundEmailAddress(payload))
	switch {
	case s.redis == nil:
		return "No notice was sent: notices need Redis."
	case sender == "" || s.isHelpinAddress(sender):
		return "No notice was sent: the sender is a Helpin address."
	case !inboundSenderAuthenticated(payload):
		return "No notice was sent: the sender's domain didn't confirm the address."
	}
	key := sharedSenderNoticeKeyPrefix + sender
	first, err := s.redis.SetNX(ctx, key, s.now().UTC().Format(time.RFC3339), sharedSenderNoticeInterval).Result()
	if err != nil {
		s.logger.WarnContext(ctx, "shared sender notice rate limit unavailable", "error", err)
		return "No notice was sent: Redis was unavailable."
	}
	if !first {
		return "No notice was sent: the sender already got one in the last 24 hours."
	}

	subject, textBody, htmlBody := sharedSenderNoticeContent(shared, payload)
	headers := []email.EmailHeader{
		{Name: "Auto-Submitted", Value: "auto-replied"},
		{Name: "X-Auto-Response-Suppress", Value: "All"},
	}
	if messageID := inboundHeaderMessageID(payload); messageID != "" {
		references := strings.TrimSpace(inboundHeaderValue(payload.Headers, "References") + " " + messageID)
		headers = append(headers,
			email.EmailHeader{Name: "In-Reply-To", Value: messageID},
			email.EmailHeader{Name: "References", Value: references},
		)
	}
	from := fmt.Sprintf("Helpin <%s>", shared)
	if _, err := s.emailClient.SendEmailWithHeadersAttachmentsAndOptionsContext(ctx, from, sender, subject, htmlBody, textBody, "", headers, nil, email.SendEmailOptions{}); err != nil {
		s.logger.ErrorContext(ctx, "shared sender notice failed", "error", err, "message_id", strings.TrimSpace(payload.MessageID))
		if delErr := s.redis.Del(ctx, key).Err(); delErr != nil {
			s.logger.WarnContext(ctx, "shared sender notice rate limit not cleared", "error", delErr)
		}
		return "No notice was sent: sending it failed."
	}
	s.logger.InfoContext(ctx, "shared sender notice sent", "message_id", strings.TrimSpace(payload.MessageID))
	return "The sender was told to reply to the original email instead."
}

func sharedSenderNoticeContent(shared string, payload model.PostmarkInboundPayload) (subject, textBody, htmlBody string) {
	original := strings.TrimSpace(payload.Subject)
	subject = "Not delivered: " + original
	if original == "" {
		subject = "Your email was not delivered"
		original = "(no subject)"
	}
	greeting := "Hi,"
	if name := strings.TrimSpace(payload.FromFull.Name); name != "" {
		greeting = "Hi " + name + ","
	}
	about := fmt.Sprintf("%s sends emails for businesses that use Helpin. Emails sent to it don't reach any of them.", shared)
	textBody = strings.Join([]string{
		greeting,
		fmt.Sprintf("Your email \"%s\" was not delivered.", original),
		about,
		"To reach the business:\n- Reply to an email you received from them. Your reply goes straight to their team.\n- Or contact them through their website.",
		"This is an automatic message, so please don't reply to it.",
	}, "\n\n")
	htmlBody = fmt.Sprintf(`<p>%s</p><p>Your email “%s” was not delivered.</p><p>%s</p><p>To reach the business:</p><ul><li>Reply to an email you received from them. Your reply goes straight to their team.</li><li>Or contact them through their website.</li></ul><p>This is an automatic message, so please don’t reply to it.</p>`,
		html.EscapeString(greeting), html.EscapeString(original), html.EscapeString(about))
	return subject, textBody, htmlBody
}

// sharedSenderRecipient returns the shared sender when the email was sent to
// it or to a plus-tagged form of it.
func (s *EmailFallbackService) sharedSenderRecipient(payload model.PostmarkInboundPayload) string {
	shared := strings.ToLower(strings.TrimSpace(s.emailClient.FromEmail()))
	if shared == "" {
		return ""
	}
	for _, address := range inboundRecipientAddresses(payload) {
		if isSharedSenderAddress(shared, address) {
			return shared
		}
	}
	return ""
}

// isSharedSenderCopy reports mail that reached Helpin only through the shared
// sender, with no recipient on the reply or inbox domains.
func (s *EmailFallbackService) isSharedSenderCopy(payload model.PostmarkInboundPayload) bool {
	shared := strings.ToLower(strings.TrimSpace(s.emailClient.FromEmail()))
	viaShared := false
	for _, address := range inboundRecipientAddresses(payload) {
		_, domain, _ := strings.Cut(address, "@")
		if s.isHelpinInboxDomain(domain) {
			return false
		}
		viaShared = viaShared || isSharedSenderAddress(shared, address)
	}
	return viaShared
}

func (s *EmailFallbackService) isHelpinInboxDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	routeDomain := ""
	if s.supportInboxService != nil {
		routeDomain = strings.ToLower(s.supportInboxService.routeDomain)
	}
	return domain != "" && (domain == strings.ToLower(s.InboundDomain()) ||
		(routeDomain != "" && (domain == routeDomain || strings.HasSuffix(domain, "."+routeDomain))))
}

// isHelpinAddress covers the shared sender's whole domain and the inbox
// domains, so a notice can never be sent to Helpin itself.
func (s *EmailFallbackService) isHelpinAddress(address string) bool {
	domain := emailDomain(address)
	if domain == "" {
		return false
	}
	sharedDomain := emailDomain(s.emailClient.FromEmail())
	return s.isHelpinInboxDomain(domain) || (sharedDomain != "" && domain == sharedDomain)
}

// isSharedSenderAddress matches the shared sender and its plus-tagged forms.
func isSharedSenderAddress(shared, address string) bool {
	sharedLocal, sharedDomain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(shared)), "@")
	local, domain, found := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	if !ok || !found || sharedLocal == "" || domain != sharedDomain {
		return false
	}
	base, _, _ := strings.Cut(local, "+")
	return base == sharedLocal
}

// helpinEmailLogIDFromMessageID reads the log ID from a Message-ID Helpin
// generated (<helpin-<log id>@domain>).
func helpinEmailLogIDFromMessageID(reference string) string {
	local, _, ok := strings.Cut(strings.Trim(strings.TrimSpace(reference), "<>"), "@")
	if !ok {
		return ""
	}
	id, found := strings.CutPrefix(local, "helpin-")
	if !found {
		return ""
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ""
	}
	return parsed.String()
}

func newestEmailLogInOneWorkspace(logs []model.SupportEmailLog) *model.SupportEmailLog {
	var newest *model.SupportEmailLog
	for i := range logs {
		if newest != nil && newest.WorkspaceID != logs[i].WorkspaceID {
			return nil
		}
		if newest == nil || logs[i].CreatedAt.After(newest.CreatedAt) {
			newest = &logs[i]
		}
	}
	return newest
}

// inboundHeaderMessageID is the sender's own Message-ID, shared by every copy
// of the email; empty when the header is missing.
func inboundHeaderMessageID(payload model.PostmarkInboundPayload) string {
	return normalizeRFCHeaderValue(inboundHeaderValue(payload.Headers, "Message-ID"))
}

// inboundEmailAutomaticReason names why an email looks automatic (auto-reply,
// mailing list, delivery report, no-reply sender, spam); empty for a person.
func inboundEmailAutomaticReason(payload model.PostmarkInboundPayload) string {
	content := inboundPayloadProjection(payload).VisibleText
	metadata := inboundEmailAIMetadata(postmarkInboundSpamSignalsFromHeaders(payload.Headers).messageMetadata(), payload, content)
	var value struct {
		Reason string `json:"email_ai_suppression_reason"`
	}
	if err := json.Unmarshal([]byte(metadata), &value); err != nil {
		return ""
	}
	return value.Reason
}

// inboundSenderAuthenticated reports whether the From address's own domain
// signed the email (DKIM) or sent it (SPF on a matching envelope sender).
func inboundSenderAuthenticated(payload model.PostmarkInboundPayload) bool {
	_, fromDomain, ok := strings.Cut(strings.ToLower(inboundEmailAddress(payload)), "@")
	if !ok || fromDomain == "" {
		return false
	}
	tests := strings.FieldsFunc(strings.ToUpper(inboundHeaderValue(payload.Headers, "X-Spam-Tests")), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	for _, test := range tests {
		if test == "DKIM_VALID_AU" {
			return true
		}
	}
	spf := strings.ToLower(inboundHeaderValue(payload.Headers, "Received-SPF"))
	if !strings.HasPrefix(spf, "pass") {
		return false
	}
	_, rest, found := strings.Cut(spf, "envelope-from=")
	if !found {
		return false
	}
	envelope := strings.FieldsFunc(rest, func(r rune) bool { return r == ';' || unicode.IsSpace(r) })
	if len(envelope) == 0 {
		return false
	}
	_, envelopeDomain, _ := strings.Cut(strings.Trim(envelope[0], `<>"`), "@")
	return envelopeDomain == fromDomain || strings.HasSuffix(envelopeDomain, "."+fromDomain)
}

// normalizedEmailThreadSubject drops reply and forward prefixes so a subject
// typed as "Re: Topic (#12)" matches the thread "Topic (#12)".
func normalizedEmailThreadSubject(subject string) string {
	value := strings.ToLower(strings.Join(strings.Fields(subject), " "))
	for {
		trimmed := false
		for _, prefix := range []string{"re:", "fw:", "fwd:", "aw:", "wg:", "sv:", "antw:", "tr:", "rif:"} {
			if rest, ok := strings.CutPrefix(value, prefix); ok {
				value = strings.TrimSpace(rest)
				trimmed = true
			}
		}
		if !trimmed {
			return value
		}
	}
}
