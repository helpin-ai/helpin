package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CRMSuggestionRevision binds a review to the exact canonical proposal, targets,
// and supporting evidence. Display projections are deliberately excluded.
func CRMSuggestionRevision(s CRMSuggestion) string {
	s.Revision, s.Signals, s.AssigneeMemberID, s.AssigneeAvailable = "", nil, nil, false
	encoded, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}
