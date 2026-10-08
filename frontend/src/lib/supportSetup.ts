import type { MCPDashboard } from './mcpTypes';
import type { SupportSetupGuide } from './setupTypes';

export function supportSetupConnectionIssue(dashboard: MCPDashboard): string | null {
  if (!dashboard.platform_enabled) return 'MCP is unavailable on this deployment. You can continue using the setup links below.';
  if (!dashboard.can_use_mcp || !dashboard.policy.enabled) return 'MCP access is disabled for this workspace. Ask a workspace admin to enable it in MCP access.';
  if (dashboard.support_setup_available === false || !dashboard.available_toolsets.includes('support') || !dashboard.available_scopes.includes('helpin.support.read')) return 'Support setup is unavailable through MCP on this deployment. Continue using the setup links below.';
  if (!dashboard.policy.allowed_toolsets.includes('support') || !dashboard.policy.allowed_scopes.includes('helpin.support.read')) return 'Support read access is restricted. Ask a workspace admin to allow it in MCP access.';
  if (!dashboard.mcp_url) return 'The MCP server URL is unavailable. Check MCP access before connecting.';
  return null;
}

export function buildSupportSetupPrompt(guide: SupportSetupGuide, slug: string, origin: string): string {
  const base = `${origin}/w/${encodeURIComponent(slug)}`;
  const links = guide.steps.filter((step) => step.path).map((step) => `- ${step.title}: ${base}${step.path}`).join('\n');
  return `Help me set up customer support in Helpin workspace ${guide.workspace_id}.\nWorkspace: ${base}\n\nStart by calling get_support_setup through my Helpin MCP connection. Confirm its workspace_id matches ${guide.workspace_id}; stop if it does not. If the tool is missing, refresh the MCP tools and check Support read access and my support administration permission. Do not treat an access denial as permission to use another account.\n\n${guide.instructions}\n\nBrowser handoffs (recheck current status before changing anything):\n${links}`;
}
