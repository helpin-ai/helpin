package geoip

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

type Result struct {
	CountryCode string
	CountryName string
	RegionName  string
	CityName    string
	Timezone    string
}

type Resolver interface {
	Lookup(addr netip.Addr) (*Result, error)
}

type Service struct {
	reader *maxminddb.Reader
}

type Options struct {
	Path        string
	DownloadURL string
	AccountID   string
	LicenseKey  string
	HTTPClient  *http.Client
}

func Open(opts Options) (*Service, error) {
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		return nil, nil
	}

	if err := ensureDatabase(path, opts); err != nil {
		return nil, err
	}

	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open maxmind db %q: %w", path, err)
	}

	return &Service{reader: reader}, nil
}

func (s *Service) Lookup(addr netip.Addr) (*Result, error) {
	if s == nil || s.reader == nil || !addr.IsValid() {
		return nil, nil
	}

	addr = addr.Unmap()

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
			Names   struct {
				English string `maxminddb:"en"`
			} `maxminddb:"names"`
		} `maxminddb:"country"`
		Subdivisions []struct {
			Names struct {
				English string `maxminddb:"en"`
			} `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		City struct {
			Names struct {
				English string `maxminddb:"en"`
			} `maxminddb:"names"`
		} `maxminddb:"city"`
		Location struct {
			TimeZone string `maxminddb:"time_zone"`
		} `maxminddb:"location"`
	}

	if err := s.reader.Lookup(addr).Decode(&record); err != nil {
		return nil, fmt.Errorf("lookup %s: %w", addr, err)
	}

	result := &Result{
		CountryCode: strings.TrimSpace(record.Country.ISOCode),
		CountryName: strings.TrimSpace(record.Country.Names.English),
		CityName:    strings.TrimSpace(record.City.Names.English),
		Timezone:    strings.TrimSpace(record.Location.TimeZone),
	}
	if len(record.Subdivisions) > 0 {
		result.RegionName = strings.TrimSpace(record.Subdivisions[0].Names.English)
	}
	if result.CountryCode == "" && result.CountryName == "" && result.RegionName == "" && result.CityName == "" && result.Timezone == "" {
		return nil, nil
	}

	return result, nil
}

func (s *Service) Close() error {
	if s == nil || s.reader == nil {
		return nil
	}
	return s.reader.Close()
}

func ensureDatabase(path string, opts Options) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat maxmind db %q: %w", path, err)
	}

	downloadURL := strings.TrimSpace(opts.DownloadURL)
	if downloadURL == "" {
		return fmt.Errorf("maxmind db %q not found and MAXMIND_DOWNLOAD_URL is empty", path)
	}

	if err := downloadDatabase(path, opts); err != nil {
		return fmt.Errorf("bootstrap maxmind db %q: %w", path, err)
	}

	return nil
}

func downloadDatabase(path string, opts Options) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create maxmind db directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), "maxmind-download-*")
	if err != nil {
		return fmt.Errorf("create temp download file: %w", err)
	}
	tempName := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempName)
	}()

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimSpace(opts.DownloadURL), nil)
	if err != nil {
		return fmt.Errorf("build maxmind download request: %w", err)
	}
	if strings.TrimSpace(opts.AccountID) != "" || strings.TrimSpace(opts.LicenseKey) != "" {
		req.SetBasicAuth(strings.TrimSpace(opts.AccountID), strings.TrimSpace(opts.LicenseKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download maxmind archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("download maxmind archive: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("write temp maxmind archive: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp maxmind archive: %w", err)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	contentDisposition := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Disposition")))
	if err := extractMMDB(tempName, path, contentType, contentDisposition, strings.ToLower(req.URL.Path)); err != nil {
		return err
	}

	slog.Info("maxmind db bootstrapped", "path", path)
	return nil
}

func extractMMDB(downloadPath, targetPath, contentType, contentDisposition, urlPath string) error {
	switch {
	case strings.Contains(contentType, "application/zip"),
		strings.Contains(urlPath, ".zip"),
		strings.Contains(contentDisposition, ".zip"):
		return extractMMDBFromZip(downloadPath, targetPath)
	case strings.Contains(contentType, "gzip"),
		strings.Contains(contentType, "x-gzip"),
		strings.Contains(urlPath, ".tar.gz"),
		strings.Contains(urlPath, ".tgz"),
		strings.Contains(contentDisposition, ".tar.gz"),
		strings.Contains(contentDisposition, ".tgz"):
		return extractMMDBFromTarGz(downloadPath, targetPath)
	default:
		return installMMDBFile(downloadPath, targetPath)
	}
}

func extractMMDBFromTarGz(downloadPath, targetPath string) error {
	file, err := os.Open(downloadPath)
	if err != nil {
		return fmt.Errorf("open maxmind tar.gz: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip maxmind archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read maxmind tar entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if strings.HasSuffix(strings.ToLower(header.Name), ".mmdb") {
			return writeReaderAtomically(targetPath, tarReader)
		}
	}

	return fmt.Errorf("maxmind archive did not contain an .mmdb file")
}

func extractMMDBFromZip(downloadPath, targetPath string) error {
	reader, err := zip.OpenReader(downloadPath)
	if err != nil {
		return fmt.Errorf("open maxmind zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".mmdb") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open maxmind zip entry: %w", err)
		}
		defer rc.Close()
		return writeReaderAtomically(targetPath, rc)
	}

	return fmt.Errorf("maxmind archive did not contain an .mmdb file")
}

func installMMDBFile(downloadPath, targetPath string) error {
	file, err := os.Open(downloadPath)
	if err != nil {
		return fmt.Errorf("open downloaded maxmind db: %w", err)
	}
	defer file.Close()

	header := make([]byte, 16)
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read downloaded maxmind db header: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind downloaded maxmind db: %w", err)
	}
	if bytes.HasPrefix(header[:n], []byte("PK\x03\x04")) {
		return extractMMDBFromZip(downloadPath, targetPath)
	}
	if n >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return extractMMDBFromTarGz(downloadPath, targetPath)
	}

	return writeReaderAtomically(targetPath, file)
}

func writeReaderAtomically(targetPath string, src io.Reader) error {
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "maxmind-mmdb-*")
	if err != nil {
		return fmt.Errorf("create temp mmdb file: %w", err)
	}
	tempName := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempName)
	}()

	if _, err := io.Copy(tempFile, src); err != nil {
		return fmt.Errorf("write mmdb file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close mmdb file: %w", err)
	}
	if err := os.Rename(tempName, targetPath); err != nil {
		return fmt.Errorf("install mmdb file: %w", err)
	}
	return nil
}
