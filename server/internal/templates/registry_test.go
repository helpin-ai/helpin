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
		"competitors_changelog_tracking_report",
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

func TestBuyingSignalToTaskUsesCanonicalSignalTaxonomy(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	tmpl, ok := registry.Get("buying_signal_to_task")
	if !ok {
		t.Fatal("buying_signal_to_task template not found")
	}
	canonical := map[string]bool{
		model.CRMSignalBuyingIntent: true, model.CRMSignalObjection: true,
		model.CRMSignalCompetitorMention: true, model.CRMSignalBudgetSignal: true,
		model.CRMSignalTimelineSignal: true, model.CRMSignalChampionSignal: true,
		model.CRMSignalRiskSignal: true,
	}
	var signalInput *Input
	for index := range tmpl.Inputs {
		if tmpl.Inputs[index].Key == "signal_types" {
			signalInput = &tmpl.Inputs[index]
			break
		}
	}
	if signalInput == nil {
		t.Fatal("signal_types input not found")
	}
	for _, option := range signalInput.Options {
		if !canonical[option.Value] {
			t.Fatalf("signal_types option %q is not a canonical CRM signal", option.Value)
		}
	}
	defaults, ok := signalInput.Default.([]interface{})
	if !ok {
		t.Fatalf("signal_types default = %#v, want a list", signalInput.Default)
	}
	for _, value := range defaults {
		name, ok := value.(string)
		if !ok || !canonical[name] {
			t.Fatalf("signal_types default %q is not a canonical CRM signal", name)
		}
	}
}

func TestEmbeddedDependencyAuditorAllowsGuardedRegistryFetches(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	tmpl, ok := registry.Get("engineering_dependency_auditor")
	if !ok {
		t.Fatal("engineering_dependency_auditor template not found")
	}
	if tmpl.Agent.Create == nil || !containsString(tmpl.Agent.Create.AllowedTools, "fetch_url") {
		t.Fatalf("dependency auditor allowed tools = %#v, want fetch_url", tmpl.Agent.Create)
	}
	prompt := embeddedTemplatePrompt(tmpl)
	if !strings.Contains(prompt, "Do not invoke curl or wget through run_command") {
		t.Fatalf("dependency auditor prompt must direct registry requests through fetch_url:\n%s", prompt)
	}
}

func TestEmbeddedReportTemplatesUseStandardTitlePattern(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	reportTemplates := map[string]string{
		"api_docs_freshness_sweep":              "Use this title format: `YYYY-MM-DD - ",
		"competitors_changelog_tracking_report": "Use this title format: `Competitors changelog tracking report - <scope/context> - YYYY-MM-DD`",
		"engineering_dependency_auditor":        "Use this title format: `YYYY-MM-DD - ",
		"docs_freshness_sweep":                  "Use this title format: `YYYY-MM-DD - ",
		"public_help_freshness_sweep":           "Use this title format: `YYYY-MM-DD - ",
		"review_merged_prs":                     "Use this title format: `YYYY-MM-DD - ",
		"engineering_security_triage":           "Use this title format: `YYYY-MM-DD - ",
		"triage_failing_checks":                 "Use this title format: `YYYY-MM-DD - ",
	}
	for key, want := range reportTemplates {
		t.Run(key, func(t *testing.T) {
			tmpl, ok := registry.Get(key)
			if !ok {
				t.Fatalf("template %q not found", key)
			}
			prompt := embeddedTemplatePrompt(tmpl)
			if !strings.Contains(prompt, want) {
				t.Fatalf("template %q missing standard report title format in prompt:\n%s", key, prompt)
			}
		})
	}
}

