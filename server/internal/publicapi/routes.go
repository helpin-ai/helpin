// Package publicapi serves Helpin's curated public REST API.
//
// Every operation is a thin, documented adapter over one public MCP tool, so the
// REST API inherits the same workspace-bound principals, scopes, read-only
// enforcement, module and RBAC checks, idempotency, audit trail, and rate limits.
package publicapi

// Route describes one public REST operation and the tool that implements it.
// Path parameters must be named after the tool argument they fill.
type Route struct {
	Method  string
	Path    string
	Tool    string
	Tag     string
	Summary string
	// Status overrides the success status (defaults to 200, or 201 for create_* tools).
	Status int
}

// Tag groups operations in the generated reference.
type Tag struct{ Name, Description string }

// Tags lists the API groups in display order.
var Tags = []Tag{
	{"Context", "The authenticated identity, workspace directory, and cross-product search."},
	{"Tasks", "Tasks, comments, checklists, and workflow state."},
	{"Planning", "Epics, sprints, objectives, labels, and workflows."},
	{"Docs", "Spaces, collections, and documents, including publishing."},
	{"CRM", "Contacts, companies, deals, pipelines, and signals."},
	{"Support", "Support conversations, messages, tags, and routing."},
	{"Agents", "Helpin agents and durable agent runs."},
}

