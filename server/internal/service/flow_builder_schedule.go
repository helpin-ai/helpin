package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/automationcron"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func setFlowSchedulePreview(state *model.FlowBuilderState, draft *model.FlowBuilderDraft, workspaceTimezone string, now time.Time) error {
	timezone := draft.ScheduleTimezone
	if timezone == "" {
		timezone = workspaceTimezone
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if timezone == "Local" {
		return fmt.Errorf("choose a named IANA timezone rather than Local")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("choose a valid IANA timezone")
	}
	state.Timezone, state.NextRuns = timezone, nil
	draft.ScheduleTimezone = timezone
	if draft.TriggerType != model.TriggerCron {
		return nil
	}
	var config model.TriggerConfigCron
	if err := json.Unmarshal(draft.TriggerConfig, &config); err != nil {
		return err
	}
	expression, _, err := resolveCronTriggerConfig(config)
	if err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		next, err := automationcron.Next(expression, now)
		if err != nil {
			return err
		}
		state.NextRuns = append(state.NextRuns, next.In(location).Format(time.RFC3339))
		now = next
	}
	return nil
}
