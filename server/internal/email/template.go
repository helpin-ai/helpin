package email

import (
	"fmt"
	"html"
)

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

// NotificationEmailHeaderHTML returns the email header with the Helpin logo and workspace name.
func NotificationEmailHeaderHTML(workspaceName string) string {
	escapedName := html.EscapeString(workspaceName)
	return fmt.Sprintf(`<!-- Header -->
                <tr>
                  <td style="padding: 24px 32px;">
                    <table role="presentation" width="100%%%%" cellspacing="0" cellpadding="0" border="0">
                      <tr>
                        <td style="text-align: left; vertical-align: middle;">
                          <a href="https://helpin.ai" target="_blank" style="text-decoration: none; display: inline-block;">
                            <img src="%s" alt="Helpin" class="helpin-logo-light" style="display: inline; width: 100px; height: auto; border: 0; outline: none;" />
                            <img src="%s" alt="Helpin" class="helpin-logo-dark" style="display: none; width: 100px; height: auto; border: 0; outline: none;" />
                          </a>
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
		helpinLightModeLogoURL,
		helpinDarkModeLogoURL,
		escapedName,
	)
}

// TaskBlockHTML returns the task/entity card block for notification emails.
// displayID is optional (e.g. "CS-1042"), title is the entity name.
func TaskBlockHTML(displayID, title string) string {
	escapedTitle := html.EscapeString(title)
	idHTML := ""
	if displayID != "" {
		idHTML = fmt.Sprintf(`<span style="font-size: 12px; color: #aaaaaa; font-family: 'SF Mono', 'Fira Code', Consolas, monospace;">%s</span>`,
			html.EscapeString(displayID))
	}
	return fmt.Sprintf(`<tr>
                          <td style="padding-bottom: 20px;">
                            <div style="border: 1px solid #eeeeee; border-radius: 8px; padding: 16px 18px;">
                              %s
                              <div style="font-size: 14px; font-weight: 600; color: #111111; line-height: 1.45;">%s</div>
                            </div>
                          </td>
                        </tr>`, idHTML, escapedTitle)
}

// CommentBlockHTML returns the styled comment/body block for notification emails.
func CommentBlockHTML(body string) string {
	return fmt.Sprintf(`<tr>
                          <td style="padding-bottom: 24px;">
                            <div style="background-color: #fafafa; border-radius: 8px; padding: 14px 16px; font-size: 14px; color: #444444; line-height: 1.55;">%s</div>
                          </td>
                        </tr>`, body)
}

// CTAButtonHTML returns the call-to-action button block.
func CTAButtonHTML(url string) string {
	return fmt.Sprintf(`<tr>
                          <td align="center" style="padding-top: 4px; padding-bottom: 8px;">
                            <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                              <tr>
                                <td style="border-radius: 7px; background-color: #111111;">
                                  <a href="%s" target="_blank" style="display: inline-block; padding: 10px 28px; font-size: 13px; font-weight: 600; color: #ffffff; text-decoration: none;">View in Helpin</a>
                                </td>
                              </tr>
                            </table>
                          </td>
                        </tr>`, html.EscapeString(url))
}

// NotificationFooterHTML returns the contextual footer line.
func NotificationFooterHTML(contextLine string) string {
	return fmt.Sprintf(`<!-- Footer -->
                <tr>
                  <td style="padding: 18px 32px; border-top: 1px solid #f0f0f0; text-align: center; font-size: 11px; color: #bbbbbb; line-height: 1.6;">
                    %s
                  </td>
                </tr>`, html.EscapeString(contextLine))
}
