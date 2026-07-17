package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ---------------------------------------------------------------------------
// Pre-router: deterministic short-circuit for non-questions.
//
// Runs before triage, planning, and retrieval so that greetings and other
// conversational non-questions never consume an LLM call or count as answer
// attempts. Anything with substantive content must fall through to the full
// pipeline — when in doubt, proceed.
// ---------------------------------------------------------------------------

const (
	// supportPreRouterModelName labels pre-router replies in message metadata
	// so analytics can separate them from model-generated answers.
	supportPreRouterModelName = "pre-router"

	// Maximum tokens for a message to be considered a pure greeting/gratitude.
	preRouteMaxTokens = 6

	supportPreRouterGreetingReply  = "Hello! What can I help you with today?"
	supportPreRouterGratitudeReply = "You're welcome! If anything else comes up, just send a message here."
)

type preRouteOutcome string

const (
	// preRouteProceed means the message has (or may have) substantive content
	// and must go through the full pipeline.
	preRouteProceed preRouteOutcome = "proceed"
	// preRouteGreet short-circuits with a lightweight greeting reply.
	preRouteGreet preRouteOutcome = "greet"
	// preRouteGratitude short-circuits with a lightweight closing reply when
	// there is no prior substantive AI turn to confirm as resolved.
	preRouteGratitude preRouteOutcome = "gratitude"
)

type preRouteDecision struct {
	Outcome preRouteOutcome
	Reason  string
	Reply   string
}

// preRouteSupportMessage classifies an incoming customer message before any
// LLM stage. Greeting short-circuits apply only on first contact
// (aiTurnCount == 0): a mid-conversation "hello??" is a nudge that the full
// pipeline (repetition/frustration signals) should see, not a fresh greeting.
func preRouteSupportMessage(content string, aiTurnCount int) preRouteDecision {
	switch {
	case aiTurnCount == 0 && isPureGreeting(content):
		return preRouteDecision{
			Outcome: preRouteGreet,
			Reason:  "pure_greeting",
			Reply:   supportPreRouterGreetingReply,
		}
	case aiTurnCount == 0 && isPureGratitude(content):
		// Conversation opened with thanks (e.g. after reading a help article).
		// Nothing to resolve; acknowledge without invoking the pipeline.
		return preRouteDecision{
			Outcome: preRouteGratitude,
			Reason:  "pure_gratitude",
			Reply:   supportPreRouterGratitudeReply,
		}
	default:
		return preRouteDecision{Outcome: preRouteProceed}
	}
}

// Core greeting words — at least one must be present.
var preRouteGreetingCore = map[string]struct{}{
	"hi": {}, "hello": {}, "hey": {}, "heya": {}, "hiya": {},
	"howdy": {}, "greetings": {}, "yo": {},
}

// Filler words allowed alongside greeting words without making the message
// substantive ("hi there", "hello good morning", "hey team").
var preRouteGreetingFiller = map[string]struct{}{
	"there": {}, "team": {}, "everyone": {}, "all": {}, "guys": {},
	"folks": {}, "friends": {}, "dear": {}, "support": {},
	"good": {}, "morning": {}, "afternoon": {}, "evening": {}, "day": {},
}

// isPureGreeting reports whether the message consists solely of greeting
// words. "Hi, how do I cancel my plan?" is NOT a greeting — every token must
// belong to the greeting vocabulary. Punctuation and emoji are ignored by
// tokenization, so "Hi 👋" still matches.
func isPureGreeting(content string) bool {
	tokens := tokenizeWords(content)
	if len(tokens) == 0 || len(tokens) > preRouteMaxTokens {
		return false
	}
	hasCore := false
	timeOfDay := false
	hasGood := false
	for _, tok := range tokens {
		if _, ok := preRouteGreetingCore[tok]; ok {
			hasCore = true
			continue
		}
		if _, ok := preRouteGreetingFiller[tok]; ok {
			switch tok {
			case "good":
				hasGood = true
			case "morning", "afternoon", "evening", "day":
				timeOfDay = true
			}
			continue
		}
		return false // any non-greeting token makes it substantive
	}
	// "good morning" counts as a greeting even without hi/hello.
	return hasCore || (hasGood && timeOfDay)
}

