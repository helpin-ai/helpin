package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default command whitelist. Can be overridden via WORKFLOW.md.
var defaultAllowedCommands = map[string]bool{
	"go":     true,
	"npm":    true,
	"npx":    true,
	"node":   true,
	"make":   true,
	"git":    true,
	"ls":     true,
	"cat":    true,
	"grep":   true,
	"find":   true,
	"head":   true,
	"tail":   true,
	"wc":     true,
	"diff":   true,
	"echo":   true,
	"mkdir":  true,
	"cp":     true,
	"mv":     true,
	"rm":     true,
	"pwd":    true,
	"python": true,
	"pip":    true,
	"cargo":  true,
	"rustc":  true,
}

func toolRunCommand(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Command string   `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	program, args, err := normalizeCommand(params.Program, params.Args, params.Command)
	if err != nil {
		return "", err
	}

	// Extract first word to check against whitelist.
	base := program
	// Handle path prefixes like ./node_modules/.bin/...
	if strings.Contains(base, "/") {
		base = base[strings.LastIndex(base, "/")+1:]
	}

	allowedListValues := allowedCommandsFor(ctx.RuntimeProfile, ctx.Config)
	allowed := make(map[string]bool, len(allowedListValues))
	switch {
	case len(ctx.RuntimeProfile.AllowedCommands) == 0:
		// No commands are permitted for this capability profile.
	case len(allowedListValues) == 0 && ctx.Config == nil:
		for cmdName := range defaultAllowedCommands {
			allowed[cmdName] = true
		}
	default:
		for _, cmdName := range allowedListValues {
			allowed[cmdName] = true
		}
	}

	if !allowed[base] {
		return "", fmt.Errorf("command %q is not allowed; allowed: %v", base, allowedList(allowed))
	}

	timeout := 2 * time.Minute
	if ctx.Config != nil && ctx.Config.CommandTimeout > 0 {
		timeout = ctx.Config.CommandTimeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx.Context, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, program, args...)
	cmd.Dir = ctx.WorkDir

	output, err := cmd.CombinedOutput()
	result := string(output)
	if len(result) > 50_000 {
		result = result[:50_000] + "\n... (truncated)"
	}

	if err != nil {
		return fmt.Sprintf("Exit code: %v\n%s", err, result), nil
	}

	return result, nil
}

func allowedList(m map[string]bool) []string {
	var list []string
	for k := range m {
		list = append(list, k)
	}
	return list
}

func normalizeCommand(program string, args []string, command string) (string, []string, error) {
	if strings.TrimSpace(program) != "" {
		return strings.TrimSpace(program), args, nil
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, fmt.Errorf("program is required")
	}
	if strings.ContainsAny(command, "|&;<>()`$") {
		return "", nil, fmt.Errorf("shell operators are not allowed; use program + args")
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("program is required")
	}
	return parts[0], parts[1:], nil
}
