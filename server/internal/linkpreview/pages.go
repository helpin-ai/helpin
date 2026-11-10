package linkpreview

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type pageContent struct{ title, description string }

// Static page descriptions deliberately exclude filters, counts, and personal values.
var workspacePages = map[string]pageContent{
	"/":                                     {title: "Workspace in Helpin", description: "Open your team’s workspace."},
	"/automation/activity":                  {title: "Automation activity in Helpin", description: "Review automation activity and agent runs."},
	"/automation/agents":                    {title: "Agents in Helpin", description: "Explore the agents available to your team."},
	"/automation/flows":                     {title: "Automation flows in Helpin", description: "Explore your team’s automation flows."},
	"/automation/skills":                    {title: "Skills in Helpin", description: "Explore reusable skills for your agents."},
	"/automation/tools":                     {title: "Tools in Helpin", description: "Explore the tools available to your agents."},
	"/automation/triggers":                  {title: "Automation triggers in Helpin", description: "Explore events that can start an automation."},
	"/crm/companies":                        {title: "Companies in Helpin", description: "Explore the companies your team works with."},
	"/crm/contacts":                         {title: "Contacts in Helpin", description: "Find and manage customer relationships."},
	"/crm/deals":                            {title: "Deals in Helpin", description: "Review and manage sales opportunities."},
	"/crm/emails":                           {title: "Email in Helpin", description: "Open your connected email workspace."},
	"/crm/insights":                         {title: "Sales insights in Helpin", description: "Review signals and insights for your sales work."},
	"/crm/meetings":                         {title: "Meetings in Helpin", description: "Review meetings and follow-up work."},
	"/crm/overview":                         {title: "Sales overview in Helpin", description: "Review your sales workspace."},
	"/crm/playbooks":                        {title: "Playbooks in Helpin", description: "Explore customer journey playbooks."},
	"/crm/review":                           {title: "Sales review in Helpin", description: "Review suggested sales actions."},
	"/dashboard":                            {title: "Dashboard in Helpin", description: "View your workspace overview."},
	"/docs":                                 {title: "Docs in Helpin", description: "Browse your team’s documents and knowledge."},
	"/docs/drafts":                          {title: "Document drafts in Helpin", description: "Continue working on your draft documents."},
	"/docs/my":                              {title: "My documents in Helpin", description: "Find the documents you own."},
	"/docs/recent":                          {title: "Recent documents in Helpin", description: "Return to your recent documents."},
	"/notifications":                        {title: "Notifications in Helpin", description: "View your updates in Helpin."},
	"/pm/epics":                             {title: "Epics in Helpin", description: "Explore larger initiatives and their tasks."},
	"/pm/labels":                            {title: "Labels in Helpin", description: "Organize work with labels."},
	"/pm/my-work":                           {title: "My work in Helpin", description: "View your work and priorities."},
	"/pm/objectives":                        {title: "Objectives in Helpin", description: "Review objectives and key results."},
	"/pm/reports":                           {title: "Project reports in Helpin", description: "Review your team’s progress."},
	"/pm/roadmap":                           {title: "Roadmap in Helpin", description: "Explore upcoming initiatives and objectives."},
	"/pm/sprints":                           {title: "Sprints in Helpin", description: "Plan and review your team’s sprints."},
	"/pm/tasks":                             {title: "Tasks in Helpin", description: "Browse and manage your team’s tasks."},
	"/settings":                             {title: "Workspace settings in Helpin", description: "Manage your workspace preferences."},
	"/settings/access":                      {title: "Module access in Helpin", description: "Open to view details."},
	"/settings/account":                     {title: "Organization details in Helpin", description: "Open to view details."},
	"/settings/ai":                          {title: "AI models in Helpin", description: "Open to view details."},
	"/settings/ai-connections":              {title: "AI models in Helpin", description: "Open to view details."},
	"/settings/automations":                 {title: "Automation settings in Helpin", description: "Open to view details."},
	"/settings/billing":                     {title: "Billing in Helpin", description: "Open to view details."},
	"/settings/chat-general":                {title: "Chat widget in Helpin", description: "Open to view details."},
	"/settings/crm-autonomy":                {title: "Deal automation in Helpin", description: "Open to view details."},
	"/settings/crm-email":                   {title: "Email and calendar in Helpin", description: "Open to view details."},
	"/settings/crm-fields":                  {title: "CRM fields in Helpin", description: "Open to view details."},
	"/settings/crm-meetings":                {title: "Meeting settings in Helpin", description: "Open to view details."},
	"/settings/crm-pipelines":               {title: "Deal pipelines in Helpin", description: "Open to view details."},
	"/settings/customer-portal":             {title: "Customer portal settings in Helpin", description: "Open to view details."},
	"/settings/delivery":                    {title: "Delivery settings in Helpin", description: "Open to view details."},
	"/settings/external-agents":             {title: "External agents in Helpin", description: "Open to view details."},
	"/settings/external-mcp":                {title: "External tools in Helpin", description: "Open to view details."},
	"/settings/general":                     {title: "Workspace settings in Helpin", description: "Open to view details."},
	"/settings/git-connections":             {title: "Git connections in Helpin", description: "Open to view details."},
	"/settings/helpcenter":                  {title: "Help center settings in Helpin", description: "Open to view details."},
	"/settings/import":                      {title: "Import and export in Helpin", description: "Open to view details."},
	"/settings/inboxes-routing":             {title: "Inboxes and routing in Helpin", description: "Open to view details."},
	"/settings/knowledge":                   {title: "Knowledge sources in Helpin", description: "Open to view details."},
	"/settings/labels":                      {title: "Label settings in Helpin", description: "Open to view details."},
	"/settings/mcp":                         {title: "MCP access in Helpin", description: "Open to view details."},
	"/settings/members":                     {title: "Workspace members in Helpin", description: "Open to view details."},
	"/settings/notifications":               {title: "Notification preferences in Helpin", description: "Open to view details."},
	"/settings/profile":                     {title: "Profile in Helpin", description: "Open to view details."},
	"/settings/recurring-tasks":             {title: "Recurring tasks in Helpin", description: "Open to view details."},
	"/settings/redirects":                   {title: "Help center redirects in Helpin", description: "Open to view details."},
	"/settings/repositories":                {title: "Repositories in Helpin", description: "Open to view details."},
	"/settings/security":                    {title: "Security in Helpin", description: "Open to view details."},
	"/settings/server":                      {title: "Signup and admins in Helpin", description: "Open to view details."},
	"/settings/support-ai-assistant":        {title: "Support AI assistant in Helpin", description: "Open to view details."},
	"/settings/support-conversation-fields": {title: "Request types and fields in Helpin", description: "Open to view details."},
	"/settings/support-sla":                 {title: "SLA policies in Helpin", description: "Open to view details."},
	"/settings/support-translation":         {title: "Live Translate in Helpin", description: "Open to view details."},
	"/settings/support-webhooks":            {title: "Support webhooks in Helpin", description: "Open to view details."},
	"/settings/system-status":               {title: "System status in Helpin", description: "Open to view details."},
	"/settings/task-templates":              {title: "Task templates in Helpin", description: "Open to view details."},
	"/settings/teams":                       {title: "Team settings in Helpin", description: "Open to view details."},
	"/setup":                                {title: "Workspace setup in Helpin", description: "Continue setting up your workspace."},
	"/sprints":                              {title: "Sprints in Helpin", description: "Plan and review your team’s sprints."},
	"/support":                              {title: "Support inbox in Helpin", description: "Review and respond to customer conversations."},
	"/support/coverage":                     {title: "Support coverage in Helpin", description: "Review knowledge coverage and improvement opportunities."},
	"/support/reports":                      {title: "Support reports in Helpin", description: "Review your support team’s performance."},
	"/support/search":                       {title: "Support search in Helpin", description: "Find customer conversations in Helpin."},
	"/tasks":                                {title: "Tasks in Helpin", description: "Open to view details."},
	"/team-goals":                           {title: "Objectives in Helpin", description: "Review objectives and key results."},
}

