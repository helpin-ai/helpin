package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type rolloutRunRow struct {
	RunID       string
	WorkspaceID string
	Status      string
	TargetType  string
	CreatedAt   time.Time
	CompletedAt *time.Time
	IsSystem    bool
	Provider    *string
	Model       *string
	PresetKey   string
}

type rolloutArtifactRow struct {
	RunID         string
	ArtifactType  string
	InlineContent *string
}

type nativeTurnDebugPayload struct {
	RuntimeKind           string   `json:"runtime_kind"`
	PlanningStage         string   `json:"planning_stage,omitempty"`
	ContinuationMode      string   `json:"continuation_mode"`
	ActiveSkillRefs       []string `json:"active_skill_refs,omitempty"`
	RepairGuidancePresent bool     `json:"repair_guidance_present"`
	RepairGuidanceSource  string   `json:"repair_guidance_source,omitempty"`
	RepairGuidanceClass   string   `json:"repair_guidance_class,omitempty"`
}

type appliedPreviewPayload struct {
	ApprovedArtifactID string    `json:"approved_artifact_id"`
	Phase              string    `json:"phase"`
	Action             string    `json:"action"`
	AppliedAt          time.Time `json:"applied_at"`
}

type rolloutSummary struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Filters     rolloutFilters          `json:"filters"`
	Groups      []rolloutProviderReport `json:"groups"`
}

type rolloutProviderReport struct {
	Provider                      string         `json:"provider"`
	Model                         string         `json:"model,omitempty"`
	PresetKey                     string         `json:"preset_key"`
	RunCount                      int            `json:"run_count"`
	CompletedRuns                 int            `json:"completed_runs"`
	FailedRuns                    int            `json:"failed_runs"`
	PausedRuns                    int            `json:"paused_runs"`
	RunningRuns                   int            `json:"running_runs"`
	CancelledRuns                 int            `json:"cancelled_runs"`
	SelectiveEligibleRuns         int            `json:"selective_eligible_runs"`
	SelectiveEligibleMissingDebug int            `json:"selective_eligible_missing_debug"`
	SelectiveValidationStatus     string         `json:"selective_validation_status"`
	AverageNativeDebugTurns       float64        `json:"average_native_debug_turns"`
	RunsWithNativeDebugArtifacts  int            `json:"runs_with_native_debug_artifacts"`
	RunsWithRepairGuidance        int            `json:"runs_with_repair_guidance"`
	ContinuationModes             map[string]int `json:"continuation_modes,omitempty"`
	AppliedActions                map[string]int `json:"applied_actions,omitempty"`
}

type rolloutFilters struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Provider    string `json:"provider,omitempty"`
	PresetKey   string `json:"preset_key,omitempty"`
	Since       string `json:"since,omitempty"`
	Until       string `json:"until,omitempty"`
}

type providerAggregateKey struct {
	Provider  string
	Model     string
	PresetKey string
}

type providerAggregate struct {
	key                           providerAggregateKey
	runCount                      int
	completedRuns                 int
	failedRuns                    int
	pausedRuns                    int
	runningRuns                   int
	cancelledRuns                 int
	selectiveEligibleRuns         int
	selectiveEligibleMissingDebug int
	totalNativeDebugTurns         int
	runsWithNativeDebugArtifacts  int
	runsWithRepairGuidance        int
	continuationModes             map[string]int
	appliedActions                map[string]int
}

type runAggregate struct {
	key               providerAggregateKey
	status            string
	selectiveEligible bool
	nativeDebugTurns  int
	hasNativeDebug    bool
	hasRepairGuidance bool
	continuationModes map[string]int
	appliedActions    map[string]int
}

func main() {
	_ = godotenv.Load()

	var filters rolloutFilters
	var asJSON bool

	flag.StringVar(&filters.WorkspaceID, "workspace", "", "limit to a workspace id")
	flag.StringVar(&filters.Provider, "provider", "", "limit to a provider (anthropic/openai/openrouter)")
	flag.StringVar(&filters.PresetKey, "preset", "", "limit to a preset key")
	flag.StringVar(&filters.Since, "since", "", "limit to runs created at or after RFC3339 timestamp")
	flag.StringVar(&filters.Until, "until", "", "limit to runs created before RFC3339 timestamp")
	flag.BoolVar(&asJSON, "json", false, "emit JSON")
	flag.Parse()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	runs, artifacts, err := loadRolloutRows(ctx, db, filters)
	if err != nil {
		log.Fatalf("load rollout rows: %v", err)
	}

	summary := buildRolloutSummary(filters, runs, artifacts)
	if asJSON {
		output, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			log.Fatalf("marshal summary: %v", err)
		}
		fmt.Println(string(output))
		return
	}
	printRolloutSummary(summary)
}

