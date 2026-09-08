package service

import (
	"regexp"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func normalizePlaybookDefinition(d model.CRMPlaybookDefinition, publishing bool) (model.CRMPlaybookDefinition, error) {
	d.Name, d.Description, d.Objective = strings.TrimSpace(d.Name), strings.TrimSpace(d.Description), strings.TrimSpace(d.Objective)
	if d.Name == "" || len(d.Name) > 160 || len(d.Description) > 2000 || len(d.Objective) > 4000 ||
		!slices.Contains([]string{"buying_intent", "sales_handoff", "renewal_recovery", "custom"}, d.Journey) {
		return d, ErrCRMPlaybookInput
	}
	if len(d.Eligibility.CommercialMotions) == 0 || len(d.Eligibility.CommercialMotions) > 7 {
		return d, ErrCRMPlaybookInput
	}
	d.Eligibility.CommercialMotions = slices.Clone(d.Eligibility.CommercialMotions)
	slices.Sort(d.Eligibility.CommercialMotions)
	d.Eligibility.CommercialMotions = slices.Compact(d.Eligibility.CommercialMotions)
	for _, motion := range d.Eligibility.CommercialMotions {
		if model.CRMSituationCategoryForMotion(motion) == "" {
			return d, ErrCRMPlaybookInput
		}
	}
	if q := d.Eligibility.Filter; q != nil {
		if len(q.Rules) > 30 || (q.Logic != "" && q.Logic != "and" && q.Logic != "or") {
			return d, ErrCRMPlaybookInput
		}
		values := 0
		for _, rule := range q.Rules {
			values += len(rule.Values)
			if rule.Value != nil {
				values++
			}
			if values > 100 || (rule.Value != nil && len(rule.Values) > 0) {
				return d, ErrCRMPlaybookInput
			}
			if len(rule.Values) > 100 || (rule.Value != nil && len(*rule.Value) > 500) {
				return d, ErrCRMPlaybookInput
			}
			for _, value := range rule.Values {
				if len(value) > 500 {
					return d, ErrCRMPlaybookInput
				}
			}
		}
	}
	r := &d.Responsibilities
	if r.OwnerRole == "" {
		r.OwnerRole = "signal_owner"
	}
	if r.ApproverRole == "" {
		r.ApproverRole = "next_action_owner"
	}
	if !slices.Contains([]string{"signal_owner", "account_owner", "deal_owner", "customer_success_owner"}, r.OwnerRole) ||
		!slices.Contains([]string{"signal_owner", "next_action_owner"}, r.ApproverRole) ||
		(r.EscalationMemberID != nil && !validSituationID(*r.EscalationMemberID)) {
		return d, ErrCRMPlaybookInput
	}
	if len(d.Milestones) > 20 {
		return d, ErrCRMPlaybookInput
	}
	d.Milestones = slices.Clone(d.Milestones)
	seen := map[string]bool{}
	for i := range d.Milestones {
		m := &d.Milestones[i]
		m.Name, m.SuccessCriteria = strings.TrimSpace(m.Name), strings.TrimSpace(m.SuccessCriteria)
		if !validPlaybookMilestoneKey(m.Key) || seen[m.Key] || m.Name == "" || len(m.Name) > 160 || len(m.SuccessCriteria) > 2000 || (publishing && m.SuccessCriteria == "") {
			return d, ErrCRMPlaybookInput
		}
		seen[m.Key] = true
	}
	p := &d.Policy
	if p.OutboundMessages == "" {
		p.OutboundMessages = "approval_required"
	}
	if p.CRMChanges == "" {
		p.CRMChanges = "approval_required"
	}
	if p.PMTasks == "" {
		p.PMTasks = "not_allowed"
	}
	for _, mode := range []string{p.OutboundMessages, p.CRMChanges, p.PMTasks} {
		if mode != "approval_required" && mode != "not_allowed" {
			return d, ErrCRMPlaybookInput
		}
	}
	if p.CheckAfterHours == 0 {
		p.CheckAfterHours = 48
	}
	if p.EscalateAfterHours == 0 {
		p.EscalateAfterHours = 168
	}
	if p.CheckAfterHours < 1 || p.CheckAfterHours > 8760 || p.EscalateAfterHours < p.CheckAfterHours || p.EscalateAfterHours > 8760 {
		return d, ErrCRMPlaybookInput
	}
	if len(p.StopConditions) == 0 {
		p.StopConditions = []string{"customer_declined", "objective_achieved"}
	}
	if len(p.StopConditions) > 10 {
		return d, ErrCRMPlaybookInput
	}
	p.StopConditions = slices.Clone(p.StopConditions)
	slices.Sort(p.StopConditions)
	p.StopConditions = slices.Compact(p.StopConditions)
	for _, condition := range p.StopConditions {
		if !slices.Contains([]string{"customer_declined", "objective_achieved", "no_longer_eligible", "contact_restricted"}, condition) {
			return d, ErrCRMPlaybookInput
		}
	}
	if publishing && (d.Objective == "" || len(d.Milestones) == 0 || r.EscalationMemberID == nil) {
		return d, ErrCRMPlaybookInput
	}
	return d, nil
}

func validPlaybookMilestoneKey(key string) bool {
	return len(key) <= 64 && regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(key)
}

func playbookTemplates() []model.CRMPlaybookDefinition {
	return []model.CRMPlaybookDefinition{
		{Name: "Buying-intent follow-up", Journey: "buying_intent", Objective: "Agree a qualified next step with the customer",
			Eligibility: model.CRMPlaybookEligibility{CommercialMotions: []string{"prospecting", "conversion"}},
			Milestones:  []model.CRMPlaybookMilestone{{Key: "intent_confirmed", Name: "Intent confirmed", SuccessCriteria: "The customer's need and relevant context are confirmed"}, {Key: "next_step_agreed", Name: "Next step agreed", SuccessCriteria: "The customer agrees to a specific next step"}}},
		{Name: "Sales-to-success handoff", Journey: "sales_handoff", Objective: "Have the receiving success owner accept a usable handoff",
			Eligibility: model.CRMPlaybookEligibility{CommercialMotions: []string{"onboarding"}},
			Milestones:  []model.CRMPlaybookMilestone{{Key: "scope_confirmed", Name: "Scope confirmed", SuccessCriteria: "Sold scope, promises and unresolved dependencies are documented"}, {Key: "handoff_accepted", Name: "Handoff accepted", SuccessCriteria: "The receiving success owner explicitly accepts the handoff"}}},
		{Name: "Renewal-risk recovery", Journey: "renewal_recovery", Objective: "Resolve the renewal risk and confirm the customer's renewal decision",
			Eligibility: model.CRMPlaybookEligibility{CommercialMotions: []string{"renewal", "retention"}},
			Milestones:  []model.CRMPlaybookMilestone{{Key: "risk_resolved", Name: "Risk resolved", SuccessCriteria: "The customer confirms that the underlying risk is addressed"}, {Key: "renewal_confirmed", Name: "Renewal confirmed", SuccessCriteria: "The renewal is confirmed independently of risk resolution"}}},
	}
}
