// Package linkpreview defines app link identities and bounded preview rendering.
package linkpreview

import (
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// FallbackDescription is used when no permitted prose description is available.
const FallbackDescription = "Open to view details."

// Tagline is the footer on page and type-only cards.
const Tagline = "One place for your team’s projects, support, sales, and docs."

// TemplateVersion must change when fonts or image layout change, so external
// consumers receive a new image URL even when record content stays the same.
const TemplateVersion = "1"

// Target is a validated route identity. Query text is never preview content.
type Target struct {
	Slug     string
	Kind     string
	ID       string
	ParentID string
	Task     string
	Run      string
	Token    string
	Path     string
	Query    url.Values
	Module   model.ModuleID
	Fallback model.LinkPreview
}

// TypeOnly creates a record-type fallback with no workspace or record details.
func TypeOnly(kind string) model.LinkPreview {
	label := map[string]string{
		"task": "Task", "epic": "Epic", "sprint": "Sprint", "objective": "Objective",
		"document": "Document", "space": "Space", "collection": "Collection",
		"conversation": "Support conversation", "mailbox": "Support inbox",
		"deal": "Deal", "contact": "Contact", "company": "Company", "meeting": "Meeting",
		"playbook": "Playbook", "agent": "Agent", "flow": "Automation flow",
		"run": "Agent run", "chat": "AI conversation",
	}[kind]
	if label == "" {
		return model.LinkPreview{Title: "Helpin", Description: FallbackDescription}
	}
	return model.LinkPreview{Title: label + " in Helpin", Type: label, Description: FallbackDescription}
}

// ModuleForKind identifies the product module needed to publish a record.
func ModuleForKind(kind string) model.ModuleID {
	switch kind {
	case "task", "epic", "sprint", "objective":
		return model.ModulePM
	case "document", "space", "collection":
		return model.ModuleDocs
	case "conversation", "mailbox":
		return model.ModuleSupport
	case "deal", "contact", "company", "meeting", "playbook":
		return model.ModuleCRM
	case "flow":
		return model.ModuleAutomation
	case "agent", "run", "chat":
		return model.ModuleAgents
	}
	return ""
}

// Resolve normalizes the app URL and retains only supported identity parameters.
func Resolve(path, rawQuery string) Target {
	t := Target{Path: "/", Query: url.Values{}, Fallback: TypeOnly("")}
	if len(path) > 2048 || len(rawQuery) > 8192 || !strings.HasPrefix(path, "/") || strings.Contains(path, "//") || strings.Contains(path, "/../") || strings.Contains(path, "/./") || strings.ContainsAny(path, "\\\x00\r\n") {
		return t
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	t.Path = path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return t
	}
	if len(parts) == 2 && (parts[0] == "share" || parts[0] == "shared") {
		if parts[0] == "share" {
			t.Kind = "document"
		}
		t.Fallback = TypeOnly(t.Kind)
		if len(parts[1]) >= 16 && len(parts[1]) <= 256 {
			t.Token = parts[1]
		}
		return t
	}
	if len(parts) < 2 || parts[0] != "w" {
		t.Fallback = globalPage(path)
		return t
	}
	t.Slug = parts[1]
	rel := "/" + strings.Join(parts[2:], "/")
	if rel == "/pm/agents" {
		q.Del("agent_id")
	}
	if rel == "/pm/agent-runs" {
		q = url.Values{}
	}
	if dest, ok := aliases[rel]; ok {
		rel = dest
	}
	if rel == "/settings/mcp" && q.Get("tab") == "external" {
		rel = "/settings/external-mcp"
	}
	t.Path = "/w/" + t.Slug + strings.TrimSuffix(rel, "/")
	t.Fallback = workspacePage(rel)
	t.Module = moduleForPath(rel)
	setRecord := func(kind, id string) {
		t.Kind, t.ID = kind, id
		t.Module, t.Fallback = ModuleForKind(kind), TypeOnly(kind)
	}
	for pattern, kind := range recordRoutes {
		if id, ok := matchRecord(pattern, rel); ok {
			setRecord(kind, id)
			break
		}
	}
	// /support/inbox predates the canonical conversation path.
	if rel == "/support/inbox" {
		t.Kind, t.ID = "", ""
		t.Fallback = workspacePage("/support")
		t.Path = "/w/" + t.Slug + "/support"
		if validID(q.Get("conversation")) {
			setRecord("conversation", q.Get("conversation"))
			t.Path += "/" + t.ID
		}
	}
	if t.Kind == "space" && q.Get("collection") == "__uncollected__" {
		// Eligibility is still checked against the containing space by the service.
		t.Query.Set("collection", "__uncollected__")
	} else if t.Kind == "space" && validID(q.Get("collection")) {
		t.ParentID = t.ID
		setRecord("collection", q.Get("collection"))
		t.Query.Set("collection", t.ID)
	}
	if t.Kind == "" {
		key, kind := "", ""
		switch rel {
		case "/automation/agents":
			key, kind = "agent_id", "agent"
		case "/automation/activity":
			key, kind = "run_id", "run"
		case "/automation/flows":
			key, kind = "show_rule", "flow"
		case "/support":
			if q.Get("custom_view") == "" && (q.Get("view") == "" || q.Get("view") == "team" || q.Get("view") == "inbox") {
				key, kind = "team_inbox", "mailbox"
				if q.Get(key) == "" {
					key = "inbox"
				}
			}
		}
		if key != "" && validID(q.Get(key)) {
			setRecord(kind, q.Get(key))
			t.Query.Set(key, t.ID)
		}
	}
	// A task overlay is explicit URL state even on a sprint or epic page.
	if t.Kind != "task" && validTaskReference(q.Get("task")) {
		setRecord("task", "")
		t.Task = q.Get("task")
		t.Query = url.Values{"task": {t.Task}}
	}
	if validID(q.Get("run")) && (t.Kind == "" || t.Kind == "task") {
		t.Run = q.Get("run")
		t.Query.Set("run", t.Run)
		if t.Kind == "" {
			setRecord("run", t.Run)
		}
	}
	if t.Kind == "" && validID(q.Get("ask_chat")) {
		setRecord("chat", q.Get("ask_chat"))
		t.Query = url.Values{"ask_chat": {t.ID}}
	}
	return t
}

func validID(value string) bool { _, err := uuid.Parse(value); return len(value) == 36 && err == nil }

func validTaskReference(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return false
	}
	if len(parts) == 2 {
		if len(parts[0]) < 2 || len(parts[0]) > 5 {
			return false
		}
		for _, c := range parts[0] {
			if c < 'A' || c > 'Z' {
				return false
			}
		}
	}
	for _, c := range parts[len(parts)-1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return parts[len(parts)-1] != ""
}

func matchRecord(pattern, path string) (string, bool) {
	prefix := pattern[:strings.LastIndex(pattern, "/")+1]
	id := strings.TrimPrefix(path, prefix)
	return id, strings.HasPrefix(path, prefix) && validID(id)
}

func moduleForPath(path string) model.ModuleID {
	if path == "/automation/agents" || path == "/automation/skills" || path == "/automation/tools" || path == "/automation/activity" {
		return model.ModuleAgents
	}
	for prefix, module := range map[string]model.ModuleID{
		"/pm": model.ModulePM, "/sprints": model.ModulePM, "/team-goals": model.ModulePM,
		"/docs": model.ModuleDocs, "/crm": model.ModuleCRM,
		"/support": model.ModuleSupport, "/automation": model.ModuleAutomation,
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return module
		}
	}
	return ""
}
