package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportSourceLinksPublicArticles(t *testing.T) {
	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	seedLinkTestWorkspace(t, db, linkTestWorkspaceID, linkTestSpaceID, model.DocsStringArray{"en", "fr"})
	seedLinkTestWorkspace(t, db, "other", "other-space", nil)
	for _, a := range []linkTestArticle{
		{documentID: "live", workspace: linkTestWorkspaceID, publicID: "aaaa1111", slug: "guide", live: true},
		{documentID: "draft", workspace: linkTestWorkspaceID, publicID: "bbbb2222", slug: "draft"},
		{documentID: "archived", workspace: linkTestWorkspaceID, publicID: "cccc3333", slug: "archived", live: true, status: model.DocStatusArchived},
	} {
		seedLinkTestArticle(t, db, linkTestSpaceID, a)
	}
	seedLinkTestArticle(t, db, "other-space", linkTestArticle{documentID: "foreign", workspace: "other", publicID: "dddd4444", slug: "foreign", live: true})
	repo := repository.NewDocsHelpcenterRepository(db, false)
	for _, tc := range []struct {
		name    string
		updates map[string]any
		base    string
	}{
		{"hosted", map[string]any{"public_url_mode": "hosted_subdomain"}, "https://sub-" + linkTestWorkspaceID + ".helpin.center"},
		{"custom", map[string]any{"public_url_mode": "custom_domain", "custom_domain": "help.example.com"}, "https://help.example.com"},
		{"proxy", map[string]any{"public_url_mode": "reverse_proxy", "reverse_proxy_host": "example.com", "reverse_proxy_base_path": "/help"}, "https://example.com/help"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Model(&model.DocsHelpcenterConfig{}).Where("workspace_id = ?", linkTestWorkspaceID).Updates(tc.updates).Error; err != nil {
				t.Fatal(err)
			}
			messages := []model.SupportMessage{{Metadata: `{"extra":{"keep":true},"ai_sources":[{"docId":"docs:live","sourceType":"docs","title":"Guide","extra":"keep"},{"docId":"docs:draft","url":"https://stale.example/draft"},{"docId":"docs:archived"},{"docId":"docs:foreign"},{"docId":"content:web","url":"https://example.com/external"}]}`}}
			hydrateSupportSourceLinks(context.Background(), repo, linkTestWorkspaceID, messages)
			var meta struct {
				Extra   map[string]bool  `json:"extra"`
				Sources []map[string]any `json:"ai_sources"`
			}
			if err := json.Unmarshal([]byte(messages[0].Metadata), &meta); err != nil {
				t.Fatal(err)
			}
			if got := meta.Sources[0]["url"]; got != tc.base+"/en/articles/guide-aaaa1111" {
				t.Fatalf("public URL = %v", got)
			}
			for _, i := range []int{1, 2, 3} {
				if url, _ := meta.Sources[i]["url"].(string); url != "" {
					t.Fatalf("non-public source linked: %v", meta.Sources[i])
				}
			}
			if !meta.Extra["keep"] || meta.Sources[0]["extra"] != "keep" || meta.Sources[4]["url"] != "https://example.com/external" {
				t.Fatalf("unrelated metadata changed: %s", messages[0].Metadata)
			}
		})
	}
	if err := db.Model(&model.DocsSpace{}).Where("id = ?", linkTestSpaceID).Update("type", model.SpaceTypeInternal).Error; err != nil {
		t.Fatal(err)
	}
	messages := []model.SupportMessage{{Metadata: `{"ai_sources":[{"docId":"docs:live","url":"https://stale.example/guide"}]}`}}
	hydrateSupportSourceLinks(context.Background(), repo, linkTestWorkspaceID, messages)
	if strings.Contains(messages[0].Metadata, "https://") {
		t.Fatalf("private article still linked: %s", messages[0].Metadata)
	}
}

