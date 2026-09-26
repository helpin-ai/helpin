//go:build !ee

package email

import "html"

// Community mail is self-contained: no remote branding images, tracking pixels,
// or links to a hosted Helpin installation.
func BrandHeaderCSS() string { return "" }
func BrandHeaderHTML() string {
	return `<tr><td style="padding-bottom:24px;font-size:24px;font-weight:600">Helpin</td></tr>`
}
func NotificationEmailHeaderHTML(workspaceName, _ string) string {
	return `<tr><td style="padding:24px;font-size:18px;font-weight:600">` + html.EscapeString(workspaceName) + `</td></tr>`
}
