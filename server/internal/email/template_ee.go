//go:build ee

package email

import (
	"fmt"
	"html"
)

const (
	helpinLightModeLogoURL = "https://helpin.ai/brand/helpin-icon-ink-128.png"
	helpinDarkModeLogoURL  = "https://helpin.ai/brand/helpin-icon-white-128.png"
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
      display: inline-block !important;
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
                <span class="helpin-logo-light" style="display: inline-block; white-space: nowrap;">
                  <img src="%s" alt="" width="36" height="36" style="display: inline-block; width: 36px; height: 36px; vertical-align: middle; border: 0; outline: none;" />
                  <span style="display: inline-block; margin-left: 10px; vertical-align: middle; color: #1e1c1a; font-size: 26px; line-height: 36px; font-weight: 700; letter-spacing: -1px;">Helpin</span>
                </span>
                <span class="helpin-logo-dark" style="display: none; white-space: nowrap;">
                  <img src="%s" alt="" width="36" height="36" style="display: inline-block; width: 36px; height: 36px; vertical-align: middle; border: 0; outline: none;" />
                  <span style="display: inline-block; margin-left: 10px; vertical-align: middle; color: #f7f5f2; font-size: 26px; line-height: 36px; font-weight: 700; letter-spacing: -1px;">Helpin</span>
                </span>
              </a>
            </td>
          </tr>`,
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
	)
}

// NotificationEmailHeaderHTML returns the email header with the Helpin logo and workspace name.
func NotificationEmailHeaderHTML(workspaceName, destinationURL string) string {
	escapedName := html.EscapeString(workspaceName)
	logoOpen := `<span style="display: inline-block;">`
	logoClose := `</span>`
	if destinationURL != "" {
		logoOpen = fmt.Sprintf(`<a href="%s" target="_blank" style="text-decoration: none; display: inline-block;">`, html.EscapeString(destinationURL))
		logoClose = `</a>`
	}
	return fmt.Sprintf(`<!-- Header -->
                <tr>
                  <td style="padding: 24px 32px;">
                    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0">
                      <tr>
                        <td style="text-align: left; vertical-align: middle;">
                          %s
                            <span class="helpin-logo-light" style="display: inline-block; white-space: nowrap;">
                              <img src="%s" alt="" width="26" height="26" style="display: inline-block; width: 26px; height: 26px; vertical-align: middle; border: 0; outline: none;" />
                              <span style="display: inline-block; margin-left: 7px; vertical-align: middle; color: #1e1c1a; font-size: 18px; line-height: 26px; font-weight: 700; letter-spacing: -0.5px;">Helpin</span>
                            </span>
                            <span class="helpin-logo-dark" style="display: none; white-space: nowrap;">
                              <img src="%s" alt="" width="26" height="26" style="display: inline-block; width: 26px; height: 26px; vertical-align: middle; border: 0; outline: none;" />
                              <span style="display: inline-block; margin-left: 7px; vertical-align: middle; color: #f7f5f2; font-size: 18px; line-height: 26px; font-weight: 700; letter-spacing: -0.5px;">Helpin</span>
                            </span>
                          %s
                        </td>
                        <td style="text-align: right; vertical-align: middle;">
                          <span style="font-size: 10px; text-transform: uppercase; letter-spacing: 0.8px; color: #bbbbbb; font-weight: 500; display: block; margin-bottom: 2px;">Workspace</span>
                          <span style="font-size: 13px; color: #666666; font-weight: 600;">%s</span>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
                <!-- Divider -->
                <tr>
                  <td style="padding: 0 32px;">
                    <div style="height: 1px; background-color: #eeeeee;"></div>
                  </td>
                </tr>`,
		logoOpen,
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
		logoClose,
		escapedName,
	)
}
