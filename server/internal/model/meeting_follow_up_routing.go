package model

// Meeting follow-up routing is action-level metadata, independent of CRM identity links.
const (
	MeetingFollowUpScopeKey          = "meeting_follow_up_scope"
	MeetingFollowUpRoutingVersionKey = "meeting_follow_up_routing_version"
	MeetingFollowUpRoutingVersion    = "v1"
)

// IsInternalMeetingFollowUp identifies explicitly classified canonical meeting follow-ups.
func IsInternalMeetingFollowUp(s CRMSuggestion) bool {
	return s.SuggestionType == CRMSuggestionFollowUp && s.ObjectType != nil &&
		*s.ObjectType == CRMObjectMeeting && s.ObjectID != nil && *s.ObjectID != "" &&
		s.Context[MeetingFollowUpScopeKey] == "internal"
}