func TestEmbeddedReportTemplatesUseSingleCreateDocumentContract(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	reportTemplates := []string{
		"api_docs_freshness_sweep",
		"competitors_changelog_tracking_report",
		"engineering_dependency_auditor",
		"docs_freshness_sweep",
		"public_help_freshness_sweep",
		"release_notes_writer",
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
			prompt := embeddedTemplatePrompt(tmpl)
			if tmpl.Agent.Create != nil && containsString(tmpl.Agent.Create.AllowedTools, "write_document_content") {
				t.Fatalf("template %q should create Docs reports with create_document content, not write_document_content: %#v", key, tmpl.Agent.Create.AllowedTools)
			}
			if tmpl.Agent.Create == nil || containsString(tmpl.Agent.Create.AllowedTools, "create_document") {
				if !strings.Contains(prompt, "Call create_document exactly once") || !strings.Contains(prompt, "with the complete report content") {
					t.Fatalf("template %q must require one create_document call with complete content:\n%s", key, prompt)
				}
			}
			if !strings.Contains(prompt, "Do not repeat the document title") {
				t.Fatalf("template %q must prevent title repetition in the document body:\n%s", key, prompt)
			}
		})
	}
}

func TestEmbeddedCompetitorsChangelogTemplateUsesNewIdentity(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	if _, ok := registry.Get("competitive_intelligence_digest"); ok {
		t.Fatal("legacy competitive_intelligence_digest template key should not be exposed")
	}
	tmpl, ok := registry.Get("competitors_changelog_tracking_report")
	if !ok {
		t.Fatal("competitors_changelog_tracking_report template not found")
	}
	if tmpl.Name != "Competitors Changelog Tracking Report" {
		t.Fatalf("template name = %q", tmpl.Name)
	}
	if tmpl.Agent.Create == nil || len(tmpl.Agent.Create.Skills) != 1 || tmpl.Agent.Create.Skills[0] != "competitors_changelog_tracking_report" {
		t.Fatalf("expected renamed skill ref, got %+v", tmpl.Agent.Create)
	}
	if tmpl.Agent.Create.ApprovalMode != "never" {
		t.Fatalf("approval mode = %q, want never", tmpl.Agent.Create.ApprovalMode)
	}
	if !strings.Contains(tmpl.Agent.Create.SystemPrompt, "runtime's built-in web search") || !strings.Contains(tmpl.Agent.Create.SystemPrompt, "not evidence that a competitor has no public changelog") {
		t.Fatalf("competitive research prompt must support runtime-native search and reject false no-changelog conclusions:\n%s", tmpl.Agent.Create.SystemPrompt)
	}
}

func TestEmbeddedCompetitorsChangelogTemplateCreatesOneDocumentWithContent(t *testing.T) {
	registry, err := LoadSystemRegistry()
	if err != nil {
		t.Fatalf("LoadSystemRegistry returned error: %v", err)
	}
	tmpl, ok := registry.Get("competitors_changelog_tracking_report")
	if !ok {
		t.Fatal("competitors_changelog_tracking_report template not found")
	}
	if tmpl.Agent.Create == nil {
		t.Fatal("expected created agent config")
	}

	prompt := tmpl.Agent.Create.SystemPrompt
	if !strings.Contains(prompt, "Use this title format: `Competitors changelog tracking report - <scope/context> - YYYY-MM-DD`") {
		t.Fatalf("competitors changelog prompt missing date-at-end title format:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Call create_document exactly once") || !strings.Contains(prompt, "with the complete report content") {
		t.Fatalf("competitors changelog prompt must require a single create_document call with content:\n%s", prompt)
	}
	if !containsString(tmpl.Agent.Create.AllowedTools, "create_document") {
		t.Fatalf("expected create_document in allowed tools, got %#v", tmpl.Agent.Create.AllowedTools)
	}
	if containsString(tmpl.Agent.Create.AllowedTools, "write_document_content") {
		t.Fatalf("write_document_content should not be allowed for this template; create_document must include content, got %#v", tmpl.Agent.Create.AllowedTools)
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

func embeddedTemplatePrompt(tmpl Template) string {
	prompt := strings.TrimSpace(tmpl.Flow.AdditionalContext)
	if prompt == "" && tmpl.Agent.Create != nil {
		prompt = strings.TrimSpace(tmpl.Agent.Create.SystemPrompt)
	}
	return prompt
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
