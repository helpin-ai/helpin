# Customer.io Workspace Lifecycle Data Contract

Helpin identifies people globally by `user_id` and keeps lifecycle context on the workspace relationship or event. A person may belong to multiple workspaces with different roles.

## Customer.io objects

| Object | `object_type_id` | Key attributes |
|---|---:|---|
| Workspace | `1` | `workspace_id`, `organization_id`, `plan`, `billing_status`, `trial_ends_at`, `trialing`, `modules`, `credits_remaining` |
| Organization | `2` | `organization_id`, `workspace_count`, `trialing_workspace_count`, `paid_workspace_count`, `highest_plan`, `monthly_due_cents` |

Workspace relationship attributes:

- `workspace_role`
- `membership_status`
- `team_membership_count`
- `relationship_source`
- `relationship_synced_at`

Organization relationship attributes:

- `organization_role`
- `relationship_source`
- `relationship_synced_at`

## Event properties

Every workspace event should include `workspace_id`, `organization_id`, `workspace_role`, `membership_status`, and the relevant billing/module properties. Events must not contain customer message bodies, document content, credentials, tokens, or payment card data.

Use `workspace_id` and event properties for behavioral segmentation. Do not put a workspace’s plan or role on the global person profile.

## Campaign Liquid context

Workspace-triggered or workspace-scoped events should render the affected workspace from event attributes, for example:

```liquid
{{event.workspace_name}}
{{event.trial_ends_at}}
```

Campaigns must suppress recipients whose relationship to the affected workspace is revoked or inactive.
