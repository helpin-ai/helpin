---
name: helpin-support-onboarding
description: Configure or resume customer support onboarding in Helpin using MCP inspection and browser handoffs. Use for support email, website chat, knowledge, team inboxes, routing, AI behavior, and testing a new support setup.
---

# Set up customer support

Use the workspace selected by the user. This workflow requires support administration permission and an MCP connection with the Support toolset and `helpin.support.read` scope. Browser tools are supplied by the client, not Helpin MCP.

1. Call `get_support_setup` with no arguments. Check `workspace_id` against the requested workspace. Follow the returned `instructions`; they are the current server-maintained workflow. Do not infer completion from a previous response.
2. Reuse existing configuration. Ask only for missing choices: email, website chat, or both; team ownership; selected knowledge sources; and whether AI should reply. Do not force every channel or AI activation.
3. Prefer supported MCP operations for documents and other available work. Request only needed scopes. For configuration steps, follow the returned browser links in `steps[].path`. If browser tools are unavailable, give the user the specific link and action, then resume inspection afterward.
4. A browser session has its own identity and permissions. Confirm the workspace and user before changes. A read-only MCP grant or service token does not authorize browser writes. Obtain authorization for UI changes; never bypass a denied operation by changing interfaces or accounts.
5. The user completes sign-in, OAuth consent, secret entry, and external approvals. Do not collect credentials in chat. Installing website chat requires repository access or the website owner's assistance.
6. Save through existing UI controls, then call `get_support_setup` again. Preserve settings the user did not ask to change. A failed step must not cause successful steps to be recreated.
7. Review knowledge, AI behavior, routing, and human handoff. Obtain approval before publishing content, sending test messages, or enabling customer-facing AI replies. A successful configuration check does not establish delivery or answer quality.
8. With approval, test the chosen channels and inspect the resulting conversation, destination inbox, response delivery, and human handoff. Report observed evidence, remaining blockers, and untested behavior. The final test remains `unable_to_verify` in the configuration snapshot because the snapshot does not attest to an end-to-end test.

If the tool is missing, refresh the client's MCP catalog and check workspace policy, Support read scope, support administration permission, and deployment enablement. Do not invent tool names or claim access has been granted.
