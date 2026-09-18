package decision

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestJevMappingAndConfidence(t *testing.T) {
	c, err := NewJev("secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != endpoint || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("invalid request destination/auth")
		}
		var request struct {
			Questions map[string]struct {
				Criteria map[string]string `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Questions) != 2 || request.Questions["route"].Criteria["option_000"] != "Payments" {
			t.Fatal("dynamic descriptions missing")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"jev-1.13.0","answers":{"route":{"type":"choice","choice":"option_000","probabilities":{"option_000":0.8,"option_001":0.2},"confidence":0.4},"other":{"type":"choice","choice":"option_001","probabilities":{"option_000":0.1,"option_001":0.9},"confidence":0.7}},"usage":{"input_tokens":50,"output_tokens":20}}`))}, nil
	})
	q := Question{Instructions: "Route", Choices: map[string]string{"billing": "Payments", "support": "Bugs"}}
	result, err := c.DecideMany(context.Background(), "synthetic", map[string]Question{"route": q, "other": q})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || result.Answers["route"].Choice != "billing" || result.Answers["route"].ProviderConfidence != .4 || result.Answers["route"].Probabilities["billing"] != .8 {
		t.Fatalf("incorrect mapped result: %+v", result)
	}
}
func TestJevRejectsInvalidResponses(t *testing.T) {
	valid := `{"model":"jev-1.13.0","answers":{"q":{"type":"choice","choice":"option_000","probabilities":{"option_000":0.8,"option_001":0.2},"confidence":0.4}},"usage":{"input_tokens":50,"output_tokens":20}}`
	cases := []struct {
		name, body string
		status     int
	}{
		{name: "auth", body: "secret customer content", status: 401},
		{name: "rate limit", body: "private", status: 429},
		{name: "redirect", body: "private", status: 302},
		{name: "unknown choice", body: strings.Replace(valid, `"choice":"option_000"`, `"choice":"unexpected"`, 1), status: 200},
		{name: "sum", body: strings.Replace(valid, `0.8`, `0.1`, 1), status: 200},
		{name: "null", body: strings.Replace(valid, `0.8`, `null`, 1), status: 200},
		{name: "version", body: strings.Replace(valid, Model, "jev-other", 1), status: 200},
		{name: "missing usage", body: strings.Replace(valid, `"input_tokens":50,`, "", 1), status: 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewJev("secret", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			_, err = c.DecideMany(context.Background(), "private", map[string]Question{"q": {Instructions: "route", Choices: map[string]string{"a": "A", "b": "B"}}})
			if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") || calls != 1 {
				t.Fatalf("unsafe result: %v calls %d", err, calls)
			}
		})
	}
}
func TestJevTimeout(t *testing.T) {
	c, err := NewJev("secret", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	start := time.Now()
	_, err = c.DecideMany(context.Background(), "test", map[string]Question{"q": {Instructions: "test", Choices: map[string]string{"a": "A", "b": "B"}}})
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("deadline not bounded")
	}
}
