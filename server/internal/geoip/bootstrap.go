package geoip

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

func (s *Service) initialize(ctx context.Context, opts Options, retryInterval time.Duration) {
	defer close(s.done)
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "maxmind initialization panicked; GeoIP unavailable", "panic", recovered)
		}
	}()

	for ctx.Err() == nil {
		reader, err := loadDatabase(ctx, opts)
		if err == nil {
			s.mu.Lock()
			s.reader = reader
			s.mu.Unlock()
			slog.InfoContext(ctx, "maxmind db loaded; GeoIP enrichment available", "path", opts.Path)
			return
		}
		if ctx.Err() != nil {
			return
		}
		delay := retryDelay(err, retryInterval, time.Now())
		slog.WarnContext(ctx, "maxmind initialization failed; retrying later", "error", err, "retry_in", delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func loadDatabase(ctx context.Context, opts Options) (*maxminddb.Reader, error) {
	// Retry opening first so an operator can install a database while we wait.
	reader, err := maxminddb.Open(opts.Path)
	if err == nil {
		return reader, nil
	}
	if strings.TrimSpace(opts.DownloadURL) == "" {
		return nil, fmt.Errorf("open maxmind db (MAXMIND_DOWNLOAD_URL is empty): %w", err)
	}
	// Replace unreadable files too, so a corrupt download cannot prevent recovery.
	if err := downloadDatabase(ctx, opts.Path, opts); err != nil {
		return nil, err
	}
	reader, err = maxminddb.Open(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("open downloaded maxmind db: %w", err)
	}
	return reader, nil
}

type downloadError struct {
	status     int
	body       string
	retryAfter string
}

func (e *downloadError) Error() string {
	return fmt.Sprintf("download maxmind archive: status %d: %s", e.status, e.body)
}

func retryDelay(err error, fallback time.Duration, now time.Time) time.Duration {
	var downloadErr *downloadError
	if !errors.As(err, &downloadErr) {
		return fallback
	}
	if downloadErr.status == http.StatusTooManyRequests {
		fallback = max(fallback, 12*time.Hour)
	}
	value := strings.TrimSpace(downloadErr.retryAfter)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		// Bound conversion to prevent duration overflow from an invalid header.
		const maxSeconds = int64((1<<63 - 1) / time.Second)
		return max(fallback, time.Duration(min(seconds, maxSeconds))*time.Second)
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(fallback, date.Sub(now))
	}
	return fallback
}
