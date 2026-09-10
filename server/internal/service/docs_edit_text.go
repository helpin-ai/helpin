package service

import (
	"fmt"
	"sort"
	"strings"
)

type documentTextRun struct {
	nodes []map[string]any
	text  string
}
type documentTextChange struct {
	run, start, end int
	replacement     string
}

func replaceDocumentText(block map[string]any, operations []documentEditOperation) error {
	runs := []documentTextRun{}
	var collect func(map[string]any)
	collect = func(node map[string]any) {
		children, _ := node["content"].([]any)
		run := documentTextRun{}
		flush := func() {
			if len(run.nodes) > 0 {
				runs = append(runs, run)
				run = documentTextRun{}
			}
		}
		for _, child := range children {
			n, ok := child.(map[string]any)
			if !ok {
				flush()
				continue
			}
			if n["type"] == "text" {
				text, _ := n["text"].(string)
				run.nodes = append(run.nodes, n)
				run.text += text
			} else {
				flush()
				collect(n)
			}
		}
		flush()
	}
	collect(block)
	changes := []documentTextChange{}
	for _, op := range operations {
		found := []documentTextChange{}
		for i, run := range runs {
			for offset := 0; offset <= len(run.text); {
				j := strings.Index(run.text[offset:], op.OldText)
				if j < 0 {
					break
				}
				start := offset + j
				found = append(found, documentTextChange{i, start, start + len(op.OldText), *op.NewText})
				offset = start + 1
			}
		}
		if len(found) != 1 {
			return fmt.Errorf("old_text must match exactly once within a paragraph or code block; found %d matches", len(found))
		}
		change := found[0]
		for _, other := range changes {
			if change.run == other.run && change.start < other.end && other.start < change.end {
				return fmt.Errorf("overlapping text replacements")
			}
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].run == changes[j].run {
			return changes[i].start > changes[j].start
		}
		return changes[i].run > changes[j].run
	})
	for _, change := range changes {
		offset := 0
		inserted := false
		for _, node := range runs[change.run].nodes {
			text, _ := node["text"].(string)
			end := offset + len(text)
			if offset < change.end && end > change.start {
				left := max(0, change.start-offset)
				right := min(len(text), change.end-offset)
				replacement := ""
				if !inserted {
					replacement = change.replacement
					inserted = true
				}
				node["text"] = text[:left] + replacement + text[right:]
			}
			offset = end
		}
	}
	removeEmptyDocumentText(block)
	return nil
}

func removeEmptyDocumentText(node map[string]any) {
	children, ok := node["content"].([]any)
	if !ok {
		return
	}
	out := make([]any, 0, len(children))
	for _, child := range children {
		n, ok := child.(map[string]any)
		if ok {
			if n["type"] == "text" && n["text"] == "" {
				continue
			}
			removeEmptyDocumentText(n)
		}
		out = append(out, child)
	}
	node["content"] = out
}
