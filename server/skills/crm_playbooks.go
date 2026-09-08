package skills

import "embed"

// CRMPlaybookRoot is deliberately separate from the ordinary built-in skill catalogue.
const CRMPlaybookRoot = "crm_playbooks"

// CRMPlaybooks contains opt-in job guidance for a Playbook-bound Beacon run.
// Embedding these files neither adds skills to saved Agents nor enables discovery
// by unrelated Agents; the connection must select and snapshot a specific job.
//
//go:embed crm_playbooks
var CRMPlaybooks embed.FS
