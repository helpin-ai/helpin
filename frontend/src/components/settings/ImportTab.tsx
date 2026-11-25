import { useEffect, useState } from 'react';
import { ShortcutImportWizard } from '@/components/pm/ShortcutImportWizard';
import { HelpCenterImportSection } from '@/components/settings/HelpCenterImportSection';
import { NextraImportWizard } from '@/components/settings/NextraImportWizard';
import { IntercomImportWizard } from '@/components/settings/IntercomImportWizard';
import { PublicHelpCenterImportWizard } from '@/components/settings/PublicHelpCenterImportWizard';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ArrowRight01Icon, Globe02Icon, Loading01Icon } from '@/lib/icons';
import { docsImportService, isPublicHelpCenterImport, type ImportStatusResponse } from '@/lib/services/docsImportService';
import { pmImportService } from '@/lib/services/pmImportService';
import shortcutIcon from '@/assets/import/shortcut.svg';
import jiraIcon from '@/assets/import/jira.svg';
import linearIcon from '@/assets/import/linear.svg';
import helpscoutIcon from '@/assets/import/helpscout.svg';
import zendeskIcon from '@/assets/import/zendesk.svg';
import nextraIcon from '@/assets/import/nextra.svg';
import intercomIcon from '@/assets/import/intercom.svg';

const IMPORT_SOURCES = [
  {
    key: 'shortcut' as const,
    title: 'Shortcut',
    description: 'Import tasks, epics, workflows, and members from Shortcut.',
    icon: shortcutIcon,
    comingSoon: false,
  },
  {
    key: 'jira' as const,
    title: 'Jira',
    description: 'Import issues, projects, and workflows from Jira.',
    icon: jiraIcon,
    comingSoon: true,
  },
  {
    key: 'linear' as const,
    title: 'Linear',
    description: 'Import issues, projects, and cycles from Linear.',
    icon: linearIcon,
    comingSoon: true,
  },
];

