package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed manifests/*/template.yaml
var systemManifestFS embed.FS

type Registry struct {
	byKey map[string]Template
	list  []Template
}

func LoadSystemRegistry() (*Registry, error) {
	return LoadRegistry(systemManifestFS)
}

func LoadRegistry(fsys fs.FS) (*Registry, error) {
	templates := make([]Template, 0)
	err := fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) != "template.yaml" {
			return nil
		}
		payload, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var tmpl Template
		if err := yaml.Unmarshal(payload, &tmpl); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		tmpl.basePath = filepath.Dir(path)
		if err := tmpl.Validate(); err != nil {
			return fmt.Errorf("validate %s: %w", path, err)
		}
		templates = append(templates, tmpl)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Key < templates[j].Key
	})

	byKey := make(map[string]Template, len(templates))
	for _, tmpl := range templates {
		key := strings.TrimSpace(tmpl.Key)
		if _, exists := byKey[key]; exists {
			return nil, fmt.Errorf("duplicate template key %q", key)
		}
		byKey[key] = tmpl
	}

	return &Registry{
		byKey: byKey,
		list:  templates,
	}, nil
}

func (r *Registry) List() []Template {
	if r == nil || len(r.list) == 0 {
		return []Template{}
	}
	out := make([]Template, len(r.list))
	copy(out, r.list)
	return out
}

func (r *Registry) Get(key string) (Template, bool) {
	if r == nil {
		return Template{}, false
	}
	tmpl, ok := r.byKey[strings.TrimSpace(key)]
	return tmpl, ok
}
