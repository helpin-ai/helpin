const MCP_WRITE_SCOPE_BY_TOOLSET: Readonly<Record<string, string>> = {
  pm: 'helpin.pm.write',
  docs: 'helpin.docs.write',
  crm: 'helpin.crm.write',
  support: 'helpin.support.write',
  agents: 'helpin.agents.run',
};

// Publishing changes public content, so it counts as a write scope even though
// it is never added automatically when writes are enabled for a toolset.
export const MCP_DOCS_PUBLISH_SCOPE = 'helpin.docs.publish';

export function isMCPWriteScope(scope: string): boolean {
  return scope.endsWith('.write') || scope === 'helpin.agents.run' || scope === MCP_DOCS_PUBLISH_SCOPE;
}

export function hasMCPWriteScope(scopes: string[]): boolean {
  return scopes.some(isMCPWriteScope);
}

export function hasMCPWriteGrant(scopes: string[], toolsets: string[]): boolean {
  return toolsets.some((toolset) => {
    const writeScope = MCP_WRITE_SCOPE_BY_TOOLSET[toolset];
    if (writeScope && scopes.includes(writeScope)) return true;
    return toolset === 'docs' && scopes.includes(MCP_DOCS_PUBLISH_SCOPE);
  });
}

export type MCPConsentRestrictions = {
  allowed_scopes: string[];
  allowed_toolsets: string[];
  read_only_required: boolean;
};

export function getMCPConsentAccess(
  scopes: string[],
  toolsets: string[],
  restrictions: MCPConsentRestrictions | undefined,
  preferReadOnly: boolean,
) {
  if (!restrictions) {
    return { scopes: [], toolsets: [], readOnly: true, canRequestWrites: false };
  }

  const allowedScopes = scopes.filter((scope) => restrictions.allowed_scopes.includes(scope));
  const allowedToolsets = toolsets.filter((toolset) => restrictions.allowed_toolsets.includes(toolset));
  const canRequestWrites = !restrictions.read_only_required
    && hasMCPWriteGrant(allowedScopes, allowedToolsets);
  const readOnly = preferReadOnly || !canRequestWrites;

  return {
    scopes: readOnly ? allowedScopes.filter((scope) => !isMCPWriteScope(scope)) : allowedScopes,
    toolsets: allowedToolsets,
    readOnly,
    canRequestWrites,
  };
}

export function ensureMCPWriteScopes(
  scopes: string[],
  toolsets: string[],
  availableScopes: string[],
): string[] {
  const result = [...scopes];
  const available = new Set(availableScopes);

  for (const toolset of toolsets) {
    const writeScope = MCP_WRITE_SCOPE_BY_TOOLSET[toolset];
    if (writeScope && available.has(writeScope) && !result.includes(writeScope)) {
      result.push(writeScope);
    }
  }

  return result;
}