// Routes is the complete public API surface, grouped by Tag.
var Routes = []Route{
	// Context
	{"GET", "/me", "get_current_context", "Context", "Get the authenticated identity and workspace", 0},
	{"GET", "/setup", "get_workspace_setup", "Context", "Inspect workspace onboarding and browser handoffs", 0},
	{"GET", "/search", "search_workspace", "Context", "Search the workspace", 0},
	{"GET", "/teams", "list_workspace_teams", "Context", "List teams", 0},
	{"GET", "/members", "list_workspace_members", "Context", "List workspace members", 0},
	{"GET", "/repositories", "list_repositories", "Context", "List connected repositories", 0},

	// Tasks
	{"GET", "/tasks", "list_tasks", "Tasks", "List tasks", 0},
	{"POST", "/tasks", "create_task", "Tasks", "Create a task", 0},
	{"POST", "/tasks/batch", "create_task_batch", "Tasks", "Create tasks in bulk under an epic", 0},
	{"GET", "/tasks/{task_id}", "get_task", "Tasks", "Get a task", 0},
	{"PATCH", "/tasks/{task_id}", "update_task", "Tasks", "Update a task", 0},
	{"GET", "/tasks/context", "get_task_context", "Tasks", "Get task context", 0},
	{"PUT", "/tasks/{task_id}/state", "update_task_state", "Tasks", "Move a task to a workflow state", 0},
	{"POST", "/tasks/{task_id}/archive", "archive_task", "Tasks", "Archive a task", 0},
	{"POST", "/tasks/{task_id}/restore", "restore_task", "Tasks", "Restore an archived task", 0},
	{"GET", "/tasks/{task_id}/comments", "list_task_comments", "Tasks", "List task comments", 0},
	{"POST", "/tasks/{task_id}/comments", "add_task_comment", "Tasks", "Add a task comment", 0},
	{"GET", "/tasks/{task_id}/checklist", "list_task_checklist", "Tasks", "List checklist items", 0},
	{"POST", "/tasks/{task_id}/checklist", "create_task_checklist_item", "Tasks", "Add a checklist item", 0},
	{"PATCH", "/tasks/{task_id}/checklist/{checklist_item_id}", "update_task_checklist_item", "Tasks", "Update a checklist item", 0},

	// Planning
	{"GET", "/epics", "list_epics", "Planning", "List epics", 0},
	{"POST", "/epics", "create_epic", "Planning", "Create an epic", 0},
	{"GET", "/epics/{epic_id}", "get_epic", "Planning", "Get an epic", 0},
	{"PATCH", "/epics/{epic_id}", "update_epic", "Planning", "Update an epic", 0},
	{"GET", "/sprints", "list_sprints", "Planning", "List sprints", 0},
	{"POST", "/sprints", "create_sprint", "Planning", "Create a sprint", 0},
	{"GET", "/sprints/{sprint_id}", "get_sprint", "Planning", "Get a sprint", 0},
	{"PATCH", "/sprints/{sprint_id}", "update_sprint", "Planning", "Update a sprint", 0},
	{"GET", "/sprints/{sprint_id}/tasks", "list_sprint_tasks", "Planning", "List a sprint's tasks", 0},
	{"GET", "/objectives", "list_objectives", "Planning", "List objectives", 0},
	{"GET", "/objectives/{objective_id}", "get_objective", "Planning", "Get an objective", 0},
	{"GET", "/labels", "list_pm_labels", "Planning", "List labels", 0},
	{"GET", "/workflows", "list_team_workflows_with_stages", "Planning", "List team workflows and their states", 0},

	// Docs
	{"GET", "/spaces", "list_spaces", "Docs", "List spaces", 0},
	{"POST", "/spaces", "create_space", "Docs", "Create a space", 0},
	{"PATCH", "/spaces/{space_id}", "update_space", "Docs", "Update a space", 0},
	{"GET", "/collections", "list_collections", "Docs", "List collections", 0},
	{"POST", "/collections", "create_collection", "Docs", "Create a collection", 0},
	{"PATCH", "/collections/{collection_id}", "update_collection", "Docs", "Update or move a collection", 0},
	{"GET", "/documents", "list_documents", "Docs", "List documents", 0},
	{"POST", "/documents", "create_document", "Docs", "Create a document", 0},
	{"GET", "/documents/search", "search_documents", "Docs", "Search documents", 0},
	{"GET", "/documents/{document_id}", "get_document", "Docs", "Get a document", 0},
	{"PATCH", "/documents/{document_id}", "update_document", "Docs", "Update document metadata", 0},
	{"GET", "/documents/{document_id}/content", "read_document", "Docs", "Read document content", 0},
	{"POST", "/documents/{document_id}/move", "move_document", "Docs", "Move a document", 0},
	{"POST", "/documents/{document_id}/archive", "archive_document", "Docs", "Archive a document", 0},
	{"POST", "/documents/{document_id}/restore", "restore_document", "Docs", "Restore an archived document", 0},
	{"POST", "/documents/{document_id}/publish", "publish_document", "Docs", "Publish a document", 0},
	{"POST", "/documents/{document_id}/unpublish", "unpublish_document", "Docs", "Unpublish a document", 0},

	// CRM
	{"GET", "/crm/contacts", "list_contacts", "CRM", "List contacts", 0},
	{"GET", "/crm/contacts/{contact_id}", "get_crm_contact", "CRM", "Get a contact", 0},
	{"PATCH", "/crm/contacts/{contact_id}", "update_crm_contact", "CRM", "Update a contact", 0},
	{"GET", "/crm/companies", "list_crm_companies", "CRM", "List companies", 0},
	{"GET", "/crm/companies/{company_id}", "get_crm_company", "CRM", "Get a company", 0},
	{"PATCH", "/crm/companies/{company_id}", "update_crm_company", "CRM", "Update a company", 0},
	{"GET", "/crm/deals", "list_deals", "CRM", "List deals", 0},
	{"POST", "/crm/deals", "create_crm_deal", "CRM", "Create a deal", 0},
	{"GET", "/crm/deals/{deal_id}", "get_crm_deal", "CRM", "Get a deal", 0},
	{"PATCH", "/crm/deals/{deal_id}", "update_crm_deal", "CRM", "Update a deal", 0},
	{"PUT", "/crm/deals/{deal_id}/stage", "update_deal_stage", "CRM", "Move a deal to a pipeline stage", 0},
	{"POST", "/crm/deals/{deal_id}/notes", "add_deal_note", "CRM", "Add a deal note", 0},
	{"GET", "/crm/pipelines", "list_crm_pipelines", "CRM", "List pipelines", 0},
	{"GET", "/crm/signals", "list_crm_signals", "CRM", "List signals", 0},
	{"POST", "/crm/activities", "add_crm_activity", "CRM", "Log an activity", 0},

	// Support
	{"GET", "/support/conversations", "list_support_conversations", "Support", "List conversations", 0},
	{"GET", "/support/conversations/{conversation_id}", "get_support_conversation", "Support", "Get a conversation", 0},
	{"PATCH", "/support/conversations/{conversation_id}", "update_support_conversation_subject", "Support", "Rename a conversation", 0},
	{"GET", "/support/conversations/{conversation_id}/messages", "list_conversation_messages", "Support", "List public messages", 0},
	{"PUT", "/support/conversations/{conversation_id}/assignee", "assign_support_conversation", "Support", "Assign or unassign a conversation", 0},
	{"PUT", "/support/conversations/{conversation_id}/inbox", "move_support_conversation", "Support", "Move a conversation to another inbox", 0},
	{"POST", "/support/conversations/{conversation_id}/tags", "add_support_conversation_tag", "Support", "Tag a conversation", 0},
	{"DELETE", "/support/conversations/{conversation_id}/tags/{tag_id}", "remove_support_conversation_tag", "Support", "Remove a tag from a conversation", 0},
	{"GET", "/support/setup", "get_support_setup", "Support", "Inspect support setup and browser handoffs", 0},
	{"GET", "/support/inboxes", "list_support_inboxes", "Support", "List inboxes", 0},
	{"GET", "/support/tags", "list_support_tags", "Support", "List tags", 0},

	// Agents
	{"GET", "/agents", "list_agents", "Agents", "List agents", 0},
	{"POST", "/agent-runs", "start_agent_run", "Agents", "Start an agent run", 202},
	{"GET", "/agent-runs/{run_id}", "get_agent_run", "Agents", "Get an agent run", 0},
	{"POST", "/agent-runs/{run_id}/cancel", "cancel_agent_run", "Agents", "Cancel an agent run", 0},
}
