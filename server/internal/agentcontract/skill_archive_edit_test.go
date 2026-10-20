package agentcontract

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestEditSkillArchivePreservesResourcesAndConfig(t *testing.T) {
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	files := map[string]string{
		"imported/SKILL.md":           "---\nname: imported\ndescription: Original\nlicense: MIT\nmetadata:\n  title: Original\n  custom_field: retain\n---\nOriginal instructions\n",
		"imported/agents/openai.yaml": "interface:\n  display_name: Original\n  short_description: Original\n  icon_small: ./assets/icon.svg\npolicy:\n  allow_implicit_invocation: false\ndependencies:\n  tools:\n    - type: mcp\n      value: example\n",
		"imported/scripts/check.sh":   "#!/bin/sh\necho sample\n",
		"imported/assets/icon.svg":    "<svg />",
	}
	for name, content := range files {
		if err := writeZipFile(zw, name, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	def, err := LoadSkillArchive(buffer.Bytes(), "imported")
	if err != nil {
		t.Fatal(err)
	}
	def.Title, def.Description, def.Instructions = "Edited", "Edited description", "Edited instructions"
	edited, _, _, err := EditSkillArchive(buffer.Bytes(), def)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSkillArchive(edited, "imported")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != def.Title || loaded.Instructions != def.Instructions || loaded.Description != def.Description {
		t.Fatalf("edit not applied: %+v", loaded)
	}
	if loaded.Policy.AllowImplicitInvocation == nil || *loaded.Policy.AllowImplicitInvocation {
		t.Fatal("policy was lost")
	}
	zr, err := zip.NewReader(bytes.NewReader(edited), int64(len(edited)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range zr.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(file.Name, "scripts/") || strings.Contains(file.Name, "assets/") {
			if string(content) != files[file.Name] {
				t.Fatal("resource changed")
			}
		}
		if strings.HasSuffix(file.Name, "SKILL.md") && (!strings.Contains(string(content), "license: MIT") || !strings.Contains(string(content), "custom_field: retain")) {
			t.Fatal("custom frontmatter lost")
		}
		if strings.HasSuffix(file.Name, "openai.yaml") && !strings.Contains(string(content), "value: example") {
			t.Fatal("dependency config lost")
		}
	}
}
