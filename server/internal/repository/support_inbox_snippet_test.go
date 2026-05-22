package repository

import "testing"

func TestCleanMessageSnippet(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		limit int
		want  string
	}{
		{
			name:  "plain text passes through",
			raw:   "Hi Marco, Thanks for the reply",
			limit: 100,
			want:  "Hi Marco, Thanks for the reply",
		},
		{
			name:  "markdown autolink unwrapped",
			raw:   "Story link: <https://example.com/story/1>",
			limit: 100,
			want:  "Story link: https://example.com/story/1",
		},
		{
			name:  "mailto autolink unwrapped",
			raw:   "Email <mailto:test@example.com> me",
			limit: 100,
			want:  "Email mailto:test@example.com me",
		},
		{
			name:  "markdown link keeps text only",
			raw:   "See [our docs](https://docs.example.com) for details",
			limit: 100,
			want:  "See our docs for details",
		},
		{
			name:  "image keeps alt text",
			raw:   "Here ![diagram](https://img.example.com/d.png) below",
			limit: 100,
			want:  "Here diagram below",
		},
		{
			name:  "table pipes collapsed to spaces",
			raw:   "Thanks!| col | col |\n|---|---|\n| a | b |",
			limit: 100,
			want:  "Thanks! col col a b",
		},
		{
			name:  "headings stripped",
			raw:   "## Update\nAll good",
			limit: 100,
			want:  "Update All good",
		},
		{
			name:  "blockquote markers stripped",
			raw:   "> Quoted reply\n> second line",
			limit: 100,
			want:  "Quoted reply second line",
		},
		{
			name:  "list bullets stripped",
			raw:   "- one\n- two\n* three",
			limit: 100,
			want:  "one two three",
		},
		{
			name:  "emphasis markers stripped",
			raw:   "Hi **Meni**, thank you _so_ much",
			limit: 100,
			want:  "Hi Meni, thank you so much",
		},
		{
			name:  "markdown hard-break escapes stripped",
			raw:   "Hi Caleb,\\\n\\\nThank you for reaching out.",
			limit: 100,
			want:  "Hi Caleb, Thank you for reaching out.",
		},
		{
			name:  "html tags stripped",
			raw:   "Hi <b>Ryan</b>, before we move on&hellip; &amp; thanks",
			limit: 100,
			want:  "Hi Ryan, before we move on&hellip; & thanks",
		},
		{
			name:  "Note prefix preserved with cleaned body",
			raw:   "Note: Story link: <https://example.com/x>",
			limit: 100,
			want:  "Note: Story link: https://example.com/x",
		},
		{
			name:  "truncates with ellipsis",
			raw:   "abcdefghij",
			limit: 5,
			want:  "abcde…",
		},
		{
			name:  "no limit returns full cleaned text",
			raw:   "Hi <b>Marco</b>, thanks",
			limit: 0,
			want:  "Hi Marco, thanks",
		},
		{
			name:  "collapses runs of whitespace",
			raw:   "line1\n\n\nline2\t\tend",
			limit: 100,
			want:  "line1 line2 end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanMessageSnippet(tt.raw, tt.limit)
			if got != tt.want {
				t.Errorf("cleanMessageSnippet(%q, %d) = %q, want %q", tt.raw, tt.limit, got, tt.want)
			}
		})
	}
}
