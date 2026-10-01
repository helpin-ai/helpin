// Package aipolicy defines the code-versioned policy for every AI action.
package aipolicy

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Modality identifies the execution adapter required by an AI action.
type Modality string

const (
	ModalityAudio         Modality = "audio"
	ModalityChat          Modality = "chat"
	ModalityEmbedding     Modality = "embedding"
	ModalityRerank        Modality = "rerank"
	ModalityExternalAgent Modality = "external_agent"
	ModalityImage         Modality = "image"
)

// Category is the customer-facing AI usage category.
type Category string

const (
	CategorySupportAI Category = "Support AI"
	CategoryDocsAI    Category = "Docs AI"
	CategoryCRMAI     Category = "CRM AI"
	CategoryProjectAI Category = "Project AI"
	CategoryAgents    Category = "Agents"
	CategorySetup     Category = "Setup"
)

// RetryClass controls whether transient provider failures may be retried.
type RetryClass string

const (
	RetryNone      RetryClass = "none"
	RetryTransient RetryClass = "transient"
)

// Autonomy describes the maximum side-effect level an action may have.
type Autonomy string

const (
	AutonomyAnalyze Autonomy = "analyze"
	AutonomyAssist  Autonomy = "assist"
	AutonomyExecute Autonomy = "execute"
)

// DataClass describes the most sensitive input accepted by an action.
type DataClass string

const (
	DataClassCustomerContent DataClass = "customer_content"
	DataClassWorkspaceData   DataClass = "workspace_data"
)

// Action is one immutable, code-versioned AI execution policy.
type Action struct {
	Key                string
	PolicyVersion      string
	FeatureKey         string
	Label              string
	Category           Category
	Origin             string
	Modality           Modality
	DefaultProvider    string
	DefaultModel       string
	AllowedModels      map[string][]string
	Fallbacks          []Route
	Timeout            time.Duration
	MaxInputTokens     int
	MaxOutputTokens    int
	MaxReasoningTokens int
	RetryClass         RetryClass
	Autonomy           Autonomy
	DataClass          DataClass
	Chargeable         bool
}

// Route is one policy-approved provider/model route.
type Route struct {
	Provider string
	Model    string
}

// Registry provides immutable AI action lookup.
type Registry struct {
	definitions []Action
	actions     map[string]Action
}

// NewRegistry constructs a registry without silently discarding duplicate definitions.
func NewRegistry(actions []Action) *Registry {
	definitions := append([]Action(nil), actions...)
	indexed := make(map[string]Action, len(actions))
	for _, action := range actions {
		if _, exists := indexed[action.Key]; !exists {
			indexed[action.Key] = action
		}
	}
	return &Registry{definitions: definitions, actions: indexed}
}

// DefaultRegistry returns the application-owned AI action registry.
func DefaultRegistry() *Registry { return NewRegistry(defaultActions()) }

// Lookup finds an action by stable key.
func (r *Registry) Lookup(key string) (Action, bool) {
	if r == nil {
		return Action{}, false
	}
	action, ok := r.actions[strings.TrimSpace(key)]
	return action, ok
}

// Actions returns definitions in stable key order.
func (r *Registry) Actions() []Action {
	if r == nil {
		return nil
	}
	actions := append([]Action(nil), r.definitions...)
	sort.Slice(actions, func(i, j int) bool { return actions[i].Key < actions[j].Key })
	return actions
}

// Validate rejects incomplete, unsafe, or contradictory policies.
func (r *Registry) Validate() error {
	if r == nil || len(r.definitions) == 0 {
		return fmt.Errorf("AI action registry is empty")
	}
	seen := make(map[string]struct{}, len(r.definitions))
	for _, action := range r.definitions {
		if strings.TrimSpace(action.Key) == "" || strings.TrimSpace(action.PolicyVersion) == "" ||
			strings.TrimSpace(action.FeatureKey) == "" || strings.TrimSpace(action.Label) == "" {
			return fmt.Errorf("AI action key, policy version, feature key, and label are required")
		}
		if _, exists := seen[action.Key]; exists {
			return fmt.Errorf("duplicate AI action %q", action.Key)
		}
		seen[action.Key] = struct{}{}
		if !validModality(action.Modality) || !validCategory(action.Category) {
			return fmt.Errorf("AI action %q has invalid modality or category", action.Key)
		}
		if strings.HasPrefix(action.Key, "support.coverage.") &&
			(action.Category != CategorySupportAI || action.Origin != "coverage") {
			return fmt.Errorf("coverage action %q must be Support AI with coverage origin", action.Key)
		}
		if action.Timeout <= 0 || strings.TrimSpace(action.DefaultProvider) == "" ||
			strings.TrimSpace(action.DefaultModel) == "" {
			return fmt.Errorf("AI action %q requires timeout and default route", action.Key)
		}
		if action.Modality == ModalityChat && action.MaxOutputTokens <= 0 {
			return fmt.Errorf("chat action %q requires an output ceiling", action.Key)
		}
		if !routeAllowed(action, Route{Provider: action.DefaultProvider, Model: action.DefaultModel}) {
			return fmt.Errorf("AI action %q default route is not allowlisted", action.Key)
		}
		if action.RetryClass != RetryNone && action.RetryClass != RetryTransient {
			return fmt.Errorf("AI action %q has invalid retry class", action.Key)
		}
		if action.Autonomy != AutonomyAnalyze && action.Autonomy != AutonomyAssist &&
			action.Autonomy != AutonomyExecute {
			return fmt.Errorf("AI action %q has invalid autonomy", action.Key)
		}
	}
	return nil
}

func routeAllowed(action Action, route Route) bool {
	models := action.AllowedModels[route.Provider]
	for _, model := range models {
		if model == route.Model {
			return true
		}
	}
	return false
}

func validModality(modality Modality) bool {
	switch modality {
	case ModalityAudio, ModalityChat, ModalityEmbedding, ModalityRerank, ModalityExternalAgent, ModalityImage:
		return true
	default:
		return false
	}
}

func validCategory(category Category) bool {
	switch category {
	case CategorySupportAI, CategoryDocsAI, CategoryCRMAI, CategoryProjectAI, CategoryAgents, CategorySetup:
		return true
	default:
		return false
	}
}
