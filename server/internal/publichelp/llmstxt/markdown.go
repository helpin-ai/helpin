package llmstxt

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	_titleLine = regexp.MustCompile(`^#\s+(.+?)\s*#*\s*$`)
	// Ferndesk opens a page with its title in brackets instead of a heading.
	_bracketTitle = regexp.MustCompile(`^\[([^\]]+)\]\s*$`)
	_metaLine     = regexp.MustCompile(`^([A-Za-z_][\w-]*):\s*(.*)$`)
	// The opening of a fenced code block, perhaps inside a quote, with the
	// first word of what follows the fence.
	_fence     = regexp.MustCompile("^(\\s*(?:>\\s*)*)(`{3,}|~{3,})\\s*([^\\s`]*)")
	_language  = regexp.MustCompile(`^[\w+#.-]+$`)
	_codeSpan  = regexp.MustCompile("`[^`\n]*`")
	_listItem  = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s`)
	_attribute = regexp.MustCompile(`([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|\{\s*"([^"]*)"\s*\})`)
	// A GitBook block tag: `{% hint style="info" %}`, `{% endhint %}`.
	_tag = regexp.MustCompile(`\{%\s*([@/\w.-]+)([^%]*)%\}`)
	// An MDX component tag, which ReadMe leaves in its Markdown: `<Card
	// title="Start" href="/start">`, `</Card>`, `<Image src="a.png" />`.
	_component = regexp.MustCompile(`</?([A-Z][\w.]*)((?:\s+[\w:-]+(?:=(?:"[^"]*"|'[^']*'|\{[^{}]*\}))?)*)\s*/?>`)
	// An Archbee directive on its own line: `:::Tab{title="One"}`, `::::`.
	_directive = regexp.MustCompile(`^(\s*):{2,}([A-Za-z][\w-]*)?(?:\[([^\]]*)\])?(?:\{([^}]*)\})?\s*$`)
	// An Archbee directive inside a line: `:Icon[label]{name="x"}`.
	_inlineDirective = regexp.MustCompile(`:[A-Za-z][\w-]*\[([^\]]*)\]\{[^}]*\}`)
	// A GitHub alert, which Featurebase uses for callouts: `> [!NOTE]`.
	_alert = regexp.MustCompile(`^(\s*>\s*)\[!(\w+)\]\s*$`)
	// Mintlify marks what was a component in a quote: `> [card] [Title](address)`,
	// `> [tab] Title`.
	_flattened = regexp.MustCompile(`^(\s*>\s*)\[(?:card|tab|accordion|step)\]\s+(.*)$`)
	_trailer   = regexp.MustCompile(`^(?:This documentation is built and hosted on \[Mintlify\]\(.*|# (?:Sub|Sibling) pages)$`)
	_stamp     = regexp.MustCompile(`^(?:Source|Last updated): \S+$`)
	// GitBook writes an icon as `<i class="fa-check">:check:</i>`.
	_icon = regexp.MustCompile(`<i class="fa-[^"]*">:[\w-]+:</i>\s*`)
)

