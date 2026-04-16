package worker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	skillspkg "github.com/helpin-ai/helpin/server/skills"
)

func CopyBuiltInSkillPackageToDir(key, destDir string) error {
	definition, ok := GetBuiltInSkill(strings.TrimSpace(key))
	if !ok {
		return fmt.Errorf("unknown built-in skill %q", key)
	}
	packagePath := strings.TrimSpace(definition.PackagePath)
	if packagePath == "" {
		return fmt.Errorf("built-in skill %q does not declare a package path", key)
	}
	return copyEmbeddedSkillPackage(skillspkg.BuiltIn, packagePath, destDir)
}

func copyEmbeddedSkillPackage(skillFS fs.FS, packagePath, destDir string) error {
	if strings.TrimSpace(destDir) == "" {
		return fmt.Errorf("destination directory is required")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	return fs.WalkDir(skillFS, packagePath, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath := strings.TrimPrefix(strings.TrimPrefix(currentPath, packagePath), "/")
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(destDir, filepath.FromSlash(relativePath))
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create skill directory %q: %w", relativePath, err)
			}
			return nil
		}
		payload, err := fs.ReadFile(skillFS, currentPath)
		if err != nil {
			return fmt.Errorf("read embedded skill file %q: %w", currentPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create skill parent directory %q: %w", relativePath, err)
		}
		if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
			return fmt.Errorf("write skill file %q: %w", relativePath, err)
		}
		return nil
	})
}

func SyncRuntimeSkillRoot(srcRoot, destRoot string) error {
	srcRoot = strings.TrimSpace(srcRoot)
	destRoot = strings.TrimSpace(destRoot)
	if srcRoot == "" || destRoot == "" {
		return nil
	}
	if err := os.RemoveAll(destRoot); err != nil {
		return fmt.Errorf("clear destination skill root: %w", err)
	}
	return filepath.Walk(srcRoot, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(srcRoot, currentPath)
		if err != nil {
			return fmt.Errorf("resolve staged skill path: %w", err)
		}
		if relativePath == "." {
			return os.MkdirAll(destRoot, 0o755)
		}
		targetPath := filepath.Join(destRoot, relativePath)
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create staged skill directory %q: %w", relativePath, err)
			}
			return nil
		}
		payload, err := os.ReadFile(currentPath)
		if err != nil {
			return fmt.Errorf("read staged skill file %q: %w", relativePath, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create staged skill parent directory %q: %w", relativePath, err)
		}
		if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
			return fmt.Errorf("write staged skill file %q: %w", relativePath, err)
		}
		return nil
	})
}