var globalPages = map[string]pageContent{
	"/":                                 {title: "Helpin", description: "One place for your team’s projects, support, sales, and docs."},
	"/billing-preview":                  {title: "Helpin", description: "One place for your team’s projects, support, sales, and docs."},
	"/demo":                             {title: "Helpin demo", description: "Explore Helpin with sample content."},
	"/email-preferences/:token":         {title: "Email preferences in Helpin", description: "Open to view details."},
	"/forgot-password":                  {title: "Account recovery in Helpin", description: "Open Helpin to continue."},
	"/github/installed":                 {title: "GitHub connection in Helpin", description: "Open Helpin to continue."},
	"/join/:token":                      {title: "Workspace invitation in Helpin", description: "Open to view details."},
	"/login":                            {title: "Sign in to Helpin", description: "Open Helpin to continue."},
	"/oauth/authorize":                  {title: "App authorization in Helpin", description: "Open Helpin to continue."},
	"/oauth/cli/authorize":              {title: "CLI authorization in Helpin", description: "Open Helpin to continue."},
	"/oauth/external-mcp/callback":      {title: "App connection in Helpin", description: "Open Helpin to continue."},
	"/onboarding":                       {title: "Get started with Helpin", description: "Set up your team’s workspace."},
	"/portal/:slug":                     {title: "Customer portal in Helpin", description: "Open your customer portal."},
	"/portal/:slug/callback":            {title: "Customer portal in Helpin", description: "Open your customer portal."},
	"/portal/:slug/help":                {title: "Customer help in Helpin", description: "Open to view details."},
	"/portal/:slug/help/:articleId":     {title: "Help article in Helpin", description: "Open to view details."},
	"/portal/:slug/new":                 {title: "New support request in Helpin", description: "Start a support request."},
	"/portal/:slug/requests/:reference": {title: "Support request in Helpin", description: "Open to view details."},
	"/portal/:slug/sign-in":             {title: "Customer portal in Helpin", description: "Open your customer portal."},
	"/portal/:slug/team/:companyId":     {title: "Customer team in Helpin", description: "Open to view details."},
	"/profile":                          {title: "Profile in Helpin", description: "Open to view details."},
	"/register":                         {title: "Get started with Helpin", description: "Create your Helpin account."},
	"/reset-password":                   {title: "Account recovery in Helpin", description: "Open Helpin to continue."},
	"/team-reply/:token":                {title: "Support conversation in Helpin", description: "Open to view details."},
	"/verify-email":                     {title: "Email verification in Helpin", description: "Open Helpin to continue."},
	"/workspaces":                       {title: "Workspaces in Helpin", description: "Choose a workspace to continue."},
}

