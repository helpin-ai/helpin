package templates

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestLoadRegistryLoadsValidManifestsInKeyOrder(t *testing.T) {
	registry, err := LoadRegistry(fstest.MapFS{
		"alpha/template.yaml": {Data: []byte(validTemplateYAML("alpha", "Alpha", "cron", model.ActionStartAgentRun, `
agent:
  pick_existing:
    constraints:
      presets: [code_builder]
      targets: [repository]
`))},
		"beta/template.yaml": {Data: []byte(validTemplateYAML("beta", "Beta", model.TriggerTaskStateEntered, model.ActionMoveToState, `
agent:
  none: true
`))},
	})
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}

	if got := keysOf(registry.List()); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("registry.List keys = %#v, want alpha/beta order", got)
	}
	if got, ok := registry.Get("beta"); !ok || got.Name != "Beta" {
		t.Fatalf("registry.Get(beta) = %#v, %v; want Beta template", got, ok)
	}
}

func TestLoadRegistryRejectsInvalidManifest(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "multiple agent modes",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agent:
  create:
    preset: documentation_agent
  pick_existing:
    constraints:
      targets: [repository]
`),
			want: "exactly one agent mode",
		},
		{
			name: "unknown category",
			content: strings.ReplaceAll(validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agent:
  reuse_system: documentation_agent
`), "categories: [workflow]", "categories: [finance]"),
			want: "unknown category",
		},
		{
			name: "unsupported action",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionRunAgent, `
agent:
  pick_existing:
    constraints:
      targets: [repository]
`),
			want: "unsupported flow action",
		},
		{
			name: "agent none with start_agent_run",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agent:
  none: true
`),
			want: "requires a non-agent action",
		},
		{
			name: "agent create with non agent action",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionMoveToState, `
agent:
  create:
    preset: documentation_agent
`),
			want: "requires start_agent_run",
		},
		{
			name: "unknown system preset",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agent:
  reuse_system: made_up_agent
`),
			want: "unknown system preset",
		},
		{
			name: "unsupported pick existing constraint",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agent:
  pick_existing:
    constraints:
      runtime_kind: native_sdk
`),
			want: "unsupported pick_existing constraint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadRegistry(fstest.MapFS{
				"bad/template.yaml": {Data: []byte(tc.content)},
			})
			if err == nil {
				t.Fatal("LoadRegistry returned nil error, want validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadRegistry error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestLoadRegistryRejectsDuplicateKeys(t *testing.T) {
	_, err := LoadRegistry(fstest.MapFS{
		"one/template.yaml": {Data: []byte(validTemplateYAML("duplicate", "One", "cron", model.ActionStartAgentRun, `
agent:
  reuse_system: documentation_agent
`))},
		"two/template.yaml": {Data: []byte(validTemplateYAML("duplicate", "Two", "cron", model.ActionStartAgentRun, `
agent:
  reuse_system: documentation_agent
`))},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate template key") {
		t.Fatalf("LoadRegistry error = %v, want duplicate template key", err)
	}
}

func TestEmbeddedSystemRegistryLoads(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	want := []string{
		"advance_on_approval",
		"buying_signal_to_task",
		"competitive_intelligence_digest",
		"dependency_auditor",
		"docs_freshness_sweep",
		"merge_when_done",
		"release_notes_writer",
		"review_merged_prs",
		"run_on_a_schedule",
		"run_on_release",
		"security_triage",
		"stale_task_escalation",
		"triage_failing_checks",
	}
	if got := keysOf(registry.List()); !reflect.DeepEqual(got, want) {
		t.Fatalf("system registry keys = %#v, want %#v", got, want)
	}
}

func keysOf(templates []Template) []string {
	keys := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		keys = append(keys, tmpl.Key)
	}
	return keys
}

func validTemplateYAML(key, name, trigger, action, agent string) string {
	return strings.TrimSpace(`
key: `+key+`
version: 1
name: `+name+`
icon: scroll
short_description: Short description.
categories: [workflow]
`+agent+`
trigger:
  type: event
  event: `+trigger+`
inputs:
  - key: name
    type: string
    required: true
    label: Name
flow:
  action: `+action+`
`) + "\n"
}

var _ fs.FS = fstest.MapFS{}
