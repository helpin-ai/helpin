package worker

import "encoding/json"

type codexThreadLifecycleResponse struct {
	Thread        codexThreadInfo `json:"thread"`
	Model         string          `json:"model"`
	ModelProvider string          `json:"modelProvider"`
}

type codexThreadInfo struct {
	ID   string  `json:"id"`
	Path *string `json:"path,omitempty"`
	Cwd  string  `json:"cwd,omitempty"`
}

type codexTurnResponse struct {
	Turn codexTurn `json:"turn"`
}

type codexTurn struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Error  *codexTurnError `json:"error,omitempty"`
}

type codexTurnError struct {
	Message           string  `json:"message"`
	AdditionalDetails *string `json:"additionalDetails,omitempty"`
}

type codexThreadStartedNotification struct {
	Thread codexThreadInfo `json:"thread"`
}

type codexTurnStartedNotification struct {
	ThreadID string    `json:"threadId"`
	Turn     codexTurn `json:"turn"`
}

type codexTurnCompletedNotification struct {
	ThreadID string    `json:"threadId"`
	Turn     codexTurn `json:"turn"`
}

type codexTurnDiffUpdatedNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Diff     string `json:"diff"`
}

type codexTurnPlanUpdatedNotification struct {
	ThreadID    string              `json:"threadId"`
	TurnID      string              `json:"turnId"`
	Explanation *string             `json:"explanation,omitempty"`
	Plan        []codexTurnPlanStep `json:"plan"`
}

type codexTurnPlanStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}

type codexItemStartedNotification struct {
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
	Item     codexThreadItem `json:"item"`
}

type codexItemCompletedNotification struct {
	ThreadID string          `json:"threadId"`
	TurnID   string          `json:"turnId"`
	Item     codexThreadItem `json:"item"`
}

type codexAgentMessageDeltaNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

type codexCommandExecutionOutputDeltaNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

type codexServerRequestResolvedNotification struct {
	ThreadID  string `json:"threadId"`
	RequestID any    `json:"requestId"`
}

type codexChatGPTAuthTokensRefreshParams struct {
	Reason            string  `json:"reason"`
	PreviousAccountID *string `json:"previousAccountId,omitempty"`
}

type codexErrorNotification struct {
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	Error     *codexTurnError `json:"error,omitempty"`
	WillRetry bool            `json:"willRetry"`
}

type codexThreadTokenUsageUpdatedNotification struct {
	ThreadID   string                `json:"threadId"`
	TurnID     string                `json:"turnId"`
	TokenUsage codexThreadTokenUsage `json:"tokenUsage"`
}

type codexThreadTokenUsage struct {
	Total codexTokenUsageBreakdown `json:"total"`
	Last  codexTokenUsageBreakdown `json:"last"`
}

type codexTokenUsageBreakdown struct {
	TotalTokens           int64 `json:"totalTokens"`
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
}

type codexThreadItem struct {
	Type             string            `json:"type"`
	ID               string            `json:"id"`
	Text             string            `json:"text,omitempty"`
	Command          string            `json:"command,omitempty"`
	Cwd              string            `json:"cwd,omitempty"`
	Status           string            `json:"status,omitempty"`
	AggregatedOutput *string           `json:"aggregatedOutput,omitempty"`
	ExitCode         *int              `json:"exitCode,omitempty"`
	DurationMs       *int64            `json:"durationMs,omitempty"`
	Tool             string            `json:"tool,omitempty"`
	Server           string            `json:"server,omitempty"`
	Arguments        json.RawMessage   `json:"arguments,omitempty"`
	Result           json.RawMessage   `json:"result,omitempty"`
	Success          *bool             `json:"success,omitempty"`
	Changes          []codexFileChange `json:"changes,omitempty"`
	Error            *codexToolError   `json:"error,omitempty"`
}

type codexFileChange struct {
	Path string          `json:"path"`
	Kind json.RawMessage `json:"kind"`
	Diff string          `json:"diff"`
}

type codexToolError struct {
	Message string `json:"message"`
}

type codexCommandExecutionRequestApprovalParams struct {
	ThreadID                        string          `json:"threadId"`
	TurnID                          string          `json:"turnId"`
	ItemID                          string          `json:"itemId"`
	ApprovalID                      *string         `json:"approvalId,omitempty"`
	Reason                          *string         `json:"reason,omitempty"`
	Command                         *string         `json:"command,omitempty"`
	Cwd                             *string         `json:"cwd,omitempty"`
	CommandActions                  json.RawMessage `json:"commandActions,omitempty"`
	AdditionalPermissions           json.RawMessage `json:"additionalPermissions,omitempty"`
	ProposedExecpolicyAmendment     json.RawMessage `json:"proposedExecpolicyAmendment,omitempty"`
	ProposedNetworkPolicyAmendments json.RawMessage `json:"proposedNetworkPolicyAmendments,omitempty"`
	AvailableDecisions              json.RawMessage `json:"availableDecisions,omitempty"`
}

type codexFileChangeRequestApprovalParams struct {
	ThreadID  string  `json:"threadId"`
	TurnID    string  `json:"turnId"`
	ItemID    string  `json:"itemId"`
	Reason    *string `json:"reason,omitempty"`
	GrantRoot *string `json:"grantRoot,omitempty"`
}

type codexPermissionsRequestApprovalParams struct {
	ThreadID    string                        `json:"threadId"`
	TurnID      string                        `json:"turnId"`
	ItemID      string                        `json:"itemId"`
	Reason      *string                       `json:"reason,omitempty"`
	Permissions codexRequestPermissionProfile `json:"permissions"`
}

type codexRequestPermissionProfile struct {
	Network    *codexAdditionalNetworkPermissions    `json:"network,omitempty"`
	FileSystem *codexAdditionalFileSystemPermissions `json:"fileSystem,omitempty"`
}

type codexGrantedPermissionProfile struct {
	Network    *codexAdditionalNetworkPermissions    `json:"network,omitempty"`
	FileSystem *codexAdditionalFileSystemPermissions `json:"fileSystem,omitempty"`
}

type codexAdditionalNetworkPermissions struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type codexAdditionalFileSystemPermissions struct {
	Read  []string `json:"read,omitempty"`
	Write []string `json:"write,omitempty"`
}

type codexToolRequestUserInputParams struct {
	ThreadID  string                          `json:"threadId"`
	TurnID    string                          `json:"turnId"`
	ItemID    string                          `json:"itemId"`
	Questions []codexToolRequestInputQuestion `json:"questions"`
}

type codexToolRequestInputQuestion struct {
	ID       string                        `json:"id"`
	Header   string                        `json:"header"`
	Question string                        `json:"question"`
	IsOther  bool                          `json:"isOther"`
	IsSecret bool                          `json:"isSecret"`
	Options  []codexToolRequestInputOption `json:"options,omitempty"`
}

type codexToolRequestInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type codexToolRequestUserInputAnswer struct {
	Answers []string `json:"answers"`
}

type codexToolRequestUserInputResponse struct {
	Answers map[string]codexToolRequestUserInputAnswer `json:"answers"`
}