// Core gratitude words — at least one must be present.
var preRouteGratitudeCore = map[string]struct{}{
	"thanks": {}, "thank": {}, "thx": {}, "ty": {}, "cheers": {},
	"thankyou": {}, "merci": {}, "gracias": {},
}

// Filler words allowed alongside gratitude words ("thank you so much", "ok
// thanks", "thanks a lot").
var preRouteGratitudeFiller = map[string]struct{}{
	"you": {}, "u": {}, "so": {}, "very": {}, "much": {}, "a": {}, "lot": {},
	"ok": {}, "okay": {}, "cool": {}, "anyway": {}, "again": {}, "for": {},
	"that": {}, "this": {}, "the": {}, "help": {}, "info": {},
}

// isPureGratitude reports whether the message is only an expression of
// thanks with no new content.
func isPureGratitude(content string) bool {
	tokens := tokenizeWords(content)
	if len(tokens) == 0 || len(tokens) > preRouteMaxTokens {
		return false
	}
	hasCore := false
	for _, tok := range tokens {
		if _, ok := preRouteGratitudeCore[tok]; ok {
			hasCore = true
			continue
		}
		if _, ok := preRouteGratitudeFiller[tok]; ok {
			continue
		}
		return false
	}
	return hasCore
}

// countSubstantiveAITurns counts AI turns that carried substance (answers or
// clarifications). Greeting turns do not qualify a later "thanks" as a
// resolution confirmation — there was nothing to confirm.
func countSubstantiveAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if !isAIReplyFromAgent(msg, agentID) {
			continue
		}
		if inferAIReplyKind(msg) == supportReplyKindGreeting {
			continue
		}
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// Hardened confirmation detection
// ---------------------------------------------------------------------------

// confirmationNegationTokens block confirmation detection when present as
// whole words: "not helpful at all" or "thanks but it still doesn't work"
// must never be treated as a resolution confirmation. The bare token "t"
// catches all tokenized negative contractions ("doesn t", "can t", "won t").
var confirmationNegationTokens = map[string]struct{}{
	"not": {}, "no": {}, "never": {}, "still": {}, "but": {}, "however": {},
	"unhelpful": {}, "nothing": {}, "unfortunately": {}, "cannot": {},
	"dont": {}, "doesnt": {}, "didnt": {}, "isnt": {}, "wasnt": {},
	"wont": {}, "cant": {}, "t": {}, "issue": {}, "problem": {}, "error": {},
}

// confirmationPhrases are matched as whole tokenized phrases.
var confirmationPhrases = []string{
	"thanks", "thank you", "that helped", "got it", "perfect",
	"that works", "that worked", "awesome", "great", "resolved", "solved",
	"that's what i needed", "all good", "helpful", "works now", "makes sense",
}

const confirmationMaxWords = 12

// isConfirmationMessage checks if a customer message is a resolution
// confirmation: whole-word phrase matching plus negation, question, and
// length guards. Its predecessor substring-matched ("great", "helpful"),
// which classified "not helpful at all" as a confirmed resolution.
func isConfirmationMessage(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "?") {
		return false // questions are never confirmations
	}
	words := tokenizeWords(lower)
	if len(words) == 0 || len(words) > confirmationMaxWords {
		return false // long messages carry new content
	}
	for _, w := range words {
		if _, blocked := confirmationNegationTokens[w]; blocked {
			return false
		}
	}
	padded := " " + strings.Join(words, " ") + " "
	for _, phrase := range confirmationPhrases {
		normalized := strings.Join(tokenizeWords(phrase), " ")
		if strings.Contains(padded, " "+normalized+" ") {
			return true
		}
	}
	return false
}
