//go:build integration

package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestConfiguredReplyDestinationsPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	for _, statement := range []string{
		"CREATE TEMP TABLE support_channel_accounts (id text, workspace_id text, provider text, status text, disabled_at timestamptz)",
		"CREATE TEMP TABLE slack_channel_routes (account_id text, channel_id text, channel_name text, purpose text)",
		"CREATE TEMP TABLE support_conversations (id text, workspace_id text, channel text, display_id int, channel_account_id text, provider_thread_key text, customer_name text, channel_identity_id text, anonymized_at timestamptz)",
		"CREATE TEMP TABLE support_messages (workspace_id text, conversation_id text, origin_conversation_id text, sender_type text, channel_account_id text, deleted_at timestamptz, created_at timestamptz)",
		"INSERT INTO support_channel_accounts VALUES ('a','ws','slack','active',NULL), ('b','ws','telegram','active',NULL), ('old','ws','telegram','disabled',now())",
		"INSERT INTO slack_channel_routes VALUES ('a','C1','billing','support'), ('a','C2','roadmap','pm')",
		"INSERT INTO support_conversations VALUES ('s','ws','slack',1,'a','C1:1',NULL,'i',NULL), ('pm','ws','slack',2,'a','C2:1',NULL,'i',NULL), ('dm','ws','telegram',3,'b',NULL,'Lena','j',NULL), ('archived','ws','telegram',4,'old',NULL,'Another sender','k',NULL)",
	} {
		mustExec(t, db, statement)
	}
	repo := repository.NewSupportConversationRepository(db)
	identity := "i"
	check := func(c model.SupportConversation, want ...string) {
		t.Helper()
		c.WorkspaceID = "ws"
		targets, err := repo.ReplyDestinations(context.Background(), &c)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, target := range targets {
			got = append(got, target.Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("conversation %s: destinations %v, want %v", c.ID, got, want)
		}
	}
	check(model.SupportConversation{ID: "unrelated"})
	check(model.SupportConversation{ID: "s", ChannelIdentityID: &identity}, "#billing")
	check(model.SupportConversation{ID: "pm", ChannelIdentityID: &identity})
	root := model.SupportConversation{ID: "root", MergeHasChannel: true, ProviderDestinations: []model.SupportProviderDestination{{ID: "s"}, {ID: "pm"}, {ID: "dm"}, {ID: "archived"}}}
	check(root, "#billing", "Lena")
	mustExec(t, db, "DELETE FROM slack_channel_routes WHERE channel_id='C1'")
	check(root, "Lena")
	root.ProviderDestinations = nil
	mustExec(t, db, "INSERT INTO support_messages VALUES ('ws','root','dm','customer','b',NULL,now())")
	check(root, "Lena")
	mustExec(t, db, "INSERT INTO support_messages VALUES ('ws','root','archived','customer','old',NULL,now()+interval '1 minute')")
	check(root)
	mustExec(t, db, "UPDATE support_channel_accounts SET status='reauthorization_required' WHERE id='b'")
	check(model.SupportConversation{ID: "dm", ChannelIdentityID: &identity})
}
