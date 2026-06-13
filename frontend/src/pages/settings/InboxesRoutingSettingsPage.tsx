import { SupportEmailForwardingTab, SupportEmailSendersTab, ConversationRoutingTab } from '@/components/settings';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { SettingsPageFrame } from './SettingsPageFrame';

const TABS = [
  { value: 'inboxes', label: 'Inboxes & Routing' },
  { value: 'email', label: 'Email Forwarding' },
  { value: 'senders', label: 'Sending Domains' },
] as const;

export type InboxesRoutingTab = (typeof TABS)[number]['value'];

export function InboxesRoutingSettingsPage({
  tab,
  onTabChange,
}: {
  tab: InboxesRoutingTab;
  onTabChange: (tab: string) => void;
}) {
  return (
    <SettingsPageFrame section="inboxes-routing">
      {({ workspaceId, currentWorkspaceName, currentWorkspaceWebsiteUrl }) => (
        <Tabs value={tab} onValueChange={onTabChange}>
          <TabsList variant="line">
            {TABS.map(({ value, label }) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="inboxes" className="mt-4">
            <ConversationRoutingTab workspaceId={workspaceId} />
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