export function ImportTab({ workspaceId, editable = true }: { workspaceId: string; editable?: boolean }) {
  const [selected, setSelected] = useState<string | null>(null);
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [activeDocsImports, setActiveDocsImports] = useState<ImportStatusResponse[]>([]);
  const [shortcutRunning, setShortcutRunning] = useState(false);

  useEffect(() => {
    workspacesService.listMembers(workspaceId).then(({ data }) => {
      if (data) setMembers(data);
    });
  }, [workspaceId]);

  useEffect(() => {
    let active = true;
    const loadRunningImports = async () => {
      const [docsResult, shortcutResult] = await Promise.all([
        docsImportService.listJobs(workspaceId),
        pmImportService.listShortcutStatuses(workspaceId),
      ]);
      if (!active) return;
      setActiveDocsImports(
        (docsResult.data ?? []).filter(
          (job) => (job.status === 'pending' || job.status === 'running') && Boolean(job.space_id),
        ),
      );
      setShortcutRunning(
        (shortcutResult.data ?? []).some(
          (job) => job.status === 'pending' || job.status === 'scanning' || job.status === 'processing',
        ),
      );
    };
    void loadRunningImports();
    const timer = window.setInterval(loadRunningImports, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [workspaceId]);

  const helpScoutActiveImport = activeDocsImports.find((job) => job.source === 'helpscout');
  const nextraActiveImport = activeDocsImports.find((job) => job.source === 'nextra');
  const publicActiveImport = activeDocsImports.find((job) => isPublicHelpCenterImport(job.source));
  const zendeskActiveImport = publicActiveImport?.source === 'zendesk_public' ? publicActiveImport : undefined;

  if (selected === 'shortcut') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <ShortcutImportWizard workspaceId={workspaceId} members={members} />
      </div>
    );
  }

  if (selected === 'helpscout') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <HelpCenterImportSection workspaceId={workspaceId} editable={editable} />
      </div>
    );
  }

  if (selected === 'intercom') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <IntercomImportWizard workspaceId={workspaceId} />
      </div>
    );
  }

  if (selected === 'public-help-center') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <PublicHelpCenterImportWizard workspaceId={workspaceId} activeJobId={publicActiveImport?.id} />
      </div>
    );
  }

  if (selected === 'zendesk') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <PublicHelpCenterImportWizard workspaceId={workspaceId} activeJobId={zendeskActiveImport?.id} zendesk />
      </div>
    );
  }

  if (selected === 'nextra') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ArrowRight01Icon className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <NextraImportWizard workspaceId={workspaceId} />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Support */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Support</h3>
        <div className="overflow-hidden rounded-lg border border-border bg-card">
          <button
            type="button"
            disabled={!editable}
            onClick={() => setSelected('intercom')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
              <img src={intercomIcon} alt="Intercom" className="h-5 w-5" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">Intercom</p>
              <p className="text-sm text-muted-foreground">Import your Help Center and past conversations with their replies, notes and tags.</p>
            </div>
            <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          </button>
        </div>
      </div>

      {/* Help Center */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Help Center</h3>
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">By address, no key needed</p>
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <button
              type="button"
              disabled={!editable}
              onClick={() => setSelected('public-help-center')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <Globe02Icon className="h-5 w-5 text-muted-foreground" aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Public help center</p>
                <p className="text-sm text-muted-foreground">Copy a help center from Intercom, Zendesk, Crisp, Freshdesk, Help Scout, HubSpot or a docs site by its address.</p>
              </div>
              {publicActiveImport ? (
                <Badge className="gap-1.5 bg-blue-100 text-blue-700 hover:bg-blue-100">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Running
                  {publicActiveImport.total > 0 &&
                    ` · ${publicActiveImport.completed + publicActiveImport.failed}/${publicActiveImport.total}`}
                </Badge>
              ) : (
                <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
          </div>
        </div>
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">With an API key</p>
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <button
              type="button"
              disabled={!editable}
              onClick={() => setSelected('zendesk')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <img src={zendeskIcon} alt="Zendesk" className="h-5 w-5 dark:invert" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Zendesk</p>
                <p className="text-sm text-muted-foreground">Import articles, sections and images with an admin email and API token, including from a help center that asks visitors to sign in.</p>
              </div>
              {zendeskActiveImport ? (
                <Badge className="gap-1.5 bg-blue-100 text-blue-700 hover:bg-blue-100">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Running
                  {zendeskActiveImport.total > 0 &&
                    ` · ${zendeskActiveImport.completed + zendeskActiveImport.failed}/${zendeskActiveImport.total}`}
                </Badge>
              ) : (
                <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
            <button
              type="button"
              disabled={!editable}
              onClick={() => setSelected('helpscout')}
              className="flex w-full items-center gap-4 border-t border-border px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <img src={helpscoutIcon} alt="HelpScout" className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">HelpScout</p>
                <p className="text-sm text-muted-foreground">Import articles, categories, and images from HelpScout Docs.</p>
              </div>
              {helpScoutActiveImport ? (
                <Badge className="gap-1.5 bg-blue-100 text-blue-700 hover:bg-blue-100">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Running
                  {helpScoutActiveImport.total > 0 &&
                    ` · ${helpScoutActiveImport.completed + helpScoutActiveImport.failed}/${helpScoutActiveImport.total}`}
                </Badge>
              ) : (
                <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
          </div>
        </div>
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">From a file</p>
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <button
              type="button"
              disabled={!editable}
              onClick={() => setSelected('nextra')}
              className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <img src={nextraIcon} alt="Nextra" className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">Nextra</p>
                <p className="text-sm text-muted-foreground">Import MDX docs, navigation, images, and redirects from a Nextra repo zip.</p>
              </div>
              {nextraActiveImport ? (
                <Badge className="gap-1.5 bg-blue-100 text-blue-700 hover:bg-blue-100">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Running
                  {nextraActiveImport.total > 0 &&
                    ` · ${nextraActiveImport.completed + nextraActiveImport.failed}/${nextraActiveImport.total}`}
                </Badge>
              ) : (
                <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Project Management */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Project Management</h3>
        <div className="overflow-hidden rounded-lg border border-border bg-background">
          {IMPORT_SOURCES.map((source, idx) => (
            <button
              key={source.key}
              type="button"
              disabled={source.comingSoon || !editable}
              onClick={() => setSelected(source.key)}
              className={cn(
                'flex w-full items-center gap-4 px-4 py-4 text-left transition-colors',
                source.comingSoon ? 'cursor-not-allowed opacity-60' : 'hover:bg-muted/40',
                idx > 0 && 'border-t border-border',
              )}
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <img src={source.icon} alt={source.title} className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{source.title}</p>
                <p className="text-sm text-muted-foreground">{source.description}</p>
              </div>
              {source.key === 'shortcut' && shortcutRunning ? (
                <Badge className="gap-1.5 bg-blue-100 text-blue-700 hover:bg-blue-100">
                  <Loading01Icon className="h-3 w-3 animate-spin" />
                  Running
                </Badge>
              ) : source.comingSoon ? (
                <Badge variant="secondary" className="text-xs">Coming Soon</Badge>
              ) : (
                <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
