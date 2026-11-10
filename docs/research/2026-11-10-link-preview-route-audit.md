# Link preview content with workspace previews enabled

This contract defines what a copied Helpin app URL will show when **Show details in link previews** is on. It gives product and engineering one content decision for every current app route, using the fields and navigation behavior in the code.

**Status:** implemented locally on `waqar-fixes`, not deployed. The original source audit reviewed `f4e97270b` on 2026-11-10. The inventory contains 141 frontend route declarations, representing 132 unique URL patterns after merging layouts and trailing-slash variants. Query-selected records and the five account-setting cases in the dynamic settings route are listed separately. No customer records were read. Implementation and verification details appear at the end of this document.

## Agreed presentation

- Put **Show details in link previews** under **Workspace settings → General → Link previews**, managed by workspace admins/owners. Enable it by default for existing and new workspaces. Preserve later opt-outs.
- One Go image renderer and one explicit route/content registry serve the app. A new route receives a generic fallback until its record mapping is registered.
- **Title:** the saved record title/name. A task key or support reference may precede it in `og:title`; put that reference in the image’s small ID line so it is not repeated in the image headline. Never show raw UUIDs. Page titles use the approved `… in Helpin` wording.
- **Meta description:** the saved description or excerpt listed below, converted to plain text. Do not prepend or append status, assignee, team, stage, dates, or other field values. Use the same descriptive text for the HTML description, `og:description`, and `twitter:description`.
- Collapse whitespace, remove markup and embedded-content syntax, and truncate at a word boundary to a target maximum of 160 characters. This is our display limit, not a platform guarantee. Do not copy raw link destinations, image alt text, code blocks, tool outputs, or nested private records into the excerpt. No AI call is needed.
- If the permitted description is blank, use exactly **Open to view details.** Do not manufacture a summary from messages or notes to fill the space.
- **Image:** Helpin branding, record type, optional readable reference, title, up to two lines of descriptive text, and at most three label/value fields. The field order below is intentional. Teams have priority for PM records.
- Use human names and formatted labels; resolve IDs through workspace-scoped relations. Multi-value Team/Assignee fields show the first name and `+N` when needed. A genuinely empty assignment shows `Unassigned`; an unavailable lookup is omitted. Never replace an unresolved relation with its ID.
- Page/fallback images, and record images without properties, use the footer **One place for your team’s projects, support, sales, and docs.** with a subtle divider. Detailed images retain their three property columns, as in the mockup. Do not restore the app domain or sign-in footer.
- The toggle authorizes only this metadata projection. Full record views continue to require their existing permissions. The renderer and metadata HTML must enforce the same policy, including cached images.

## Record preview content

`D` below means the exact fallback **Open to view details.** The fields are for the image only. Empty optional values are omitted unless a fallback is specified.

