package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
)

const testIntercomCSV = "https://downloads.intercomcdn.com/i/o/cgcz66wp/1/abc/RotaCloud+Leave+Import+Template.csv?expires=1&signature=x"

func stubImportHTTP(t *testing.T, contentType, body string) {
	t.Helper()
	original := helpscout.ImageHTTPClient
	helpscout.ImageHTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{contentType}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	t.Cleanup(func() { helpscout.ImageHTTPClient = original })
}

func TestIsSourceAttachmentURL(t *testing.T) {
	tests := []struct {
		href string
		want bool
	}{
		{testIntercomCSV, true},
		{"https://downloads.intercomcdn.com/i/o/1/guide.PDF", true},
		{"https://downloads.intercomcdn.com/i/o/1/page.html", false},
		{"https://downloads.intercomcdn.com/i/o/1/logo.svg", false},
		{"https://example.com/template.csv", false},
		{"http://downloads.intercomcdn.com/i/o/1/template.csv", false},
	}
	for _, tt := range tests {
		if got := isSourceAttachmentURL(tt.href); got != tt.want {
			t.Errorf("isSourceAttachmentURL(%q) = %v, want %v", tt.href, got, tt.want)
		}
	}
}

func TestImportExternalFile_StoresAllowedTypeByExtension(t *testing.T) {
	// The CDN labels the CSV as a forced download; the stored type comes from the extension.
	stubImportHTTP(t, "application/force-download", "name,date\n")
	uploader := &fakeImageUploader{}

	got, err := importExternalFile(context.Background(), "ws-1", testIntercomCSV, uploader)
	if err != nil {
		t.Fatalf("importExternalFile: %v", err)
	}
	if uploader.filename != "RotaCloud-Leave-Import-Template.csv" || uploader.contentType != "text/csv" || uploader.body != "name,date\n" {
		t.Fatalf("upload = %q %q %q", uploader.filename, uploader.contentType, uploader.body)
	}
	if got != "https://cdn.helpin.ai/docs-import/ws-1/RotaCloud-Leave-Import-Template.csv" {
		t.Fatalf("stored url = %q", got)
	}
}

func TestImportExternalFile_RejectsHTMLErrorPage(t *testing.T) {
	stubImportHTTP(t, "text/html; charset=utf-8", "<html>expired</html>")
	uploader := &fakeImageUploader{}

	if _, err := importExternalFile(context.Background(), "ws-1", testIntercomCSV, uploader); err == nil {
		t.Fatal("want an error for an HTML page")
	}
	if uploader.filename != "" {
		t.Fatalf("uploaded %q, want nothing", uploader.filename)
	}
}

func TestImportExternalFile_RejectsOtherTypes(t *testing.T) {
	_, err := importExternalFile(context.Background(), "ws-1", "https://downloads.intercomcdn.com/i/o/1/x.html", &fakeImageUploader{})
	if err != errAttachmentType {
		t.Fatalf("err = %v, want errAttachmentType", err)
	}
}

func TestCopyDocumentAttachments_RewritesOnlySourceFileLinks(t *testing.T) {
	doc := docsimport.Node{Type: "doc", Content: []docsimport.Node{{
		Type: "paragraph",
		Content: []docsimport.Node{
			{Type: "text", Text: "template", Marks: []docsimport.Mark{docsimport.LinkMark(testIntercomCSV)}},
			{Type: "text", Text: "docs", Marks: []docsimport.Mark{docsimport.LinkMark("https://example.com/a.csv")}},
			{Type: "text", Text: "failed", Marks: []docsimport.Mark{docsimport.LinkMark("https://downloads.intercomcdn.com/i/o/2/b.pdf")}},
		},
	}}}
	copyDocumentAttachments(&doc, func(source string) string {
		if source == testIntercomCSV {
			return "https://cdn.helpin.ai/a.csv"
		}
		return ""
	})
	texts := doc.Content[0].Content
	want := []string{"https://cdn.helpin.ai/a.csv", "https://example.com/a.csv", "https://downloads.intercomcdn.com/i/o/2/b.pdf"}
	for i, w := range want {
		if got := texts[i].Marks[0].Attrs["href"]; got != w {
			t.Errorf("link %d = %v, want %s", i, got, w)
		}
	}
}
