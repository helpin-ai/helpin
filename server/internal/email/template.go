package email

import (
	"fmt"
	"html"
)

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
