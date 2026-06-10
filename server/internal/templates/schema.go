package templates

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gopkg.in/yaml.v3"
)

type Template struct {
	Key              string    `yaml:"key" json:"key"`
	Version          int       `yaml:"version" json:"version"`
	Name             string    `yaml:"name" json:"name"`
	Icon             string    `yaml:"icon" json:"icon"`
	ShortDescription string    `yaml:"short_description" json:"short_description"`
	DescriptionRef   string    `yaml:"description_ref,omitempty" json:"description_ref,omitempty"`
	Categories       []string  `yaml:"categories,omitempty" json:"categories"`
	Agent            AgentSpec `yaml:"agent" json:"agent"`
	Trigger          Trigger   `yaml:"trigger" json:"trigger"`
	Inputs           []Input   `yaml:"inputs,omitempty" json:"inputs"`
	Flow             FlowSpec  `yaml:"flow" json:"flow"`
	basePath         string
}

type AgentSpec struct {
	Create       *AgentCreateSpec       `yaml:"create,omitempty" json:"create,omitempty"`
	ReuseSystem  string                 `yaml:"reuse_system,omitempty" json:"reuse_system,omitempty"`
	PickExisting *AgentPickExistingSpec `yaml:"pick_existing,omitempty" json:"pick_existing,omitempty"`
	None         bool                   `yaml:"none,omitempty" json:"none,omitempty"`
}

type AgentCreateSpec struct {
	Preset          string   `yaml:"preset" json:"preset"`
	RuntimeKind     string   `yaml:"runtime_kind,omitempty" json:"runtime_kind,omitempty"`
	NameTemplate    string   `yaml:"name_template,omitempty" json:"name_template,omitempty"`
	SystemPromptRef string   `yaml:"system_prompt_ref,omitempty" json:"system_prompt_ref,omitempty"`
	SystemPrompt    string   `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
	Skills          []string `yaml:"skills,omitempty" json:"skills,omitempty"`
	AllowedTools    []string `yaml:"allowed_tools,omitempty" json:"allowed_tools,omitempty"`
	AllowedTargets  []string `yaml:"allowed_targets,omitempty" json:"allowed_targets,omitempty"`
	ApprovalMode    string   `yaml:"approval_mode,omitempty" json:"approval_mode,omitempty"`
}

type AgentPickExistingSpec struct {
	Required    bool                    `yaml:"required,omitempty" json:"required,omitempty"`
	Constraints PickExistingConstraints `yaml:"constraints,omitempty" json:"constraints,omitempty"`
}

type PickExistingConstraints struct {
	Presets []string `yaml:"presets,omitempty" json:"presets,omitempty"`
	Targets []string `yaml:"targets,omitempty" json:"targets,omitempty"`
}

func (c *PickExistingConstraints) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == 0 {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("constraints must be a mapping")
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		switch key {
		case "presets":
			if err := node.Content[i+1].Decode(&c.Presets); err != nil {
				return err
			}
		case "targets":
			if err := node.Content[i+1].Decode(&c.Targets); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported pick_existing constraint %q", key)
		}
	}
	return nil
}

type Trigger struct {
	Type  string `yaml:"type" json:"type"`
	Event string `yaml:"event,omitempty" json:"event,omitempty"`
}

type Input struct {
	Key         string      `yaml:"key" json:"key"`
	Type        string      `yaml:"type" json:"type"`
	Required    bool        `yaml:"required,omitempty" json:"required,omitempty"`
	Label       string      `yaml:"label" json:"label"`
	Section     string      `yaml:"section,omitempty" json:"section,omitempty"`
	HelpText    string      `yaml:"help_text,omitempty" json:"help_text,omitempty"`
	Placeholder string      `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	DependsOn   string      `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	ShowIf      string      `yaml:"show_if,omitempty" json:"show_if,omitempty"`
	SpaceType   string      `yaml:"space_type,omitempty" json:"space_type,omitempty"`
	Min         *int        `yaml:"min,omitempty" json:"min,omitempty"`
	Max         *int        `yaml:"max,omitempty" json:"max,omitempty"`
	Options     []Option    `yaml:"options,omitempty" json:"options,omitempty"`
}

type Option struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label" json:"label"`
}

type FlowSpec struct {
	Action              string                 `yaml:"action" json:"action"`
	NameTemplate        string                 `yaml:"name_template,omitempty" json:"name_template,omitempty"`
	DescriptionTemplate string                 `yaml:"description_template,omitempty" json:"description_template,omitempty"`
	Target              map[string]string      `yaml:"target,omitempty" json:"target,omitempty"`
	Conditions          []map[string]string    `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	Parameters          map[string]interface{} `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	AdditionalContext   string                 `yaml:"additional_context,omitempty" json:"additional_context,omitempty"`
}

