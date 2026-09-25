//go:build !ee

package email

import (
	"strings"
	"testing"
)

func TestCommunityMailHasNoRemoteBranding(t *testing.T) {
	for _, html := range []string{BrandHeaderHTML(), NotificationEmailHeaderHTML("<Example>")} {
		for _, forbidden := range []string{"https://", "http://", "<img", "helpin.ai"} {
			if strings.Contains(html, forbidden) {
				t.Fatalf("community email contains %q", forbidden)
			}
		}
	}
	if !strings.Contains(NotificationEmailHeaderHTML("<Example>"), "&lt;Example&gt;") {
		t.Fatal("workspace name not escaped")
	}
}