// clean takes the title and description out of a page's Markdown and turns
// what the platforms add to Markdown into Markdown: it drops the front
// matter and the notes for AI agents above the title, and rewrites GitBook
// tags, MDX components and Archbee directives as plain text and links.
func clean(raw string) (title, description, body string) {
	raw = strings.ReplaceAll(strings.TrimPrefix(raw, "\ufeff"), "\r\n", "\n")
	meta, rest := frontMatter(raw)
	title, description = meta["title"], meta["description"]
	lines := strings.Split(rest, "\n")
	// Notes for AI agents come as quotes before the title. On a page without
	// a title, they end with the quote that names the index.
	top, notes := 0, 0
	for top < len(lines) && (strings.TrimSpace(lines[top]) == "" || strings.HasPrefix(lines[top], ">")) {
		top++
		if strings.Contains(lines[top-1], indexName) {
			for notes = top; notes < len(lines) && strings.HasPrefix(lines[notes], ">"); notes++ {
			}
		}
	}
	headed := top < len(lines) && _titleLine.MatchString(lines[top])
	if headed {
		title = cmp.Or(title, _titleLine.FindStringSubmatch(lines[top])[1])
		notes = top + 1
	}
	lines = lines[notes:]
	if first := skipBlank(lines, 0); !headed && first < len(lines) && _bracketTitle.MatchString(lines[first]) {
		title = cmp.Or(title, _bracketTitle.FindStringSubmatch(lines[first])[1])
		lines = lines[first+1:]
	}
	// Mintlify signs every page, and ReadMe ends a page with lists of the
	// pages around it.
	if end := slices.IndexFunc(lines, _trailer.MatchString); end >= 0 {
		lines = lines[:end]
	}
	// Mintlify and Fern put the page's description in a one-line quote below
	// the title. A callout there starts with a bold or bracketed label.
	if at := skipBlank(lines, 0); headed && at < len(lines) && strings.HasPrefix(lines[at], "> ") &&
		(at+1 == len(lines) || strings.TrimSpace(lines[at+1]) == "") {
		if quote := strings.TrimSpace(lines[at][2:]); quote != "" && !strings.ContainsAny(quote[:1], "*[#") {
			description = cmp.Or(description, quote)
			lines = lines[at+1:]
		}
	}
	// Kustomer repeats the page's address and date below the description.
	for at := skipBlank(lines, 0); at < len(lines) && _stamp.MatchString(lines[at]); at = skipBlank(lines, 0) {
		lines = lines[at+1:]
	}
	return strings.TrimSpace(title), description, rewrite(lines)
}

// skipBlank returns the index of the first line at or after from that is not
// blank.
func skipBlank(lines []string, from int) int {
	for from < len(lines) && strings.TrimSpace(lines[from]) == "" {
		from++
	}
	return from
}

// frontMatter splits the YAML block at the top of a page from the rest and
// returns its plain "key: value" lines.
func frontMatter(raw string) (map[string]string, string) {
	meta := map[string]string{}
	if !strings.HasPrefix(raw, "---\n") {
		return meta, raw
	}
	block, rest, closed := strings.Cut(raw[4:]+"\n", "\n---\n")
	if !closed {
		return meta, raw
	}
	for line := range strings.SplitSeq(block, "\n") {
		match := _metaLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value := strings.TrimSpace(match[2])
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else if len(value) > 1 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
		// A value that continues on the next lines is not worth a YAML parser.
		if value != "" && !strings.ContainsAny(value[:1], ">|[{") {
			meta[match[1]] = value
		}
	}
	return meta, rest
}

// rewrite turns the lines of a page into plain Markdown. Code blocks are
// kept as they are, apart from the options after the language.
func rewrite(lines []string) string {
	var out strings.Builder
	// quote is the indent of an open GitBook hint, whose lines become a
	// Markdown quote; fence is the marker that closes the open code block.
	quoting, quote, fence, inList := false, "", "", false
	emit := func(line string) {
		if quoting {
			line = quote + "> " + strings.TrimPrefix(line, quote)
		}
		out.WriteString(strings.TrimRight(line, " \t") + "\n")
	}
	for _, line := range lines {
		if fence != "" {
			if closing := strings.TrimLeft(line, " \t>"); strings.HasPrefix(closing, fence) && strings.Trim(closing, fence[:1]+" \t") == "" {
				fence = ""
			}
			emit(line)
			continue
		}
		if match := _fence.FindStringSubmatch(line); match != nil {
			fence = match[2]
			// Mintlify follows the language with options: "ts theme={...}".
			if !_language.MatchString(match[3]) {
				match[3] = ""
			}
			emit(match[1] + match[2] + match[3])
			continue
		}
		changed, hint, block := convert(line)
		if hint == hintCloses {
			quoting = false
		}
		// These exports mark code with fences, so text indented outside a list
		// is layout left over from HTML, not a code block.
		if !inList && len(changed)-len(strings.TrimLeft(changed, " \t")) >= 4 {
			changed = strings.TrimLeft(changed, " \t")
		}
		if _listItem.MatchString(changed) {
			inList = true
		} else if changed != "" && changed == strings.TrimLeft(changed, " \t") {
			inList = false
		}
		if block {
			emit("")
		}
		emit(changed)
		if block {
			emit("")
		}
		if hint == hintOpens {
			quoting, quote = true, line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		}
	}
	return strings.TrimSpace(out.String())
}

