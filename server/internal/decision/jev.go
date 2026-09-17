// Package decision provides typed non-generative semantic decisions.
package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Model pins the evaluated TypeSafe model rather than a moving alias.
const Model = "jev-1.13.0"
const endpoint = "https://api.typesafe.ai/v1/systemone"

// Question defines dynamic choices. IDs are mapped to opaque provider names.
type Question struct {
	Instructions string
	Choices      map[string]string
}

// Answer separates provider concentration confidence from option probability.
type Answer struct {
	Choice             string             `json:"choice"`
	Probabilities      map[string]float64 `json:"probabilities"`
	ProviderConfidence float64            `json:"provider_confidence"`
}

// Result contains validated decisions and measured provider usage.
type Result struct {
	Model        string            `json:"model"`
	Answers      map[string]Answer `json:"answers"`
	InputTokens  int64             `json:"input_tokens"`
	OutputTokens int64             `json:"output_tokens"`
	LatencyMS    int64             `json:"latency_ms"`
}

// Provider evaluates independent questions against shared state.
type Provider interface {
	DecideMany(context.Context, string, map[string]Question) (*Result, error)
}

// Jev uses the direct System One endpoint with a bounded, non-retrying request.
type Jev struct {
	key     string
	timeout time.Duration
	http    *http.Client
}

// NewJev constructs a pooled client; no request occurs during initialization.
func NewJev(key string, timeout time.Duration) (*Jev, error) {
	if strings.TrimSpace(key) == "" || timeout <= 0 || timeout > 2*time.Second {
		return nil, errors.New("Jev requires a key and a deadline of at most two seconds")
	}
	return &Jev{key: key, timeout: timeout, http: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// DecideMany returns a validated probability distribution for every question.
func (c *Jev) DecideMany(ctx context.Context, state string, questions map[string]Question) (*Result, error) {
	start := time.Now()
	if len(state) == 0 || len(state) > 16000 || len(questions) == 0 || len(questions) > 64 {
		return nil, errors.New("decision input limits exceeded")
	}
	type wireQuestion struct {
		Type         string            `json:"type"`
		Instructions string            `json:"instructions"`
		Criteria     map[string]string `json:"criteria"`
	}
	wire := map[string]wireQuestion{}
	mappings := map[string]map[string]string{}
	for name, q := range questions {
		if len(q.Choices) < 2 || len(q.Choices) > 255 || strings.TrimSpace(q.Instructions) == "" {
			return nil, errors.New("invalid decision question")
		}
		ids := make([]string, 0, len(q.Choices))
		for id := range q.Choices {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		criteria := map[string]string{}
		mapping := map[string]string{}
		for i, id := range ids {
			if id == "" || strings.TrimSpace(q.Choices[id]) == "" {
				return nil, errors.New("invalid decision choice")
			}
			alias := fmt.Sprintf("option_%03d", i)
			criteria[alias] = q.Choices[id]
			mapping[alias] = id
		}
		wire[name] = wireQuestion{Type: "choice", Instructions: q.Instructions, Criteria: criteria}
		mappings[name] = mapping
	}
	payload, err := json.Marshal(struct {
		Model     string                  `json:"model"`
		State     string                  `json:"state"`
		Questions map[string]wireQuestion `json:"questions"`
	}{Model: Model, State: state, Questions: wire})
	if err != nil {
		return nil, errors.New("encode decision request")
	}
	if len(payload) > 128000 {
		return nil, errors.New("decision payload too large")
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("construct decision request")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("decision provider unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("decision provider status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("invalid decision response")
	}
	var body struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type          string              `json:"type"`
			Choice        string              `json:"choice"`
			Probabilities map[string]*float64 `json:"probabilities"`
			Confidence    *float64            `json:"confidence"`
		} `json:"answers"`
		Usage struct {
			Input  *int64 `json:"input_tokens"`
			Output *int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &body) != nil || body.Model != Model || len(body.Answers) != len(questions) || body.Usage.Input == nil || body.Usage.Output == nil || *body.Usage.Input < 0 || *body.Usage.Output < 0 {
		return nil, errors.New("invalid decision response contract")
	}
	result := &Result{Model: body.Model, Answers: map[string]Answer{}, InputTokens: *body.Usage.Input, OutputTokens: *body.Usage.Output, LatencyMS: time.Since(start).Milliseconds()}
	for name, mapping := range mappings {
		a, ok := body.Answers[name]
		if !ok || a.Type != "choice" || a.Confidence == nil || !probability(*a.Confidence) || len(a.Probabilities) != len(mapping) {
			return nil, errors.New("invalid decision answer")
		}
		choice, ok := mapping[a.Choice]
		if !ok {
			return nil, errors.New("unknown decision choice")
		}
		sum, highest := 0.0, 0.0
		ps := map[string]float64{}
		for alias, id := range mapping {
			p := a.Probabilities[alias]
			if p == nil || !probability(*p) {
				return nil, errors.New("invalid decision probabilities")
			}
			ps[id] = *p
			sum += *p
			highest = math.Max(highest, *p)
		}
		if math.Abs(sum-1) > .002 || ps[choice] < highest-1e-7 {
			return nil, errors.New("inconsistent decision probabilities")
		}
		result.Answers[name] = Answer{Choice: choice, Probabilities: ps, ProviderConfidence: *a.Confidence}
	}
	return result, nil
}
func probability(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
