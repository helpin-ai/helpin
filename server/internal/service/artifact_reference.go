package service

import "strings"

func artifactReference(artifactID string) string {
	return helpinResourceURI("artifacts", artifactID)
}

func helpinResourceURI(collection, resourceID string) string {
	collection = strings.Trim(strings.TrimSpace(collection), "/")
	resourceID = strings.TrimSpace(resourceID)
	if collection == "" || resourceID == "" {
		return ""
	}
	return "helpin://" + collection + "/" + resourceID
}

// helpinMarkdownLink returns the complete model-facing Markdown link for an
// addressable Helpin resource. Tool consumers should copy this value verbatim
// instead of reconstructing a link from IDs.
func helpinMarkdownLink(label, collection, resourceID string) string {
	resourceURI := helpinResourceURI(collection, resourceID)
	label = strings.TrimSpace(label)
	if label == "" || resourceURI == "" {
		return ""
	}
	label = strings.NewReplacer(
		`\`, `\\`,
		`[`, `\[`,
		`]`, `\]`,
		"\r", " ",
		"\n", " ",
	).Replace(label)
	return "[" + label + "](" + resourceURI + ")"
}

func helpinTaskMarkdownLink(taskKey, name, taskID string) string {
	label := strings.TrimSpace(taskKey)
	if separator := strings.LastIndex(label, "-"); separator <= 0 || separator == len(label)-1 {
		label = strings.TrimSpace(name)
	}
	return helpinMarkdownLink(label, "tasks", taskID)
}

func helpinMarkdownLinkForEntityType(label, entityType, resourceID string) string {
	collections := map[string]string{
		"task":                 "tasks",
		"epic":                 "epics",
		"sprint":               "sprints",
		"objective":            "objectives",
		"document":             "documents",
		"crm_contact":          "contacts",
		"crm_company":          "companies",
		"crm_deal":             "deals",
		"support_conversation": "support-conversations",
		"agent_run":            "agent-runs",
		"artifact":             "artifacts",
	}
	return helpinMarkdownLink(label, collections[strings.TrimSpace(entityType)], resourceID)
}
