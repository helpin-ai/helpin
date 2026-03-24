package service

import (
	"testing"
)

func TestParseAIResponse(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantOK         bool
		wantContent    string
		wantCanAnswer  bool
		wantConfidence float64
	}{
		{
			name:           "pure JSON",
			raw:            `{"content": "Hello there!", "can_answer": true, "source_doc_ids": [], "confidence": 0.9}`,
			wantOK:         true,
			wantContent:    "Hello there!",
			wantCanAnswer:  true,
			wantConfidence: 0.9,
		},
		{
			name:           "fenced JSON with ```json",
			raw:            "```json\n{\"content\": \"Hi!\", \"can_answer\": true, \"source_doc_ids\": [], \"confidence\": 0.85}\n```",
			wantOK:         true,
			wantContent:    "Hi!",
			wantCanAnswer:  true,
			wantConfidence: 0.85,
		},
		{
			name:           "fenced JSON with ``` (no language hint)",
			raw:            "```\n{\"content\": \"No hint\", \"can_answer\": false, \"source_doc_ids\": [], \"confidence\": 0.5}\n```",
			wantOK:         true,
			wantContent:    "No hint",
			wantCanAnswer:  false,
			wantConfidence: 0.5,
		},
		{
			name:           "markdown text followed by embedded ```json block",
			raw:            "ContentStudio is a tool for social media management.\n\n## Features\n- Scheduling\n- Analytics\n\n```json\n{\"content\": \"ContentStudio is a tool for social media management.\", \"can_answer\": true, \"source_doc_ids\": [\"abc\"], \"confidence\": 0.95}\n```",
			wantOK:         true,
			wantContent:    "ContentStudio is a tool for social media management.",
			wantCanAnswer:  true,
			wantConfidence: 0.95,
		},
		{
			name:           "markdown text followed by trailing raw JSON object",
			raw:            "ContentStudio provides analytics.\n\n- Scheduling\n- Reports\n\n{\"content\":\"ContentStudio provides analytics.\",\"can_answer\":true,\"source_doc_ids\":[\"abc\"],\"confidence\":0.95}",
			wantOK:         true,
			wantContent:    "ContentStudio provides analytics.",
			wantCanAnswer:  true,
			wantConfidence: 0.95,
		},
		{
			name:          "markdown text with embedded JSON but empty contract content uses text before block",
			raw:           "Here is your answer about X.\n\n```json\n{\"content\": \"\", \"can_answer\": true, \"source_doc_ids\": [], \"confidence\": 0.8}\n```",
			wantOK:        true,
			wantContent:   "Here is your answer about X.",
			wantCanAnswer: true,
		},
		{
			name:        "plain conversational text with no JSON at all",
			raw:         "I don't have information about that topic, but I can help with other questions!",
			wantOK:      false,
			wantContent: "I don't have information about that topic, but I can help with other questions!",
		},
		{
			name:        "plain text with trailing invalid JSON block is preserved",
			raw:         "Here is my answer.\n\n```json\nnot valid json\n```",
			wantOK:      false,
			wantContent: "Here is my answer.\n\n```json\nnot valid json\n```",
		},
		{
			name:        "plain text with trailing raw non-contract JSON is preserved",
			raw:         "Here is my answer.\n\n{\"name\":\"test\",\"value\":42}",
			wantOK:      false,
			wantContent: "Here is my answer.\n\n{\"name\":\"test\",\"value\":42}",
		},
		{
			name:           "JSON with extra whitespace and newlines",
			raw:            "\n\n  ```json\n  {\n    \"content\": \"Trimmed\",\n    \"can_answer\": true,\n    \"source_doc_ids\": [],\n    \"confidence\": 0.7\n  }\n  ```\n\n",
			wantOK:         true,
			wantContent:    "Trimmed",
			wantCanAnswer:  true,
			wantConfidence: 0.7,
		},
		{
			name:           "long markdown with emoji and embedded JSON (real-world case)",
			raw:            "Hello! Welcome to support! 👋\n\nHere's what I found:\n\n## Overview\nThis is a great product.\n\n- Feature A\n- Feature B\n\n```json\n{\"content\": \"Hello! Welcome to support!\", \"can_answer\": true, \"source_doc_ids\": [\"doc1\", \"doc2\"], \"confidence\": 0.92}\n```",
			wantOK:         true,
			wantContent:    "Hello! Welcome to support!",
			wantCanAnswer:  true,
			wantConfidence: 0.92,
		},
		{
			name:        "user shares code with ```json that is NOT an AI contract",
			raw:         "Here's an example:\n\n```json\n{\"name\": \"test\", \"value\": 42}\n```\n\nHope that helps!",
			wantOK:      false,
			wantContent: "Here's an example:\n\n```json\n{\"name\": \"test\", \"value\": 42}\n```\n\nHope that helps!",
		},
		{
			name:        "empty string",
			raw:         "",
			wantOK:      false,
			wantContent: "",
		},
		{
			name:        "only whitespace",
			raw:         "   \n\n  ",
			wantOK:      false,
			wantContent: "   \n\n  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contract, cleaned, ok := parseAIResponse(tt.raw)

			if ok != tt.wantOK {
				t.Fatalf("parseAIResponse() ok = %v, want %v", ok, tt.wantOK)
			}

			if ok {
				if contract.Content != tt.wantContent {
					t.Errorf("contract.Content = %q, want %q", contract.Content, tt.wantContent)
				}
				if contract.CanAnswer != tt.wantCanAnswer {
					t.Errorf("contract.CanAnswer = %v, want %v", contract.CanAnswer, tt.wantCanAnswer)
				}
				if tt.wantConfidence > 0 && contract.Confidence != tt.wantConfidence {
					t.Errorf("contract.Confidence = %v, want %v", contract.Confidence, tt.wantConfidence)
				}
			} else {
				if cleaned != tt.wantContent {
					t.Errorf("cleanedContent = %q, want %q", cleaned, tt.wantContent)
				}
			}
		})
	}
}

func TestIsTemplateLikeAIContent(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{content: "Your answer in markdown", want: true},
		{content: "  <customer-facing answer in markdown>  ", want: true},
		{content: "Your response here", want: true},
		{content: "ContentStudio does include AI features for caption generation.", want: false},
	}

	for _, tt := range tests {
		if got := isTemplateLikeAIContent(tt.content); got != tt.want {
			t.Errorf("isTemplateLikeAIContent(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}
