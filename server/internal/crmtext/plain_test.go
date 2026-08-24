package crmtext

import "testing"

func TestPlainTextPreservesInlineTagBoundaries(t *testing.T) {
	got := PlainText(`<p>Can you send pri<strong>cing</strong> &amp; procurement details?</p><p>Thanks.</p>`)
	want := "Can you send pricing & procurement details? Thanks."
	if got != want {
		t.Fatalf("PlainText() = %q, want %q", got, want)
	}
}

func TestPreferredBodySanitizesHTMLFallbackAndTruncatesRunes(t *testing.T) {
	got := PreferredBody("", `<div>价格<strong>计划</strong></div>`, 3)
	if got != "价格计" {
		t.Fatalf("PreferredBody() = %q, want %q", got, "价格计")
	}
}