func TestSupportSourceLinksRefreshAndUnavailable(t *testing.T) {
	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	seedLinkTestWorkspace(t, db, linkTestWorkspaceID, linkTestSpaceID, nil)
	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "live", workspace: linkTestWorkspaceID, publicID: "aaaa1111", slug: "guide", live: true})
	repo := repository.NewDocsHelpcenterRepository(db)
	messages := []model.SupportMessage{{Metadata: `{"ai_sources":[{"docId":"docs:live","url":"https://old.example/old"}]}`}}
	messages[0].WorkspaceID = linkTestWorkspaceID
	service := &SupportAIService{db: db}
	service.hydrateSourceLinks(context.Background(), &messages[0])
	if !strings.Contains(messages[0].Metadata, "/articles/guide-aaaa1111") {
		t.Fatalf("new reply not linked: %s", messages[0].Metadata)
	}
	if err := db.Model(&model.DocsHelpcenterArticlePublication{}).Where("document_id = ?", "live").Update("slug", "renamed").Error; err != nil {
		t.Fatal(err)
	}
	inbox := &SupportInboxService{docsHelpcenterRepo: repo}
	inbox.hydrateSourceLinks(context.Background(), linkTestWorkspaceID, messages)
	if !strings.Contains(messages[0].Metadata, "/articles/renamed-aaaa1111") {
		t.Fatalf("old reply not refreshed: %s", messages[0].Metadata)
	}
	if err := db.Model(&model.DocsHelpcenterConfig{}).Where("workspace_id = ?", linkTestWorkspaceID).Update("is_published", false).Error; err != nil {
		t.Fatal(err)
	}
	inbox.hydrateSourceLinks(context.Background(), linkTestWorkspaceID, messages)
	if strings.Contains(messages[0].Metadata, "https://") {
		t.Fatalf("unpublished help center linked: %s", messages[0].Metadata)
	}
	// Lookup failure degrades to text, preserving the conversation and other metadata.
	if err := db.Exec("DROP TABLE docs_helpcenter_configs").Error; err != nil {
		t.Fatal(err)
	}
	messages[0].Metadata = `{"keep":true,"ai_sources":[{"docId":"docs:live","url":"https://old.example/old"}]}`
	inbox.hydrateSourceLinks(context.Background(), linkTestWorkspaceID, messages)
	if strings.Contains(messages[0].Metadata, "https://") || !strings.Contains(messages[0].Metadata, `"keep":true`) {
		t.Fatalf("lookup failure: %s", messages[0].Metadata)
	}
}

func TestSupportSourceLinksSkipsUnrelatedMetadata(t *testing.T) {
	repo := repository.NewDocsHelpcenterRepository(nil) // any query here would panic
	for _, raw := range []string{`{broken`, `{}`, `{"ai_sources":[{"docId":"content:page","url":"https://example.com"}]}`, `{"ai_sources":[{"docId":"docs:private","sourceType":"external_mcp"}]}`} {
		messages := []model.SupportMessage{{Metadata: raw}}
		hydrateSupportSourceLinks(context.Background(), repo, "workspace", messages)
		if messages[0].Metadata != raw {
			t.Fatalf("unrelated metadata changed: %s", messages[0].Metadata)
		}
	}
}

func TestSupportCitationDocumentIDLegacyAndSourceTypes(t *testing.T) {
	const id = "b9d86e50-b73b-4c8d-a26c-75f3427a1e19"
	for _, tc := range []struct{ raw, want string }{
		{`{"docId":"` + id + `"}`, id},
		{`{"docId":"docs:` + id + `","sourceType":"docs"}`, id},
		{`{"docId":"` + id + `","sourceType":"content"}`, ""},
		{`{"docId":"docs:` + id + `","sourceType":"external_mcp"}`, ""},
	} {
		var source map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tc.raw), &source); err != nil {
			t.Fatal(err)
		}
		if got := supportCitationDocumentID(source); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.raw, got, tc.want)
		}
	}
}
