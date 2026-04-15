package repository

import "gorm.io/gorm"

// SupportCoverageRepository provides data access for support events,
// coverage topics, gaps, evidence, suggestions, and snapshots.
type SupportCoverageRepository struct {
	db *gorm.DB
}

// NewSupportCoverageRepository creates a new SupportCoverageRepository.
func NewSupportCoverageRepository(db *gorm.DB) *SupportCoverageRepository {
	return &SupportCoverageRepository{db: db}
}
