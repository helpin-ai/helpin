package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	dockChatExternalImageMaxBytes    = 10 << 20
	dockChatExternalImageConcurrency = 4
)

func isDockChatExternalMediaURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || strings.TrimSpace(u.Hostname()) == "" {
		return false
	}
	return isAllowedSupportPreviewURL(u)
}

func newDockChatExternalMediaClient() *http.Client {
	transport := &http.Transport{
		DialContext:           dockChatSafeDialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   dockChatExternalImageConcurrency,
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req == nil || req.URL == nil || !isDockChatExternalMediaURL(req.URL.String()) {
				return errors.New("unsafe redirect")
			}
			return nil
		},
	}
}

func dockChatSafeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("resolve media host: %w", err)
	}
	for _, ip := range ips {
		if isDisallowedDockChatMediaIP(ip.IP) {
			return nil, errors.New("media host address is not allowed")
		}
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func isDisallowedDockChatMediaIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast()
}

// fetchDockChatExternalImage reads a public image transiently. It never stores
// external content; the returned data URL exists only for the provider call.
func fetchDockChatExternalImage(ctx context.Context, client *http.Client, rawURL string) (string, string, error) {
	if !isDockChatExternalMediaURL(rawURL) {
		return "", "", errors.New("external media URL is not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/gif,image/webp")
	req.Header.Set("User-Agent", "Helpin-Ask-Media/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch external media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("external media returned an error")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !isDockChatMediaType(contentType) || !strings.HasPrefix(contentType, "image/") {
		return "", "", errors.New("external URL is not an image")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, dockChatExternalImageMaxBytes+1))
	if err != nil {
		return "", "", err
	}
	if len(body) > dockChatExternalImageMaxBytes {
		return "", "", errors.New("external image is too large")
	}
	if err := validateDockChatMediaSignature(contentType, body); err != nil {
		return "", "", err
	}
	return contentType, "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func (s *DockChatService) hydrateDockChatExternalImages(ctx context.Context, attachments []dockChatMediaAttachment) ([]dockChatMediaAttachment, error) {
	client := s.externalMediaClient
	if client == nil {
		client = newDockChatExternalMediaClient()
		defer client.CloseIdleConnections()
	}
	group, downloadCtx := errgroup.WithContext(ctx)
	group.SetLimit(dockChatExternalImageConcurrency)
	for i, attachment := range attachments {
		if downloadCtx.Err() != nil {
			break
		}
		if attachment.Source != "hosted_link" {
			continue
		}
		group.Go(func() (err error) {
			defer func() {
				if recover() != nil {
					err = errors.New("unexpected hosted image download failure")
				}
			}()
			if err := downloadCtx.Err(); err != nil {
				return err
			}
			contentType, dataURL, err := fetchDockChatExternalImage(downloadCtx, client, attachment.URL)
			if err != nil {
				// Optional remote images remain best-effort, but caller
				// cancellation stops the whole batch and queued downloads.
				return downloadCtx.Err()
			}
			attachments[i].FileType = contentType
			attachments[i].URL = dataURL
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Downloads finish in any order. Compact only after every worker has
	// stopped so the original attachment order and metadata are preserved.
	result := attachments[:0]
	for _, attachment := range attachments {
		if attachment.Source == "hosted_link" && attachment.FileType == "" {
			continue
		}
		result = append(result, attachment)
	}
	return result, nil
}
