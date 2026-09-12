package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrCRMPipelineConflict indicates an editor is saving an outdated pipeline.
var ErrCRMPipelineConflict = errors.New("this pipeline changed since you opened it; refresh and try again")

// ErrCRMPipelineNotFound indicates a pipeline is missing or inaccessible.
var ErrCRMPipelineNotFound = errors.New("pipeline not found")

// CRMPipelineValidationError contains a user-safe pipeline validation message.
type CRMPipelineValidationError struct{ Message string }

func (e *CRMPipelineValidationError) Error() string { return e.Message }

// ValidateCRMPipelineStages validates stage fields and normalizes group order.
func ValidateCRMPipelineStages(stages []CRMPipelineStage) error {
	types := map[string]int{CRMStageTypeOpen: 0, CRMStageTypeWon: 1, CRMStageTypeLost: 2}
	seen := make(map[string]bool, len(stages))
	open := false
	for i := range stages {
		stage := &stages[i]
		stage.Name = strings.TrimSpace(stage.Name)
		if stage.Name == "" {
			return &CRMPipelineValidationError{Message: "stage name cannot be empty"}
		}
		if _, ok := types[stage.StageType]; !ok {
			return &CRMPipelineValidationError{Message: "stage type must be open, won, or lost"}
		}
		if stage.Probability < 0 || stage.Probability > 100 {
			return &CRMPipelineValidationError{Message: "stage probability must be a whole number between 0 and 100"}
		}
		if stage.ID != "" && seen[stage.ID] {
			return &CRMPipelineValidationError{Message: "duplicate stage ID"}
		}
		if stage.ID != "" {
			seen[stage.ID] = true
		}
		if stage.StageType == CRMStageTypeOpen {
			open = true
		}
		if stage.StageType == CRMStageTypeWon {
			stage.Probability = 100
		}
		if stage.StageType == CRMStageTypeLost {
			stage.Probability = 0
		}
	}
	if !open {
		return &CRMPipelineValidationError{Message: "pipeline must contain at least one open stage"}
	}
	sort.SliceStable(stages, func(i, j int) bool {
		if stages[i].StageType != stages[j].StageType {
			return types[stages[i].StageType] < types[stages[j].StageType]
		}
		return stages[i].Position < stages[j].Position
	})
	for i := range stages {
		stages[i].Position = i
	}
	return nil
}

// ValidateCRMPipelineStageChanges protects existing stage identities and deal outcomes.
func ValidateCRMPipelineStageChanges(existing, stages []CRMPipelineStage, migrations map[string]string) error {
	if err := ValidateCRMPipelineStages(stages); err != nil {
		return err
	}
	old := make(map[string]CRMPipelineStage, len(existing))
	retained := make(map[string]CRMPipelineStage, len(stages))
	types := make(map[string]bool)
	for _, stage := range existing {
		old[stage.ID] = stage
	}
	for _, stage := range stages {
		types[stage.StageType] = true
		if stage.ID == "" {
			continue
		}
		previous, ok := old[stage.ID]
		if !ok {
			return &CRMPipelineValidationError{Message: "stage does not belong to this pipeline; refresh and try again"}
		}
		if previous.DealCount > 0 && previous.StageType != stage.StageType {
			return &CRMPipelineValidationError{Message: "cannot change the type of an occupied stage; move its deals first"}
		}
		retained[stage.ID] = stage
	}
	for _, stage := range existing {
		if !types[stage.StageType] {
			return &CRMPipelineValidationError{Message: fmt.Sprintf("keep at least one %s stage in this pipeline", stage.StageType)}
		}
		if _, ok := retained[stage.ID]; !ok && stage.DealCount > 0 && migrations[stage.ID] == "" {
			return &CRMPipelineValidationError{Message: "choose a destination stage before deleting a stage with deals"}
		}
	}
	for source, destination := range migrations {
		from, exists := old[source]
		_, stillPresent := retained[source]
		to, valid := retained[destination]
		if !exists || stillPresent {
			return &CRMPipelineValidationError{Message: "deal migrations must refer to removed stages in this pipeline"}
		}
		if !valid || source == destination {
			return &CRMPipelineValidationError{Message: "choose a retained stage in this pipeline as the destination"}
		}
		if from.StageType != to.StageType {
			return &CRMPipelineValidationError{Message: "the destination stage must have the same type as the removed stage"}
		}
	}
	return nil
}
