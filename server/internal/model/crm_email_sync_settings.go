package model

import (
	"encoding/json"
	"strings"
	"time"
)

// CRMEmailSyncSettings defines the email sync configuration for the CRM module.
type CRMEmailSyncSettings struct {
	ID                     string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string          `json:"workspace_id" gorm:"type:uuid;uniqueIndex;not null"`
	HistoricalSyncDays     int             `json:"historical_sync_days" gorm:"not null;default:90"`
	FilterMode             string          `json:"filter_mode" gorm:"not null;default:'blocklist'"`
	FilterPatterns         json.RawMessage `json:"filter_patterns" gorm:"type:jsonb;not null;default:'[]'"`
	InternalExclusion      string          `json:"internal_exclusion" gorm:"not null;default:'none'"`
	IncludePrivateMeetings bool            `json:"include_private_meetings" gorm:"not null;default:false"`
	IncludeSoloMeetings    bool            `json:"include_solo_meetings" gorm:"not null;default:false"`
	RecordCreationMode     string          `json:"record_creation_mode" gorm:"not null;default:'selective'"`
	BlockedRecordPrefixes  json.RawMessage `json:"blocked_record_prefixes" gorm:"type:jsonb;not null;default:'[]'"`
	CreatedAt              time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMEmailSyncSettings) TableName() string { return "crm_email_sync_settings" }

// DefaultBlockedFilterPatterns contains common automated/system email patterns to block.
var DefaultBlockedFilterPatterns = []string{
	"noreply@*",
	"no-reply@*",
	"mailer-daemon@*",
	"postmaster@*",
	"*@noreply.github.com",
	"*@notifications.google.com",
	"*@mail.google.com",
	"*@bounce.google.com",
	"*@amazonses.com",
	"*@email.notifications.apple.com",
	"*@facebookmail.com",
	"*@linkedin.com",
	"*@marketing.linkedin.com",
	"*@e.linkedin.com",
	"*@reply.github.com",
	"*@docs.google.com",
	"*@calendar.google.com",
	"*@groups.google.com",
	"*@bounce.linkedin.com",
	"*@bounces.google.com",
	"*@shopify.com",
	"*@sendgrid.net",
	"*@mailchimp.com",
	"*@mandrillapp.com",
	"*@intercom-mail.com",
}

