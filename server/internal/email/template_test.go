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
	header := NotificationEmailHeaderHTML(`<Acme & Co>`)

	for _, want := range []string{
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
		`&lt;Acme &amp; Co&gt;`,
	} {
		if !strings.Contains(header, want) {
			t.Errorf("NotificationEmailHeaderHTML() missing %q", want)
		}
	}
}