| Profile | Title | Meta description source, then fallback | Image fields in order | Code source |
| --- | --- | --- | --- | --- |
| Task | `task.name`; metadata reference `task.task_key` or display ID | `task.description` → D | **Team**, **Status** (workflow state name), **Assignee** (all assigned members, compacted) | [Task model](../../server/internal/model/pm_task.go), [copy URL](../../frontend/src/lib/pmTaskLinks.ts) |
| Epic | `epic.name` | `epic.description` → D | **Team**, **Status** (epic workflow state), **Owner** | [Epic model](../../server/internal/model/pm_epic.go) |
| Sprint | `sprint.name` | `sprint.description` → D | **Team**, **Status** (existing sprint status), **Dates** (start–end) | [Sprint model](../../server/internal/model/pm_sprint.go) |
| Objective | `objective.name` | `objective.description` → D | **Teams**, **Status**, **Progress** | [Objective model](../../server/internal/model/pm_objective.go) |
| Support conversation | `conversation.subject`; reference `#display_id` | D. There is no conversation description field. | **Inbox**, **Status**, **Assignee** (human name; agent name if assigned only to an agent) | [Conversation and mailbox models](../../server/internal/model/support_inbox.go) |
| Support inbox | `mailbox.name` | `mailbox.description` → D | **Team** (linked team), if present. No unread/member counts. | [Mailbox model](../../server/internal/model/support_inbox.go) |
| Document | `document.title` | `document.excerpt` → D | **Space**, **Status**, **Owner** | [Docs models](../../server/internal/model/docs.go) |
| Space | `space.name` | D. There is no space description field. | **Teams**, **Type** (Internal / Help center) | [Space model](../../server/internal/model/docs.go) |
| Collection | `collection.name` | `collection.description` → D | **Space**, **Teams** (inherited from space) | [Collection model](../../server/internal/model/docs.go) |
| Deal | `deal.name`; readable deal display ID may appear in the ID line | D. There is no standard deal description field. | **Pipeline**, **Stage**, **Owner** | [Deal model](../../server/internal/model/crm_deal.go) |
| Contact | trimmed `first_name + last_name`; use `Contact in Helpin` if blank, never substitute email | `contact.description` → D | **Role** (`job_title`), **Stage** (`lifecycle_stage`), **Owner** | [Contact model](../../server/internal/model/crm_contact.go) |
| Company | `company.name` | `company.description` → D | **Industry**, **Owner**, **Website** (`domain`, displayed as text) | [Company model](../../server/internal/model/crm_company.go) |
| Meeting | `meeting.title` | D. Recording intelligence is not an external description. | **Scheduled** (workspace time zone), **Platform**, **Owner** | [Meeting model](../../server/internal/model/crm_meeting.go) |
| Playbook | saved `playbook.draft.name`, as in the detail page | saved `playbook.draft.description` → D | **Publication** (Draft / Published / Unpublished changes), **Enrollment** (Allowed / Stopped), **Journey** | [Playbook definition](../../server/internal/model/crm_playbook.go), [detail page](../../frontend/src/pages/crm/PlaybookDetail.tsx) |
| Agent | `agent.name` | D. Agent `role` is a label, not a prose description; prompts are excluded. | **Teams**, **Role**, **Status** | [Agent model](../../server/internal/model/agent.go) |
| Automation flow | `rule.name` | `rule.description` → D | **Team**, **Status** (Enabled / Disabled), **Trigger** (human label) | [Rule model](../../server/internal/model/automation_rule.go) |
| Agent run | `{agent.name} · Agent run`; fallback `Agent run in Helpin` | D. Do not derive text from run input, output summary, errors, or logs. | **Team** (target’s team if eligible, otherwise agent’s teams), **Status**, **Started** | [Run model](../../server/internal/model/agent.go), [session title derivation](../../server/internal/service/coding_session.go) |
| AI conversation | `chat.title` only for eligible workspace-visible chats | D. Do not use the first/last message. | **Module**, **Updated**, when available | [Dock chat model](../../server/internal/model/dock_chat.go) |
| Public document share | token-authorized `document.title` | token-authorized `document.excerpt` → D | **Status**, **Updated**. Do not expose private containing space, team, or owner. | [Document share handler](../../server/internal/handler/docs.go) |
| Public AI conversation | active public projection `dock_chat.title` | D, even though the token allows reading the transcript | **Updated** | [Public projection](../../server/internal/model/public_share.go) |
| Public agent run | active public projection `agent_run.title` | D | **Status**, **Started**, from the sanitized public session only | [Public share source](../../server/internal/service/public_share_source.go) |

Objective Progress uses `stats.key_result_avg_pct` when key results exist; otherwise `stats.epic_progress_pct` when linked epics exist. Omit it when neither basis exists. Do not combine those percentages or invent an objective owner from `created_by`.

Use the existing workflow/state names for task, epic, sprint, conversation, and run statuses. Sprint status is read-only in the model and the repository computes it from its dates; previews reuse that same `computeSprintStatus` function. Assignee/owner resolution uses the canonical task-owner join table and compacts multiple names without displaying the same person twice.

A document owner is not necessarily its author. The field is labeled **Owner**, matching `owner_id`. A task has an epic, sprint, and team; it has no separate project relation in its model. **Team** replaces the sample card’s fictional Project field. **Pipeline** replaces the sample deal’s monetary Value field; amounts, forecast values, and custom financial properties are excluded from the initial external preview contract.

## Eligibility and narrower access

The accepted default-on setting publishes the selected metadata for ordinary workspace PM, support, CRM, and agent records. It does not authenticate the person viewing an external card. PM team membership and support mailbox membership still restrict opening the record. Those normal access boundaries are not proof that a crawler is a teammate, and a team/mailbox assignment alone cannot be used as a global “restricted item” flag: that would suppress most of these previews. This is a deliberate small public metadata projection authorized by the workspace setting.

The following narrower scopes always get a type-only card on ordinary app URLs:

- Docs in a `team_only` space or with a narrower document visibility; creator-scoped drafts; archived/deleted documents. A valid explicitly public document-share token has its own authorized projection.
- AI chats with `private` or `module` visibility, and runs backed by those chats. Only `workspace` chats are eligible on ordinary links. An active `/shared/:token` is a separate explicit public grant.
- PM records without a team, and objectives without teams: the current PM service treats these as admin/owner-only. Use a type-only preview.
- Records deleted, archived, anonymized, unavailable, in another workspace, or in a module unavailable to the deployment/workspace. If archived CRM/other resources do not have an archive field, do not invent one.
- Customer portal requests, customer team pages, and portal help articles require a portal session and remain type-only. Workspace preview preferences do not turn them into public customer content.
- If a meeting with any visibility other than `workspace` is encountered, use a type-only card. The constants include private/participant values, but the current meeting service only accepts `workspace`; this is a fail-closed rule, not a claim that private meeting UI is shipped.
- A run whose underlying target or chat is restricted gets a type-only card too. Do not expose the target’s title or team through the run.

Type-only means **{Record type} in Helpin**, **Open to view details.**, no record reference or properties, and the agreed footer. Missing resources and permission failures should use the same presentation rather than revealing whether a sensitive record exists. For a disabled/unavailable module, use the neutral Helpin fallback.

