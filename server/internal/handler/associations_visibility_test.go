package handler

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestFilterGroupedAssociations(t *testing.T) {
	response := &model.GroupedAssociationsResponse{
		TaskRelationships:    model.TaskRelationshipGroups{Blocking: []model.TaskRelationshipSummary{{RelationshipID: "task-link"}}},
		Tasks:                []model.AssociationObjectSummary{{ObjectID: "task-1"}},
		CRMRecords:           []model.AssociationObjectSummary{{ObjectID: "deal-1"}},
		SupportConversations: []model.AssociationObjectSummary{{ObjectID: "conversation-1"}},
		Docs:                 []model.AssociationObjectSummary{{ObjectID: "doc-1"}},
	}

	filterGroupedAssociations(response, true, false, false, true)

	if len(response.Tasks) != 1 || len(response.TaskRelationships.Blocking) != 1 {
		t.Fatal("project associations should remain visible")
	}
	if len(response.Docs) != 1 {
		t.Fatal("docs associations should remain visible")
	}
	if len(response.CRMRecords) != 0 || len(response.SupportConversations) != 0 {
		t.Fatal("inaccessible cross-app associations must be removed")
	}
}
