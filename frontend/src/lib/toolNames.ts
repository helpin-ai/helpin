const HELPIN_MCP_PREFIX = 'mcp__helpin__';

// Deliberately narrower than the server's agentcontract.CanonicalToolName, which
// also folds legacy names onto their replacements (read_file -> read_files,
// ripgrep -> repository_search, find_symbol -> read_symbol, find_callers /
// find_callees -> trace_symbol). The server aliases because it is deciding what
// an agent may call *now*; this runs over stored transcripts and must render what
// was actually called *then*. Folding here would relabel a historical read_file
// call as read_files and hand it read_files' argument shape, which it never had.
// So: strip the MCP prefix only, and keep the legacy branches in
// toolCallPresentation.ts alive for old runs.
export function canonicalToolName(name: unknown): string {
  return typeof name === 'string' ? name.trim().replace(new RegExp(`^${HELPIN_MCP_PREFIX}`), '') : '';
}

export function displayToolName(name: unknown): string {
  return canonicalToolName(name);
}

export function isToolName(name: unknown, canonical: string): boolean {
  return canonicalToolName(name) === canonical;
}

