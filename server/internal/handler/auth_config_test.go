package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthHandler_GetConfig_ReportsSetupGuide(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{name: "setup guide enabled", enabled: true},
		{name: "setup guide disabled", enabled: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newAuthHandlerTestFixture(t)
			h.SetSetupGuideEnabled(tt.enabled)

			rec := httptest.NewRecorder()
			h.GetConfig(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
			var cfg map[string]any
			decodeJSONResponse(t, rec, &cfg)
			got, ok := cfg["setup_guide_enabled"].(bool)
			if !ok || got != tt.enabled {
				t.Fatalf("setup_guide_enabled = %v, want %v", cfg["setup_guide_enabled"], tt.enabled)
			}
		})
	}
}
