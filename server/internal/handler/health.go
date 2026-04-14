package handler

import (
	"log/slog"
	"net/http"
	"net/netip"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/geoip"
	"github.com/helpin-ai/helpin/server/internal/requestmeta"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

// HealthHandler handles health check and system setup requests.
type HealthHandler struct {
	s3Client      *storage.S3Client
	geoIPResolver geoip.Resolver
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(s3Client *storage.S3Client, geoIPResolver geoip.Resolver) *HealthHandler {
	return &HealthHandler{s3Client: s3Client, geoIPResolver: geoIPResolver}
}

// Check returns a 200 OK health check response.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type headerViewEntry struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type headerViewResponse struct {
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Query           string            `json:"query,omitempty"`
	Host            string            `json:"host"`
	RemoteAddr      string            `json:"remote_addr"`
	ClientIP        string            `json:"client_ip,omitempty"`
	ClientIPSource  string            `json:"client_ip_source,omitempty"`
	ForwardedIP     string            `json:"forwarded_ip,omitempty"`
	GeoIP           *geoLookupView    `json:"geoip,omitempty"`
	SelectedHeaders map[string]string `json:"selected_headers"`
	Headers         []headerViewEntry `json:"headers"`
}

type geoLookupView struct {
	CountryCode string `json:"country_code,omitempty"`
	CountryName string `json:"country_name,omitempty"`
	RegionName  string `json:"region_name,omitempty"`
	CityName    string `json:"city_name,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

var selectedDebugHeaders = []string{
	"Accept-Language",
	"CF-Connecting-IP",
	"CF-IPCountry",
	"CloudFront-Viewer-City",
	"CloudFront-Viewer-Country",
	"CloudFront-Viewer-Country-Region",
	"CloudFront-Viewer-Time-Zone",
	"Forwarded",
	"True-Client-IP",
	"User-Agent",
	"X-Original-Forwarded-For",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Forwarded-Port",
	"X-Real-Ip",
	"X-Vercel-IP-City",
	"X-Vercel-IP-Country",
	"X-Vercel-IP-Country-Region",
	"X-Vercel-IP-Timezone",
}

// ViewHeaders returns a sanitized view of the inbound request headers so we can
// inspect which IP/country/location signals are available at the API edge.
func (h *HealthHandler) ViewHeaders(w http.ResponseWriter, r *http.Request) {
	headers := make([]headerViewEntry, 0, len(r.Header))
	for name, values := range r.Header {
		headers = append(headers, headerViewEntry{
			Name:   name,
			Values: sanitizeHeaderValues(name, values),
		})
	}
	sort.Slice(headers, func(i, j int) bool {
		return strings.ToLower(headers[i].Name) < strings.ToLower(headers[j].Name)
	})

	selected := make(map[string]string, len(selectedDebugHeaders))
	for _, name := range selectedDebugHeaders {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			selected[http.CanonicalHeaderKey(name)] = value
		}
	}

	clientIP, clientIPText, clientIPSource := resolveRequestClientIP(r)

	resp := headerViewResponse{
		Method:          r.Method,
		Path:            r.URL.Path,
		Query:           r.URL.RawQuery,
		Host:            r.Host,
		RemoteAddr:      r.RemoteAddr,
		ClientIP:        clientIPText,
		ClientIPSource:  clientIPSource,
		ForwardedIP:     firstForwardedIP(r.Header.Get("X-Forwarded-For")),
		SelectedHeaders: selected,
		Headers:         headers,
	}
	if geo := h.lookupGeoIP(clientIP); geo != nil {
		resp.GeoIP = geo
	}
	writeJSON(w, http.StatusOK, resp)
}

// EnsureStorageCORS sets the S3 bucket CORS policy so browsers on any origin
// can upload files via presigned PUT URLs. Call once after deployment instead
// of blocking server startup.
// GET /api/system/ensure-cors
func (h *HealthHandler) EnsureStorageCORS(w http.ResponseWriter, r *http.Request) {
	if h.s3Client == nil {
		writeError(w, http.StatusServiceUnavailable, "S3 storage not configured")
		return
	}
	if err := h.s3Client.EnsureCORS(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "ensure storage CORS failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set storage CORS policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func sanitizeHeaderValues(name string, values []string) []string {
	switch strings.ToLower(name) {
	case "authorization", "cookie", "x-session-token":
		if len(values) == 0 {
			return values
		}
		return []string{"[redacted]"}
	default:
		return append([]string(nil), values...)
	}
}

func resolveRequestClientIP(r *http.Request) (netip.Addr, string, string) {
	clientIP, ok := requestmeta.ExtractClientIP(r)
	if !ok {
		return netip.Addr{}, "", ""
	}
	return clientIP.Addr, clientIP.Addr.String(), clientIP.Source
}

func firstForwardedIP(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func (h *HealthHandler) lookupGeoIP(addr netip.Addr) *geoLookupView {
	if h.geoIPResolver == nil || !requestmeta.IsPublicIP(addr) {
		return nil
	}

	result, err := h.geoIPResolver.Lookup(addr)
	if err != nil || result == nil {
		return nil
	}

	return &geoLookupView{
		CountryCode: result.CountryCode,
		CountryName: result.CountryName,
		RegionName:  result.RegionName,
		CityName:    result.CityName,
		Timezone:    result.Timezone,
	}
}
