package service

import "testing"

func TestSupportReplyIsPlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "bare placeholder word", content: "placeholder", want: true},
		{name: "placeholder with case and punctuation", content: "  [Placeholder.] ", want: true},
		{name: "todo marker", content: "TODO", want: true},
		{name: "punctuation only", content: "...", want: true},
		{name: "unfilled insert slot", content: "You can reset it here: [insert link]", want: true},
		{name: "unsigned template", content: "Thanks for writing in!\n\nBest,\n[Your Name]", want: true},
		{name: "lorem ipsum", content: "Lorem ipsum dolor sit amet.", want: true},
		{name: "greeting", content: "Hi! How can I help today?", want: false},
		{name: "mentions placeholder text", content: "The caption field shows placeholder text until you type.", want: false},
		{name: "short acknowledgment", content: "Thanks, Daniel.", want: false},
		{name: "non-latin reply", content: "Merci, je vérifie cela.", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportReplyIsPlaceholder(tt.content); got != tt.want {
				t.Errorf("supportReplyIsPlaceholder(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
