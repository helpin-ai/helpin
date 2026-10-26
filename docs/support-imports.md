# Importing from Intercom

This page is for admins moving to Helpin from Intercom, and for contributors
changing the import. **Settings → Import & export → Intercom** imports two
things from one access token:

- the Help Center, which needs the `docs.import` permission;
- past conversations, which need `support.admin`.

The code is in:
- `server/internal/intercom` (API client and a fake API for tests);
- `server/internal/service/docs_import_intercom.go`;
- `server/internal/service/support_import_intercom*.go`.

## Access token

Create an app in Intercom's Developer Hub and copy its access token. It needs
read access to articles, conversations and contacts. Tokens only work in the
data region of their workspace, so choose United States, Europe or Australia
to match.

The token is stored encrypted on the import job:
- the Help Center import uses `PM_IMPORT_ENCRYPTION_KEY`, or
  `CRM_ENCRYPTION_KEY` when that is unset;
- the conversation import uses the same keys.

Operators can point all regions at another host, such as a proxy, with
`INTERCOM_API_BASE_URL`.

## Help Center

- Collections become Docs collections in the chosen space, nested as in
  Intercom.
- Articles become documents with their images copied.
- Every collection and article URL redirects to its new page.
- Published articles stay published unless you choose **All as drafts**;
  **Publish all** publishes drafts too.
- Articles not in a collection are imported uncategorized.
- Only the default language is imported.

The import runs as a durable workflow and stores a checkpoint per article, so
it resumes after a restart. Running it again imports only articles that are
not in the space yet.

## Conversations

Conversations created on or after the chosen date are imported, oldest first:

| Intercom | Helpin |
| --- | --- |
| Open or snoozed | Open, and already read by the team |
| Closed | Resolved at the last close time |
| "Priority" flag | High priority |
| Email conversation | Email channel; others use the chat channel |
| Contact name and email | Customer name and email |
| Replies (customer, teammate, bot) | Public replies. Teammates match Helpin members by email; bots appear as AI. |
| Notes | Internal notes |
| Tags | Tags, created by name when missing |

Not imported:
- attachments;
- assignments;
- status-change events;
- ratings.

Imported conversations go to the chosen inbox. Imported chats have no chat
session, so replies to them go by email.

Each imported conversation, with its messages and tags, is written in one
transaction together with a record of its Intercom ID:
- running an import again skips conversations already imported;
- a resumed import counts its own work correctly.

A worker in the API processes 25 conversations at a time and saves its
position after each batch, so an import survives restarts. One Intercom
import runs per workspace at a time; **Stop import** keeps what was already
imported.

Imported history is kept out of live features:
- it never gets SLA targets;
- it sends no webhooks, and neither do the messages copied with it;
- new replies after the import do send webhooks.
