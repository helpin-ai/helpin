package email

import "fmt"

const (
	helpinLightModeLogoURL = "https://assets.helpin.ai/logos/helpin-light-mode-logo.png"
	helpinDarkModeLogoURL  = "https://assets.helpin.ai/logos/helpin-dark-mode-logo.png"
)

// BrandHeaderCSS returns CSS used by branded email headers.
func BrandHeaderCSS() string {
	return `<style>
  .helpin-logo-dark {
    display: none !important;
  }

  @media (prefers-color-scheme: dark) {
    .helpin-logo-light {
      display: none !important;
    }

    .helpin-logo-dark {
      display: block !important;
    }
  }
</style>`
}

// BrandHeaderHTML returns the shared Helpin logo block for email templates.
func BrandHeaderHTML() string {
	return fmt.Sprintf(`
          <tr>
            <td align="center" style="padding-bottom: 32px;">
              <a href="https://helpin.ai" target="_blank" style="display: inline-block; text-decoration: none;">
                <img src="%s" alt="Helpin" class="helpin-logo-light" style="display: block; width: 152px; max-width: 100%%; height: auto; border: 0; outline: none; text-decoration: none;" />
                <img src="%s" alt="Helpin" class="helpin-logo-dark" style="display: none; width: 152px; max-width: 100%%; height: auto; border: 0; outline: none; text-decoration: none;" />
              </a>
            </td>
          </tr>`,
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
	)
}
