package agentcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	skillspkg "github.com/helpin-ai/helpin/server/skills"
)

// ErrCRMPlaybookJourneyUnsupported rejects implicit fallback to an unrelated job.
var ErrCRMPlaybookJourneyUnsupported = errors.New("playbook journey has no supported Beacon skill")

// CaptureCRMPlaybookSpecialization selects and freezes only Beacon's core and relevant job.
// It does not modify preset defaults, discover workspace skills, or grant tools.
func CaptureCRMPlaybookSpecialization(journey string) (model.CRMPlaybookSpecialization, error) {
	jobKey, ok := crmPlaybookJobKey(journey)
	if !ok {
		return model.CRMPlaybookSpecialization{}, ErrCRMPlaybookJourneyUnsupported
	}
	core, ok := GetBuiltInSkill("crm_record_operations")
	if !ok {
		return model.CRMPlaybookSpecialization{}, errors.New("Beacon core skill unavailable")
	}
	coreSnapshot, err := captureCRMPlaybookSkill(skillspkg.BuiltIn, core.PackagePath, "core")
	if err != nil {
		return model.CRMPlaybookSpecialization{}, err
	}
	jobSnapshot, err := captureCRMPlaybookSkill(skillspkg.CRMPlaybooks,
		path.Join(skillspkg.CRMPlaybookRoot, journey), "job")
	if err != nil {
		return model.CRMPlaybookSpecialization{}, err
	}
	if jobSnapshot.Key != jobKey {
		return model.CRMPlaybookSpecialization{}, errors.New("Playbook skill identity mismatch")
	}
	prompt := BuiltInPresetPrompt(model.AgentPresetCRMOperator)
	if prompt == nil || strings.TrimSpace(*prompt) == "" {
		return model.CRMPlaybookSpecialization{}, errors.New("Beacon prompt unavailable")
	}
	snapshot := model.CRMPlaybookSpecialization{
		SchemaVersion: 1, PresetKey: model.AgentPresetCRMOperator, PresetPrompt: *prompt,
		Journey: journey, Skills: []model.CRMPlaybookSkillSnapshot{coreSnapshot, jobSnapshot},
	}
	snapshot.Version, err = crmSpecializationVersion(snapshot)
	if err != nil {
		return model.CRMPlaybookSpecialization{}, err
	}
	return snapshot, ValidateCRMPlaybookSpecialization(snapshot)
}

// CaptureCRMPlaybookSpecializationForPrompt freezes the saved Agent's effective prompt.
// It is a publication helper, not an API for replacing the shared Beacon preset.
func CaptureCRMPlaybookSpecializationForPrompt(journey, prompt string) (model.CRMPlaybookSpecialization, error) {
	snapshot, err := CaptureCRMPlaybookSpecialization(journey)
	if err != nil {
		return snapshot, err
	}
	snapshot.PresetPrompt = prompt
	snapshot.Version, err = crmSpecializationVersion(snapshot)
	if err != nil {
		return snapshot, err
	}
	return snapshot, ValidateCRMPlaybookSpecialization(snapshot)
}

// ValidateCRMPlaybookSpecialization checks captured contents without consulting today's
// skill definitions. A digest is an integrity check, not proof of user authorization;
// future execution must load published snapshots from trusted workspace storage.
func ValidateCRMPlaybookSpecialization(snapshot model.CRMPlaybookSpecialization) error {
	jobKey, supported := crmPlaybookJobKey(snapshot.Journey)
	if !supported || snapshot.SchemaVersion != 1 || snapshot.PresetKey != model.AgentPresetCRMOperator ||
		strings.TrimSpace(snapshot.PresetPrompt) == "" || len(snapshot.PresetPrompt) > 128*1024 ||
		len(snapshot.Skills) != 2 || !validCRMContentVersion(snapshot.Version) {
		return errors.New("invalid Playbook specialization")
	}
	if snapshot.Skills[0].Key != "crm_record_operations" || snapshot.Skills[0].Role != "core" ||
		snapshot.Skills[1].Key != jobKey || snapshot.Skills[1].Role != "job" {
		return errors.New("incorrect Playbook skill selection")
	}
	for _, skill := range snapshot.Skills {
		if err := validateCRMPlaybookSkill(skill); err != nil {
			return err
		}
	}
	version, err := crmSpecializationVersion(snapshot)
	if err != nil {
		return err
	}
	if version != snapshot.Version {
		return errors.New("Playbook specialization content changed")
	}
	return nil
}

func crmPlaybookJobKey(journey string) (string, bool) {
	switch journey {
	case "buying_intent":
		return "crm_buying_intent_follow_up", true
	case "sales_handoff":
		return "crm_sales_success_handoff", true
	case "renewal_recovery":
		return "crm_renewal_risk_recovery", true
	default:
		return "", false
	}
}

func captureCRMPlaybookSkill(source fs.FS, root, role string) (model.CRMPlaybookSkillSnapshot, error) {
	definition, err := loadSkillPackage(source, root)
	if err != nil {
		return model.CRMPlaybookSkillSnapshot{}, err
	}
	snapshot := model.CRMPlaybookSkillSnapshot{Key: definition.Key, Title: definition.Title, Role: role}
	err = fs.WalkDir(source, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("skill package contains a non-regular file")
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return fmt.Errorf("read Playbook skill resource: %w", err)
		}
		snapshot.Files = append(snapshot.Files, model.CRMPlaybookSkillFile{
			Path: strings.TrimPrefix(name, root+"/"), Data: data,
		})
		return nil
	})
	if err != nil {
		return model.CRMPlaybookSkillSnapshot{}, err
	}
	snapshot.Version, err = crmContentVersion(snapshot.Files)
	return snapshot, err
}

func validateCRMPlaybookSkill(skill model.CRMPlaybookSkillSnapshot) error {
	if len(skill.Files) == 0 || len(skill.Files) > 64 || !validCRMContentVersion(skill.Version) {
		return errors.New("invalid Playbook skill package")
	}
	files := make(skillArchiveFS, len(skill.Files))
	previous, size := "", 0
	for _, file := range skill.Files {
		size += len(file.Data)
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.Contains(file.Path, "\\") ||
			file.Path <= previous || size > 2*1024*1024 {
			return errors.New("invalid Playbook skill resource")
		}
		previous = file.Path
		files[file.Path] = file.Data
	}
	definition, err := loadSkillPackage(files, "")
	if err != nil {
		return fmt.Errorf("validate captured Playbook skill: %w", err)
	}
	if definition.Key != skill.Key || definition.Title != skill.Title {
		return errors.New("captured Playbook skill metadata changed")
	}
	version, err := crmContentVersion(skill.Files)
	if err != nil {
		return err
	}
	if version != skill.Version {
		return errors.New("Playbook skill content changed")
	}
	return nil
}

func crmSpecializationVersion(snapshot model.CRMPlaybookSpecialization) (string, error) {
	snapshot.Version = ""
	return crmContentVersion(snapshot)
}

func crmContentVersion(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode Playbook specialization: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func validCRMContentVersion(version string) bool {
	if len(version) != 64 || strings.ToLower(version) != version {
		return false
	}
	_, err := hex.DecodeString(version)
	return err == nil
}
