package decision

import (
	"errors"
	"math"
)

// Policy controls an independently enabled semantic decision feature.
type Policy struct {
	Mode       string
	Threshold  float64
	DailyLimit int
}

// Validate checks policy values before serving any decisions.
func (p Policy) Validate() error {
	if p.Mode != "off" && p.Mode != "shadow" && p.Mode != "primary" {
		return errors.New("invalid decision mode")
	}
	if math.IsNaN(p.Threshold) || math.IsInf(p.Threshold, 0) || p.Threshold < .5 || p.Threshold > 1 || p.DailyLimit < 1 {
		return errors.New("invalid decision threshold or limit")
	}
	return nil
}

// ValidateResult rejects malformed choices and distributions at the service
// boundary, including cached results and alternate provider implementations.
func ValidateResult(result *Result, questions map[string]Question) error {
	if result == nil || result.Model != Model || len(result.Answers) != len(questions) || result.InputTokens < 0 || result.OutputTokens < 0 {
		return errors.New("invalid decision result")
	}
	for key, question := range questions {
		answer, ok := result.Answers[key]
		if !ok || len(answer.Probabilities) != len(question.Choices) {
			return errors.New("incomplete decision result")
		}
		if _, ok := question.Choices[answer.Choice]; !ok {
			return errors.New("unknown decision choice")
		}
		sum := 0.0
		for choice := range question.Choices {
			probability, ok := answer.Probabilities[choice]
			if !ok || math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 || probability > answer.Probabilities[answer.Choice] {
				return errors.New("invalid decision probability")
			}
			sum += probability
		}
		if math.Abs(sum-1) > .001 {
			return errors.New("invalid decision distribution")
		}
	}
	return nil
}
