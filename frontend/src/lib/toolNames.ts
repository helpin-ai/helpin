const HELPIN_MCP_PREFIX = 'mcp__helpin__';

export function canonicalToolName(name: unknown): string {
  return typeof name === 'string' ? name.trim().replace(new RegExp(`^${HELPIN_MCP_PREFIX}`), '') : '';
}

export function displayToolName(name: unknown): string {
  return canonicalToolName(name);
}

export function isToolName(name: unknown, canonical: string): boolean {
  return canonicalToolName(name) === canonical;
}