func (t Template) Validate() error {
	if strings.TrimSpace(t.Key) == "" {
		return fmt.Errorf("template key is required")
	}
	if t.Version <= 0 {
		return fmt.Errorf("template %q version must be positive", t.Key)
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("template %q name is required", t.Key)
	}
	if strings.TrimSpace(t.ShortDescription) == "" {
		return fmt.Errorf("template %q short_description is required", t.Key)
	}
	if err := validateCategories(t.Key, t.Categories); err != nil {
		return err
	}
	mode, err := t.Agent.mode()
	if err != nil {
		return fmt.Errorf("template %q: %w", t.Key, err)
	}
	if err := validateFlowAction(t.Flow.Action); err != nil {
		return fmt.Errorf("template %q: %w", t.Key, err)
	}
	if mode == "none" && t.Flow.Action == model.ActionStartAgentRun {
		return fmt.Errorf("template %q: agent none requires a non-agent action", t.Key)
	}
	if mode != "none" && t.Flow.Action != model.ActionStartAgentRun {
		return fmt.Errorf("template %q: agent mode %s requires start_agent_run", t.Key, mode)
	}
	if mode == "reuse_system" && !knownSystemPreset(t.Agent.ReuseSystem) {
		return fmt.Errorf("template %q: unknown system preset %q", t.Key, t.Agent.ReuseSystem)
	}
	if mode == "create" && strings.TrimSpace(t.Agent.Create.Preset) != "" && !knownSystemPreset(t.Agent.Create.Preset) {
		return fmt.Errorf("template %q: unknown create preset %q", t.Key, t.Agent.Create.Preset)
	}
	if err := validateInputs(t.Key, t.Inputs); err != nil {
		return err
	}
	if strings.TrimSpace(t.Trigger.Type) == "" {
		return fmt.Errorf("template %q trigger.type is required", t.Key)
	}
	if strings.TrimSpace(t.Trigger.Event) == "" && t.Trigger.Type != model.TriggerCron {
		return fmt.Errorf("template %q trigger.event is required", t.Key)
	}
	return nil
}

func (a AgentSpec) mode() (string, error) {
	count := 0
	mode := ""
	if a.Create != nil {
		count++
		mode = "create"
	}
	if strings.TrimSpace(a.ReuseSystem) != "" {
		count++
		mode = "reuse_system"
	}
	if a.PickExisting != nil {
		count++
		mode = "pick_existing"
	}
	if a.None {
		count++
		mode = "none"
	}
	if count != 1 {
		return "", fmt.Errorf("exactly one agent mode must be set")
	}
	return mode, nil
}

func validateCategories(templateKey string, categories []string) error {
	for _, category := range categories {
		if !knownCategory(category) {
			return fmt.Errorf("template %q: unknown category %q", templateKey, category)
		}
	}
	return nil
}

func knownCategory(category string) bool {
	switch strings.TrimSpace(category) {
	case "", "engineering", "sales", "support", "marketing", "docs", "workflow":
		return true
	default:
		return false
	}
}

func validateFlowAction(action string) error {
	switch strings.TrimSpace(action) {
	case model.ActionStartAgentRun, model.ActionMoveToState, model.ActionMergeBranch, model.ActionRunCommand:
		return nil
	default:
		return fmt.Errorf("unsupported flow action %q", action)
	}
}

func validateInputs(templateKey string, inputs []Input) error {
	seen := map[string]bool{}
	for _, input := range inputs {
		key := strings.TrimSpace(input.Key)
		if key == "" {
			return fmt.Errorf("template %q input key is required", templateKey)
		}
		if seen[key] {
			return fmt.Errorf("template %q duplicate input key %q", templateKey, key)
		}
		seen[key] = true
		if strings.TrimSpace(input.Type) == "" {
			return fmt.Errorf("template %q input %q type is required", templateKey, key)
		}
		if strings.TrimSpace(input.Label) == "" {
			return fmt.Errorf("template %q input %q label is required", templateKey, key)
		}
	}
	return nil
}

func knownSystemPreset(preset string) bool {
	switch strings.TrimSpace(preset) {
	case model.AgentPresetEpicPlanner,
		model.AgentPresetTaskPlanner,
		model.AgentPresetCRMOperator,
		model.AgentPresetSupportAgent,
		model.AgentPresetDocumentationAgent,
		model.AgentPresetMarketer,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
		model.AgentPresetResearcher:
		return true
	default:
		return false
	}
}