func loadRolloutRows(ctx context.Context, db *gorm.DB, filters rolloutFilters) ([]rolloutRunRow, []rolloutArtifactRow, error) {
	query := db.WithContext(ctx).
		Table("agent_runs").
		Select("agent_runs.id AS run_id, agent_runs.workspace_id, agent_runs.status, agent_runs.target_type, agent_runs.created_at, agent_runs.completed_at, agents.is_system, agents.provider, agents.model, agents.preset_key").
		Joins("JOIN agents ON agents.id = agent_runs.agent_id AND agents.workspace_id = agent_runs.workspace_id").
		Where("agent_runs.runtime_kind = ?", "native_sdk").
		Where("agents.preset_key IN ?", []string{model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner})

	if value := strings.TrimSpace(filters.WorkspaceID); value != "" {
		query = query.Where("agent_runs.workspace_id = ?", value)
	}
	if value := strings.TrimSpace(filters.Provider); value != "" {
		query = query.Where("agents.provider = ?", value)
	}
	if value := strings.TrimSpace(filters.PresetKey); value != "" {
		query = query.Where("agents.preset_key = ?", value)
	}
	if value := strings.TrimSpace(filters.Since); value != "" {
		since, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, nil, fmt.Errorf("parse --since: %w", err)
		}
		query = query.Where("agent_runs.created_at >= ?", since.UTC())
	}
	if value := strings.TrimSpace(filters.Until); value != "" {
		until, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, nil, fmt.Errorf("parse --until: %w", err)
		}
		query = query.Where("agent_runs.created_at < ?", until.UTC())
	}

	var runs []rolloutRunRow
	if err := query.Order("agent_runs.created_at ASC").Scan(&runs).Error; err != nil {
		return nil, nil, err
	}
	if len(runs) == 0 {
		return nil, nil, nil
	}

	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		runIDs = append(runIDs, run.RunID)
	}

	var artifacts []rolloutArtifactRow
	if err := db.WithContext(ctx).
		Table("agent_run_artifacts").
		Select("run_id, artifact_type, inline_content").
		Where("run_id IN ?", runIDs).
		Where("artifact_type IN ?", []string{model.AgentRunArtifactTypeNativeTurnDebug, model.AgentRunArtifactTypeApprovedPreviewApplied}).
		Order("sequence_no ASC, created_at ASC").
		Scan(&artifacts).Error; err != nil {
		return nil, nil, err
	}

	return runs, artifacts, nil
}

func buildRolloutSummary(filters rolloutFilters, runs []rolloutRunRow, artifacts []rolloutArtifactRow) rolloutSummary {
	runState := make(map[string]*runAggregate, len(runs))
	groupOrder := make([]providerAggregateKey, 0)
	seenGroups := make(map[providerAggregateKey]bool)

	for _, run := range runs {
		key := providerAggregateKey{
			Provider:  normalizedProvider(run.Provider),
			Model:     normalizedOptional(run.Model),
			PresetKey: strings.TrimSpace(run.PresetKey),
		}
		runState[run.RunID] = &runAggregate{
			key:               key,
			status:            strings.TrimSpace(run.Status),
			selectiveEligible: rolloutRunWouldBeSelectiveEligible(run),
			continuationModes: make(map[string]int),
			appliedActions:    make(map[string]int),
		}
		if !seenGroups[key] {
			seenGroups[key] = true
			groupOrder = append(groupOrder, key)
		}
	}

	for _, artifact := range artifacts {
		state := runState[artifact.RunID]
		if state == nil || artifact.InlineContent == nil {
			continue
		}
		switch strings.TrimSpace(artifact.ArtifactType) {
		case model.AgentRunArtifactTypeNativeTurnDebug:
			var payload nativeTurnDebugPayload
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
				continue
			}
			state.hasNativeDebug = true
			state.nativeDebugTurns++
			mode := strings.TrimSpace(payload.ContinuationMode)
			if mode == "" {
				mode = "unknown"
			}
			state.continuationModes[mode]++
			if payload.RepairGuidancePresent || strings.TrimSpace(payload.RepairGuidanceClass) != "" {
				state.hasRepairGuidance = true
			}
		case model.AgentRunArtifactTypeApprovedPreviewApplied:
			var payload appliedPreviewPayload
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
				continue
			}
			action := strings.TrimSpace(payload.Action)
			if action == "" {
				action = "unknown"
			}
			state.appliedActions[action]++
		}
	}

	groups := make(map[providerAggregateKey]*providerAggregate, len(groupOrder))
	for _, key := range groupOrder {
		groups[key] = &providerAggregate{
			key:               key,
			continuationModes: make(map[string]int),
			appliedActions:    make(map[string]int),
		}
	}

	for _, state := range runState {
		group := groups[state.key]
		group.runCount++
		switch state.status {
		case model.AgentRunStatusCompleted:
			group.completedRuns++
		case model.AgentRunStatusFailed:
			group.failedRuns++
		case model.AgentRunStatusPaused:
			group.pausedRuns++
		case model.AgentRunStatusRunning:
			group.runningRuns++
		case model.AgentRunStatusCancelled:
			group.cancelledRuns++
		}
		if state.selectiveEligible {
			group.selectiveEligibleRuns++
			if !state.hasNativeDebug {
				group.selectiveEligibleMissingDebug++
			}
		}
		if state.hasNativeDebug {
			group.runsWithNativeDebugArtifacts++
		}
		if state.hasRepairGuidance {
			group.runsWithRepairGuidance++
		}
		group.totalNativeDebugTurns += state.nativeDebugTurns
		for mode, count := range state.continuationModes {
			group.continuationModes[mode] += count
		}
		for action, count := range state.appliedActions {
			group.appliedActions[action] += count
		}
	}

	reportGroups := make([]rolloutProviderReport, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groups[key]
		if group == nil || group.runCount == 0 {
			continue
		}
		report := rolloutProviderReport{
			Provider:                      key.Provider,
			Model:                         key.Model,
			PresetKey:                     key.PresetKey,
			RunCount:                      group.runCount,
			CompletedRuns:                 group.completedRuns,
			FailedRuns:                    group.failedRuns,
			PausedRuns:                    group.pausedRuns,
			RunningRuns:                   group.runningRuns,
			CancelledRuns:                 group.cancelledRuns,
			SelectiveEligibleRuns:         group.selectiveEligibleRuns,
			SelectiveEligibleMissingDebug: group.selectiveEligibleMissingDebug,
			SelectiveValidationStatus:     selectiveValidationStatus(group),
			AverageNativeDebugTurns:       float64(group.totalNativeDebugTurns) / float64(group.runCount),
			RunsWithNativeDebugArtifacts:  group.runsWithNativeDebugArtifacts,
			RunsWithRepairGuidance:        group.runsWithRepairGuidance,
			ContinuationModes:             sortedIntMap(group.continuationModes),
			AppliedActions:                sortedIntMap(group.appliedActions),
		}
		reportGroups = append(reportGroups, report)
	}

	return rolloutSummary{
		GeneratedAt: time.Now().UTC(),
		Filters:     filters,
		Groups:      reportGroups,
	}
}