These rules require a metadata-specific service. Existing authenticated services are not suitable anonymous authorization checks: several PM and support code paths treat a missing actor as an internal/unscoped call. Validate workspace, module availability, record membership, narrower visibility, and parent relationships before returning the allowlisted fields. Never pass a full authenticated DTO through the public metadata endpoint.

Evidence: [PM team access](../../server/internal/service/pm_team_access.go), [support mailbox access](../../server/internal/service/support_mailbox.go), [docs visibility](../../server/internal/service/docs_space.go), [draft list filtering](../../server/internal/repository/docs_document.go), [meeting visibility validation](../../server/internal/service/crm_meeting.go), [chat visibility](../../server/internal/model/dock_chat.go).

## URL parameters and selected records

The following parameters select records or views. Ignore all other query text for generated titles/descriptions: search strings, email addresses, filters, sorting, pagination, callback values, and user-supplied `*_title` / `*_description` must never be reflected into metadata.

| URL shape | Final content decision |
| --- | --- |
| `/w/:slug/pm/tasks/:taskId` | Resolve the task from the path in that workspace. A copied URL can also contain `?task=TEAM-123`; the path remains authoritative. Do not assume the UUID path currently accepts a human task key. |
| `/w/:slug/…?task=TEAM-123` or `?task=123` | Resolve the workspace-scoped display reference and use Task, even when a sprint/epic/list is behind the drawer. Confirm the resolved task key before publishing; do not trust a user-supplied team prefix. |
| `/w/:slug/…?run=:runId`, including task routes | Use Agent run after validating the run and its target. If combined with a task path/query, the run must belong to that task; otherwise retain the task preview. |
| `/w/:slug/automation/activity?run_id=:runId` | Agent run. The page opens a run drawer for this parameter. |
| `/w/:slug/automation/activity?execution_id=…` | Automation activity page. This is a list filter, not a standalone execution detail route; no execution payload is exposed. |
| `/w/:slug/automation/agents?agent_id=:agentId` | Agent. The page opens that saved agent. `agent_id` on activity/flows pages is only a filter and keeps the page preview. |
| `/w/:slug/automation/flows?show_rule=:ruleId` | Automation flow. Fetch the saved name/description; ignore `show_rule_title`. `show_trigger`, `workflow`, `team`, `template`, and builder inputs keep the general Flows card. |
| `/w/:slug/docs/spaces/:spaceId?collection=:collectionId` | Collection, after verifying it belongs to that space and workspace. `collection=__uncollected__` shows **Uncategorized documents in Helpin**, D, and no properties. An invalid collection falls back to the space without reflecting its ID. |
| `/w/:slug/support/inbox?conversation=:id` | Legacy route: the layout redirects to `/support/:conversationId`; use Support conversation. Without `conversation`, `/support/inbox` redirects to the general inbox. `conversation` on `/support` alone does not select a record in the inspected layout; keep its page preview. |
| `/w/:slug/support?team_inbox=:mailboxId` | Support inbox, after resolving a real mailbox. The legacy `inbox` parameter also selects a mailbox; `team_inbox` takes priority. Personal/system views, `custom_view`, and search/filter parameters keep the general Support inbox card. |
| `/w/:slug/…?ask_chat=:chatId` | AI conversation if workspace-visible; otherwise **AI conversation in Helpin** with no details. The application consumes/removes this parameter after opening the dock; once absent from a copied browser URL, the URL cannot identify that chat. |
| Multiple unrelated record selectors | Apply the supported task-overlay and matching task/run cases above first. For other conflicting selectors, use the explicit path record when there is one; otherwise the underlying page fallback. Do not combine metadata from unrelated records. |
| UI selection stored only in Zustand/component state | Preview the page actually named in the URL. Opening a drawer does not guarantee a browser-copied URL names the record; use the existing record Copy link action where the UI keeps selection out of the URL. |

Only preserve validated identity parameters in the metadata canonical URL. Never include tracking/search/credential parameters. Legacy redirects must preserve or discard identity parameters exactly as the frontend does: `/pm/agents` explicitly clears `agent_id`, and `/pm/agent-runs` resets to activity page 1; do not manufacture record previews for those aliases. `/automation/runs` and `/pm/activity` preserve their validated run filters. Hash fragments are not sent to the server and cannot select a server-rendered preview.

Evidence: [task copy links](../../frontend/src/lib/pmTaskLinks.ts), [task and run selection](../../frontend/src/components/pm/GlobalTaskPanel.tsx), [task navigation](../../frontend/src/components/pm/task-detail/taskRouteNavigation.ts), [agents route](../../frontend/src/routes/_authenticated/w/$slug/automation/agents.tsx), [activity page](../../frontend/src/pages/automation/AutomationActivity.tsx), [flow selection](../../frontend/src/pages/automation/AutomationFlows.tsx), [collection selection](../../frontend/src/pages/docs/spaceDetail/nodeSelection.ts), [support URL schema](../../frontend/src/lib/supportInboxRouting.ts), [chat link consumption](../../frontend/src/components/agents/dock/useSharedChatLink.ts).

