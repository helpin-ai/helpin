package temporalapp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) buildPlanningCodeContext(ctx context.Context, state *resolvedRunState, specText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare planning repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 3, 80)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "pnpm-workspace.yaml", "turbo.json", "tsconfig.json",
		"go.mod", "go.work", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 6, 1200)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, specText)
	if err != nil {
		return "", err
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, 8, 1400)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "Repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key manifests and architecture anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Relevant implementation files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Relevant implementation files: no confident file matches were found from the approved spec. Treat uncertain areas as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func (a *AgentRunActivities) buildDraftSpecCodeContext(ctx context.Context, state *resolvedRunState, seedText string) (string, error) {
	if state.repository == nil || state.integration == nil {
		return "", nil
	}

	workDir, err := workerpkg.PrepareWorkspace(ctx, state.integration, state.repository.FullName, state.accessToken)
	if err != nil {
		return "", fmt.Errorf("prepare draft-spec repository workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	treeEntries, err := collectPlanningTree(workDir, 2, 50)
	if err != nil {
		return "", err
	}

	manifestCandidates := []string{
		"package.json", "go.mod", "go.work", "Cargo.toml", "pyproject.toml",
		"README.md", "WORKFLOW.md",
	}
	manifestSnippets := collectStaticFileSnippets(workDir, manifestCandidates, 4, 900)

	relevantFiles, err := selectRelevantPlanningFiles(workDir, seedText)
	if err != nil {
		return "", err
	}
	fileLimit := 5
	if len(relevantFiles) > fileLimit {
		relevantFiles = relevantFiles[:fileLimit]
	}
	fileSnippets := collectStaticFileSnippets(workDir, relevantFiles, fileLimit, 900)

	var sections []string
	sections = append(sections, fmt.Sprintf("Repository root: %s", state.repository.FullName))
	if len(treeEntries) > 0 {
		sections = append(sections, "High-level repository structure:\n"+strings.Join(treeEntries, "\n"))
	}
	if len(manifestSnippets) > 0 {
		sections = append(sections, "Key product and platform anchors:\n"+strings.Join(manifestSnippets, "\n\n"))
	}
	if len(fileSnippets) > 0 {
		sections = append(sections, "Likely relevant product surface files:\n"+strings.Join(fileSnippets, "\n\n"))
	} else {
		sections = append(sections, "Likely relevant product surface files: no confident matches were found. Treat repo-specific assumptions as risks or open questions.")
	}

	return strings.Join(sections, "\n\n"), nil
}

func collectPlanningTree(root string, maxDepth, maxEntries int) ([]string, error) {
	type item struct {
		path  string
		depth int
	}
	var items []item
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if shouldSkipPlanningPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth >= maxDepth && d.IsDir() {
			items = append(items, item{path: rel + "/", depth: depth})
			return filepath.SkipDir
		}
		if len(items) >= maxEntries {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		label := rel
		if d.IsDir() {
			label += "/"
		}
		items = append(items, item{path: label, depth: depth})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprintf("%s%s", strings.Repeat("  ", item.depth), item.path))
	}
	return out, nil
}

func collectStaticFileSnippets(root string, candidates []string, limit, maxChars int) []string {
	results := make([]string, 0, limit)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		fullPath := filepath.Join(root, candidate)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		results = append(results, fmt.Sprintf("[%s]\n%s", candidate, truncatePlanningText(string(content), maxChars)))
		if len(results) >= limit {
			break
		}
	}
	return results
}

func selectRelevantPlanningFiles(root, specText string) ([]string, error) {
	keywords := planningKeywords(specText)
	type candidate struct {
		path  string
		score int
	}
	candidates := make([]candidate, 0, 64)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || shouldSkipPlanningPath(rel) || !looksLikePlanningSourceFile(rel) {
			return nil
		}

		score := planningFileBaseScore(rel)
		lowerRel := strings.ToLower(rel)
		for _, keyword := range keywords {
			if strings.Contains(lowerRel, keyword) {
				score += 8
			}
		}
		if score <= 0 {
			return nil
		}
		candidates = append(candidates, candidate{path: rel, score: score})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].score > candidates[j].score
	})

	limit := 8
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for _, item := range candidates {
		if seen[item.path] {
			continue
		}
		seen[item.path] = true
		out = append(out, item.path)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func planningKeywords(specText string) []string {
	normalized := strings.ToLower(specText)
	replacer := strings.NewReplacer(
		"\n", " ", "\t", " ", ",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ",
		"{", " ", "}", " ", "[", " ", "]", " ", "/", " ", "\\", " ", "-", " ", "_", " ",
	)
	normalized = replacer.Replace(normalized)
	words := strings.Fields(normalized)
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true,
		"into": true, "will": true, "story": true, "stories": true, "spec": true, "product": true,
		"epic": true, "user": true, "users": true, "should": true, "have": true, "must": true,
		"plan": true, "planning": true, "acceptance": true, "criteria": true, "when": true, "then": true,
		"given": true, "goal": true, "goals": true, "risk": true, "risks": true,
	}
	freq := make(map[string]int)
	for _, word := range words {
		if len(word) < 4 || stop[word] {
			continue
		}
		freq[word]++
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(freq))
	for word, count := range freq {
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	limit := 12
	if len(pairs) < limit {
		limit = len(pairs)
	}
	keywords := make([]string, 0, limit)
	for idx := 0; idx < limit; idx++ {
		keywords = append(keywords, pairs[idx].word)
	}
	return keywords
}

func shouldSkipPlanningPath(rel string) bool {
	rel = filepath.ToSlash(strings.ToLower(rel))
	skipParts := []string{
		".git/", "node_modules/", "vendor/", "dist/", "build/", ".next/", ".turbo/", ".cache/",
		"coverage/", "tmp/", "temp/", "bin/", "public/", "assets/", "storybook-static/",
	}
	for _, part := range skipParts {
		if strings.Contains(rel, part) {
			return true
		}
	}
	return false
}

func looksLikePlanningSourceFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".yaml", ".yml", ".md", ".py", ".rb", ".java", ".kt", ".rs":
		return true
	default:
		return false
	}
}

func planningFileBaseScore(rel string) int {
	lower := filepath.ToSlash(strings.ToLower(rel))
	score := 0
	switch {
	case strings.Contains(lower, "router"), strings.Contains(lower, "routes/"):
		score += 12
	case strings.Contains(lower, "handler"), strings.Contains(lower, "controller"):
		score += 11
	case strings.Contains(lower, "service"):
		score += 10
	case strings.Contains(lower, "model"), strings.Contains(lower, "schema"), strings.Contains(lower, "entity"):
		score += 9
	case strings.Contains(lower, "repository"), strings.Contains(lower, "store"):
		score += 8
	case strings.Contains(lower, "test"):
		score += 6
	case strings.Contains(lower, "component"), strings.Contains(lower, "page"):
		score += 7
	}
	if strings.HasSuffix(lower, "package.json") || strings.HasSuffix(lower, "go.mod") || strings.HasSuffix(lower, "cargo.toml") || strings.HasSuffix(lower, "pyproject.toml") || strings.HasSuffix(lower, "readme.md") {
		score += 14
	}
	return score
}

func truncatePlanningText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... (truncated)"
}
