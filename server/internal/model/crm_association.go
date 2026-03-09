package model

import "time"

// CRM object types for associations.
const (
	CRMObjectContact = "contact"
	CRMObjectCompany = "company"
	CRMObjectDeal    = "deal"
)

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
