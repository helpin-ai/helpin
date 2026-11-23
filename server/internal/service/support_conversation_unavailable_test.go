package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Deep links open a conversation by ID wherever it lives; missing and
// access-filtered records must look identical to the caller.
func TestGetConversation_UnavailableIsIndistinguishableAndAccessRulesHold(t *testing.T) {
	f := newInboxAlertFixture(t)
	db := f.db
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db), repository.NewSupportMessageRepository(db), nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, repository.NewSupportCannedResponseRepository(db), nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	as := func(userID, role string) context.Context {
		return authorization.WithActor(context.Background(), &authorization.Actor{UserID: userID, WorkspaceID: f.workspaceID, WorkspaceMemberID: "wm-" + userID, Role: role})
	}
	get := func(ctx context.Context, id string) error {
		conv, err := inbox.GetConversation(ctx, f.workspaceID, id)
		if err == nil && (conv == nil || conv.ID != id) {
			t.Fatalf("GetConversation(%s) returned %+v", id, conv)
		}
		return err
	}
	move := func(id, mailboxID string) {
		mustExec(t, db, `UPDATE support_conversations SET mailbox_id = ? WHERE id = ?`, mailboxID, id)
	}
	bea, sam, ben := as("billing-bea", model.RoleMember), as("shared-sam", model.RoleMember), as("billing-ben", model.RoleMember)
	admin := as("admin-ada", model.RoleAdmin)

	conv := f.conversation(t, f.inboxes["Billing"])
	if err := get(bea, conv.ID); err != nil {
		t.Fatalf("member opens own inbox conversation: %v", err)
	}

	missing := get(bea, uuid.NewString())
	nonMember := get(sam, conv.ID)
	for name, err := range map[string]error{"missing": missing, "non-member": nonMember} {
		if !errors.Is(err, ErrSupportConversationUnavailable) {
			t.Fatalf("%s: err = %v, want ErrSupportConversationUnavailable", name, err)
		}
	}
	if missing.Error() != nonMember.Error() {
		t.Fatalf("missing %q and denied %q responses differ", missing, nonMember)
	}

	// Moved into another inbox the viewer can still access: same ID opens.
	move(conv.ID, f.inboxes["Sales"])
	if err := get(ben, conv.ID); err != nil {
		t.Fatalf("moved with maintained access: %v", err)
	}
	// The move removed access for a Billing-only member.
	if err := get(bea, conv.ID); !errors.Is(err, ErrSupportConversationUnavailable) {
		t.Fatalf("move that removes access: err = %v", err)
	}

	// Archived inbox: members lose access, elevated viewers keep it.
	move(conv.ID, f.inboxes["Billing"])
	mustExec(t, db, `UPDATE support_mailboxes SET active = false WHERE id = ?`, f.inboxes["Billing"])
	if err := get(bea, conv.ID); !errors.Is(err, ErrSupportConversationUnavailable) {
		t.Fatalf("archived inbox member: err = %v", err)
	}
	if err := get(admin, conv.ID); err != nil {
		t.Fatalf("admin opens archived-inbox conversation: %v", err)
	}
}
