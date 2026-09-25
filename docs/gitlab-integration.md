# Connect GitLab repositories

Use this guide to connect a GitLab.com or self-hosted GitLab account to a Helpin
organization, then make selected repositories available to workspaces. The
current integration uses **access tokens**, not an OAuth redirect flow.

## Connect an account

1. An organization owner or admin opens Git connections and selects **Connect
   GitLab**.
2. Enter the GitLab origin, such as `https://gitlab.com`, and select Personal,
   Group, or Project Access Token.
3. Supply the token and optional label and default commit-author details.
4. Connect, then enable the intended repositories in the workspace catalog.

The current UI requests `api`, `read_repository`, and `write_repository` scopes
and Maintainer access for the full branch, merge-request, and webhook workflow.
Actual operations remain limited by the token owner's GitLab access. Connecting
successfully verifies the token's current-user response; it does not prove that
every project operation will succeed.

The base URL defaults to `https://gitlab.com`. The backend accepts an HTTP or
HTTPS origin and strips paths, queries, and fragments because it calls GitLab's
`/api/v4` API from the root. Use HTTPS for a shared installation.

## Operator configuration

Configure `GIT_OAUTH_ENCRYPTION_KEY` for encrypted Git credential storage. The
variable retains its historical name even though this GitLab flow uses tokens.
The service requires a 32-byte decoded key. Generate a 64-character hex value:

```sh
openssl rand -hex 32
```

Keep the configured key stable while encrypted credentials exist. There are no
`GITLAB_CLIENT_ID`, `GITLAB_CLIENT_SECRET`, or GitLab OAuth callback setup steps
for this connection path.

## API and ownership

`POST /api/organizations/{id}/git/gitlab/connect` accepts:

| Field | Meaning |
| --- | --- |
| `base_url` | GitLab origin; defaults to GitLab.com |
| `token` | Required access token |
| `auth_type` | `personal_token` (default), `group_token`, or `project_token` |
| `label` | Optional connection label |
| `default_commit_author_name` / `default_commit_author_email` | Optional commit-author defaults |

The optional `workspace_id` query parameter selects a return workspace within
the organization. Credentials are stored encrypted at organization scope;
workspace repository selection remains a separate access boundary. If a token
expires, is revoked, or loses project access, update the connection before
expecting repository operations to work again.

## Implementation references

- [Connection UI](../frontend/src/components/settings/OrgGitConnectionsTab.tsx)
- [Request and response types](../server/internal/model/git.go)
- [HTTP handler](../server/internal/handler/git.go)
- [Token validation and credential storage](../server/internal/service/git.go)
- [GitLab API client](../server/internal/gitlab/client.go)
