package repository

import (
	"reflect"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (q *previewLookup) deal(id string) *model.LinkPreview {
	var deal model.CRMDeal
	if !q.get(&deal, id, "id, name, display_id, pipeline_id, stage_id, owner_member_id") {
		return nil
	}
	card := previewCard("deal", deal.Name, "", previewField("Pipeline", q.name("crm_pipelines", deal.PipelineID)), previewField("Stage", q.name("crm_pipeline_stages", deal.StageID)), previewField("Owner", q.member(deal.OwnerMemberID, false)))
	card.Reference = deal.DisplayID
	return card
}

func (q *previewLookup) contact(id string) *model.LinkPreview {
	var contact model.CRMContact
	if !q.get(&contact, id, "id, first_name, last_name, description, job_title, lifecycle_stage, owner_member_id") {
		return nil
	}
	return previewCard("contact", strings.TrimSpace(contact.FirstName+" "+stringValue(contact.LastName)), stringValue(contact.Description), previewField("Role", stringValue(contact.JobTitle)), previewField("Stage", previewLabel(contact.LifecycleStage)), previewField("Owner", q.member(contact.OwnerMemberID, false)))
}

func (q *previewLookup) company(id string) *model.LinkPreview {
	var company model.CRMCompany
	if !q.get(&company, id, "id, name, description, industry, owner_member_id, domain") {
		return nil
	}
	return previewCard("company", company.Name, stringValue(company.Description), previewField("Industry", stringValue(company.Industry)), previewField("Owner", q.member(company.OwnerMemberID, false)), previewField("Website", stringValue(company.Domain)))
}

func (q *previewLookup) meeting(id string) *model.LinkPreview {
	var meeting model.CRMMeeting
	if !q.get(&meeting, id, "id, title, scheduled_start_at, platform, owner_member_id", "visibility = 'workspace'") {
		return nil
	}
	return previewCard("meeting", meeting.Title, "", previewField("Scheduled", q.date(meeting.ScheduledStartAt, true)), previewField("Platform", previewLabel(meeting.Platform)), previewField("Owner", q.member(meeting.OwnerMemberID, false)))
}

func (q *previewLookup) playbook(id string) *model.LinkPreview {
	var playbook model.CRMPlaybook
	if !q.get(&playbook, id, "id, draft, published_version_id, accepting_customers") {
		return nil
	}
	publication := "Draft"
	if playbook.PublishedVersionID != nil {
		var version model.CRMPlaybookVersion
		if q.get(&version, *playbook.PublishedVersionID, "id, definition") {
			publication = "Published"
			if !reflect.DeepEqual(playbook.Draft, version.Definition) {
				publication = "Unpublished changes"
			}
		}
	}
	enrollment := "Stopped"
	if playbook.AcceptingCustomers {
		enrollment = "Allowed"
	}
	return previewCard("playbook", playbook.Draft.Name, playbook.Draft.Description, previewField("Publication", publication), previewField("Enrollment", enrollment), previewField("Journey", previewLabel(playbook.Draft.Journey)))
}
