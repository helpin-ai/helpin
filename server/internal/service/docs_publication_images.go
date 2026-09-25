package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type publicationAttachmentRepository interface {
	GetByID(context.Context, string) (*model.PMAttachment, error)
}
type publicationImageLocator interface {
	DocsImageKeyFromURL(string, string) (string, bool)
}

// Publishing creates a public copy; the imported source and editor attachment
// stay private. Never fetch an arbitrary URL or copy across workspace/document
// boundaries. Source attributes are retained only in non-rendered provenance.
func materializePublicationImages(ctx context.Context, store publicationArtifactStore, attachments publicationAttachmentRepository, workspaceID, documentID string, raw json.RawMessage) (json.RawMessage, error) {
	if !bytes.Contains(raw, []byte("/api/docs/images/content")) && !bytes.Contains(raw, []byte("/api/pm/attachments/")) {
		return raw, nil
	}
	if store == nil {
		return nil, fmt.Errorf("image publication storage is not configured")
	}
	var root tiptap.Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	var visit func(*tiptap.Node) error
	visit = func(node *tiptap.Node) error {
		original, _ := json.Marshal(node)
		changed := false
		for _, attr := range []string{"src", "darkSrc", "href", "poster"} {
			value, ok := node.Attrs[attr].(string)
			if !ok {
				continue
			}
			parsed, err := url.Parse(value)
			if err != nil {
				continue
			}
			key := ""
			contentType := ""
			switch {
			case parsed.Path == "/api/docs/images/content":
				locator, ok := store.(publicationImageLocator)
				if !ok {
					return fmt.Errorf("imported image publication is not configured")
				}
				key, ok = locator.DocsImageKeyFromURL(value, workspaceID)
				if !ok {
					return fmt.Errorf("imported image does not belong to this workspace or deployment")
				}
				contentType = mime.TypeByExtension(path.Ext(key))
			case strings.HasPrefix(parsed.Path, "/api/pm/attachments/"):
				parts := strings.Split(strings.TrimPrefix(parsed.Path, "/api/pm/attachments/"), "/")
				if len(parts) != 2 || parts[1] != "content" || attachments == nil {
					return fmt.Errorf("invalid publication attachment")
				}
				attachment, err := attachments.GetByID(ctx, parts[0])
				if err != nil {
					return err
				}
				if attachment == nil || !attachment.IsUploaded || attachment.WorkspaceID != workspaceID || attachment.EntityID != documentID || attachment.EntityType != "editor_upload" {
					return fmt.Errorf("publication attachment does not belong to this document")
				}
				key, contentType = attachment.StorageKey, attachment.ContentType
			default:
				continue
			}
			data, err := store.GetObject(ctx, key)
			if err != nil {
				return fmt.Errorf("read publication image: %w", err)
			}
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			digest := sha256.Sum256([]byte(key))
			publicKey := "helpcenter/" + workspaceID + "/articles/" + documentID + "/images/" + hex.EncodeToString(digest[:]) + path.Ext(key)
			if err := store.PutObject(ctx, publicKey, contentType, int64(len(data)), bytes.NewReader(data), true); err != nil {
				return err
			}
			publicURL := store.PublicURL(publicKey)
			if publicURL == "" {
				return fmt.Errorf("public image URL is not configured")
			}
			node.Attrs[attr] = publicURL
			changed = true
		}
		if markup, ok := node.Attrs["html"].(string); ok && (strings.Contains(markup, "/api/docs/images/content") || strings.Contains(markup, "/api/pm/attachments/")) {
			fragments, err := html.ParseFragment(strings.NewReader(markup), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
			if err != nil {
				return fmt.Errorf("parse publication image HTML: %w", err)
			}
			var rewrite func(*html.Node) error
			rewrite = func(element *html.Node) error {
				for i, attr := range element.Attr {
					if attr.Key != "src" && attr.Key != "href" && attr.Key != "poster" {
						continue
					}
					media := &tiptap.Node{Type: "image", Attrs: map[string]any{"src": attr.Val}}
					if err := visit(media); err != nil {
						return err
					}
					element.Attr[i].Val = media.Attrs["src"].(string)
				}
				for child := element.FirstChild; child != nil; child = child.NextSibling {
					if err := rewrite(child); err != nil {
						return err
					}
				}
				return nil
			}
			var output strings.Builder
			for _, fragment := range fragments {
				if err := rewrite(fragment); err != nil {
					return err
				}
				if err := html.Render(&output, fragment); err != nil {
					return err
				}
			}
			node.Attrs["html"] = output.String()
			changed = true
		}
		if changed {
			if _, exists := node.Attrs["publishedFrom"]; !exists {
				var source tiptap.Node
				_ = json.Unmarshal(original, &source)
				node.Attrs["publishedFrom"] = source
			}
		}
		for i := range node.Content {
			if err := visit(&node.Content[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(&root); err != nil {
		return nil, err
	}
	return json.Marshal(root)
}
