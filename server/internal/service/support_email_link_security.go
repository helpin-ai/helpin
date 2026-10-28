package service

import (
	"context"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Lookups run one after another with a short per-link timeout; this bounds a full email.
const supportEmailLinkScanTimeout = 20 * time.Second

// supportEmailLinkScanner checks the links in a saved inbound email.
type supportEmailLinkScanner interface {
	ScanEmailLinks(ctx context.Context, msg *model.SupportMessage) []model.SupportLinkSecurity
}

// ScanEmailLinks checks the reputation of links in an inbound customer email,
// including addresses hidden behind link text. Email links are only looked up,
// never fetched: opening one could confirm a tracking link or unsubscribe the customer.
func (s *SupportLinkPreviewService) ScanEmailLinks(ctx context.Context, msg *model.SupportMessage) []model.SupportLinkSecurity {
	if s == nil || s.linkScanner == nil || !isCustomerEmailMessage(msg) {
		return nil
	}
	urls := extractSupportEmailURLs(msg.HTMLBody, msg.Content, s.maxScans)
	if len(urls) == 0 {
		return nil
	}
	return s.scanLinks(ctx, urls)
}

// extractSupportEmailURLs returns link targets from an email's HTML, then any
// addresses written out in its text, normalized and without duplicates.
func extractSupportEmailURLs(htmlBody, text string, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	seen := make(map[string]struct{})
	var result []string
	tokenizer := html.NewTokenizer(strings.NewReader(htmlBody))
	for len(result) < limit {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.DataAtom != atom.A && token.DataAtom != atom.Area {
			continue
		}
		for _, attr := range token.Attr {
			if attr.Key == "href" {
				result = appendSupportLinkURL(result, seen, attr.Val, false)
			}
		}
	}
	for _, candidate := range supportPreviewURLPattern.FindAllString(text, -1) {
		if len(result) >= limit {
			break
		}
		result = appendSupportLinkURL(result, seen, candidate, true)
	}
	return result
}

// SetEmailLinkScanner enables reputation checks for links in inbound customer email.
func (s *EmailFallbackService) SetEmailLinkScanner(scanner supportEmailLinkScanner) *EmailFallbackService {
	if s == nil {
		return nil
	}
	s.emailLinkScanner = scanner
	return s
}

// scanInboundEmailLinksAsync checks links after the email is saved, so a slow
// lookup never delays the inbound webhook. A message update event lets the
// inbox refetch the email and show any warning.
func (s *EmailFallbackService) scanInboundEmailLinksAsync(msg *model.SupportMessage, actorID string) {
	if s.emailLinkScanner == nil || s.messageRepo == nil || !isCustomerEmailMessage(msg) {
		return
	}
	message := *msg
	run := s.runAsync
	if run == nil {
		run = func(task func()) { go task() }
	}
	run(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("inbound email link check panicked", "message_id", message.ID, "panic", recovered)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), supportEmailLinkScanTimeout)
		defer cancel()
		s.scanInboundEmailLinks(ctx, &message, actorID)
	})
}

func (s *EmailFallbackService) scanInboundEmailLinks(ctx context.Context, msg *model.SupportMessage, actorID string) {
	security := s.emailLinkScanner.ScanEmailLinks(ctx, msg)
	if !supportLinkSecurityHasVerdict(security) {
		return
	}
	// Save only the verdicts: the AI may have recorded a decision on this message meanwhile.
	metadata, err := s.messageRepo.SetMetadataKey(ctx, msg.ID, supportLinkSecurityMetadataKey, security)
	if err != nil {
		s.logger.WarnContext(ctx, "persist inbound email link security failed",
			"workspace_id", msg.WorkspaceID, "message_id", msg.ID, "error", err)
		return
	}
	msg.Metadata = metadata
	s.wsPublisher.Publish(websocket.SupportMessageUpdatedEvent(msg.WorkspaceID, msg, actorID))
}

func isCustomerEmailMessage(msg *model.SupportMessage) bool {
	return msg != nil && !msg.IsInternal && strings.EqualFold(strings.TrimSpace(msg.SenderType), "customer")
}

// supportLinkSecurityHasVerdict reports whether any lookup returned a real answer.
// Unknown results show nothing, so storing them would only add writes.
func supportLinkSecurityHasVerdict(security []model.SupportLinkSecurity) bool {
	for _, entry := range security {
		if entry.Status != supportLinkSecurityUnknown {
			return true
		}
	}
	return false
}
