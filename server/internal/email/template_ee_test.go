//go:build ee

package email

import (
	"strings"
	"testing"
)

func TestBrandHeaderHTMLUsesThemeCorrectLogoMarks(t *testing.T) {
	header := BrandHeaderHTML()

	for _, want := range []string{
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
		`class="helpin-logo-light"`,
		`class="helpin-logo-dark"`,
		`>Helpin</span>`,
	} {
		if !strings.Contains(header, want) {
			t.Errorf("BrandHeaderHTML() missing %q", want)
		}
	}
}

func TestNotificationEmailHeaderHTMLUsesLogoMarksAndEscapesWorkspace(t *testing.T) {
	header := NotificationEmailHeaderHTML(`<Acme & Co>`, "https://app.helpin.ai/w/acme/notifications")

	for _, want := range []string{
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
		`&lt;Acme &amp; Co&gt;`,
		`href="https://app.helpin.ai/w/acme/notifications"`,
	} {
		if !strings.Contains(header, want) {
			t.Errorf("NotificationEmailHeaderHTML() missing %q", want)
		}
	}
	if strings.Contains(header, `href="https://helpin.ai"`) {
		t.Fatal("notification header links to the marketing site")
	}
	if strings.Contains(NotificationEmailHeaderHTML("Acme", ""), "<a ") {
		t.Fatal("notification header should not link to an unrelated workspace")
	}
}