var aliases = map[string]string{
	"/automation":                   "/automation/flows",
	"/automation/library":           "/automation/triggers",
	"/automation/runs":              "/automation/activity",
	"/automation/tools/connections": "/settings/external-mcp",
	"/crm":                          "/crm/overview",
	"/pm":                           "/pm/my-work",
	"/pm/activity":                  "/automation/activity",
	"/pm/agent-runs":                "/automation/activity",
	"/pm/agents":                    "/automation/agents",
	"/pm/flows":                     "/automation/flows",
	"/pm/library":                   "/automation/triggers",
	"/pm/support":                   "/support",
	"/pm/tool-catalog":              "/automation/tools",
	"/settings/ai-automations":      "/automation/activity",
	"/settings/workflows":           "/automation/flows",
}

var recordRoutes = map[string]string{
	"/crm/companies/:id":      "company",
	"/crm/contacts/:id":       "contact",
	"/crm/deals/:id":          "deal",
	"/crm/meetings/:id":       "meeting",
	"/crm/playbooks/:id":      "playbook",
	"/docs/documents/:id":     "document",
	"/docs/spaces/:id":        "space",
	"/pm/coding-sessions/:id": "run",
	"/pm/epics/:id":           "epic",
	"/pm/objectives/:id":      "objective",
	"/pm/sprints/:id":         "sprint",
	"/pm/tasks/:id":           "task",
	"/sprints/:id":            "sprint",
	"/support/:id":            "conversation",
}

func pagePreview(p pageContent) model.LinkPreview {
	return model.LinkPreview{Title: p.title, Description: p.description}
}

func workspacePage(path string) model.LinkPreview {
	if p, ok := workspacePages[path]; ok {
		return pagePreview(p)
	}
	for pattern, kind := range recordRoutes {
		prefix := pattern[:strings.LastIndex(pattern, "/")+1]
		if strings.HasPrefix(path, prefix) && !strings.Contains(strings.TrimPrefix(path, prefix), "/") {
			return TypeOnly(kind)
		}
	}
	if strings.HasPrefix(path, "/settings/") && strings.Count(path, "/") == 2 {
		return pagePreview(workspacePages["/settings/profile"])
	}
	return TypeOnly("")
}

func globalPage(path string) model.LinkPreview {
	if p, ok := globalPages[path]; ok {
		return pagePreview(p)
	}
	actual := strings.Split(strings.Trim(path, "/"), "/")
	for pattern, p := range globalPages {
		expected := strings.Split(strings.Trim(pattern, "/"), "/")
		if len(expected) != len(actual) {
			continue
		}
		matches := true
		for i, part := range expected {
			if !strings.HasPrefix(part, ":") && part != actual[i] {
				matches = false
				break
			}
		}
		if matches {
			return pagePreview(p)
		}
	}
	return TypeOnly("")
}
