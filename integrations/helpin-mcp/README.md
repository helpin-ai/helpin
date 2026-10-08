# Helpin MCP workflow package

See the [complete Helpin Public MCP capability and operations guide](../../docs/public-mcp-server.md) for the architecture, tool catalog, authorization model, UI, limits, and rollout controls.

This integration is implemented for controlled beta. The endpoint below is the
configured hosted destination, not a guarantee that the service is enabled for
your deployment or workspace. Check the linked guide's enablement requirements
before connecting. Self-hosted clients use their operator's configured MCP URL.

Connect an authorized remote MCP client to:

```text
https://mcp.helpin.ai/mcp
```

Use OAuth for an individual user. Use a restricted service token only for a named, headless workflow. Each credential is bound to one Helpin workspace, and the server reapplies workspace policy, membership, RBAC, module access, scopes, and toolsets on every call.

Client configuration uses the same remote URL:

```json
{
  "mcpServers": {
    "helpin": {
      "url": "https://mcp.helpin.ai/mcp"
    }
  }
}
```

The `skills/` folders are Codex-compatible adapters. The Markdown files in `prompts/` are portable workflow sources for clients that support MCP prompts or their own command format. No package contains a credential.

Safe defaults:

- start with context, PM read, Docs read, and agent read toolsets
- keep the connection read-only unless bounded mutations are needed
- use a stable `idempotency_key` on every mutation
- poll `get_agent_run`; v1 does not send completion webhooks
- stop before customer-visible sends, Docs publishing, deletion, member/security changes, or integration management

## Support onboarding

Use the [support onboarding skill](skills/helpin-support-onboarding/SKILL.md) to
inspect existing support setup and continue through browser handoffs. Connect as
a support administrator with Support read access; add Docs scopes only when
needed. `get_support_setup` returns current checks and instructions. The external
client must supply browser tools, or the user follows the returned links manually.
This workflow does not grant browser permissions or automatically enable live AI.

The manifest's excluded actions describe direct MCP operations. Support onboarding
can guide separately authorized browser actions through the existing Helpin UI;
it does not expand a token's scopes or bypass workspace policy.
