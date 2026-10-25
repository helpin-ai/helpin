package service

import (
	"strings"
	"unicode"
)

// supportReplyFillerWords are whole replies that only stand in for a message.
var supportReplyFillerWords = map[string]bool{
	"placeholder": true, "placeholder text": true, "placeholder reply": true,
	"placeholder message": true, "todo": true, "tbd": true, "tbc": true, "n/a": true,
	"na": true, "none": true, "null": true, "nil": true, "undefined": true, "test": true,
	"reply": true, "response": true, "message": true, "content": true, "text": true,
	"sample": true, "example": true, "xxx": true, "your reply": true,
	"your reply here": true, "reply here": true,
}

// supportReplyTemplateMarkers flag unfilled template slots inside a longer reply.
var supportReplyTemplateMarkers = []string{
	"lorem ipsum",
	"[insert",
	"<insert",
	"{insert",
	"[placeholder",
	"<placeholder",
	"{placeholder",
	"[your name]",
	"your reply here",
	"your message here",
	"reply goes here",
	"message goes here",
	"text goes here",
}

// supportReplyIsPlaceholder reports whether customer-facing text is a stand-in
// such as "placeholder", "TODO" or "[insert link]" rather than a written
// message. A model may fill a required content field this way when it did
// not mean to reply; sending it would email the customer a meaningless word.
func supportReplyIsPlaceholder(content string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(content), " "))
	for _, marker := range supportReplyTemplateMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	bare := strings.TrimFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return bare == "" || supportReplyFillerWords[bare]
}
