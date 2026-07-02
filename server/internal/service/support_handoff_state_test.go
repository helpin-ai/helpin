package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveHandoffState(t *testing.T) {
	cases := []struct {
		name         string
		hasRecipient bool
		withinHours  bool
		want         string
	}{
		{"online within hours -> live", true, true, model.HandoffStateLive},
		{"online outside hours -> live (presence trumps hours)", true, false, model.HandoffStateLive},
		{"nobody, within hours -> busy", false, true, model.HandoffStateBusy},
		{"nobody, outside hours -> after_hours", false, false, model.HandoffStateAfterHours},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveHandoffState(c.hasRecipient, c.withinHours); got != c.want {
				t.Fatalf("resolveHandoffState(%v,%v) = %q, want %q", c.hasRecipient, c.withinHours, got, c.want)
			}
		})
	}
}