// DefaultBlockedRecordPrefixes contains common automated/system email local-part prefixes to block.
var DefaultBlockedRecordPrefixes = []string{
	"abm-alerts",
	"academy",
	"account",
	"accounts",
	"accountconfirm",
	"accounting",
	"accounts.receivable",
	"admin",
	"admin-webhook",
	"adobesign",
	"adp.wfn.application",
	"adpfeedback",
	"agent",
	"alert",
	"alerts",
	"amazon-offers",
	"announcements",
	"app",
	"applied",
	"application",
	"auto-confirm",
	"auto-reply",
	"automated",
	"automation",
	"autonotification",
	"azuredevops",
	"badmail+canned.response",
	"billing",
	"blog",
	"boardingpass",
	"chaseonline",
	"collections",
	"communications",
	"community",
	"concierge",
	"confirmation",
	"confluence",
	"consulting",
	"contact",
	"contactcloud",
	"cs-reply",
	"customer-reviews-messages",
	"customer.service",
	"customercare",
	"customerexperience",
	"customermarketing",
	"customerservice",
	"customersuccess",
	"customersuccessmanagers",
	"customersupport",
	"demandbasenews",
	"developer",
	"digest",
	"discover",
	"do-not-reply",
	"donotreply",
	"dse_na2",
	"dse_na3",
	"dse_na4",
	"ebay",
	"ebuy_admin",
	"echosign",
	"emailinfo",
	"emailreminderservice",
	"enterprise",
	"eticket",
	"etickets",
	"eventinvites",
	"events",
	"express",
	"failed-payments",
	"feedback",
	"fidelity.alerts",
	"global.insights",
	"global.vendorportal",
	"googlecloud",
	"guestfolio",
	"hello",
	"help",
	"helpmenow",
	"hp.mimecast",
	"hsfs",
	"hsfsmarketing",
	"info",
	"insights",
	"integrationsupport",
	"invitations",
	"invoice",
	"it_notification",
	"itinerary",
	"jira",
	"learn",
	"list-unsubscribe",
	"login",
	"loopsbot",
	"mail_administrator",
	"mailer",
	"mailer-daemon",
	"mailroom",
	"marketing",
	"marketingupdates",
	"marketplace-messages",
	"mcinfo",
	"message",
	"messenger",
	"mobilewebboardingpass",
	"mongodb-atlas-alerts",
	"msonlineservicesteam",
	"myresume",
	"netsuite",
	"news",
	"newsletter",
	"newsletters",
	"no-reply",
	"no-reply-aws",
	"no-response",
	"no.reply",
	"no_reply",
	"no_such_address+canned.response",
	"noreply",
	"notification",
	"notifications",
	"notify",
	"nytdirect",
	"office365",
	"office365reports",
	"onlinebanking",
	"onlineservice",
	"orders",
	"partnercomms",
	"partners-notif",
	"payments-messages",
	"peerinsightsvendorsuccess",
	"pkginfo",
	"postman-team",
	"postmaster",
	"postoffice",
	"prime",
	"privacy",
	"procurement.global",
	"product",
	"product-team",
	"productinfo",
	"purchasing",
	"qa_support",
	"quarantine",
	"receipt",
	"receipts",
	"reception",
	"reimbursements",
	"reminder",
	"renewalreminder",
	"renewals",
	"replies",
	"reply",
	"request.global",
	"reservation",
	"reservations",
	"return",
	"root",
	"sales",
	"salesops",
	"secure-reply",
	"security",
	"securitytips",
	"service",
	"servicerecoverysupport",
	"services",
	"sessions",
	"shipment-tracking",
	"shipping_notification",
	"signup_confirm",
	"startups",
	"startups-help",
	"subprocessors",
	"support",
	"supportrequests",
	"surge-alerts",
	"survey",
	"system",
	"systemmessage",
	"team",
	"thehubspotteam",
	"tips",
	"trackingupdates",
	"trailblazercommunity-notif",
	"travel",
	"travelwizard",
	"twilio",
	"uber",
	"ubereats",
	"uberone",
	"unitedairlines",
	"unsubscribe",
	"upcoming-invoice",
	"update",
	"updates",
	"virtualofficevoicemails",
	"webdesk",
	"webinar",
	"webmaster",
	"welcome",
	"workflow",
	"workspace",
	"wrikeyour_recent_stay",
	"your_recent_stay",
	"your-advocate",
	"yourdomosummary",
}

// DefaultEmailSyncSettings returns the default email sync settings.
func DefaultEmailSyncSettings() CRMEmailSyncSettings {
	filterPatterns, _ := json.Marshal(DefaultBlockedFilterPatterns)
	blockedPrefixes, _ := json.Marshal(DefaultBlockedRecordPrefixes)

	return CRMEmailSyncSettings{
		HistoricalSyncDays:     90,
		FilterMode:             "blocklist",
		FilterPatterns:         filterPatterns,
		InternalExclusion:      "none",
		IncludePrivateMeetings: false,
		IncludeSoloMeetings:    false,
		RecordCreationMode:     "selective",
		BlockedRecordPrefixes:  blockedPrefixes,
	}
}

// UpdateCRMEmailSyncSettingsRequest is the payload for updating email sync settings.
type UpdateCRMEmailSyncSettingsRequest struct {
	HistoricalSyncDays     *int             `json:"historical_sync_days"`
	FilterMode             *string          `json:"filter_mode"`
	FilterPatterns         *json.RawMessage `json:"filter_patterns"`
	InternalExclusion      *string          `json:"internal_exclusion"`
	IncludePrivateMeetings *bool            `json:"include_private_meetings"`
	IncludeSoloMeetings    *bool            `json:"include_solo_meetings"`
	RecordCreationMode     *string          `json:"record_creation_mode"`
	BlockedRecordPrefixes  *json.RawMessage `json:"blocked_record_prefixes"`
}