## Every workspace route

All paths in this table are relative to `/w/:slug`. Trailing slashes and parent layout routes share the same result. A **record profile** refers to the exact title, description, and image fields defined above. A **page** has the shown fixed title and description, no live counters or record fields, and the agreed tagline. Valid record-selecting parameters take precedence as specified above.

| Route | Preview with toggle on | Meta description / routing note | Source |
| --- | --- | --- | --- |
| `/` | Workspace in Helpin | Open your team’s workspace. No personalized home-module inference. | [route](../../frontend/src/routes/_authenticated/w/$slug/index.tsx) |
| `/automation` | Automation flows in Helpin | Same preview as `/automation/flows`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/index.tsx) |
| `/automation/activity` | Automation activity in Helpin | Review automation activity and agent runs. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/activity.tsx) |
| `/automation/agents` | Agents in Helpin | Explore the agents available to your team. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/agents.tsx) |
| `/automation/flows` | Automation flows in Helpin | Explore your team’s automation flows. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/flows.tsx) |
| `/automation/library` | Automation triggers in Helpin | Same preview as `/automation/triggers`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/library.tsx) |
| `/automation/runs` | Automation activity in Helpin | Same preview as `/automation/activity`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/runs.tsx) |
| `/automation/skills` | Skills in Helpin | Explore reusable skills for your agents. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/skills.tsx) |
| `/automation/tools` | Tools in Helpin | Explore the tools available to your agents. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/tools/index.tsx) |
| `/automation/tools/connections` | External tools in Helpin | Same preview as `/settings/external-mcp`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/tools/connections.tsx) |
| `/automation/triggers` | Automation triggers in Helpin | Explore events that can start an automation. | [route](../../frontend/src/routes/_authenticated/w/$slug/automation/triggers.tsx) |
| `/crm` | Sales overview in Helpin | Same preview as `/crm/overview`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/index.tsx) |
| `/crm/companies` | Companies in Helpin | Explore the companies your team works with. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/companies/index.tsx) |
| `/crm/companies/:companyId` | **Company profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/companies/$companyId.tsx) |
| `/crm/contacts` | Contacts in Helpin | Find and manage customer relationships. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/contacts/index.tsx) |
| `/crm/contacts/:contactId` | **Contact profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/contacts/$contactId.tsx) |
| `/crm/deals` | Deals in Helpin | Review and manage sales opportunities. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/deals/index.tsx) |
| `/crm/deals/:dealId` | **Deal profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/deals/$dealId.tsx) |
| `/crm/emails` | Email in Helpin | Open your connected email workspace. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/emails/index.tsx) |
| `/crm/insights` | Sales insights in Helpin | Review signals and insights for your sales work. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/insights.tsx) |
| `/crm/meetings` | Meetings in Helpin | Review meetings and follow-up work. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/meetings/index.tsx) |
| `/crm/meetings/:meetingId` | **Meeting profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/meetings/$meetingId.tsx) |
| `/crm/overview` | Sales overview in Helpin | Review your sales workspace. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/overview.tsx) |
| `/crm/playbooks` | Playbooks in Helpin | Explore customer journey playbooks. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/playbooks/index.tsx) |
| `/crm/playbooks/:playbookId` | **Playbook profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/playbooks/$playbookId.tsx) |
| `/crm/review` | Sales review in Helpin | Review suggested sales actions. | [route](../../frontend/src/routes/_authenticated/w/$slug/crm/review.tsx) |
| `/dashboard` | Dashboard in Helpin | View your workspace overview. | [route](../../frontend/src/routes/_authenticated/w/$slug/dashboard.tsx) |
| `/docs` | Docs in Helpin | Browse your team’s documents and knowledge. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/index.tsx) |
| `/docs/documents/:docId` | **Document profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/documents/$docId.tsx) |
| `/docs/drafts` | Document drafts in Helpin | Continue working on your draft documents. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/drafts.tsx) |
| `/docs/my` | My documents in Helpin | Find the documents you own. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/my.tsx) |
| `/docs/recent` | Recent documents in Helpin | Return to your recent documents. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/recent.tsx) |
| `/docs/spaces/:spaceId` | **Space profile** | Use the profile’s description source and image fields. `collection` can select a Collection profile. | [route](../../frontend/src/routes/_authenticated/w/$slug/docs/spaces/$spaceId.tsx) |
| `/notifications` | Notifications in Helpin | View your updates in Helpin. | [route](../../frontend/src/routes/_authenticated/w/$slug/notifications.tsx) |
| `/pm` | My work in Helpin | Same preview as `/pm/my-work`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/index.tsx) |
| `/pm/activity` | Automation activity in Helpin | Same preview as `/automation/activity`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/activity.tsx) |
| `/pm/agent-runs` | Automation activity in Helpin | Redirects to Automation activity with page 1; page preview. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/agent-runs.tsx) |
| `/pm/agents` | Agents in Helpin | Redirects to Agents and explicitly clears `agent_id`; page preview. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/agents.tsx) |
| `/pm/coding-sessions/:sessionId` | **Agent run profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/coding-sessions/$sessionId.tsx) |
| `/pm/epics` | Epics in Helpin | Explore larger initiatives and their tasks. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/epics/index.tsx) |
| `/pm/epics/:epicId` | **Epic profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/epics/$epicId.tsx) |
| `/pm/flows` | Automation flows in Helpin | Same preview as `/automation/flows`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/flows.tsx) |
| `/pm/labels` | Labels in Helpin | Organize work with labels. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/labels.tsx) |
| `/pm/library` | Automation triggers in Helpin | Same preview as `/automation/triggers`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/library.tsx) |
| `/pm/my-work` | My work in Helpin | View your work and priorities. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/my-work.tsx) |
| `/pm/objectives` | Objectives in Helpin | Review objectives and key results. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/objectives/index.tsx) |
| `/pm/objectives/:objectiveId` | **Objective profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/objectives/$objectiveId.tsx) |
| `/pm/reports` | Project reports in Helpin | Review your team’s progress. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx) |
| `/pm/roadmap` | Roadmap in Helpin | Explore upcoming initiatives and objectives. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/roadmap.tsx) |
| `/pm/sprints` | Sprints in Helpin | Plan and review your team’s sprints. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/sprints/index.tsx) |
| `/pm/sprints/:sprintId` | **Sprint profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/sprints/$sprintId.tsx) |
| `/pm/support` | Support inbox in Helpin | Same preview as `/support`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/support.tsx) |
| `/pm/tasks` | Tasks in Helpin | Browse and manage your team’s tasks. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/tasks/index.tsx) |
| `/pm/tasks/:taskId` | **Task profile** | Use the profile’s description source and image fields. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/tasks/$taskId.tsx) |
| `/pm/tool-catalog` | Tools in Helpin | Same preview as `/automation/tools`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/pm/tool-catalog.tsx) |
| `/settings` | Workspace settings in Helpin | Manage your workspace preferences. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/index.tsx) |
| `/settings/:section` | Known account-setting title below | Only the five named cases below; unknown section redirects to Profile. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/$section.tsx) |
| `/settings/access` | Module access in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/access.tsx) |
| `/settings/ai` | AI models in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/ai.tsx) |
| `/settings/ai-automations` | Automation activity in Helpin | Same preview as `/automation/activity`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/ai-automations.tsx) |
| `/settings/ai-connections` | AI models in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/ai-connections.tsx) |
| `/settings/automations` | Automation settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/automations.tsx) |
| `/settings/billing` | Billing in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/billing.tsx) |
| `/settings/chat-general` | Chat widget in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/chat-general.tsx) |
| `/settings/crm-autonomy` | Deal automation in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/crm-autonomy.tsx) |
| `/settings/crm-email` | Email and calendar in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/crm-email.tsx) |
| `/settings/crm-fields` | CRM fields in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/crm-fields.tsx) |
| `/settings/crm-meetings` | Meeting settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/crm-meetings.tsx) |
| `/settings/crm-pipelines` | Deal pipelines in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/crm-pipelines.tsx) |
| `/settings/customer-portal` | Customer portal settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/customer-portal.tsx) |
| `/settings/delivery` | Delivery settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/delivery.tsx) |
| `/settings/external-agents` | External agents in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/external-agents.tsx) |
| `/settings/external-mcp` | External tools in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/external-mcp.tsx) |
| `/settings/general` | Workspace settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/general.tsx) |
| `/settings/helpcenter` | Help center settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/helpcenter.tsx) |
| `/settings/import` | Import and export in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/import.tsx) |
| `/settings/inboxes-routing` | Inboxes and routing in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/inboxes-routing.tsx) |
| `/settings/knowledge` | Knowledge sources in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/knowledge.tsx) |
| `/settings/labels` | Label settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/labels.tsx) |
| `/settings/mcp` | MCP access in Helpin | Open to view details. A legacy external-tools tab redirects to External tools. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/mcp.tsx) |
| `/settings/members` | Workspace members in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/members.tsx) |
| `/settings/recurring-tasks` | Recurring tasks in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/recurring-tasks.tsx) |
| `/settings/redirects` | Help center redirects in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/redirects.tsx) |
| `/settings/repositories` | Repositories in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/repositories.tsx) |
| `/settings/server` | Signup and admins in Helpin | Open to view details. Community page; hosted edition redirects to Workspace settings. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/server.tsx) |
| `/settings/support-ai-assistant` | Support AI assistant in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/support-ai-assistant.tsx) |
| `/settings/support-conversation-fields` | Request types and fields in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/support-conversation-fields.tsx) |
| `/settings/support-sla` | SLA policies in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/support-sla.tsx) |
| `/settings/support-translation` | Live Translate in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/support-translation.tsx) |
| `/settings/support-webhooks` | Support webhooks in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/support-webhooks.tsx) |
| `/settings/system-status` | System status in Helpin | Open to view details. Community page; hosted edition redirects to Workspace settings. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/system-status.tsx) |
| `/settings/task-templates` | Task templates in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/task-templates.tsx) |
| `/settings/teams` | Team settings in Helpin | Open to view details. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/teams.tsx) |
| `/settings/workflows` | Automation flows in Helpin | Same preview as `/automation/flows`; follow the parameter rules above. | [route](../../frontend/src/routes/_authenticated/w/$slug/settings/workflows.tsx) |
| `/setup` | Workspace setup in Helpin | Continue setting up your workspace. | [route](../../frontend/src/routes/_authenticated/w/$slug/setup.tsx) |
| `/sprints` | Sprints in Helpin | Plan and review your team’s sprints. | [route](../../frontend/src/routes/_authenticated/w/$slug/sprints/index.tsx) |
| `/sprints/:sprintId` | **Sprint profile** | Use the profile’s description source and image fields. Same content as `/pm/sprints/:sprintId`. | [route](../../frontend/src/routes/_authenticated/w/$slug/sprints/$sprintId.tsx) |
| `/support` | Support inbox in Helpin | Review and respond to customer conversations. | [route](../../frontend/src/routes/_authenticated/w/$slug/support/_inbox/index.tsx) |
| `/support/:conversationId` | **Support conversation profile** | Use the profile’s description source and image fields. Reserve the legacy `inbox` path value for the redirect behavior described above. | [route](../../frontend/src/routes/_authenticated/w/$slug/support/_inbox/$conversationId.tsx) |
| `/support/coverage` | Support coverage in Helpin | Review knowledge coverage and improvement opportunities. | [route](../../frontend/src/routes/_authenticated/w/$slug/support/coverage.tsx) |
| `/support/reports` | Support reports in Helpin | Review your support team’s performance. | [route](../../frontend/src/routes/_authenticated/w/$slug/support/reports.tsx) |
| `/support/search` | Support search in Helpin | Find customer conversations in Helpin. | [route](../../frontend/src/routes/_authenticated/w/$slug/support/search.tsx) |
| `/tasks` | Tasks in Helpin | Open to view details. This legacy route is currently a placeholder, not the PM task list. | [route](../../frontend/src/routes/_authenticated/w/$slug/tasks.tsx) |
| `/team-goals` | Objectives in Helpin | Review objectives and key results. | [route](../../frontend/src/routes/_authenticated/w/$slug/team-goals.tsx) |

