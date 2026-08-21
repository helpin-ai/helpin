package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionLLMCallsUseAICompleter(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	for _, filename := range files {
		if strings.HasSuffix(filename, "_test.go") || filename == "ai_completion.go" || filename == "ai_usage_meter.go" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "ChatCompletion" {
				violations = append(violations, filename)
			}
			return true
		})
	}
	if len(violations) != 0 {
		t.Fatalf("production services call ChatCompletion directly: %v", violations)
	}
}
