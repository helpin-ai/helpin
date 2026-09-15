package service

import (
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func validateAgentRunDeliveryMode(mode string) error {
	switch mode {
	case "", "publish", "preview":
		return nil
	}
	return fmt.Errorf("delivery_mode must be publish or preview")
}

func agentRunIsPreview(run *model.AgentRun) bool {
	if run == nil {
		return false
	}
	var input model.AgentRunInputPayload
	return json.Unmarshal(run.Input, &input) == nil && input.DeliveryMode == "preview"
}
