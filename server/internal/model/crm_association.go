package model

import "time"

// CRM object types for associations.
const (
	CRMObjectContact             = "contact"
	CRMObjectCompany             = "company"
	CRMObjectDeal                = "deal"
	CRMObjectMeeting             = "meeting"
	CRMObjectEpic                = "epic"
	CRMObjectTask                = "task"
	CRMObjectSupportConversation = "support_conversation"

	// CRMAssociationLabelDealCustomer identifies the one account or independent
	// contact that owns the commercial relationship for a deal.
	CRMAssociationLabelDealCustomer = "deal_customer"
	// CRMAssociationLabelDealPrimaryContact identifies the primary person on a
	// company-backed deal. Other linked contacts remain unlabeled participants.
	CRMAssociationLabelDealPrimaryContact = "deal_primary_contact"
)

// CRMAssociationEnriched extends CRMAssociation with the linked object's display info.
type CRMAssociationEnriched struct {
	CRMAssociation
	LinkedObjectName        string  `json:"linked_object_name"`
	LinkedObjectDisplayID   string  `json:"linked_object_display_id"`
	LinkedObjectStatus      *string `json:"linked_object_status,omitempty"`
	LinkedObjectStatusColor *string `json:"linked_object_status_color,omitempty"`
	Inferred                bool    `json:"inferred,omitempty"`
	ContextLabel            *string `json:"context_label,omitempty"`
}

// CRMAssociation represents a relationship between two CRM objects.
type CRMAssociation struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	FromObjectType   string    `json:"from_object_type" gorm:"not null"`
	FromObjectID     string    `json:"from_object_id" gorm:"type:uuid;not null"`
	ToObjectType     string    `json:"to_object_type" gorm:"not null"`
	ToObjectID       string    `json:"to_object_id" gorm:"type:uuid;not null"`
	AssociationLabel *string   `json:"association_label"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CRMAssociation) TableName() string { return "crm_associations" }

// CreateCRMAssociationRequest is the payload for creating an association.
type CreateCRMAssociationRequest struct {
	WorkspaceID      string  `json:"workspace_id"`
	FromObjectType   string  `json:"from_object_type"`
	FromObjectID     string  `json:"from_object_id"`
	ToObjectType     string  `json:"to_object_type"`
	ToObjectID       string  `json:"to_object_id"`
	AssociationLabel *string `json:"association_label"`
}
