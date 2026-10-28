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

The token is stored encrypted on the import job and deleted when the import
finishes, fails or is stopped:
- the Help Center import uses `PM_IMPORT_ENCRYPTION_KEY`, or
  `CRM_ENCRYPTION_KEY` when that is unset;
- the conversation import uses the same keys.

The wizard checks the Help Center and conversations separately. A token that
can read only one of them still imports that one.

Operators can point all regions at another host, such as a proxy, with
`INTERCOM_API_BASE_URL`.

## Help Center

- Collections become Docs collections in the chosen space, nested as in
  Intercom. The space must be able to publish to the Help Center, and only
  one import into a space runs at a time.
- A collection is reused only when an earlier import created it: the same
  name under the same parent. A collection whose slug is already taken gets
  a numbered slug, such as `faq-2`.
- Articles become documents with their images copied.
- Every collection and article URL redirects to its new page.
- Published articles stay published unless you choose **All as drafts**;
  **Publish all** publishes drafts too.
- Articles not in a collection are imported uncategorized.
- Only the default language is imported.

The import runs as a durable workflow and stores a checkpoint per article, so
it resumes after a restart. Running it again imports only articles that are
not in the space yet. An article that fails part way is removed, so the next
run imports it cleanly.

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
| Tags | Tags, created by name when missing; names are cut to 60 characters |
| Attachments | A line naming each file; the files are not copied |

Not imported:
- attachment files;
- assignments;
- status-change events;
- ratings;
- parts beyond the 500 that Intercom returns for one conversation.

Imported conversations go to the chosen inbox. Imported chats have no chat
session, so replies to them go by email.

Each imported conversation, with its messages and tags, is written in one
transaction together with a record of its Intercom ID:
- running an import again skips conversations already imported;
- a resumed import counts its own work correctly.

A worker in the API processes 25 conversations at a time and saves its
position after each batch, so an import survives restarts. Imports in
different workspaces take turns batch by batch. One Intercom import runs per
workspace at a time; **Stop import** keeps what was already imported.

When Intercom cannot be reached, the batch is retried after 1, 2, 4, 8, 16
and then 30 minutes, and the import stops after 8 failures in a row. It
stops at once when Intercom refuses the token or the request.

Imported history is kept out of live features:
- it is never judged against SLA targets. An imported conversation that is
  open, or that a customer reopens later, gets the targets of its matching
  policy counted from that moment, the cutover. A conversation a teammate
  already answered in Intercom has no first-response target, while a reply
  made in Helpin after the import counts as usual; the customer's
  open question gets a next-response target from the cutover. Closed
  imports get no targets;
- it sends no webhooks, and neither do the messages copied with it;
- its messages are not tagged or translated by AI;
- new replies after the import do send webhooks.

Imported conversations do count in Support reports, on the dates they
happened in Intercom.
