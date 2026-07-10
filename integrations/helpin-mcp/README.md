# Helpin MCP workflow package

See the [complete Helpin Public MCP capability and operations guide](../../docs/HELPIN_PUBLIC_MCP.md) for the architecture, tool catalog, authorization model, UI, limits, and rollout controls.

Connect a remote MCP client to:

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
