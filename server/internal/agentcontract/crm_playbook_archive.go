package agentcontract

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// BuildCRMPlaybookSkillArchive preserves every captured resource byte for runtime delivery.
// Unlike the catalogue renderer, it does not reconstruct SKILL.md or drop references.
// The host must authorize a bound publication before serving this package to a run.
func BuildCRMPlaybookSkillArchive(skill model.CRMPlaybookSkillSnapshot) ([]byte, string, error) {
	if !fs.ValidPath(skill.Key) || skill.Key == "." || path.Base(skill.Key) != skill.Key || strings.Contains(skill.Key, "\\") {
		return nil, "", errors.New("invalid Playbook skill archive root")
	}
	if err := validateCRMPlaybookSkill(skill); err != nil {
		return nil, "", err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range skill.Files {
		if err := writeZipFile(writer, path.Join(skill.Key, file.Path), string(file.Data)); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(output.Bytes())
	return output.Bytes(), hex.EncodeToString(digest[:]), nil
}
