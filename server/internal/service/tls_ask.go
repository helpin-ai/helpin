package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrInvalidAskDomain marks ask input that is rejected before any lookup.
var ErrInvalidAskDomain = errors.New("invalid ask domain")

const (
	_tlsAskCacheTTL   = 60 * time.Second
	_tlsAskCacheLimit = 20000
	_maxDomainLength  = 253
	_maxLabelLength   = 63
)

// Hostnames that terminate TLS on the caddy ingress class without a registered
// custom domain row. Suffix entries must start with a dot.
var (
	_tlsAskFirstPartyExact = map[string]struct{}{
		"helpcenter.helpin.ai":       {},
		"helpcenter-stage.helpin.ai": {},
	}
	_tlsAskFirstPartySuffixes = []string{".helpin.center"}
)

type tlsAskCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

// TLSAskService answers Caddy on-demand TLS "ask" checks: it decides whether a
// certificate may be issued for a hostname. Denials must stay cheap because the
// endpoint is exposed to internet scanner traffic.
type TLSAskService struct {
	hcRepo        *repository.DocsHelpcenterRepository
	extraExact    map[string]struct{}
	extraSuffixes []string
	now           func() time.Time
	mu            sync.Mutex
	cache         map[string]tlsAskCacheEntry
}

// NewTLSAskService builds the service. extraAllowed entries are exact hostnames,
// or suffix rules when prefixed with "." or "*." (per-cluster escape hatch for
// domains that share the caddy ingress class but live outside our database).
func NewTLSAskService(hcRepo *repository.DocsHelpcenterRepository, extraAllowed []string) *TLSAskService {
	s := &TLSAskService{
		hcRepo:     hcRepo,
		extraExact: map[string]struct{}{},
		now:        time.Now,
		cache:      map[string]tlsAskCacheEntry{},
	}
	for _, entry := range extraAllowed {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if trimmed, ok := strings.CutPrefix(entry, "*."); ok {
			s.extraSuffixes = append(s.extraSuffixes, "."+trimmed)
		} else if strings.HasPrefix(entry, ".") {
			s.extraSuffixes = append(s.extraSuffixes, entry)
		} else {
			s.extraExact[entry] = struct{}{}
		}
	}
	return s
}

// Allowed reports whether a certificate may be issued for the normalized domain.
// Both allow and deny results are cached for a short TTL.
func (s *TLSAskService) Allowed(ctx context.Context, domain string) (bool, error) {
	if s.isFirstParty(domain) {
		return true, nil
	}
	if allowed, ok := s.cached(domain); ok {
		return allowed, nil
	}
	registered, err := s.hcRepo.CustomDomainRegistered(ctx, domain)
	if err != nil {
		return false, err
	}
	s.store(domain, registered)
	return registered, nil
}

// NormalizeTLSAskDomain lowercases and strips a single trailing dot, then
// rejects anything that cannot be a routable customer hostname: empty values,
// IP addresses, ports, wildcards, bare labels, and invalid characters. It
// returns ErrInvalidAskDomain so callers can reject without touching storage.
func NormalizeTLSAskDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || len(domain) > _maxDomainLength {
		return "", ErrInvalidAskDomain
	}
	if !strings.Contains(domain, ".") {
		return "", ErrInvalidAskDomain
	}
	if net.ParseIP(domain) != nil {
		return "", ErrInvalidAskDomain
	}
	for _, label := range strings.Split(domain, ".") {
		if !validAskLabel(label) {
			return "", ErrInvalidAskDomain
		}
	}
	return domain, nil
}

func validAskLabel(label string) bool {
	if label == "" || len(label) > _maxLabelLength {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func (s *TLSAskService) isFirstParty(domain string) bool {
	if _, ok := _tlsAskFirstPartyExact[domain]; ok {
		return true
	}
	if _, ok := s.extraExact[domain]; ok {
		return true
	}
	for _, suffix := range _tlsAskFirstPartySuffixes {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	for _, suffix := range s.extraSuffixes {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

func (s *TLSAskService) cached(domain string) (allowed, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[domain]
	if !ok || s.now().After(entry.expiresAt) {
		return false, false
	}
	return entry.allowed, true
}

func (s *TLSAskService) store(domain string, allowed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Cap memory under scanner abuse: wiping is cheap and only costs one extra
	// indexed query per domain after a reset.
	if len(s.cache) >= _tlsAskCacheLimit {
		s.cache = map[string]tlsAskCacheEntry{}
	}
	s.cache[domain] = tlsAskCacheEntry{allowed: allowed, expiresAt: s.now().Add(_tlsAskCacheTTL)}
}
