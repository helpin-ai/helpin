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
)

const dockChatExternalImageMaxBytes = 10 << 20

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
func fetchDockChatExternalImage(ctx context.Context, rawURL string) (string, string, error) {
	if !isDockChatExternalMediaURL(rawURL) {
		return "", "", errors.New("external media URL is not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/gif,image/webp")
	req.Header.Set("User-Agent", "Helpin-Ask-Media/1.0")
	resp, err := newDockChatExternalMediaClient().Do(req)
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