// GetDefaultBlockedRecordPrefixes returns a copy of the DefaultBlockedRecordPrefixes slice.
func GetDefaultBlockedRecordPrefixes() []string {
	result := make([]string, len(DefaultBlockedRecordPrefixes))
	copy(result, DefaultBlockedRecordPrefixes)
	return result
}

// ShouldFilterEmail checks whether an email address should be filtered based on sync settings.
func ShouldFilterEmail(settings *CRMEmailSyncSettings, emailAddr string) bool {
	if settings == nil {
		return false
	}
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	if emailAddr == "" {
		return false
	}

	var patterns []string
	if err := json.Unmarshal(settings.FilterPatterns, &patterns); err != nil {
		return false
	}

	matched := matchesAnyPattern(emailAddr, patterns)

	if settings.FilterMode == "blocklist" {
		return matched
	}
	// allowlist mode: filter if NOT matched
	return !matched
}

// ShouldFilterEmailParticipants applies the configured address filter to all
// external participants in a conversation. Blocklist mode excludes a message
// when any external participant matches; allowlist mode includes a message
// when at least one external participant matches.
func ShouldFilterEmailParticipants(settings *CRMEmailSyncSettings, accountEmail, fromAddr string, toAddrs, ccAddrs []string) bool {
	if settings == nil {
		return false
	}
	self := strings.ToLower(strings.TrimSpace(accountEmail))
	participants := make([]string, 0, 1+len(toAddrs)+len(ccAddrs))
	participants = append(participants, fromAddr)
	participants = append(participants, toAddrs...)
	participants = append(participants, ccAddrs...)

	var patterns []string
	if err := json.Unmarshal(settings.FilterPatterns, &patterns); err != nil {
		return false
	}
	matched := false
	seenExternal := false
	seen := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		emailAddr := strings.ToLower(strings.TrimSpace(participant))
		if emailAddr == "" || emailAddr == self {
			continue
		}
		if _, exists := seen[emailAddr]; exists {
			continue
		}
		seen[emailAddr] = struct{}{}
		seenExternal = true
		if matchesAnyPattern(emailAddr, patterns) {
			matched = true
			if settings.FilterMode == "blocklist" {
				return true
			}
		}
	}
	if settings.FilterMode == "allowlist" {
		return !seenExternal || !matched
	}
	return false
}

// matchesAnyPattern checks if an email matches any of the given glob-like patterns.
func matchesAnyPattern(email string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*@") {
			// Domain match: *@domain.com
			domain := p[1:] // "@domain.com"
			if strings.HasSuffix(email, domain) {
				return true
			}
		} else if strings.HasSuffix(p, "@*") {
			// Prefix match: prefix@*
			prefix := p[:len(p)-1] // "prefix@"
			if strings.HasPrefix(email, prefix) {
				return true
			}
		} else {
			// Exact match
			if email == p {
				return true
			}
		}
	}
	return false
}

// IsInternalEmail checks if an email is internal (all participants share the
// same domain as the account).
func IsInternalEmail(settings *CRMEmailSyncSettings, fromAddr string, toAddrs []string, ccAddrs []string, accountEmail string) bool {
	if settings.InternalExclusion != "exclude" {
		return false
	}

	accountDomain := domainFromEmail(accountEmail)
	if accountDomain == "" {
		return false
	}

	if domainFromEmail(fromAddr) != accountDomain {
		return false
	}

	for _, to := range toAddrs {
		if domainFromEmail(to) != accountDomain {
			return false
		}
	}
	for _, cc := range ccAddrs {
		if domainFromEmail(cc) != accountDomain {
			return false
		}
	}
	return true
}

// IsBlockedRecordPrefix checks if the local part of an email is in the blocked prefixes list.
func IsBlockedRecordPrefix(settings *CRMEmailSyncSettings, emailAddr string) bool {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	localPart := localPartFromEmail(emailAddr)
	if localPart == "" {
		return false
	}

	var prefixes []string
	if err := json.Unmarshal(settings.BlockedRecordPrefixes, &prefixes); err != nil {
		return false
	}

	for _, p := range prefixes {
		if strings.EqualFold(localPart, p) {
			return true
		}
	}
	return false
}

func domainFromEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func localPartFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(parts[0])
}