The dynamic `/settings/:section` route handles these additional paths. All have description **Open to view details.**, no personal/account values, and no image properties.

| Route | Title |
| --- | --- |
| `/settings/profile` | Profile in Helpin |
| `/settings/security` | Security in Helpin |
| `/settings/account` | Organization details in Helpin |
| `/settings/git-connections` | Git connections in Helpin |
| `/settings/notifications` | Notification preferences in Helpin |

## Public, portal, account, and system routes

A public-share token is resolved through its existing share service and must still be active. It does not authorize fetching arbitrary related records. Token/error pages never expose email addresses, customer names, workspace invitation details, authentication codes, session identifiers, or raw error text. Do not redeem a token or invoke a state-changing endpoint while preparing a preview.

| Route | Preview with toggle on | Meta description / image fields | Source |
| --- | --- | --- | --- |
| `/` | Helpin | One place for your team’s projects, support, sales, and docs. No image properties. | [route](../../frontend/src/routes/index.tsx) |
| `/billing-preview` | Helpin | One place for your team’s projects, support, sales, and docs. No image properties. | [route](../../frontend/src/routes/billing-preview.tsx) |
| `/demo` | Helpin demo | Explore Helpin with sample content. No image properties. | [route](../../frontend/src/routes/demo.tsx) |
| `/email-preferences/:token` | Email preferences in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/email-preferences/$token.tsx) |
| `/forgot-password` | Account recovery in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/forgot-password.tsx) |
| `/github/installed` | GitHub connection in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/_authenticated/github/installed.tsx) |
| `/join/:token` | Workspace invitation in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/join/$token.tsx) |
| `/login` | Sign in to Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/login.tsx) |
| `/oauth/authorize` | App authorization in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/_authenticated/oauth/authorize.tsx) |
| `/oauth/cli/authorize` | CLI authorization in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/_authenticated/oauth/cli/authorize.tsx) |
| `/oauth/external-mcp/callback` | App connection in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/_authenticated/oauth/external-mcp/callback.tsx) |
| `/onboarding` | Get started with Helpin | Set up your team’s workspace. No image properties. | [route](../../frontend/src/routes/_authenticated/onboarding.tsx) |
| `/portal/:slug` | Customer portal in Helpin | Open your customer portal. No image properties. | [route](../../frontend/src/routes/portal/$slug/index.tsx) |
| `/portal/:slug/callback` | Customer portal in Helpin | Open your customer portal. No image properties. | [route](../../frontend/src/routes/portal/$slug/callback.tsx) |
| `/portal/:slug/help` | Customer help in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/portal/$slug/help.index.tsx) |
| `/portal/:slug/help/:articleId` | Help article in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/portal/$slug/help.$articleId.tsx) |
| `/portal/:slug/new` | New support request in Helpin | Start a support request. No image properties. | [route](../../frontend/src/routes/portal/$slug/new.tsx) |
| `/portal/:slug/requests/:reference` | Support request in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/portal/$slug/requests/$reference.tsx) |
| `/portal/:slug/sign-in` | Customer portal in Helpin | Open your customer portal. No image properties. | [route](../../frontend/src/routes/portal/$slug/sign-in.tsx) |
| `/portal/:slug/team/:companyId` | Customer team in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/portal/$slug/team.$companyId.tsx) |
| `/profile` | Profile in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/_authenticated/profile.tsx) |
| `/register` | Get started with Helpin | Create your Helpin account. No image properties. | [route](../../frontend/src/routes/register.tsx) |
| `/reset-password` | Account recovery in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/reset-password.tsx) |
| `/share/:shareToken` | **Public document share profile** | Active share: excerpt → D; image Status / Updated. Invalid/revoked: Document in Helpin, D, no properties. | [route](../../frontend/src/routes/share/$shareToken.tsx) |
| `/shared/:shareToken` | **Public AI conversation or Public agent run profile** | Dispatch by returned resource_type. Invalid/revoked: Helpin, D, no properties. | [route](../../frontend/src/routes/shared/$shareToken.tsx) |
| `/team-reply/:token` | Support conversation in Helpin | Open to view details. No image properties. | [route](../../frontend/src/routes/team-reply/$token.tsx) |
| `/verify-email` | Email verification in Helpin | Open Helpin to continue. No image properties. | [route](../../frontend/src/routes/verify-email.tsx) |
| `/workspaces` | Workspaces in Helpin | Choose a workspace to continue. No image properties. | [route](../../frontend/src/routes/_authenticated/workspaces.tsx) |

