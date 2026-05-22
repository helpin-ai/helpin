# GitLab.com Integration

Helpin uses one Helpin-owned GitLab.com OAuth application for multi-tenant GitLab access. Customers do not create their own GitLab OAuth applications for the default GitLab.com flow.

## Product Model

- Helpin registers and operates the GitLab.com OAuth application.
- A Helpin organization admin clicks `Authorize GitLab`.
- GitLab asks that admin to authorize the Helpin OAuth application.
- Helpin stores the resulting OAuth credential encrypted and scoped to the Helpin organization.
- Helpin lists projects visible to the authorizing GitLab user.
- Workspace admins then enable specific repositories in Helpin's workspace repository catalog.

This differs from GitHub. GitHub uses a Helpin GitHub App installation with installation-scoped access. GitLab.com OAuth is user-delegated, so the authorizing GitLab user controls what projects Helpin can access.

## Helpin App Setup

Create the OAuth application in GitLab.com under a Helpin-controlled account or group:

- Name: `Helpin`
- Redirect URI: `https://<api-host>/api/git/gitlab/callback`
- Confidential: enabled
- Scopes:
  - `api`
  - `read_user`
  - `read_repository`
  - `write_repository`

Configure the backend:

```bash
GITLAB_CLIENT_ID=<application-id>
GITLAB_CLIENT_SECRET=<application-secret>
GITLAB_OAUTH_REDIRECT_URL=https://<api-host>/api/git/gitlab/callback
GITLAB_BASE_URL=https://gitlab.com
GIT_OAUTH_ENCRYPTION_KEY=<64-hex-character-key>
```

Generate the encryption key with:

```bash
openssl rand -hex 32
```

## Customer Access Requirements

The GitLab user who authorizes Helpin must have access to the projects the customer wants to use.

Recommended customer setup:

- Create a dedicated GitLab service or bot user.
- Add it to the relevant GitLab group or projects.
- Grant `Maintainer` where Helpin should create project webhooks.
- Grant `Developer` or higher where Helpin should push branches or create merge requests.

`Reporter` access is enough for read-only visibility, but not for delivery automation.

## Operational Notes

- If the authorizing GitLab user loses project access, Helpin loses that access too.
- Reconnecting GitLab in Helpin updates the organization credential.
- Repository selection inside Helpin remains the workspace-level access boundary for agents, automations, and delivery defaults.
- Customer-owned OAuth applications are not part of the default GitLab.com flow. Treat that as a future enterprise/self-managed option only.
