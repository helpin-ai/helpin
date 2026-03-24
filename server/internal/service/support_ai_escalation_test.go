package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ---------------------------------------------------------------------------
// wordBigrams
// ---------------------------------------------------------------------------

func TestWordBigrams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of bigrams
	}{
		{name: "empty", input: "", want: 0},
		{name: "single word", input: "hello", want: 0},
		{name: "two words", input: "hello world", want: 1},
		{name: "three words", input: "how are you", want: 2},
		{name: "strips punctuation", input: "hello, world!", want: 1},
		{name: "case insensitive", input: "Hello World", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordBigrams(tt.input)
			if len(got) != tt.want {
				t.Errorf("wordBigrams(%q) = %d bigrams, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestWordBigrams_Content(t *testing.T) {
	bigrams := wordBigrams("how can I reset my password")
	expected := []string{"how can", "can i", "i reset", "reset my", "my password"}
	for _, b := range expected {
		if _, ok := bigrams[b]; !ok {
			t.Errorf("wordBigrams missing expected bigram %q", b)
		}
	}
}

// ---------------------------------------------------------------------------
// jaccardSimilarity
// ---------------------------------------------------------------------------

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    map[string]struct{}
		b    map[string]struct{}
		want float64
		tol  float64
	}{
		{
			name: "both empty",
			a:    map[string]struct{}{},
			b:    map[string]struct{}{},
			want: 0,
			tol:  0.01,
		},
		{
			name: "identical",
			a:    map[string]struct{}{"a b": {}, "b c": {}},
			b:    map[string]struct{}{"a b": {}, "b c": {}},
			want: 1.0,
			tol:  0.01,
		},
		{
			name: "disjoint",
			a:    map[string]struct{}{"a b": {}},
			b:    map[string]struct{}{"c d": {}},
			want: 0,
			tol:  0.01,
		},
		{
			name: "partial overlap",
			a:    map[string]struct{}{"a b": {}, "b c": {}, "c d": {}},
			b:    map[string]struct{}{"a b": {}, "b c": {}, "d e": {}},
			want: 0.5, // 2 / 4
			tol:  0.01,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.a, tt.b)
			if got < tt.want-tt.tol || got > tt.want+tt.tol {
				t.Errorf("jaccardSimilarity() = %f, want %f (±%f)", got, tt.want, tt.tol)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detectRepetitionLoop
// ---------------------------------------------------------------------------

func TestDetectRepetitionLoop(t *testing.T) {
	tests := []struct {
		name    string
		current string
		history []model.SupportMessage
		wantNil bool
	}{
		{
			name:    "no history",
			current: "how do I reset my password",
			history: nil,
			wantNil: true,
		},
		{
			name:    "empty message",
			current: "",
			history: []model.SupportMessage{{SenderType: "customer", Content: "hi"}},
			wantNil: true,
		},
		{
			name:    "first question - no repetition",
			current: "how do I reset my password",
			history: []model.SupportMessage{
				{SenderType: "customer", Content: "how do I reset my password"},
				{SenderType: "ai", Content: "Go to settings > security > reset."},
			},
			wantNil: true, // only 1 prior similar = below threshold of 2
		},
		{
			name:    "third ask triggers escalation",
			current: "how do I reset my password",
			history: []model.SupportMessage{
				{SenderType: "customer", Content: "how do I reset my password"},
				{SenderType: "ai", Content: "Go to settings > security > reset."},
				{SenderType: "customer", Content: "how do I reset my password"},
				{SenderType: "ai", Content: "Navigate to settings, then security."},
			},
			wantNil: false,
		},
		{
			name:    "different questions - no trigger",
			current: "what are your pricing plans",
			history: []model.SupportMessage{
				{SenderType: "customer", Content: "how do I reset my password"},
				{SenderType: "ai", Content: "Go to settings > security > reset."},
				{SenderType: "customer", Content: "how do I change my email"},
				{SenderType: "ai", Content: "Go to account settings."},
			},
			wantNil: true,
		},
		{
			name:    "short message exact match triggers",
			current: "hi",
			history: []model.SupportMessage{
				{SenderType: "customer", Content: "hi"},
				{SenderType: "ai", Content: "Hello! How can I help?"},
				{SenderType: "customer", Content: "hi"},
				{SenderType: "ai", Content: "Hello again! What can I do for you?"},
			},
			wantNil: false,
		},
		{
			name:    "customer messages without AI reply not counted",
			current: "how do I reset my password",
			history: []model.SupportMessage{
				{SenderType: "customer", Content: "how do I reset my password"},
				{SenderType: "customer", Content: "how do I reset my password"},
				// No AI reply follows either message
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectRepetitionLoop(tt.current, tt.history)
			if tt.wantNil && got != nil {
				t.Errorf("detectRepetitionLoop() = %+v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("detectRepetitionLoop() = nil, want signal")
			}
			if !tt.wantNil && got != nil && got.Reason != escalationReasonRepetitionLoop {
				t.Errorf("reason = %q, want %q", got.Reason, escalationReasonRepetitionLoop)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detectFrustration
// ---------------------------------------------------------------------------

func TestDetectFrustration(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantNil bool
	}{
		{name: "neutral message", content: "Can you help me with my account?", wantNil: true},
		{name: "empty", content: "", wantNil: true},
		{name: "single low-weight keyword", content: "This is disappointing", wantNil: true},
		{
			name:    "high weight triggers",
			content: "This is ridiculous, completely useless!",
			wantNil: false,
		},
		{
			name:    "multiple medium keywords",
			content: "This is terrible and unacceptable, still not working",
			wantNil: false,
		},
		{
			name:    "exclamation marks add weight",
			content: "This is terrible!!! I am frustrated",
			wantNil: false,
		},
		{
			name:    "all caps with frustration keyword",
			content: "THIS IS BROKEN and it is unacceptable, still not working!!!",
			wantNil: false, // caps(0.2) + exclamation(0.2) + unacceptable(0.25) + still not working(0.25) = 0.9
		},
		{
			name:    "mild frustration below threshold",
			content: "I am a bit disappointed again",
			wantNil: true, // 0.15 + 0.15 = 0.30 < 0.6
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFrustration(tt.content)
			if tt.wantNil && got != nil {
				t.Errorf("detectFrustration(%q) = %+v, want nil", tt.content, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("detectFrustration(%q) = nil, want signal", tt.content)
			}
			if !tt.wantNil && got != nil && got.Reason != escalationReasonFrustration {
				t.Errorf("reason = %q, want %q", got.Reason, escalationReasonFrustration)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hasConsecutiveCapsWords
// ---------------------------------------------------------------------------

func TestHasConsecutiveCapsWords(t *testing.T) {
	tests := []struct {
		name string
		text string
		n    int
		want bool
	}{
		{name: "no caps", text: "hello world", n: 3, want: false},
		{name: "two caps only", text: "THIS IS broken", n: 3, want: false},
		{name: "three caps", text: "THIS IS BROKEN please help", n: 3, want: true},
		{name: "caps with punctuation", text: "WHAT THE HECK!", n: 3, want: true},
		{name: "single char caps ignored", text: "I AM A person", n: 3, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasConsecutiveCapsWords(tt.text, tt.n)
			if got != tt.want {
				t.Errorf("hasConsecutiveCapsWords(%q, %d) = %v, want %v", tt.text, tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detectConsecutiveLowConfidence
// ---------------------------------------------------------------------------

func aiMsg(confidence float64) model.SupportMessage {
	meta := AIMessageMetadata{AIAutoReply: true, AIConfidence: confidence}
	b, _ := json.Marshal(meta)
	return model.SupportMessage{SenderType: "ai", Metadata: string(b)}
}

func customerMsg(content string) model.SupportMessage {
	return model.SupportMessage{SenderType: "customer", Content: content}
}

func TestDetectConsecutiveLowConfidence(t *testing.T) {
	threshold := 0.70

	tests := []struct {
		name    string
		history []model.SupportMessage
		wantNil bool
	}{
		{
			name:    "no AI messages",
			history: []model.SupportMessage{customerMsg("hello")},
			wantNil: true,
		},
		{
			name: "one low confidence - not enough",
			history: []model.SupportMessage{
				customerMsg("question"),
				aiMsg(0.72), // within margin (0.70 + 0.10 = 0.80)
			},
			wantNil: true,
		},
		{
			name: "two consecutive marginal - triggers",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.75),
				customerMsg("q2"),
				aiMsg(0.73),
			},
			wantNil: false,
		},
		{
			name: "streak broken by high confidence",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.72),
				customerMsg("q2"),
				aiMsg(0.95), // breaks streak
				customerMsg("q3"),
				aiMsg(0.73),
			},
			wantNil: true, // only 1 consecutive after the break
		},
		{
			name: "high confidence messages - no trigger",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.90),
				customerMsg("q2"),
				aiMsg(0.85),
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectConsecutiveLowConfidence(tt.history, threshold)
			if tt.wantNil && got != nil {
				t.Errorf("detectConsecutiveLowConfidence() = %+v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("detectConsecutiveLowConfidence() = nil, want signal")
			}
			if !tt.wantNil && got != nil && got.Reason != escalationReasonConsecutiveLowConf {
				t.Errorf("reason = %q, want %q", got.Reason, escalationReasonConsecutiveLowConf)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// evaluatePreLLMEscalation (priority order)
// ---------------------------------------------------------------------------

func TestEvaluatePreLLMEscalation_PriorityOrder(t *testing.T) {
	// Build history that triggers both repetition AND consecutive low confidence.
	history := []model.SupportMessage{
		customerMsg("how do I reset my password"),
		aiMsg(0.72), // marginal
		customerMsg("how do I reset my password"),
		aiMsg(0.73), // marginal
	}

	// Current message is the same question again — should trigger repetition (higher priority).
	signal := evaluatePreLLMEscalation("how do I reset my password", history, 0.70)
	if signal == nil {
		t.Fatal("evaluatePreLLMEscalation() = nil, want signal")
	}
	if signal.Reason != escalationReasonRepetitionLoop {
		t.Errorf("reason = %q, want %q (repetition has priority over consecutive low conf)", signal.Reason, escalationReasonRepetitionLoop)
	}
}

func TestEvaluatePreLLMEscalation_NoSignal(t *testing.T) {
	history := []model.SupportMessage{
		customerMsg("how do I reset my password"),
		aiMsg(0.95),
	}
	signal := evaluatePreLLMEscalation("what are your pricing plans", history, 0.70)
	if signal != nil {
		t.Errorf("evaluatePreLLMEscalation() = %+v, want nil", signal)
	}
}

func TestEvaluatePreLLMEscalation_FrustrationOnly(t *testing.T) {
	history := []model.SupportMessage{
		customerMsg("how do I connect my account"),
		aiMsg(0.90),
	}
	signal := evaluatePreLLMEscalation("This is ridiculous and completely useless!", history, 0.70)
	if signal == nil {
		t.Fatal("evaluatePreLLMEscalation() = nil, want frustration signal")
	}
	if signal.Reason != escalationReasonFrustration {
		t.Errorf("reason = %q, want %q", signal.Reason, escalationReasonFrustration)
	}
}

// ---------------------------------------------------------------------------
// evaluatePostAnswerEscalation
// ---------------------------------------------------------------------------

func TestEvaluatePostAnswerEscalation(t *testing.T) {
	tests := []struct {
		name              string
		history           []model.SupportMessage
		currentConfidence float64
		wantNil           bool
	}{
		{
			name:              "too few data points",
			history:           []model.SupportMessage{aiMsg(0.90)},
			currentConfidence: 0.70,
			wantNil:           true, // only 2 points (1 history + 1 current)
		},
		{
			name: "declining trend triggers",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.92),
				customerMsg("q2"),
				aiMsg(0.80),
			},
			currentConfidence: 0.68, // drop = 0.92 - 0.68 = 0.24 > 0.20
			wantNil:           false,
		},
		{
			name: "stable trend - no trigger",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.85),
				customerMsg("q2"),
				aiMsg(0.83),
			},
			currentConfidence: 0.82, // drop = 0.85 - 0.82 = 0.03 < 0.20
			wantNil:           true,
		},
		{
			name: "improving trend - no trigger",
			history: []model.SupportMessage{
				customerMsg("q1"),
				aiMsg(0.70),
				customerMsg("q2"),
				aiMsg(0.80),
			},
			currentConfidence: 0.90,
			wantNil:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluatePostAnswerEscalation(tt.history, tt.currentConfidence)
			if tt.wantNil && got != nil {
				t.Errorf("evaluatePostAnswerEscalation() = %+v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("evaluatePostAnswerEscalation() = nil, want signal")
			}
			if !tt.wantNil && got != nil && got.Reason != escalationReasonDecliningSatisfy {
				t.Errorf("reason = %q, want %q", got.Reason, escalationReasonDecliningSatisfy)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkHardEscalation (moved function, verify still works)
// ---------------------------------------------------------------------------

func TestCheckHardEscalation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "no match", content: "How do I reset?", want: ""},
		{name: "human request", content: "I want to talk to a human", want: "customer_requested_human"},
		{name: "live agent", content: "Connect me to a live agent please", want: "customer_requested_human"},
		{name: "billing", content: "I need a refund", want: "billing_topic"},
		{name: "cancel", content: "Cancel my account now", want: "billing_topic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkHardEscalation(tt.content)
			if got != tt.want {
				t.Errorf("checkHardEscalation(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isConfirmationMessage (moved function, verify still works)
// ---------------------------------------------------------------------------

func TestIsConfirmationMessage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "thanks", content: "Thanks!", want: true},
		{name: "that helped", content: "Oh that helped a lot", want: true},
		{name: "question", content: "How do I reset?", want: false},
		{name: "empty", content: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConfirmationMessage(tt.content)
			if got != tt.want {
				t.Errorf("isConfirmationMessage(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
