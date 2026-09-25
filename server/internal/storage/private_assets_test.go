package storage

import (
	"net/url"
	"strings"
	"testing"
)

func TestPrivateBucketAssetBoundaries(t *testing.T) {
	client := NewS3Client("fixture", "fixture", "helpin", "garage", "http://garage:3900", "", "http://files.test")
	client.ConfigureAssetAccess("https://app.test/", true)
	if client.HasPublicURL() {
		t.Fatal("private attachments must use signed downloads")
	}
	for _, key := range []string{"docs-import/ws/secret.png", "ws/attachment.png", "helpcenter/ws/../private.png", "helpcenter//images/x", "users/ws/private/x", "helpcenter/ws/images/%2e%2e"} {
		if PublicAssetKey(key) {
			t.Fatalf("publicly exposed %q", key)
		}
	}
	for _, key := range []string{"helpcenter/ws/articles/doc/image.png", "users/user/avatar/a.png", "workspaces/ws/logo/a.png"} {
		if !PublicAssetKey(key) || !strings.HasPrefix(client.PublicURL(key), "https://app.test/api/public/assets/") {
			t.Fatalf("public asset unavailable %q", key)
		}
	}
	privateURL := client.PublicURL("docs-import/ws/import/image.png")
	if key, ok := client.DocsImageKeyFromURL(privateURL, "ws"); !ok || key != "docs-import/ws/import/image.png" {
		t.Fatal("private image URL failed round trip")
	}
	if _, ok := client.DocsImageKeyFromURL(privateURL, "other"); ok {
		t.Fatal("cross-workspace image accepted")
	}
	for _, key := range []string{"docs-import/ws/../other/image.png", "docs-import/ws/%2e%2e/image.png"} {
		if DocsImageKey(key, "ws") {
			t.Fatal("path traversal accepted")
		}
	}
	signed, err := client.GeneratePresignedPutURL("helpcenter/ws/images/a.png", "image/png", 3, true)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(signed)
	if parsed.Query().Get("X-Amz-Signature") == "" || strings.Contains(strings.ToLower(signed), "x-amz-acl") {
		t.Fatal("Garage upload must be signed without unsupported ACLs")
	}
}

func TestLegacyCDNURLsRemainUnchanged(t *testing.T) {
	client := NewS3Client("fixture", "fixture", "bucket", "us-east-1", "https://s3.test", "https://cdn.test")
	client.ConfigureAssetAccess("https://dashboard.test", false)
	if !client.HasPublicURL() || client.PublicURL("docs-import/ws/image.png") != "https://cdn.test/docs-import/ws/image.png" {
		t.Fatal("legacy CDN addressing changed")
	}
}
