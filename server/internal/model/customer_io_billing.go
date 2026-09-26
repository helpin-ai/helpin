package model

type CustomerIOOrganizationSummary struct {
	MemberCount        int
	WorkspaceCount     int
	TrialingWorkspaces int
	ActiveWorkspaces   int
	LockedWorkspaces   int
	HighestPlan        string
	HasTrialWorkspace  bool
	HasPaidWorkspace   bool
	PaidWorkspaceCount int
	MonthlyDueCents    int
}
