// Package readers lists the help center platforms that can be read by address.
package readers

import (
	"github.com/helpin-ai/helpin/server/internal/publichelp"
	"github.com/helpin-ai/helpin/server/internal/publichelp/crisp"
	"github.com/helpin-ai/helpin/server/internal/publichelp/freshdesk"
	"github.com/helpin-ai/helpin/server/internal/publichelp/helpscoutdocs"
	"github.com/helpin-ai/helpin/server/internal/publichelp/hubspot"
	"github.com/helpin-ai/helpin/server/internal/publichelp/llmstxt"
	"github.com/helpin-ai/helpin/server/internal/publichelp/zendesk"
)

// All returns a reader for every platform, in the order they are asked about
// an address: readers that recognize one platform first, the general ones
// last.
func All(fetch *publichelp.Fetcher) []publichelp.Reader {
	return []publichelp.Reader{
		zendesk.New(fetch),
		crisp.New(fetch),
		freshdesk.New(fetch),
		helpscoutdocs.New(fetch),
		hubspot.New(fetch),
		llmstxt.New(fetch),
	}
}
