package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type dockCoverageReader struct {
	detail *model.SupportCoverageGapDetail
}

func (r dockCoverageReader) GetGapDetail(_ context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	if r.detail.WorkspaceID != workspaceID || r.detail.ID != gapID {
		return nil, gorm.ErrRecordNotFound
	}
	return r.detail, nil
}

func coverageDockService(t *testing.T) (*DockChatService, *model.SupportCoverageGapDetail) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:coverage_dock_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats ( flow_builder TEXT,
 id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, title TEXT, visibility TEXT,
 module_id TEXT, support_conversation_id TEXT, coverage_gap_id TEXT, initial_context TEXT,
 execution_enabled BOOLEAN DEFAULT false, active_run_id TEXT, archived_at DATETIME,
 last_message_at DATETIME, created_at DATETIME, updated_at DATETIME);
 CREATE UNIQUE INDEX coverage_dock_unique ON dock_chats(workspace_id, coverage_gap_id) WHERE archived_at IS NULL;`).Error; err != nil {
		t.Fatal(err)
	}
	detail := &model.SupportCoverageGapDetail{
		SupportCoverageGap:  model.SupportCoverageGap{ID: "gap-1", WorkspaceID: "ws-1", Title: "Invoice corrections", Status: "open", V1GapType: "missing_article", LastSeenAt: time.Now().UTC()},
		AnalysisExplanation: &model.SupportCoverageAnalysisExplanation{CustomerNeed: "Correct an invoice", AIFailure: "No correction guidance"},
	}
	s := &DockChatService{chatRepo: repository.NewDockChatRepository(db), coverageContextReader: dockCoverageReader{detail}}
	return s, detail
}

func TestCoverageDockChatReusesThreadAndKeepsOriginalFindings(t *testing.T) {
	s, detail := coverageDockService(t)
	id := detail.ID
	request := model.CreateDockChatRequest{CoverageGapID: &id}
	chat, err := s.CreateChat(context.Background(), "ws-1", "user-1", request)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Visibility != model.DockChatVisibilityModule || chat.ModuleID == nil || *chat.ModuleID != model.ModuleSupport {
		t.Fatalf("wrong scope: %+v", chat)
	}
	if chat.ActiveRunID != nil {
		t.Fatal("opening a chat started execution")
	}
	if chat.InitialContext == nil || !strings.Contains(chat.InitialContext.Content, "No correction guidance") {
		t.Fatalf("missing findings: %+v", chat.InitialContext)
	}
	original := chat.InitialContext.Content
	detail.AnalysisExplanation.AIFailure = "Updated finding"
	again, err := s.CreateChat(context.Background(), "ws-1", "user-1", request)
	if err != nil || again.ID != chat.ID || again.InitialContext.Content != original {
		t.Fatalf("lost saved context or duplicated thread: %+v %v", again, err)
	}
	if _, err := s.CreateChat(context.Background(), "ws-2", "user-1", request); err == nil {
		t.Fatal("cross-workspace gap accepted")
	}
}

func TestCoverageDockContextCannotBeUnpinnedByNavigation(t *testing.T) {
	id := "gap-1"
	contexts := dockChatAttachedContexts(&model.DockChat{CoverageGapID: &id}, map[string]interface{}{"entity_type": "document", "entity_id": "doc-1"}, nil)
	if len(contexts) != 2 || contexts[0].EntityType != "support_coverage_gap" || contexts[0].EntityID != id {
		t.Fatalf("lost gap: %+v", contexts)
	}
}

func TestCoverageDockDelegatesTheGapDispositionContract(t *testing.T) {
	_, detail := coverageDockService(t)
	context := coverageDockTurnContext(detail)
	if !strings.Contains(context, "delegate to the documentation agent targeting support_coverage_gap/gap-1") {
		t.Fatal("missing gap-targeted delegation")
	}
	if strings.Contains(context, "Finish every run by calling complete_support_coverage_gap") {
		t.Fatal("Ask Agent received the child's completion contract")
	}
}

func TestCoverageDockFindingsPrioritizePreparedWork(t *testing.T) {
	_, detail := coverageDockService(t)
	detail.Suggestions = []model.SupportGapSuggestion{{Status: "draft", IsActive: true}}
	if !strings.Contains(coverageDockFindings(detail).Content, "Review the proposed fix") {
		t.Fatal("prepared work still suggests drafting")
	}
}

func TestCoverageDockFindingsSaveSourceAndAnalysisDetails(t *testing.T) {
	s, detail := coverageDockService(t)
	detail.AnalysisExplanation.HumanResolution = "Billing reviews requests."
	detail.AnalysisExplanation.DecisionReason = "Several conversations needed a human answer."
	id := "conv-1"
	detail.Evidence = []model.SupportGapEvidenceView{{SupportGapEvidence: model.SupportGapEvidence{ID: "ev-1", ConversationID: &id, Excerpt: "How can I correct my invoice?"}}}
	detail.EvidenceAll = 9
	chat, err := s.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{CoverageGapID: &detail.ID})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(chat.InitialContext)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chat.InitialContext.Content, "**What worked:** Billing reviews requests.") {
		t.Fatal("confirmed resolution missing from brief")
	}
	if snapshot["source_summary"] != "Showing the latest 1 of 9 evidence records." || snapshot["sources"] == nil || snapshot["details"] == nil {
		t.Fatalf("incomplete snapshot: %s", encoded)
	}
	original := string(encoded)
	detail.Evidence[0].Excerpt = "Changed evidence"
	detail.AnalysisExplanation.DecisionReason = "Changed rationale"
	again, err := s.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{CoverageGapID: &detail.ID})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(again.InitialContext)
	if string(encoded) != original {
		t.Fatal("source or analysis snapshot changed on reopen")
	}
}

func TestCoverageDockSourceMessageRoles(t *testing.T) {
	_, detail := coverageDockService(t)
	messageID, conversationID := "message-1", "conversation-1"
	detail.Evidence = []model.SupportGapEvidenceView{{SupportGapEvidence: model.SupportGapEvidence{ID: "ev-1", MessageID: &messageID, ConversationID: &conversationID, EvidenceType: "human_reply_after_ai", Excerpt: "Billing reviews corrections."}, SenderRole: "ai"}}
	findings := coverageDockFindings(detail)
	if !strings.Contains(findings.Sources[0].Label, "AI reply") {
		t.Fatal("source lost the message author role")
	}
}

func TestCoverageDockRejectsClosedAndUnshareableAssociations(t *testing.T) {
	s, detail := coverageDockService(t)
	id, conversation := detail.ID, "conversation-1"
	private := model.DockChatVisibilityPrivate
	for _, request := range []model.CreateDockChatRequest{
		{CoverageGapID: &id, SupportConversationID: &conversation},
		{CoverageGapID: &id, Visibility: &private},
	} {
		if _, err := s.CreateChat(context.Background(), "ws-1", "user-1", request); err == nil {
			t.Fatal("invalid association accepted")
		}
	}
	detail.Status = "done"
	if _, err := s.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{CoverageGapID: &id}); err == nil {
		t.Fatal("closed gap can start work")
	}
}

func TestCoverageDockSharesReadAccessAndEnforcesTurnPermissions(t *testing.T) {
	s, detail := coverageDockService(t)
	id := detail.ID
	chat, err := s.CreateChat(context.Background(), "ws-1", "user-1", model.CreateDockChatRequest{CoverageGapID: &id})
	if err != nil {
		t.Fatal(err)
	}
	s.authz = authorization.NewAuthzService(nil, dockChatMemberRepo{}, dockChatModuleRepo{
		"member-user-1": {model.ModuleSupport}, "member-user-2": {model.ModuleSupport},
	})
	shared, err := s.FindCoverageGapChat(context.Background(), "ws-1", "user-2", id)
	if err != nil || shared == nil || shared.ID != chat.ID {
		t.Fatalf("teammate lost shared work: %+v %v", shared, err)
	}
	if _, err := s.FindCoverageGapChat(context.Background(), "ws-1", "outsider", id); err == nil {
		t.Fatal("non-support user saw gap work")
	}
	if err := s.authorizeCoverageChatTurn(context.Background(), chat, "user-2"); err == nil {
		t.Fatal("teammate bypassed creator-owned execution")
	}
	s.authz = authorization.NewAuthzService(nil, dockChatMemberRepo{}, dockChatModuleRepo{})
	allowed, err := s.canAccessChat(context.Background(), chat, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("creator retained access after support access was revoked")
	}
}