Portal article URLs are deliberately type-only: [the portal article handler](../../server/internal/handler/customer_portal_help.go) calls `authorized()` before returning an article. Public Help Center article URLs are a different frontend and use published public content. Preserve their existing author-configured `og_title`, `og_description`, and image; fall back through SEO description and excerpt as implemented in [Help Center SEO](../../help-center/src/lib/seo.ts). Do not replace customer Help Center branding with the internal app card or expose a draft/preview article through this workspace toggle. Marketing website routes are also outside this app-route contract.

Skills, tools, triggers, pipelines, labels, team settings, and template settings currently have catalog/settings URLs rather than independent record detail routes in this inventory. A selected local row/modal does not create a distinct share target. Their previews remain the corresponding page card unless a supported identity URL is explicitly added later.

## Rendering and delivery rules

- Normalize trailing slashes and known aliases before matching. Prefer literal route matches such as `/support/search` and `/support/reports` before the dynamic `/support/:conversationId` matcher.
- Resolve the workspace slug, deployment edition/module availability, record identity, and all relations server-side. Scope every lookup to that workspace; matching UUID syntax alone is insufficient.
- Use one small allowlisted metadata response for HTML and image rendering. Do not let query parameters provide arbitrary title/description/image text.
- Metadata must be present in the first HTML response for the exact copied URL. Updating `document.title` after React loads does not implement this contract.
- Keep rendering in Go. Extend the existing app-host HTML routing layer for the current hosted frontend; provide the equivalent path in Community deployments. A Netlify edge source file alone does not run in the Community static server.
- Use revisioned image URLs and a bounded render cache. Permission, share revocation, deletion, visibility, workspace setting, and template changes must invalidate eligibility/content. Recheck eligibility on the image endpoint; a permanently public cached object must not bypass the setting.
- Requests for ineligible/missing resources or failed lookups return the type/page fallback. A failed lookup must not reuse another record’s image or reveal whether an inaccessible resource exists.
- Keep app previews noindex; noindex is not an access control. Do not log bearer tokens or place them in image text. Valid explicit share URLs may retain their share token as the link identity; authentication/callback tokens must not appear in metadata URLs or image cache keys.
- The Go renderer outputs the designed image. Communication apps independently decide how much metadata/image to display and may retain older previews; changing Helpin’s setting cannot retract an already-copied image.

