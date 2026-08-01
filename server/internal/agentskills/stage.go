package agentskills

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type SkillPackageStore interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
}

var stagedSkillNamePattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func EffectiveRuntimeRefs(agent *model.Agent) model.AgentSkillRefs {
	if agent == nil {
		return nil
	}
	explicitRefs := agent.Skills.Normalize()
	if shouldUseBuiltInPresetSkillDefaults(agent) {
		if bundle, ok := worker.BuiltInPresetSkillBundleForPreset(agent.EffectivePresetKey()); ok {
			if len(explicitRefs) > 0 {
				return mergeRequiredBuiltInCoreSkillRefs(bundle.CoreSkillKeys, explicitRefs)
			}
			runtimeSkillKeys := worker.RuntimeSkillKeysForPresetBundle(bundle)
			refs := make(model.AgentSkillRefs, 0, len(runtimeSkillKeys))
			for _, key := range runtimeSkillKeys {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				refs = append(refs, model.AgentSkillRef{Key: key})
			}
			return refs
		}
	}
	return explicitRefs
}

func mergeRequiredBuiltInCoreSkillRefs(coreSkillKeys []string, explicitRefs model.AgentSkillRefs) model.AgentSkillRefs {
	if len(coreSkillKeys) == 0 {
		return explicitRefs
	}

	refs := make(model.AgentSkillRefs, 0, len(coreSkillKeys)+len(explicitRefs))
	usedExplicitRefs := make([]bool, len(explicitRefs))
	for _, coreKey := range coreSkillKeys {
		coreKey = worker.CanonicalBuiltInSkillKey(coreKey)
		if coreKey == "" {
			continue
		}

		matchedIndex := -1
		for index, ref := range explicitRefs {
			// A workspace skill with the same key must not replace a required,
			// product-owned preset skill. It remains available as an additional
			// explicit skill below.
			if usedExplicitRefs[index] || ref.SkillID != nil {
				continue
			}
			if worker.CanonicalBuiltInSkillKey(ref.Key) == coreKey {
				matchedIndex = index
				break
			}
		}
		if matchedIndex >= 0 {
			refs = append(refs, explicitRefs[matchedIndex])
			usedExplicitRefs[matchedIndex] = true
			continue
		}
		refs = append(refs, model.AgentSkillRef{Key: coreKey})
	}

	for index, ref := range explicitRefs {
		if !usedExplicitRefs[index] {
			refs = append(refs, ref)
		}
	}
	return refs
}

func shouldUseBuiltInPresetSkillDefaults(agent *model.Agent) bool {
	if agent == nil || !agent.IsSystem {
		return false
	}
	versionKey := strings.TrimSpace(agent.EffectivePresetVersionKey())
	if versionKey == "" {
		return true
	}
	return versionKey == defaultBuiltInPresetVersionKey(agent.EffectivePresetKey())
}

func defaultBuiltInPresetVersionKey(presetKey string) string {
	switch strings.TrimSpace(presetKey) {
	case model.AgentPresetEpicPlanner:
		return "epic_planner_default"
	case model.AgentPresetTaskPlanner:
		return "task_planner_default"
	case model.AgentPresetCRMOperator:
		return "crm_operator_default"
	case model.AgentPresetSupportAgent:
		return "support_agent_default"
	case model.AgentPresetDocumentationAgent:
		return "documentation_agent_default"
	case model.AgentPresetMarketer:
		return "marketer_default"
	case model.AgentPresetCodeBuilder:
		return "code_builder_default"
	case model.AgentPresetReviewAgent:
		return "review_agent_default"
	case model.AgentPresetCommandAgent:
		return "command_agent_default"
	default:
		return ""
	}
}

func StageInto(
	ctx context.Context,
	workspaceID string,
	agent *model.Agent,
	allowedTools []string,
	lookup WorkspaceSkillLookup,
	store SkillPackageStore,
	destRoot string,
) (Resolution, error) {
	refs := EffectiveRuntimeRefs(agent)
	if len(refs) == 0 {
		return Resolution{Refs: model.AgentSkillRefs{}, Definitions: nil}, nil
	}

	resolution, err := Resolve(ctx, workspaceID, refs, lookup)
	if err != nil {
		return Resolution{}, err
	}
	runtimeKind := ""
	if agent != nil {
		runtimeKind = strings.TrimSpace(agent.RuntimeKind)
	}
	if err := ValidateRuntimeAndTools(runtimeKind, allowedTools, resolution.Definitions); err != nil {
		return Resolution{}, err
	}
	if strings.TrimSpace(destRoot) == "" {
		return Resolution{}, fmt.Errorf("skill staging root is required")
	}
	if err := os.RemoveAll(destRoot); err != nil {
		return Resolution{}, fmt.Errorf("clear skill staging root: %w", err)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return Resolution{}, fmt.Errorf("create skill staging root: %w", err)
	}

	for index, ref := range resolution.Refs {
		definition := resolution.Definitions[index]
		stageDir := filepath.Join(destRoot, stagedSkillDirName(index, definition.Key))
		switch {
		case ref.SkillID != nil:
			if lookup == nil {
				return Resolution{}, fmt.Errorf("workspace skill lookup is not configured")
			}
			if store == nil {
				return Resolution{}, fmt.Errorf("skill package store is not configured")
			}
			skill, err := lookup.GetByID(ctx, workspaceID, *ref.SkillID)
			if err != nil {
				return Resolution{}, err
			}
			if skill == nil || skill.IsArchived {
				return Resolution{}, fmt.Errorf("workspace skill not found")
			}
			if ref.VersionKey != nil && strings.TrimSpace(*ref.VersionKey) != "" && strings.TrimSpace(*ref.VersionKey) != strings.TrimSpace(skill.VersionKey) {
				return Resolution{}, fmt.Errorf("workspace skill %q version mismatch", skill.Key)
			}
			archiveData, err := store.GetObject(ctx, skill.PackageObjectKey)
			if err != nil {
				return Resolution{}, fmt.Errorf("load workspace skill package %q: %w", skill.Key, err)
			}
			if err := worker.ExtractSkillArchiveToDir(archiveData, stageDir); err != nil {
				return Resolution{}, fmt.Errorf("stage workspace skill %q: %w", skill.Key, err)
			}
		default:
			if err := worker.CopyBuiltInSkillPackageToDir(definition.Key, stageDir); err != nil {
				return Resolution{}, fmt.Errorf("stage built-in skill %q: %w", definition.Key, err)
			}
		}
		if err := rewriteStagedSkillRuntimeToolNames(stageDir); err != nil {
			return Resolution{}, err
		}
	}

	return resolution, nil
}

func rewriteStagedSkillRuntimeToolNames(stageDir string) error {
	return filepath.WalkDir(stageDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read staged skill markdown %q: %w", path, err)
		}
		rendered := worker.RenderRuntimeToolNamesInInstructions(string(payload))
		if rendered == string(payload) {
			return nil
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("write staged skill markdown %q: %w", path, err)
		}
		return nil
	})
}

func stagedSkillDirName(index int, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "skill"
	}
	key = stagedSkillNamePattern.ReplaceAllString(key, "_")
	key = strings.Trim(key, "_")
	if key == "" {
		key = "skill"
	}
	return fmt.Sprintf("%02d-%s", index+1, key)
}
