package service

import (
	"fmt"
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// The channel behaviour matrix pins how every channel value, including unknown
// and unnormalised ones, resolves in each policy consumer. Adding a channel
// must only add rows; changing an existing row is a behaviour change.
var supportChannelMatrixInputs = []string{"", "widget", "chat", "email", "portal", "api", "internal", "intercom", " Email ", "Widget", "telegram"}

func TestSupportChannelMatrixAIChannelEnabled(t *testing.T) {
	// want lists the inputs enabled for each saved selection, with portal AI
	// drafts off and on.
	for _, tc := range []struct {
		selection string
		drafts    bool
		want      []string
	}{
		{"", false, []string{"", "widget", "chat", "Widget"}},
		{"chat", false, []string{"", "widget", "chat", "Widget"}},
		{"email", false, []string{"email", " Email "}},
		{"both", false, []string{"", "widget", "chat", "email", " Email ", "Widget"}},
		{"invalid", false, nil},
		{"chat", true, []string{"", "widget", "chat", "portal", "Widget"}},
		{"invalid", true, []string{"portal"}},
	} {
		settings := model.DefaultSupportInboxSettings()
		settings.AIReplyChannels = tc.selection
		settings.PortalEnabled = tc.drafts
		settings.PortalAIMode = ""
		if tc.drafts {
			settings.PortalAIMode = model.SupportPortalAIModeInternalNote
		}
		for _, channel := range supportChannelMatrixInputs {
			want := slices.Contains(tc.want, channel)
			if got := model.SupportAIChannelEnabled(settings, channel); got != want {
				t.Errorf("selection=%q drafts=%v channel=%q: got %v want %v", tc.selection, tc.drafts, channel, got, want)
			}
		}
	}
}

func TestSupportChannelMatrixTriage(t *testing.T) {
	// Callers normalise before resolving the triage setting, so unnormalised
	// values deliberately fall through to the widget setting.
	setting := map[string]string{
		"": "widget", "widget": "widget", "chat": "widget", "email": "email", "portal": "widget",
		"api": "internal", "internal": "internal", "intercom": "widget", " Email ": "widget", "Widget": "widget", "telegram": "widget",
	}
	for _, channel := range supportChannelMatrixInputs {
		for _, enabled := range []string{"widget", "email", "internal"} {
			settings := model.SupportInboxSettings{
				TriageWidgetEnabled:   enabled == "widget",
				TriageEmailEnabled:    enabled == "email",
				TriageInternalEnabled: enabled == "internal",
			}
			want := setting[channel] == enabled
			if got := triageChannelEnabled(settings, channel); got != want {
				t.Errorf("channel=%q enabled=%s: got %v want %v", channel, enabled, got, want)
			}
		}
	}

	ruleChannels := []string{"widget", "email", "api", "internal", " Email ", "Widget"}
	for _, channel := range supportChannelMatrixInputs {
		if got, want := validSupportTriageChannel(channel), slices.Contains(ruleChannels, channel); got != want {
			t.Errorf("rule channel %q: got %v want %v", channel, got, want)
		}
	}
}

func TestSupportChannelMatrixSLAPolicyChannels(t *testing.T) {
	selectable := []string{"widget", "email", "portal", "api", "internal"}
	for _, channel := range supportChannelMatrixInputs {
		_, err := normalizeSLAList([]string{channel}, supportSLAChannelAllowed, "channel")
		// Values are trimmed, not lowercased, before the check; blanks are skipped.
		trimmed := map[string]string{" Email ": "Email"}[channel]
		if trimmed == "" {
			trimmed = channel
		}
		want := trimmed == "" || slices.Contains(selectable, trimmed)
		if got := err == nil; got != want {
			t.Errorf("SLA channel %q: accepted=%v want %v (%v)", channel, got, want, err)
		}
	}
}

func TestSupportChannelMatrixDefaultMailbox(t *testing.T) {
	if shouldUseDefaultMailboxForChannel(nil, "email") {
		t.Fatal("nil settings must not use the default inbox")
	}
	for _, channel := range supportChannelMatrixInputs {
		for _, tc := range []struct {
			triage, widget, email, internal bool
			fallback                        string
		}{
			{false, true, true, true, "shared"},
			{true, true, true, true, "shared"},
			{true, true, true, true, "default"},
			{true, false, false, false, "shared"},
		} {
			settings := &model.SupportInboxSettings{
				TriageEnabled: tc.triage, TriageWidgetEnabled: tc.widget, TriageEmailEnabled: tc.email,
				TriageInternalEnabled: tc.internal, TriageFallbackBehavior: tc.fallback,
			}
			// Widget conversations never take the default inbox; any other channel
			// does unless triage handles it with a shared fallback.
			want := true
			switch channel {
			case "widget", "Widget":
				want = false
			default:
				if tc.triage && tc.fallback == "shared" {
					want = !(tc.widget || tc.email || tc.internal)
				}
			}
			name := fmt.Sprintf("channel=%q %+v", channel, tc)
			if got := shouldUseDefaultMailboxForChannel(settings, channel); got != want {
				t.Errorf("%s: got %v want %v", name, got, want)
			}
		}
	}
}

func TestSupportChannelMatrixChatSession(t *testing.T) {
	// Only a widget-sourced conversation has a live chat session by source;
	// imported conversations keep their provenance and have none.
	for _, source := range supportChannelMatrixInputs {
		want := source == "widget"
		if got := model.SupportChannelHasChatSession(source); got != want {
			t.Errorf("source=%q: got %v want %v", source, got, want)
		}
	}
}
