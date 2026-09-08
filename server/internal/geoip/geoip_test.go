package geoip

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadDatabaseDownloadsTarGzArchive(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	expected := []byte("fake-mmdb-data")

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "acct-123" || pass != "license-456" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, user, pass)
		}

		var archive bytes.Buffer
		gzipWriter := gzip.NewWriter(&archive)
		tarWriter := tar.NewWriter(gzipWriter)
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: "GeoLite2-City_20260413/GeoLite2-City.mmdb",
			Mode: 0o644,
			Size: int64(len(expected)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(expected); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
		if err := tarWriter.Close(); err != nil {
			t.Fatalf("close tar writer: %v", err)
		}
		if err := gzipWriter.Close(); err != nil {
			t.Fatalf("close gzip writer: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/gzip"}},
			Body:       io.NopCloser(bytes.NewReader(archive.Bytes())),
		}, nil
	})}

	err := downloadDatabase(context.Background(), targetPath, Options{
		Path:        targetPath,
		DownloadURL: "https://download.maxmind.test/GeoLite2-City.tar.gz",
		AccountID:   "acct-123",
		LicenseKey:  "license-456",
		HTTPClient:  client,
	})
	if err != nil {
		t.Fatalf("ensure database: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected file content: got %q want %q", string(got), string(expected))
	}
}

func TestExtractMMDBFallsBackToArchiveSniffing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	downloadPath := filepath.Join(tempDir, "download.bin")
	targetPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	expected := []byte("zip-mmdb-data")

	var archive bytes.Buffer
	if err := writeZipArchive(&archive, "GeoLite2-City.mmdb", expected); err != nil {
		t.Fatalf("write zip archive: %v", err)
	}
	if err := os.WriteFile(downloadPath, archive.Bytes(), 0o644); err != nil {
		t.Fatalf("write download file: %v", err)
	}

	if err := extractMMDB(downloadPath, targetPath, "application/octet-stream", "", "/download"); err != nil {
		t.Fatalf("extract mmdb: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected file content: got %q want %q", string(got), string(expected))
	}
}

func writeZipArchive(dst *bytes.Buffer, name string, contents []byte) error {
	zw := zip.NewWriter(dst)
	file, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return zw.Close()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
