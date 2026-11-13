package linkpreview

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MetaStart and MetaEnd delimit the replaceable head block in the SPA shell.
const MetaStart = "<!-- helpin-meta:start -->"
const MetaEnd = "<!-- helpin-meta:end -->"

var headTemplate = template.Must(template.New("preview-head").Parse(`<title>{{.Title}}</title>
<meta name="description" content="{{.Card.Description}}" />
<meta name="robots" content="noindex, nofollow, noarchive, nosnippet" />
<meta name="googlebot" content="noindex, nofollow, noarchive, nosnippet" />
<meta property="og:type" content="website" />
<meta property="og:site_name" content="Helpin" />
<meta property="og:title" content="{{.Title}}" />
<meta property="og:description" content="{{.Card.Description}}" />
<meta property="og:url" content="{{.Card.URL}}" />
<meta property="og:image" content="{{.Card.ImageURL}}" />
<meta property="og:image:type" content="image/png" />
<meta property="og:image:width" content="1200" />
<meta property="og:image:height" content="630" />
<meta property="og:image:alt" content="{{.Title}}" />
<meta name="twitter:card" content="summary_large_image" />
<meta name="twitter:title" content="{{.Title}}" />
<meta name="twitter:description" content="{{.Card.Description}}" />
<meta name="twitter:image" content="{{.Card.ImageURL}}" />
<meta name="twitter:image:alt" content="{{.Title}}" />`))

// Head returns escaped metadata and absolute URLs for the configured app origin.
// Origins come from deployment configuration, never Host or forwarded headers.
func Head(card model.LinkPreview, appOrigin, apiOrigin string) (model.LinkPreview, string, error) {
	card.URL = absolute(appOrigin, card.URL)
	card.ImageURL = absolute(apiOrigin, card.ImageURL)
	title := card.Title
	if card.Reference != "" {
		title = card.Reference + " · " + title
	}
	var out bytes.Buffer
	err := headTemplate.Execute(&out, struct {
		Title string
		Card  model.LinkPreview
	}{Title: title, Card: card})
	return card, out.String(), err
}

// Inject replaces only the metadata block, preserving the SPA boot and assets.
func Inject(html, head string) string {
	start, end := strings.Index(html, MetaStart), strings.Index(html, MetaEnd)
	if start < 0 || end < start {
		return html
	}
	return html[:start] + MetaStart + "\n" + head + "\n" + html[end:]
}

func absolute(origin, path string) string {
	base, err := url.Parse(origin)
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return ""
	}
	rel, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return origin + "/"
	}
	base.Path, base.RawPath, base.RawQuery, base.Fragment = rel.Path, rel.RawPath, rel.RawQuery, ""
	base.User = nil
	return base.String()
}
