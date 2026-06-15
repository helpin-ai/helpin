package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestFormatAssignmentSystemMessageUsesActorFirstCopy(t *testing.T) {
	cases := []struct {
		name         string
		target       assignmentTargetKind
		actorName    string
		targetName   string
		actorUserID  string
		targetUserID string
		want         string
	}{
		{
			name:         "assigns teammate",
			target:       assignmentTargetUser,
			actorName:    "Sarah Khan",
			targetName:   "Ali Raza",
			actorUserID:  "user-sarah",
			targetUserID: "user-ali",
			want:         "Sarah assigned this conversation to Ali.",
		},
		{
			name:         "takes conversation",
			target:       assignmentTargetUser,
			actorName:    "Sarah Khan",
			targetName:   "Sarah Khan",
			actorUserID:  "user-sarah",
			targetUserID: "user-sarah",
			want:         "Sarah took this conversation.",
		},
		{
			name:        "unassigns",
			target:      assignmentTargetUnassign,
			actorName:   "Sarah Khan",
			actorUserID: "user-sarah",
			want:        "Sarah moved this conversation to unassigned.",
		},
		{
			name:        "assigns agent",
			target:      assignmentTargetAgent,
			actorName:   "Sarah Khan",
			targetName:  "Billing AI",
			actorUserID: "user-sarah",
			want:        "Sarah assigned this conversation to Billing AI.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAssignmentSystemMessage(tc.target, tc.actorName, tc.targetName, tc.actorUserID, tc.targetUserID)
			if got != tc.want {
				t.Fatalf("formatAssignmentSystemMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAutoMoveMessageUsesRoutingSourceCopy(t *testing.T) {
	if got := autoMoveMessage(model.SupportConversationTriageSourceRule, nil, "Billing"); got != "Routing rule moved to inbox 'Billing'." {
		t.Fatalf("rule autoMoveMessage() = %q", got)
	}
	if got := autoMoveMessage(model.SupportConversationTriageSourceAI, nil, "Billing"); got != "AI routing moved to inbox 'Billing'." {
		t.Fatalf("ai autoMoveMessage() = %q", got)
	}
}
