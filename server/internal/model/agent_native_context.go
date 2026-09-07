package model

import "fmt"

// AgentNativeContextConfig controls the opt-in native context canary. Limits
// must be selected for the resolved model; zero context_window cannot enable it.
type AgentNativeContextConfig struct {
	Enabled          bool  `json:"enabled"`
	ContextWindow    int   `json:"context_window,omitempty"`
	InputLimit       int   `json:"input_limit,omitempty"`
	MaxOutputTokens  int   `json:"max_output_tokens,omitempty"`
	TriggerTokens    int   `json:"trigger_tokens,omitempty"`
	KeepRecentTokens int   `json:"keep_recent_tokens,omitempty"`
	SummaryTokens    int   `json:"summary_tokens,omitempty"`
	SafetyTokens     int   `json:"safety_tokens,omitempty"`
	MaxTotalTokens   int64 `json:"max_total_tokens,omitempty"`
}

// Validate rejects unsafe configurations before an agent can be launched.
func (c AgentNativeContextConfig) Validate() error {
	if c.MaxTotalTokens < 0 {
		return fmt.Errorf("native_context.max_total_tokens must be nonnegative")
	}
	if !c.Enabled {
		return nil
	}
	if c.ContextWindow < 4096 || c.ContextWindow > 4000000 {
		return fmt.Errorf("native_context.context_window must be between 4096 and 4000000")
	}
	output, safety := c.MaxOutputTokens, c.SafetyTokens
	if output == 0 {
		output = min(16384, c.ContextWindow/4)
	}
	if safety == 0 {
		safety = min(4096, c.ContextWindow/16)
	}
	input := c.InputLimit
	if input == 0 {
		input = c.ContextWindow - output - safety
	}
	if input <= 0 || output <= 0 || safety < 0 || input > c.ContextWindow-output-safety {
		return fmt.Errorf("native context input/output limits exceed context window")
	}
	trigger, recent, summary := c.TriggerTokens, c.KeepRecentTokens, c.SummaryTokens
	if trigger == 0 {
		trigger = min(64000, input)
	}
	if recent == 0 {
		recent = min(16000, trigger/4)
	}
	if summary == 0 {
		summary = min(4000, trigger/8)
	}
	if trigger <= 0 || trigger > input || recent <= 0 || summary <= 0 || recent+summary >= trigger {
		return fmt.Errorf("invalid native context trigger, summary or retained-history budget")
	}
	return nil
}
