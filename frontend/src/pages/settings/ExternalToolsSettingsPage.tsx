import { useState } from 'react';
import { Navigate, useLocation } from '@tanstack/react-router';
import { ExternalMCPAddButton, ExternalMCPConnections } from '@/components/automation/ExternalMCPConnections';
import { SupportChannels } from '@/components/settings/channels/SupportChannels';
import { SlackChannels } from '@/components/settings/channels/SlackChannels';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { externalToolsAccess } from '@/lib/settingsSections';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

type Tab = 'apps' | 'mcp';

export function ExternalToolsSettingsPage(props: { requestedTab?: Tab; onTabChange: (tab: Tab) => void }) {
  return (
    <SettingsPageFrame section="external-tools">
      {context => <ExternalToolsTabs {...context} {...props} />}
    </SettingsPageFrame>
  );
}

function ExternalToolsTabs({ workspaceId, currentWorkspaceSlug, permissions, requestedTab, onTabChange }: SettingsPageContext & {
  requestedTab?: Tab;
  onTabChange: (tab: Tab) => void;
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const feedInbox = useLocation({ select: location => new URLSearchParams(location.searchStr).get('slack_inbox') ?? undefined });
  const telegramShortcut = useLocation({ select: location => location.hash === 'telegram' });
  const { canManageSettings } = permissions;
  const available = externalToolsAccess(canManageSettings, permissions.permissionSet, permissions.modules);
  if (!available.apps && !available.mcp) {
    return <p className="text-sm text-muted-foreground">You don’t have access to external tools in this workspace.</p>;
  }
  const defaultTab = available.apps ? 'apps' : 'mcp';
  if (requestedTab && !available[requestedTab]) {
    return <Navigate to="/w/$slug/settings/external-tools" params={{ slug: currentWorkspaceSlug }} search={{ tab: defaultTab }} replace />;
  }
  const tab = requestedTab ?? defaultTab;
  const slack = available.slack && (
    <SlackChannels key="slack" workspaceId={workspaceId} slug={currentWorkspaceSlug}
      canManageFeed={permissions.has('support.admin')} initialInbox={feedInbox} hasCRM={permissions.canAccessModule('crm') && permissions.has('crm.read')} hasSupport={permissions.canAccessModule('support')} hasPM={permissions.canAccessModule('pm')} />
  );
  const telegram = available.telegram && <SupportChannels key="telegram" workspaceId={workspaceId} editable />;
  return (
    <Tabs value={tab} onValueChange={value => onTabChange(value === 'mcp' ? 'mcp' : 'apps')}>
      <div className="flex items-center justify-between gap-3">
        <TabsList variant="line" aria-label="External tools">
          {available.apps && <TabsTrigger value="apps">Apps</TabsTrigger>}
          {available.mcp && <TabsTrigger value="mcp">MCP servers</TabsTrigger>}
        </TabsList>
        {tab === 'mcp' && canManageSettings && <ExternalMCPAddButton workspaceId={workspaceId} onAdd={() => setCreateOpen(true)} />}
      </div>
      {available.apps && (
        <TabsContent value="apps" className="mt-4 space-y-4">
          {telegramShortcut ? [telegram, slack] : [slack, telegram]}
        </TabsContent>
      )}
      {available.mcp && (
        <TabsContent value="mcp" className="mt-4">
          <ExternalMCPConnections
            workspaceId={workspaceId}
            workspaceSlug={currentWorkspaceSlug}
            canManageSettings={canManageSettings}
            canEditCustomAgents={permissions.hasAny('pm.edit', 'docs.edit', 'crm.edit', 'support.edit')}
            canEditPresetAgents={permissions.has('pm.edit')}
            createOpen={createOpen}
            onCreateOpenChange={setCreateOpen}
          />
        </TabsContent>
      )}
    </Tabs>
  );
}
