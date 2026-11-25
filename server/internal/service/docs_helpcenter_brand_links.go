package service

import (
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

// Hosted help center platforms; a link to one of them is the company's help
// center even on another domain.
var _helpCenterPlatformSuffixes = []string{
	".intercom.help", ".zendesk.com", ".freshdesk.com", ".helpscoutdocs.com", ".gitbook.io",
	".document360.io", ".helpjuice.com", ".crisp.help", ".notion.site",
}

var _helpCenterHostLabels = map[string]bool{
	"help": true, "support": true, "docs": true, "kb": true, "knowledge": true,
	"helpcenter": true, "faq": true, "faqs": true, "hilfe": true, "aide": true, "ayuda": true,
}

var (
	_helpCenterPath = regexp.MustCompile(`(?i)^/(help|support|hc|docs|knowledge(-?base)?|kb|faqs?|help-?cent(er|re)|support-?cent(er|re))(/|$)`)
	_helpCenterText = regexp.MustCompile(`(?i)^(help|support|help\s*(cent(er|re)|&\s*support)|support\s*cent(er|re)|knowledge\s*base|docs|documentation|faqs?)$`)
)

// helpCenterLink scores how likely a link leads to the site's help center:
// 3 for a help subdomain or a hosted help center platform, 2 for a help path
// on the same site, 1 for a same-site link labelled "Help" or "Support", plus
// 1 when a scored link also carries such a label. It returns the link without
// query or fragment, or a score of 0.
func helpCenterLink(a *html.Node, base *url.URL) (string, int) {
	target := resolveBrandURL(base, strings.TrimSpace(htmlAttr(a, "href")))
	parsed, err := url.Parse(target)
	if target == "" || err != nil || parsed.Hostname() == "" {
		return "", 0
	}
	host, baseHost := strings.ToLower(parsed.Hostname()), strings.ToLower(base.Hostname())
	site, _ := publicsuffix.EffectiveTLDPlusOne(host)
	baseSite, _ := publicsuffix.EffectiveTLDPlusOne(baseHost)
	sameSite := site != "" && site == baseSite
	labelled := _helpCenterText.MatchString(strings.Join(strings.Fields(nodeTextContent(a)), " "))
	score := 0
	switch {
	case hasAnySuffix(host, _helpCenterPlatformSuffixes):
		score = 3
	case sameSite && host != baseHost && _helpCenterHostLabels[strings.SplitN(host, ".", 2)[0]]:
		score = 3
	case sameSite && _helpCenterPath.MatchString(parsed.Path):
		score = 2
	case sameSite && labelled && host != baseHost:
		score = 1
	}
	if score == 0 {
		return "", 0
	}
	if labelled {
		score++
	}
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed.String(), score
}

func hasAnySuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
