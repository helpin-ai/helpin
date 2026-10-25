package storage

import (
	"net/url"
	"strings"
	"testing"
)

func TestSeparateBrowserPresignOrigin(t *testing.T) {
	client := NewS3Client("test-access", "test-secret", "test-bucket", "us-east-1", "http://minio:9000", "https://files.example.test/test-bucket", "https://files.example.test")
	for _, method := range []string{"put", "get"} {
		var raw string
		var err error
		if method == "put" {
			raw, err = client.GeneratePresignedPutURL("attachment/test.txt", "text/plain", 4, false)
		} else {
			raw, err = client.GeneratePresignedGetURL("attachment/test.txt", "test.txt")
		}
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if u.Scheme != "https" || u.Host != "files.example.test" || u.Path != "/test-bucket/attachment/test.txt" || u.Query().Get("X-Amz-Signature") == "" {
			t.Fatal("presign did not use public origin and signature")
		}
	}
	if client.endpointURL != "http://minio:9000" {
		t.Fatal("internal endpoint changed")
	}
}

func TestInlineGetURLStaysTheSameWithinTheHour(t *testing.T) {
	client := NewS3Client("test-access", "test-secret", "test-bucket", "us-east-1", "http://minio:9000", "")
	first, err := client.GeneratePresignedInlineGetURL("support/image.png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.GeneratePresignedInlineGetURL("support/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("inline URL changed between calls:\n%s\n%s", first, second)
	}
	u, err := url.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if date := q.Get("X-Amz-Date"); !strings.HasSuffix(date, "0000Z") {
		t.Fatalf("X-Amz-Date = %q, want the start of an hour", date)
	}
	if got := q.Get("X-Amz-Expires"); got != "7200" {
		t.Fatalf("X-Amz-Expires = %q, want 7200 so the URL lasts at least an hour", got)
	}
	if got := q.Get("response-cache-control"); got != "private, max-age=3600" {
		t.Fatalf("response-cache-control = %q", got)
	}
	if got := q.Get("response-content-disposition"); got != "inline" {
		t.Fatalf("response-content-disposition = %q", got)
	}
}

func TestDownloadGetURLStillSignsAtRequestTime(t *testing.T) {
	client := NewS3Client("test-access", "test-secret", "test-bucket", "us-east-1", "http://minio:9000", "")
	raw, err := client.GeneratePresignedGetURL("support/report.pdf", "report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Query().Get("X-Amz-Expires"); got != "3600" {
		t.Fatalf("X-Amz-Expires = %q, want 3600", got)
	}
}
