package agentcontract

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type SkillDefinition struct {
	Key               string
	Title             string
	Description       string
	SourceKind        string
	PackagePath       string
	Instructions      string
	RequiredTools     []string
	SupportedRuntimes []string
	Policy            SkillPolicy
	Interface         SkillInterface
}

type SkillInterface struct {
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
	IconSmall        string `yaml:"icon_small"`
	IconLarge        string `yaml:"icon_large"`
	BrandColor       string `yaml:"brand_color"`
	DefaultPrompt    string `yaml:"default_prompt"`
}

type SkillPolicy struct {
	AllowImplicitInvocation            *bool                      `yaml:"allow_implicit_invocation,omitempty"`
	CompletionRequiresInteractionKinds []string                   `yaml:"completion_requires_interaction_kinds,omitempty"`
	InteractionContracts               []SkillInteractionContract `yaml:"interaction_contracts,omitempty"`
}

type SkillInteractionContract struct {
	Kind       string                               `yaml:"kind,omitempty"`
	Schema     string                               `yaml:"schema,omitempty"`
	Transports map[string]SkillInteractionTransport `yaml:"transports,omitempty"`
}

type SkillInteractionTransport struct {
	Type       string `yaml:"type,omitempty"`
	ToolName   string `yaml:"tool_name,omitempty"`
	BlockLabel string `yaml:"block_label,omitempty"`
}

type SkillDependency struct {
	Type        string `yaml:"type"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Transport   string `yaml:"transport,omitempty"`
	URL         string `yaml:"url,omitempty"`
}

type skillOpenAIConfig struct {
	Interface    SkillInterface `yaml:"interface"`
	Policy       SkillPolicy    `yaml:"policy"`
	Dependencies struct {
		Tools []SkillDependency `yaml:"tools,omitempty"`
	} `yaml:"dependencies"`
}

type skillFrontmatter struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Metadata    skillFrontmatterDetails `yaml:"metadata"`
}

type skillFrontmatterDetails struct {
	Title             string   `yaml:"title"`
	RequiredTools     []string `yaml:"required_tools"`
	SupportedRuntimes []string `yaml:"supported_runtimes"`
}

func LoadBuiltInSkills(skillFS fs.FS, root string) ([]SkillDefinition, error) {
	entries, err := fs.ReadDir(skillFS, root)
	if err != nil {
		return nil, fmt.Errorf("read built-in skill root %q: %w", root, err)
	}

	out := make([]SkillDefinition, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skill, err := loadSkillPackage(skillFS, path.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return out, nil
}

func loadSkillPackage(skillFS fs.FS, packagePath string) (SkillDefinition, error) {
	skillDocPath := path.Join(packagePath, "SKILL.md")
	payload, err := fs.ReadFile(skillFS, skillDocPath)
	if err != nil {
		return SkillDefinition{}, fmt.Errorf("read %q: %w", skillDocPath, err)
	}

	frontmatter, body, err := parseSkillMarkdown(payload)
	if err != nil {
		return SkillDefinition{}, fmt.Errorf("parse %q: %w", skillDocPath, err)
	}

	openAIConfig, err := loadSkillOpenAIConfig(skillFS, path.Join(packagePath, "agents", "openai.yaml"))
	if err != nil {
		return SkillDefinition{}, err
	}

	key := strings.TrimSpace(frontmatter.Name)
	if key == "" {
		key = path.Base(packagePath)
	}

	definition := SkillDefinition{
		Key:               key,
		Title:             deriveSkillTitle(frontmatter, openAIConfig, key),
		Description:       deriveSkillDescription(frontmatter, openAIConfig),
		SourceKind:        "built_in",
		PackagePath:       packagePath,
		Instructions:      strings.TrimSpace(body),
		RequiredTools:     append([]string(nil), frontmatter.Metadata.RequiredTools...),
		SupportedRuntimes: append([]string(nil), frontmatter.Metadata.SupportedRuntimes...),
		Policy:            openAIConfig.Policy,
		Interface:         openAIConfig.Interface,
	}

	if definition.Description == "" {
		return SkillDefinition{}, fmt.Errorf("skill %q is missing a description", packagePath)
	}
	if definition.Instructions == "" {
		return SkillDefinition{}, fmt.Errorf("skill %q has empty instructions", packagePath)
	}

	return definition, nil
}

func loadSkillOpenAIConfig(skillFS fs.FS, configPath string) (skillOpenAIConfig, error) {
	payload, err := fs.ReadFile(skillFS, configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return skillOpenAIConfig{}, nil
		}
		return skillOpenAIConfig{}, fmt.Errorf("read %q: %w", configPath, err)
	}
	var config skillOpenAIConfig
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return skillOpenAIConfig{}, fmt.Errorf("parse %q: %w", configPath, err)
	}
	return config, nil
}

func parseSkillMarkdown(payload []byte) (skillFrontmatter, string, error) {
	content := strings.ReplaceAll(string(payload), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return skillFrontmatter{}, "", fmt.Errorf("missing YAML frontmatter")
	}
	bodyStart := strings.Index(content[len("---\n"):], "\n---\n")
	if bodyStart < 0 {
		return skillFrontmatter{}, "", fmt.Errorf("unterminated YAML frontmatter")
	}
	bodyStart += len("---\n")
	fmBlock := content[len("---\n"):bodyStart]
	body := strings.TrimSpace(content[bodyStart+len("\n---\n"):])

	var frontmatter skillFrontmatter
	if err := yaml.Unmarshal([]byte(fmBlock), &frontmatter); err != nil {
		return skillFrontmatter{}, "", err
	}
	frontmatter.Name = strings.TrimSpace(frontmatter.Name)
	frontmatter.Description = strings.TrimSpace(frontmatter.Description)
	frontmatter.Metadata.Title = strings.TrimSpace(frontmatter.Metadata.Title)
	if frontmatter.Name == "" {
		return skillFrontmatter{}, "", fmt.Errorf("frontmatter field \"name\" is required")
	}
	if frontmatter.Description == "" {
		return skillFrontmatter{}, "", fmt.Errorf("frontmatter field \"description\" is required")
	}
	return frontmatter, body, nil
}

func deriveSkillTitle(frontmatter skillFrontmatter, openAIConfig skillOpenAIConfig, key string) string {
	if title := strings.TrimSpace(openAIConfig.Interface.DisplayName); title != "" {
		return title
	}
	if title := strings.TrimSpace(frontmatter.Metadata.Title); title != "" {
		return title
	}
	return strings.TrimSpace(strings.ReplaceAll(key, "_", " "))
}

func deriveSkillDescription(frontmatter skillFrontmatter, openAIConfig skillOpenAIConfig) string {
	if description := strings.TrimSpace(openAIConfig.Interface.ShortDescription); description != "" {
		return description
	}
	return strings.TrimSpace(frontmatter.Description)
}
