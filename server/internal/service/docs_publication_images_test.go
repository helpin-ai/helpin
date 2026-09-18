package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type publicationImageStoreFixture struct {
	*fakePublicationArtifactStore
	locator *storage.S3Client
}

func (s publicationImageStoreFixture) DocsImageKeyFromURL(raw, ws string) (string, bool) {
	return s.locator.DocsImageKeyFromURL(raw, ws)
}

type publicationAttachmentFixture struct{ attachment *model.PMAttachment }

func (r publicationAttachmentFixture) GetByID(context.Context, string) (*model.PMAttachment, error) {
	return r.attachment, nil
}

func TestPublishPrivateImagesCopiesOnlyAuthorizedSources(t *testing.T) {
	locator := storage.NewS3Client("test", "test", "helpin", "garage", "http://garage:3900", "")
	locator.ConfigureAssetAccess("https://app.test", true)
	for _, tc := range []struct {
		name, src, workspace, document string
		attachment                     *model.PMAttachment
		wantError                      bool
	}{
		{name: "import", src: locator.PublicURL("docs-import/ws/image.png"), workspace: "ws", document: "doc"},
		{name: "cross workspace import", src: locator.PublicURL("docs-import/other/image.png"), workspace: "ws", document: "doc", wantError: true},
		{name: "editor attachment", src: "/api/pm/attachments/att/content", workspace: "ws", document: "doc", attachment: &model.PMAttachment{ID: "att", WorkspaceID: "ws", EntityID: "doc", EntityType: "editor_upload", IsUploaded: true, StorageKey: "ws/att.png", ContentType: "image/png"}},
		{name: "other document", src: "/api/pm/attachments/att/content", workspace: "ws", document: "doc", attachment: &model.PMAttachment{ID: "att", WorkspaceID: "ws", EntityID: "private-doc", EntityType: "editor_upload", IsUploaded: true, StorageKey: "ws/secret.png"}, wantError: true},
		{name: "missing attachment", src: "/api/pm/attachments/missing/content", workspace: "ws", document: "doc", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := publicationImageStoreFixture{&fakePublicationArtifactStore{objects: map[string][]byte{"docs-import/ws/image.png": []byte("png"), "ws/att.png": []byte("png")}, puts: map[string][]byte{}, publicRead: map[string]bool{}}, locator}
			raw, _ := json.Marshal(tiptap.Node{Type: "doc", Content: []tiptap.Node{{Type: "resizableImage", Attrs: map[string]any{"src": tc.src}}}})
			result, err := materializePublicationImages(context.Background(), store, publicationAttachmentFixture{tc.attachment}, tc.workspace, tc.document, raw)
			if tc.wantError {
				if err == nil || len(store.puts) != 0 {
					t.Fatal("unauthorized image was published")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var root tiptap.Node
			_ = json.Unmarshal(result, &root)
			if !strings.HasPrefix(root.Content[0].Attrs["src"].(string), "https://assets.example.com/helpcenter/ws/articles/doc/images/") {
				t.Fatal("publication still uses private source")
			}
			if root.Content[0].Attrs["publishedFrom"] == nil || len(store.puts) != 1 {
				t.Fatal("missing provenance or public copy")
			}
		})
	}
}

func TestPublishImportedHTMLImage(t *testing.T) {
	locator := storage.NewS3Client("test", "test", "helpin", "garage", "http://garage:3900", "")
	locator.ConfigureAssetAccess("https://app.test", true)
	store := publicationImageStoreFixture{&fakePublicationArtifactStore{objects: map[string][]byte{"docs-import/ws/image.png": []byte("png")}, puts: map[string][]byte{}, publicRead: map[string]bool{}}, locator}
	raw, _ := json.Marshal(tiptap.Node{Type: "doc", Content: []tiptap.Node{{Type: "htmlBlock", Attrs: map[string]any{"html": "<div><img alt=\"Diagram\" src=\"" + locator.PublicURL("docs-import/ws/image.png") + "\"></div>"}}}})
	result, err := materializePublicationImages(context.Background(), store, nil, "ws", "doc", raw)
	if err != nil {
		t.Fatal(err)
	}
	var root tiptap.Node
	_ = json.Unmarshal(result, &root)
	rendered := root.Content[0].Attrs["html"].(string)
	if strings.Contains(rendered, "/api/docs/images/content") || !strings.Contains(rendered, "https://assets.example.com/helpcenter/ws/articles/doc/images/") {
		t.Fatal("HTML image still uses private source")
	}
	if root.Content[0].Attrs["publishedFrom"] == nil {
		t.Fatal("missing source provenance")
	}
}
