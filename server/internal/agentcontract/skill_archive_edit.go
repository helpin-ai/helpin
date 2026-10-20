package agentcontract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	skillspkg "github.com/helpin-ai/helpin/server/skills"
	"gopkg.in/yaml.v3"
)

// BuildBuiltInSkillArchive includes supporting resources, not just SKILL.md.
func BuildBuiltInSkillArchive(def SkillDefinition) ([]byte, string, string, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	err := fs.WalkDir(skillspkg.BuiltIn, def.PackagePath, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(skillspkg.BuiltIn, name)
		if err != nil {
			return err
		}
		return writeZipFile(zw, path.Join(def.Key, strings.TrimPrefix(name, def.PackagePath+"/")), string(content))
	})
	if err != nil {
		return nil, "", "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", "", err
	}
	return out.Bytes(), SkillVersionForBytes(out.Bytes()), def.Key + ".zip", nil
}

// EditSkillArchive preserves resources and provider configuration while replacing
// the editable skill definition. Archive validation is shared with imports.
func EditSkillArchive(original []byte, def SkillDefinition) ([]byte, string, string, error) {
	if _, err := LoadSkillArchive(original, def.SourceKind); err != nil {
		return nil, "", "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		return nil, "", "", err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, "", "", err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, "", "", err
		}
		if path.Base(file.Name) == "SKILL.md" {
			content, err = editSkillMarkdown(content, def)
			if err != nil {
				return nil, "", "", err
			}
		}
		if strings.HasSuffix(file.Name, "agents/openai.yaml") {
			var config map[string]any
			if err := yaml.Unmarshal(content, &config); err != nil {
				return nil, "", "", err
			}
			if ui, ok := config["interface"].(map[string]any); ok {
				if _, ok := ui["display_name"]; ok {
					ui["display_name"] = def.Title
				}
				if _, ok := ui["short_description"]; ok {
					ui["short_description"] = def.Description
				}
			}
			content, err = yaml.Marshal(config)
			if err != nil {
				return nil, "", "", err
			}
		}
		header := file.FileHeader
		writer, err := zw.CreateHeader(&header)
		if err != nil {
			return nil, "", "", err
		}
		if _, err := writer.Write(content); err != nil {
			return nil, "", "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", "", fmt.Errorf("close skill archive: %w", err)
	}
	return out.Bytes(), SkillVersionForBytes(out.Bytes()), def.Key + ".zip", nil
}

func editSkillMarkdown(original []byte, def SkillDefinition) ([]byte, error) {
	content := strings.ReplaceAll(string(original), "\r\n", "\n")
	end := strings.Index(content[len("---\n"):], "\n---\n") + len("---\n")
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(content[len("---\n"):end]), &fields); err != nil {
		return nil, err
	}
	fields["name"], fields["description"] = def.Key, def.Description
	metadata, ok := fields["metadata"].(map[string]any)
	if !ok {
		metadata = map[string]any{}
		fields["metadata"] = metadata
	}
	metadata["title"], metadata["required_tools"], metadata["supported_runtimes"] = def.Title, def.RequiredTools, def.SupportedRuntimes
	frontmatter, err := yaml.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("---\n%s---\n\n%s\n", frontmatter, strings.TrimSpace(def.Instructions))), nil
}