func rolloutRunWouldBeSelectiveEligible(run rolloutRunRow) bool {
	if !run.IsSystem {
		return false
	}
	switch strings.TrimSpace(run.PresetKey) {
	case model.AgentPresetEpicPlanner:
		return strings.TrimSpace(run.TargetType) == "epic"
	case model.AgentPresetTaskPlanner:
		return strings.TrimSpace(run.TargetType) == "task"
	default:
		return false
	}
}

func selectiveValidationStatus(group *providerAggregate) string {
	if group == nil || group.selectiveEligibleRuns == 0 {
		return "not_applicable"
	}
	if group.runsWithNativeDebugArtifacts == 0 {
		return "not_validated_no_native_debug"
	}
	if group.selectiveEligibleMissingDebug > 0 {
		return "partial_missing_native_debug"
	}
	return "validated"
}

func normalizedProvider(provider *string) string {
	if provider == nil || strings.TrimSpace(*provider) == "" {
		return "unknown"
	}
	return strings.TrimSpace(*provider)
}

func normalizedOptional(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sortedIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]int, len(keys))
	for _, key := range keys {
		ordered[key] = values[key]
	}
	return ordered
}

func printRolloutSummary(summary rolloutSummary) {
	fmt.Printf("Generated: %s\n", summary.GeneratedAt.Format(time.RFC3339))
	if summary.Filters.WorkspaceID != "" || summary.Filters.Provider != "" || summary.Filters.PresetKey != "" || summary.Filters.Since != "" || summary.Filters.Until != "" {
		fmt.Printf("Filters: workspace=%q provider=%q preset=%q since=%q until=%q\n",
			summary.Filters.WorkspaceID,
			summary.Filters.Provider,
			summary.Filters.PresetKey,
			summary.Filters.Since,
			summary.Filters.Until,
		)
	}
	if len(summary.Groups) == 0 {
		fmt.Println("No matching native planner runs found.")
		return
	}
	for _, group := range summary.Groups {
		fmt.Printf("\nProvider=%s", group.Provider)
		if group.Model != "" {
			fmt.Printf(" model=%s", group.Model)
		}
		fmt.Printf(" preset=%s\n", group.PresetKey)
		fmt.Printf("  runs: total=%d completed=%d failed=%d paused=%d running=%d cancelled=%d\n",
			group.RunCount, group.CompletedRuns, group.FailedRuns, group.PausedRuns, group.RunningRuns, group.CancelledRuns)
		fmt.Printf("  selective_path: eligible=%d missing_debug=%d status=%s\n",
			group.SelectiveEligibleRuns, group.SelectiveEligibleMissingDebug, group.SelectiveValidationStatus)
		fmt.Printf("  native_turn_debug: runs=%d avg_turns=%.2f repair_runs=%d\n",
			group.RunsWithNativeDebugArtifacts, group.AverageNativeDebugTurns, group.RunsWithRepairGuidance)
		fmt.Printf("  continuation_modes: %s\n", formatIntMap(group.ContinuationModes))
		fmt.Printf("  applied_actions: %s\n", formatIntMap(group.AppliedActions))
	}
}

func formatIntMap(values map[string]int) string {
	if len(values) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}
