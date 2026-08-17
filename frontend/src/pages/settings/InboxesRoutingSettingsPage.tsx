import { SupportEmailForwardingTab, SupportEmailSendersTab, ConversationRoutingTab } from '@/components/settings';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { SettingsPageFrame } from './SettingsPageFrame';

export const INBOXES_ROUTING_TABS = [
  { value: 'inboxes', label: 'Inboxes' },
  { value: 'routing', label: 'Routing & Assignment' },
  { value: 'email', label: 'Email Forwarding' },
  { value: 'senders', label: 'Sender Addresses' },
] as const;

export type InboxesRoutingTab = (typeof INBOXES_ROUTING_TABS)[number]['value'];

export function normalizeInboxesRoutingTab(value: unknown): InboxesRoutingTab {
  if (value === 'routing-assignment') return 'routing';
  return INBOXES_ROUTING_TABS.some((tab) => tab.value === value)
    ? value as InboxesRoutingTab
    : 'inboxes';
}

export function InboxesRoutingSettingsPage({
  tab,
  createInbox = false,
  onTabChange,
}: {
  tab: InboxesRoutingTab;
  createInbox?: boolean;
  onTabChange: (tab: string) => void;
}) {
  return (
    <SettingsPageFrame section="inboxes-routing">
      {({ workspaceId, currentWorkspaceName, currentWorkspaceWebsiteUrl }) => (
        <Tabs value={tab} onValueChange={onTabChange}>
          <TabsList variant="line">
            {INBOXES_ROUTING_TABS.map(({ value, label }) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="inboxes" className="mt-4">
            <ConversationRoutingTab workspaceId={workspaceId} section="inboxes" createInbox={createInbox} />
          </TabsContent>
          <TabsContent value="routing" className="mt-4">
            <ConversationRoutingTab workspaceId={workspaceId} section="routing" />
          </TabsContent>
          <TabsContent value="email" className="mt-4">
            <SupportEmailForwardingTab workspaceId={workspaceId} />
          </TabsContent>
          <TabsContent value="senders" className="mt-4">
            <SupportEmailSendersTab workspaceId={workspaceId} workspaceName={currentWorkspaceName} workspaceWebsiteUrl={currentWorkspaceWebsiteUrl} />
          </TabsContent>
        </Tabs>
      )}
    </SettingsPageFrame>
  );
}
