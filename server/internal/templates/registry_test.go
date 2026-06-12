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
		{
			name: "unknown yaml field",
			content: validTemplateYAML("bad", "Bad", "cron", model.ActionStartAgentRun, `
agents:
  reuse_system: documentation_agent
`),
			want: "field agents not found",
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
		"api_docs_freshness_sweep",
		"buying_signal_to_task",
		"competitive_intelligence_digest",
		"docs_freshness_sweep",
		"engineering_dependency_auditor",
		"engineering_security_triage",
		"merge_when_done",
		"public_help_freshness_sweep",
		"release_notes_writer",
		"review_merged_prs",
		"run_on_a_schedule",
		"run_on_release",
		"stale_task_escalation",
		"triage_failing_checks",
	}
	if got := keysOf(registry.List()); !reflect.DeepEqual(got, want) {
		t.Fatalf("system registry keys = %#v, want %#v", got, want)
	}
}

func TestEmbeddedReportTemplatesUseStandardTitlePattern(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	reportTemplates := []string{
		"api_docs_freshness_sweep",
		"competitive_intelligence_digest",
		"engineering_dependency_auditor",
		"docs_freshness_sweep",
		"public_help_freshness_sweep",
		"review_merged_prs",
		"engineering_security_triage",
		"triage_failing_checks",
	}
	for _, key := range reportTemplates {
		t.Run(key, func(t *testing.T) {
			tmpl, ok := registry.Get(key)
			if !ok {
				t.Fatalf("template %q not found", key)
			}
			prompt := strings.TrimSpace(tmpl.Flow.AdditionalContext)
			if prompt == "" && tmpl.Agent.Create != nil {
				prompt = strings.TrimSpace(tmpl.Agent.Create.SystemPrompt)
			}
			if !strings.Contains(prompt, "Use this title format: `YYYY-MM-DD - ") {
				t.Fatalf("template %q missing standard report title format in prompt:\n%s", key, prompt)
			}
		})
	}
}

func TestReleaseNotesWriterExplainsPrereleaseOption(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	tmpl, ok := registry.Get("release_notes_writer")
	if !ok {
		t.Fatal("release_notes_writer template not found")
	}

	var prereleaseInput *Input
	for i := range tmpl.Inputs {
		if tmpl.Inputs[i].Key == "include_prerelease" {
			prereleaseInput = &tmpl.Inputs[i]
			break
		}
	}
	if prereleaseInput == nil {
		t.Fatal("include_prerelease input not found")
	}
	if prereleaseInput.Label != "Also run for prereleases" {
		t.Fatalf("include_prerelease label = %q, want clear prerelease wording", prereleaseInput.Label)
	}
	if !strings.Contains(prereleaseInput.HelpText, "normal GitHub releases") || !strings.Contains(prereleaseInput.HelpText, "beta, RC, or preview") {
		t.Fatalf("include_prerelease help_text = %q, want normal release and prerelease examples", prereleaseInput.HelpText)
	}
	if !strings.Contains(tmpl.Agent.Create.SystemPrompt, "If include_prerelease is false") {
		t.Fatalf("release notes writer prompt does not explain include_prerelease behavior:\n%s", tmpl.Agent.Create.SystemPrompt)
	}
}

func TestReleaseNotesWriterCollectionDependsOnDestinationSpace(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	tmpl, ok := registry.Get("release_notes_writer")
	if !ok {
		t.Fatal("release_notes_writer template not found")
	}

	var spaceInput *Input
	var collectionInput *Input
	for i := range tmpl.Inputs {
		switch tmpl.Inputs[i].Key {
		case "destination_space_id":
			spaceInput = &tmpl.Inputs[i]
		case "destination_collection_id":
			collectionInput = &tmpl.Inputs[i]
		}
	}
	if spaceInput == nil {
		t.Fatal("destination_space_id input not found")
	}
	if spaceInput.Type != "space" {
		t.Fatalf("destination_space_id type = %q, want space", spaceInput.Type)
	}
	if collectionInput == nil {
		t.Fatal("destination_collection_id input not found")
	}
	if collectionInput.DependsOn != "destination_space_id" {
		t.Fatalf("destination_collection_id depends_on = %q, want destination_space_id", collectionInput.DependsOn)
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
