import { SupportChannels } from '@/components/settings/channels/SupportChannels'
import { SlackChannels } from '@/components/settings/channels/SlackChannels'
import { SettingsSection } from '@/components/settings/SettingsSection'
import { Button } from '@/components/ui/button'
import { Mail01Icon, Message01Icon } from '@/lib/icons'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportChannelsSettingsPage() {
  return (
    <SettingsPageFrame section="support-channels">
      {({ workspaceId, currentWorkspaceSlug, permissions }) => (
        <WorkspaceChannelSettings
          workspaceId={workspaceId}
          slug={currentWorkspaceSlug}
          hasSupport={permissions.canAccessModule('support')}
          hasPM={permissions.canAccessModule('pm')}
          manageSlack={permissions.has('settings.manage')}
          manageSupport={permissions.has('support.admin')}
        />
      )}
    </SettingsPageFrame>
  )
}

export function WorkspaceChannelSettings({
  workspaceId,
  slug,
  manageSlack,
  manageSupport,
  hasSupport = true,
  hasPM = true,
}: {
  workspaceId: string
  slug: string
  manageSlack: boolean
  hasSupport?: boolean
  hasPM?: boolean
  manageSupport: boolean
}) {
  return (
    <div className="space-y-4">
      {hasSupport && (
        <div className="divide-y rounded-lg border border-border/70">
          {[
            { label: 'Chat', icon: Message01Icon, section: 'chat-general' },
            {
              label: 'Email',
              icon: Mail01Icon,
              section: 'inboxes-routing?tab=email',
            },
          ].map(({ label, icon: Icon, section }) => (
            <div key={label} className="flex items-center gap-3 px-4 py-3">
              <Icon className="size-5" />
              <span className="flex-1 text-sm font-semibold">{label}</span>
              <Button size="sm" variant="ghost" asChild>
                <a href={`/w/${encodeURIComponent(slug)}/settings/${section}`}>
                  Manage
                </a>
              </Button>
            </div>
          ))}
        </div>
      )}
      {(hasSupport || hasPM) && manageSlack && (
        <SlackChannels
          workspaceId={workspaceId}
          slug={slug}
          hasSupport={hasSupport}
          hasPM={hasPM}
        />
      )}
      {hasSupport && (
        <SettingsSection title="Telegram">
          <div className="pt-3">
            <SupportChannels
              workspaceId={workspaceId}
              editable={manageSupport}
            />
          </div>
        </SettingsSection>
      )}
    </div>
  )
}
