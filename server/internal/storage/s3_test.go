package storage

import (
	"net/url"
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