// What a line does to a GitBook hint.
const (
	hintOpens  = 1
	hintCloses = 2
)

// convert rewrites the platform tags on one line outside a code block. hint
// tells whether a GitBook hint opens or closes on the line, and block that a
// tag became text of its own, which blank lines must set apart.
func convert(line string) (changed string, hint int, block bool) {
	if match := _directive.FindStringSubmatch(line); match != nil {
		replacement := component(match[2], attributes(match[4]))
		return match[1] + replacement, 0, replacement != ""
	}
	stand := func(replacement string) string {
		block = block || replacement != ""
		return replacement
	}
	changed = outsideCode(line, func(text string) string {
		text = _tag.ReplaceAllStringFunc(text, func(tag string) string {
			match := _tag.FindStringSubmatch(tag)
			switch match[1] {
			case "hint":
				hint = hintOpens
				return ""
			case "endhint":
				hint = hintCloses
				return ""
			case "content-ref":
				// The tag wraps a Markdown link to the same page.
				return ""
			}
			return stand(component(match[1], attributes(match[2])))
		})
		text = _component.ReplaceAllStringFunc(text, func(tag string) string {
			if strings.HasPrefix(tag, "</") {
				return ""
			}
			match := _component.FindStringSubmatch(tag)
			return stand(component(match[1], attributes(match[2])))
		})
		return _icon.ReplaceAllString(_inlineDirective.ReplaceAllString(text, "$1"), "")
	})
	if match := _flattened.FindStringSubmatch(changed); match != nil {
		changed = match[1] + match[2]
		if !strings.HasPrefix(match[2], "[") {
			changed = match[1] + "**" + match[2] + "**"
		}
	}
	if match := _alert.FindStringSubmatch(changed); match != nil {
		changed = match[1] + "**" + strings.ToUpper(match[2][:1]) + strings.ToLower(match[2][1:]) + "**"
	}
	return changed, hint, block
}

// outsideCode applies change to the parts of a line that are not inline
// code, where a tag is an example and not markup.
func outsideCode(line string, change func(string) string) string {
	var out strings.Builder
	last := 0
	for _, span := range _codeSpan.FindAllStringIndex(line, -1) {
		out.WriteString(change(line[last:span[0]]))
		out.WriteString(line[span[0]:span[1]])
		last = span[1]
	}
	out.WriteString(change(line[last:]))
	return out.String()
}

// attributes reads the `name="value"` pairs of a tag.
func attributes(text string) map[string]string {
	out := map[string]string{}
	for _, match := range _attribute.FindAllStringSubmatch(text, -1) {
		out[match[1]] = cmp.Or(match[2], match[3], match[4])
	}
	return out
}

// component is the Markdown that stands for a platform's own block: an image
// for an image, a link for a card or an embed, a bold line for a titled tab
// or step, and nothing for a tag that only wraps content.
func component(name string, attrs map[string]string) string {
	target := cmp.Or(attrs["href"], attrs["url"])
	if target == "" && attrs["src"] != "" && strings.Contains(strings.ToLower(name), "image") {
		return "![" + attrs["alt"] + "](" + attrs["src"] + ")"
	}
	target = cmp.Or(target, attrs["src"])
	// An API field is named by where it goes: `<ParamField query="limit">`.
	label := cmp.Or(attrs["title"], attrs["label"], attrs["header"], attrs["name"], attrs["path"], attrs["query"], attrs["body"])
	switch {
	case target != "" && label != "":
		return "**[" + label + "](" + target + ")**"
	case target != "":
		return "[" + target + "](" + target + ")"
	case label != "":
		return "**" + label + "**"
	}
	return ""
}
