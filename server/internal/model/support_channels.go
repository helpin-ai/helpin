package model

// Support channel registry. A conversation's channel names the medium a
// customer used; every policy that enumerates channels reads it from here, so
// adding a channel is one entry. Mirror changes in
// frontend/src/lib/supportChannels.ts.
//
// Keys are matched exactly; callers normalise as they always have, and unknown
// values (imports such as "intercom", or free-form API sources) get each
// consumer's existing fallback.

const (
	SupportChannelWidget   = "widget"
	SupportChannelEmail    = "email"
	SupportChannelPortal   = "portal"
	SupportChannelAPI      = "api"
	SupportChannelInternal = "internal"
	SupportChannelTelegram = "telegram"
)

// SupportChannelTransport is how replies reach the customer.
type SupportChannelTransport string

const (
	SupportTransportChat   SupportChannelTransport = "chat"   // live widget session
	SupportTransportPortal SupportChannelTransport = "portal" // customer portal, with email notices
	SupportTransportEmail  SupportChannelTransport = "email"  // outbound email
	SupportTransportNone   SupportChannelTransport = "none"   // no channel-specific delivery
	// SupportTransportProvider delivers through a connected account's provider
	// adapter and the channel outbox.
	SupportTransportProvider SupportChannelTransport = "provider"
)

// SupportAIMode names the AI reply setting that governs a channel.
type SupportAIMode string

const (
	SupportAIModeChat   SupportAIMode = "chat"   // ai_reply_channels chat
	SupportAIModeEmail  SupportAIMode = "email"  // ai_reply_channels email
	SupportAIModePortal SupportAIMode = "portal" // portal AI drafts
	SupportAIModeNone   SupportAIMode = ""
)

// SupportTriageSetting names the per-channel triage toggle that applies.
type SupportTriageSetting string

const (
	SupportTriageSettingWidget   SupportTriageSetting = "widget"
	SupportTriageSettingEmail    SupportTriageSetting = "email"
	SupportTriageSettingInternal SupportTriageSetting = "internal"
)

// SupportChannelSpec is the policy one channel applies.
type SupportChannelSpec struct {
	Key       string
	Transport SupportChannelTransport
	AIMode    SupportAIMode
	Triage    SupportTriageSetting
	// TriageRuleChannel allows triage rules to target the channel.
	TriageRuleChannel bool
	// SLAChannel allows SLA policies to target the channel.
	SLAChannel bool
	// SkipsDefaultMailbox leaves new conversations unrouted instead of placing
	// them in the workspace default inbox.
	SkipsDefaultMailbox bool
	// SetupEvidence counts resolved conversations toward workspace activation.
	SetupEvidence bool
}

var supportChannels = []SupportChannelSpec{
	{Key: SupportChannelWidget, Transport: SupportTransportChat, AIMode: SupportAIModeChat, Triage: SupportTriageSettingWidget, TriageRuleChannel: true, SLAChannel: true, SkipsDefaultMailbox: true, SetupEvidence: true},
	{Key: SupportChannelEmail, Transport: SupportTransportEmail, AIMode: SupportAIModeEmail, Triage: SupportTriageSettingEmail, TriageRuleChannel: true, SLAChannel: true, SetupEvidence: true},
	{Key: SupportChannelPortal, Transport: SupportTransportPortal, AIMode: SupportAIModePortal, Triage: SupportTriageSettingWidget, SLAChannel: true},
	{Key: SupportChannelAPI, Transport: SupportTransportNone, Triage: SupportTriageSettingInternal, TriageRuleChannel: true, SLAChannel: true, SetupEvidence: true},
	{Key: SupportChannelInternal, Transport: SupportTransportNone, Triage: SupportTriageSettingInternal, TriageRuleChannel: true, SLAChannel: true},
	{Key: SupportChannelTelegram, Transport: SupportTransportProvider, AIMode: SupportAIModeChat, Triage: SupportTriageSettingWidget, TriageRuleChannel: true, SLAChannel: true, SetupEvidence: true},
}

// LookupSupportChannel matches key exactly.
func LookupSupportChannel(key string) (SupportChannelSpec, bool) {
	for _, spec := range supportChannels {
		if spec.Key == key {
			return spec, true
		}
	}
	return SupportChannelSpec{}, false
}

// SupportAIChannelSupported reports whether AI may run on a normalised
// channel. Blank and "chat" are the widget's aliases.
func SupportAIChannelSupported(channel string) bool {
	if channel == "" || channel == string(SupportAIModeChat) {
		return true
	}
	spec, ok := LookupSupportChannel(channel)
	return ok && spec.AIMode != SupportAIModeNone
}

// SupportAIModeChannels lists the channels governed by an AI reply mode, in
// registry order.
func SupportAIModeChannels(mode SupportAIMode) []string {
	channels := []string{}
	for _, spec := range supportChannels {
		if spec.AIMode == mode {
			channels = append(channels, spec.Key)
		}
	}
	return channels
}

// SupportSetupEvidenceChannels lists the channels whose resolved
// conversations count toward workspace activation, in registry order.
func SupportSetupEvidenceChannels() []string {
	var channels []string
	for _, spec := range supportChannels {
		if spec.SetupEvidence {
			channels = append(channels, spec.Key)
		}
	}
	return channels
}

// SupportChannelHasChatSession reports whether a conversation source implies a
// live chat session. Imported conversations keep their provenance as source
// and have none.
func SupportChannelHasChatSession(source string) bool {
	spec, ok := LookupSupportChannel(source)
	return ok && spec.Transport == SupportTransportChat
}