## Findings from the original audit

1. Before implementation, only `/share/:token` received dynamic app HTML metadata, using a document title with a static image. The [transformer](../../frontend/netlify/edge-functions/share-meta.ts) now supports ordinary workspace URLs, portal routes, and `/shared/:token` through the Go metadata endpoint.
2. The [route title helper](../../frontend/src/lib/pageTitle.ts) is a client-side fallback and is incomplete for this purpose. It is not a reusable server-side preview registry today.
3. Support conversation descriptions in the mockup are sample copy. The model has message previews, including internal-message markers, but no dedicated description. Those message fields will not populate external metadata under this contract.
4. Deals also lack a standard description. Agents have a role and instructions, not a prose description. Agent session `summary` exists in a DTO, but the inspected constructor does not populate it; it is not a reliable preview source.
5. Tasks can have multiple owners, objectives can have multiple teams, and docs owners differ from authors. The display rules above account for those cases.
6. Browser-copy links cannot identify record panels that only exist in local state. The existing copy helpers or URL selectors are required for those records; no metadata renderer can recover absent browser state.
7. The earlier mockup contains sample fields and a property-prefixed metadata description. The implementation uses this finalized contract; this implementation does not update that historical mockup.

## Audit scope

The original audit was read-only source analysis plus this reference document. All route and model references point to the reviewed checkout. The route catalog is a source inventory, not a claim that all routes are available in every edition or workspace. Subsequent implementation remained local and did not read customer records or change any deployed workspace setting.

