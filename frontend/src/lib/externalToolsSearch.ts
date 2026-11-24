type ExternalToolsSearch = {
  tab?: 'apps' | 'mcp';
  external_mcp_oauth?: 'connected' | 'failed';
  external_mcp_server_id?: string;
  external_mcp_resume?: 'failed';
  external_mcp_popup?: '1';
};

function optionalString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

export function validateExternalToolsSearch(search: Record<string, unknown>): ExternalToolsSearch {
  return {
    tab: search.tab === 'apps' || search.tab === 'mcp' ? search.tab : undefined,
    external_mcp_oauth: search.external_mcp_oauth === 'connected' || search.external_mcp_oauth === 'failed' ? search.external_mcp_oauth : undefined,
    external_mcp_server_id: optionalString(search.external_mcp_server_id),
    external_mcp_resume: search.external_mcp_resume === 'failed' ? 'failed' : undefined,
    external_mcp_popup: search.external_mcp_popup === '1' ? '1' : undefined,
  };
}
