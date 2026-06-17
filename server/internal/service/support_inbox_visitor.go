package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GetVisitorContext assembles visitor intelligence for a support conversation.
func (s *SupportInboxService) GetVisitorContext(ctx context.Context, workspaceID, conversationID string) (*model.VisitorContextResponse, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	resp := &model.VisitorContextResponse{
		OtherConversations: []model.VisitorOtherConversation{},
	}

	// Find the latest session for device/location info
	var session *model.SupportWidgetSession
	session, err = s.sessionRepo.GetLatestByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		slog.ErrorContext(ctx, "visitor context: get session by conversation failed", "error", err)
	}
	if session == nil && conversation.AnonymousID != nil {
		session, err = s.sessionRepo.GetLatestByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: get session by anonymous_id failed", "error", err)
		}
	}

	if session != nil {
		ts := session.CreatedAt.Format(time.RFC3339)
		resp.SessionCreatedAt = &ts

		// Device info from user agent
		if session.UserAgent != nil && *session.UserAgent != "" {
			device := parseUserAgent(*session.UserAgent)
			resp.Device = &device
		}

		// Location info
		resp.Location = &model.VisitorLocation{
			Timezone:    session.Timezone,
			Locale:      session.Locale,
			LastPageURL: session.LastPageURL,
			CountryCode: session.CountryCode,
			CountryName: session.CountryName,
			RegionName:  session.RegionName,
			CityName:    session.CityName,
		}
	}

	if conversation.CRMContactID != nil && *conversation.CRMContactID != "" {
		lastActiveAt, err := s.sessionRepo.GetLatestActivityByContactID(ctx, workspaceID, *conversation.CRMContactID)
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: get latest contact activity failed", "error", err, "contact_id", *conversation.CRMContactID)
		} else if lastActiveAt != nil {
			formatted := lastActiveAt.UTC().Format(time.RFC3339)
			source := "crm_contact"
			resp.LastActiveAt = &formatted
			resp.LastActiveSource = &source
		}
	}
	if resp.LastActiveAt == nil && conversation.AnonymousID != nil && *conversation.AnonymousID != "" {
		lastActiveAt, err := s.sessionRepo.GetLatestActivityByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: get latest anonymous activity failed", "error", err, "anonymous_id", *conversation.AnonymousID)
		} else if lastActiveAt != nil {
			formatted := lastActiveAt.UTC().Format(time.RFC3339)
			source := "anonymous_id"
			resp.LastActiveAt = &formatted
			resp.LastActiveSource = &source
		}
	}

	// CRM contact data
	if conversation.CRMContactID != nil && *conversation.CRMContactID != "" {
		contact, err := s.contactRepo.GetByID(ctx, *conversation.CRMContactID)
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: get CRM contact failed", "error", err, "contact_id", *conversation.CRMContactID)
		}
		if contact != nil {
			source := ""
			if contact.Source != nil {
				source = *contact.Source
			}
			name := contact.FirstName
			if contact.LastName != nil && *contact.LastName != "" {
				name += " " + *contact.LastName
			}

			customProps := make(map[string]string)
			for k, v := range contact.CustomProperties {
				if str, ok := v.(string); ok && str != "" {
					customProps[k] = str
				} else if v != nil {
					customProps[k] = fmt.Sprintf("%v", v)
				}
			}

			resp.Contact = &model.VisitorContactData{
				ID:               contact.ID,
				Name:             &name,
				Email:            contact.Email,
				Phone:            contact.Phone,
				JobTitle:         contact.JobTitle,
				LifecycleStage:   contact.LifecycleStage,
				LeadStatus:       contact.LeadStatus,
				Source:           source,
				CustomProperties: customProps,
			}
		}
	}

	// Other conversations
	const otherConvsLimit = 25

	var (
		otherConvs []model.SupportConversation
		totalCount int
	)
	if conversation.CRMContactID != nil && *conversation.CRMContactID != "" {
		convs, total, err := s.conversationRepo.ListByContact(ctx, workspaceID, *conversation.CRMContactID, model.PMPagination{Page: 1, PerPage: otherConvsLimit})
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: list contact conversations failed", "error", err)
		}
		otherConvs = convs
		totalCount = int(total)
	} else if conversation.AnonymousID != nil {
		convs, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
		if err != nil {
			slog.ErrorContext(ctx, "visitor context: list anonymous conversations failed", "error", err)
		}
		totalCount = len(convs)
		if len(convs) > otherConvsLimit {
			convs = convs[:otherConvsLimit]
		}
		otherConvs = convs
	}

	for _, c := range otherConvs {
		if c.ID == conversationID {
			continue
		}
		resp.OtherConversations = append(resp.OtherConversations, model.VisitorOtherConversation{
			ID:        c.ID,
			DisplayID: c.DisplayID,
			Subject:   c.Subject,
			Status:    c.Status,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}

	resp.TotalConversations = totalCount

	return resp, nil
}

// ── User-Agent Parsing ──────────────────────────────────────────────

var (
	reBrowserChrome  = regexp.MustCompile(`(?i)(?:Chrome|CriOS)/(\d+[\.\d]*)`)
	reBrowserFirefox = regexp.MustCompile(`(?i)(?:Firefox|FxiOS)/(\d+[\.\d]*)`)
	reBrowserSafari  = regexp.MustCompile(`(?i)Version/(\d+[\.\d]*).*Safari`)
	reBrowserEdge    = regexp.MustCompile(`(?i)Edg(?:e|A|iOS)?/(\d+[\.\d]*)`)
	reBrowserOpera   = regexp.MustCompile(`(?i)(?:OPR|Opera)/(\d+[\.\d]*)`)

	reOSWindows = regexp.MustCompile(`Windows NT (\d+[\.\d]*)`)
	reOSMac     = regexp.MustCompile(`Mac OS X (\d+[_\.\d]*)`)
	reOSLinux   = regexp.MustCompile(`Linux`)
	reOSAndroid = regexp.MustCompile(`Android (\d+[\.\d]*)`)
	reOSiOS     = regexp.MustCompile(`(?:iPhone|iPad).*OS (\d+[_\.\d]*)`)
)

// parseUserAgent extracts browser, OS, and device type from a user-agent string.
func parseUserAgent(ua string) model.VisitorDeviceInfo {
	info := model.VisitorDeviceInfo{
		Browser:    "Unknown",
		OS:         "Unknown",
		DeviceType: "desktop",
	}

	// Browser detection (order matters: Edge before Chrome, Opera before Chrome)
	switch {
	case reBrowserEdge.MatchString(ua):
		m := reBrowserEdge.FindStringSubmatch(ua)
		info.Browser = "Edge"
		if len(m) > 1 {
			info.BrowserVersion = m[1]
		}
	case reBrowserOpera.MatchString(ua):
		m := reBrowserOpera.FindStringSubmatch(ua)
		info.Browser = "Opera"
		if len(m) > 1 {
			info.BrowserVersion = m[1]
		}
	case reBrowserFirefox.MatchString(ua):
		m := reBrowserFirefox.FindStringSubmatch(ua)
		info.Browser = "Firefox"
		if len(m) > 1 {
			info.BrowserVersion = m[1]
		}
	case reBrowserChrome.MatchString(ua):
		if !reBrowserSafari.MatchString(ua) || strings.Contains(ua, "Chrome") {
			m := reBrowserChrome.FindStringSubmatch(ua)
			info.Browser = "Chrome"
			if len(m) > 1 {
				info.BrowserVersion = m[1]
			}
		}
	case reBrowserSafari.MatchString(ua):
		m := reBrowserSafari.FindStringSubmatch(ua)
		info.Browser = "Safari"
		if len(m) > 1 {
			info.BrowserVersion = m[1]
		}
	}

	// OS detection
	switch {
	case reOSiOS.MatchString(ua):
		m := reOSiOS.FindStringSubmatch(ua)
		info.OS = "iOS"
		if len(m) > 1 {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
		if strings.Contains(ua, "iPad") {
			info.DeviceType = "tablet"
		} else {
			info.DeviceType = "mobile"
		}
	case reOSAndroid.MatchString(ua):
		m := reOSAndroid.FindStringSubmatch(ua)
		info.OS = "Android"
		if len(m) > 1 {
			info.OSVersion = m[1]
		}
		info.DeviceType = "mobile"
		if strings.Contains(ua, "Tablet") || strings.Contains(ua, "SM-T") {
			info.DeviceType = "tablet"
		}
	case reOSMac.MatchString(ua):
		m := reOSMac.FindStringSubmatch(ua)
		info.OS = "macOS"
		if len(m) > 1 {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
	case reOSWindows.MatchString(ua):
		m := reOSWindows.FindStringSubmatch(ua)
		info.OS = "Windows"
		if len(m) > 1 {
			info.OSVersion = mapWindowsVersion(m[1])
		}
	case reOSLinux.MatchString(ua):
		info.OS = "Linux"
	}

	return info
}

// mapWindowsVersion maps NT version to marketing name.
func mapWindowsVersion(ntVersion string) string {
	switch ntVersion {
	case "10.0":
		return "10/11"
	case "6.3":
		return "8.1"
	case "6.2":
		return "8"
	case "6.1":
		return "7"
	default:
		return ntVersion
	}
}