## Implementation checklist

Approved for implementation after the audit. Keep this as the single task document.

- [x] Add a default-on workspace preference and admin/owner control in General settings. Description: “Anyone with a link can see its title, description, and key details.”
- [x] Add a shared route registry, scoped preview repository, and metadata service implementing the profiles, aliases, selectors, and visibility rules above.
- [x] Render bounded PNG cards in Go with embedded fonts, readable colors, labeled fields, and a bounded cache that cannot bypass current eligibility.
- [x] Serve metadata and image endpoints; inject tags into the first app HTML response on Netlify and Community. Preserve the existing SPA boot, assets, public shares, and normal navigation.
- [x] Review the implementation against this route inventory, compile the affected backend/frontend code, and update this document with actual coverage and limitations. Do not run tests, per the user's instruction.

Existing behavior to preserve: `/share/:token` titles and revoked-token fallbacks; app SPA assets and auth redirects; all legacy navigation aliases; existing workspace edit permissions and save behavior; explicit public grants independent of workspace preview preference. No new external rendering service, production data access, or deployment is part of this implementation.

## Implementation and rollout

The [route registry](../../server/internal/linkpreview/routes.go) and [page registry](../../server/internal/linkpreview/pages.go) implement the profiles, aliases, and URL selectors above. The [preview service](../../server/internal/service/link_preview.go) resolves fresh eligibility for both metadata and image requests. The [repository](../../server/internal/repository/link_preview.go) and its module-specific files select permitted fields with workspace-scoped relations. Support preview runs remain creator-only, including through explicit public shares. Unknown run targets use a generic fallback.

The [Go renderer](../../server/internal/linkpreview/render.go) produces 1200×630 PNGs with up to three labeled properties. It adds Go shaping/rasterization dependencies and [bundled open-source fonts](../../server/internal/linkpreview/fonts/README.md), without a browser, Node rendering process, Vercel service, or remote font fetch. Fonts cover Latin, Arabic, Devanagari, CJK and monochrome emoji; unsupported glyphs can still use the font's missing-glyph symbol. Chinese glyph forms use Noto Sans SC. Rendering is limited to two simultaneous images, with a maximum 16 MiB/64-card cache. HTTP responses disallow CDN/browser caching; the image endpoint rechecks current eligibility before using cached bytes. Communication apps can still retain their own earlier previews.

Apply [migration `202611100004`](../../server/internal/dbmigrate/sql/202611100004_workspace_link_previews.sql) as part of the normal deployment. It enables previews for existing and new workspaces without resetting later opt-outs. Workspace admins/owners change the preference under General settings using the existing Save action. The toggle controls ordinary app links; active explicit public-share links retain their separate public grant.

Delivery uses the existing hosting paths:

- Netlify's [edge transformer](../../frontend/netlify/edge-functions/share-meta.ts) obtains escaped metadata from Go for supported app paths and proxies the image endpoint when the image uses the app origin. Unknown deploy-preview hosts keep the static shell.
- Community's [Nginx configuration](../../community/images/nginx.conf) requests the transformed shell from Go. Compose sets `APP_INDEX_URL=http://helpin-frontend/index.html` so the API fetches the raw shell without recursion.
- The standalone [frontend image](../../frontend/Dockerfile) accepts `HELPIN_API_ORIGIN` for its internal API proxy. Set the backend's `APP_INDEX_URL` to a reachable raw `/index.html` URL when its normal `APP_BASE_URL` cannot be reached internally. `APP_BASE_URL` and optional `PUBLIC_API_BASE_URL` remain the externally reachable origins used in metadata.
- Vite development uses the same Go metadata endpoint and has a local image proxy. No live development server was restarted for this change.

If the API or shell lookup fails, the frontend keeps its static SPA fallback. New record routes need an explicit registry mapping before they expose record details; future unknown routes remain generic. Pages whose selected record exists only in local React state cannot generate a record-specific preview from a copied browser URL.

Verification on 2026-11-10: Community API compilation passed with `CGO_ENABLED=0`; Enterprise API compilation passed with `-tags ee`. Frontend application and Vite configuration TypeScript checks passed with `--noEmit`. Documentation naming/link checks passed (185 local links), and `git diff --check` passed. Existing fixtures were adjusted for the new workspace column and metadata contract. No tests were run. Database queries, migrations, PNG appearance, Nginx routing, crawler behavior, and live Slack previews have not been exercised at runtime; compilation and source review do not establish those outcomes. No deployment or production migration was performed.
