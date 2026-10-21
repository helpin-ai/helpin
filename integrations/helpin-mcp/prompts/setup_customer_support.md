# Set up customer support

Call `get_support_setup` and confirm the connected workspace matches the user's
intended workspace. Follow the returned instructions and reuse existing settings.
Ask only for missing channel, ownership, knowledge, and AI decisions.

Use available MCP operations or the returned browser handoff links. If browser
access is unavailable, give the next link and action to the user and resume after
they finish. Browser writes require authorization and the correct signed-in
identity; do not bypass denied MCP operations through another account. Users
handle credentials, OAuth consent, and external approvals themselves.

Re-read setup after saved changes. Obtain approval before publishing content,
sending test messages, or enabling live AI replies. Test the chosen channels and
human handoff, and report observed outcomes separately from configuration checks.
