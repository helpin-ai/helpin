const MCP_WRITE_SCOPE_BY_TOOLSET: Readonly<Record<string, string>> = {
  pm: 'helpin.pm.write',
  docs: 'helpin.docs.write',
  crm: 'helpin.crm.write',
  agents: 'helpin.agents.run',
};

export function isMCPWriteScope(scope: string): boolean {
  return scope.endsWith('.write') || scope === 'helpin.agents.run';
}

export function hasMCPWriteScope(scopes: string[]): boolean {
  return scopes.some(isMCPWriteScope);
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
