package agentcontract

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func crmPlaybookContextFixture(t *testing.T) (model.CRMPlaybookContextRequest, model.CRMPlaybookVersion, model.CRMSituationItem, model.CRMPlaybookSpecialization) {
	t.Helper()
	snapshot, err := CaptureCRMPlaybookSpecialization("renewal_recovery")
	if err != nil {
		t.Fatal(err)
	}
	ws, pb, ver, signal, owner, deal := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	checkpoint := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	version := model.CRMPlaybookVersion{
		ID: ver, WorkspaceID: ws, PlaybookID: pb, Version: 1,
		Definition: model.CRMPlaybookDefinition{
			Journey: "renewal_recovery", Objective: "Resolve risk and confirm renewal",
			Policy: model.CRMPlaybookPolicy{OutboundMessages: "approval_required", CRMChanges: "approval_required", PMTasks: "not_allowed"},
		},
	}
	data, err := json.Marshal(version.Definition)
	if err != nil {
		t.Fatal(err)
	}
	version.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(data))
	item := model.CRMSituationItem{OwnerAvailable: true, Situation: model.CRMSituation{
		ID: signal, WorkspaceID: ws, PlaybookID: &pb, PlaybookVersionID: &ver,
		Lifecycle: model.CRMSituationOpen, Revision: 7, DealID: &deal, OwnerMemberID: &owner,
		Objective: "Resolve the API blocker before this renewal", NextStep: "Check the customer's reply",
		NextCheckpointAt: &checkpoint,
	}}
	req := model.CRMPlaybookContextRequest{
		WorkspaceID: ws, PlaybookID: pb, PlaybookVersionID: ver, SituationID: signal,
		ExpectedSituationRevision: 7, SpecializationVersion: snapshot.Version,
		Target: model.AgentRunTargetContext{TargetType: "crm_deal", TargetID: deal},
	}
	return req, version, item, snapshot
}

func TestCRMPlaybookContextPreservesCanonicalProgress(t *testing.T) {
	req, version, item, snapshot := crmPlaybookContextFixture(t)
	before, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	context, err := PrepareCRMPlaybookContext(req, version, item, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if context.SituationID != req.SituationID || context.PlaybookVersionID != version.ID || context.SpecializationVersion != snapshot.Version ||
		context.CustomerObjective != item.Situation.Objective || context.PlaybookObjective != version.Definition.Objective ||
		context.NextStep != item.Situation.NextStep || context.OwnerMemberID != *item.Situation.OwnerMemberID ||
		context.NextCheckpointAt == nil || !context.NextCheckpointAt.Equal(*item.Situation.NextCheckpointAt) {
		t.Fatalf("context lost customer commitments: %#v", context)
	}
	repeated, err := PrepareCRMPlaybookContext(req, version, item, snapshot)
	if err != nil || !reflect.DeepEqual(context, repeated) {
		t.Fatalf("repeated preparation changed the context: %v", err)
	}
	*context.NextCheckpointAt = context.NextCheckpointAt.Add(24 * time.Hour)
	context.CustomerObjective = "changed by caller"
	after, err := json.Marshal(item)
	if err != nil || string(before) != string(after) {
		t.Fatalf("preparation rewrote durable progress: %v", err)
	}
}

func TestCRMPlaybookContextSupportsExistingCRMTargets(t *testing.T) {
	for _, kind := range []string{"crm_deal", "crm_contact", "crm_company"} {
		t.Run(kind, func(t *testing.T) {
			req, version, item, snapshot := crmPlaybookContextFixture(t)
			id := uuid.NewString()
			switch kind {
			case "crm_deal":
				item.Situation.DealID = &id
			case "crm_contact":
				item.Situation.ContactID = &id
			case "crm_company":
				item.Situation.CompanyID = &id
			}
			req.Target = model.AgentRunTargetContext{TargetType: kind, TargetID: id}
			result, err := PrepareCRMPlaybookContext(req, version, item, snapshot)
			if err != nil || result.Target != req.Target {
				t.Fatalf("existing CRM target rejected: %#v %v", result, err)
			}
		})
	}
}

func TestCRMPlaybookContextRejectsUnsafeBindings(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*model.CRMPlaybookContextRequest, *model.CRMPlaybookVersion, *model.CRMSituationItem, *model.CRMPlaybookSpecialization)
	}{
		{name: "wrong customer", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.Target.TargetID = uuid.NewString()
		}},
		{name: "wrong process", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.SituationID = uuid.NewString()
		}},
		{name: "foreign workspace", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.WorkspaceID = uuid.NewString()
		}},
		{name: "wrong policy", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.PlaybookVersionID = uuid.NewString()
		}},
		{name: "wrong skills", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.SpecializationVersion = "changed"
		}},
		{name: "stale context", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.ExpectedSituationRevision--
		}},
		{name: "paused", edit: func(_ *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, i *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			i.Situation.Lifecycle = model.CRMSituationPaused
		}},
		{name: "closed", edit: func(_ *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, i *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			i.Situation.Lifecycle = model.CRMSituationClosed
		}},
		{name: "manual signal", edit: func(_ *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, i *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			i.Situation.PlaybookVersionID = nil
		}},
		{name: "missing owner", edit: func(_ *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, i *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			i.Situation.OwnerMemberID = nil
		}},
		{name: "inactive owner", edit: func(_ *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, i *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			i.OwnerAvailable = false
		}},
		{name: "unsupported target", edit: func(r *model.CRMPlaybookContextRequest, _ *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			r.Target.TargetType = "task"
		}},
		{name: "changed published policy", edit: func(_ *model.CRMPlaybookContextRequest, v *model.CRMPlaybookVersion, _ *model.CRMSituationItem, _ *model.CRMPlaybookSpecialization) {
			v.Definition.Policy.OutboundMessages = "automatic"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, version, item, snapshot := crmPlaybookContextFixture(t)
			test.edit(&req, &version, &item, &snapshot)
			if _, err := PrepareCRMPlaybookContext(req, version, item, snapshot); err == nil {
				t.Fatal("unsafe customer context accepted")
			}
		})
	}
}

func TestCRMPlaybookContextKeepsSeparateProcessesForSameCustomer(t *testing.T) {
	req, version, item, snapshot := crmPlaybookContextFixture(t)
	first, err := PrepareCRMPlaybookContext(req, version, item, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.NewString()
	item.Situation.ID, req.SituationID = other, other
	second, err := PrepareCRMPlaybookContext(req, version, item, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.SituationID == second.SituationID || first.Target != second.Target {
		t.Fatal("independent objectives were conflated")
	}
}
