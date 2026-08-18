import { useEffect, useState } from 'react';
import { ShortcutImportWizard } from '@/components/pm/ShortcutImportWizard';
import { HelpCenterImportSection } from '@/components/settings/HelpCenterImportSection';
import { NextraImportWizard } from '@/components/settings/NextraImportWizard';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ArrowRight01Icon, Loading01Icon } from '@/lib/icons';
import { docsImportService, type ImportStatusResponse } from '@/lib/services/docsImportService';
import { pmImportService } from '@/lib/services/pmImportService';
import shortcutIcon from '@/assets/import/shortcut.svg';
import jiraIcon from '@/assets/import/jira.svg';
import linearIcon from '@/assets/import/linear.svg';
import helpscoutIcon from '@/assets/import/helpscout.svg';
import nextraIcon from '@/assets/import/nextra.svg';

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
      {/* Project Management */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Project Management</h3>
        <p className="text-sm text-muted-foreground">Select a source to import from</p>
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

      {/* Help Center */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Help Center</h3>
        <div className="overflow-hidden rounded-lg border border-border bg-card">
          <button
            type="button"
            disabled={!editable}
            onClick={() => setSelected('helpscout')}
            className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40"
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
          <button
            type="button"
            disabled={!editable}
            onClick={() => setSelected('nextra')}
            className="flex w-full items-center gap-4 border-t border-border px-4 py-4 text-left transition-colors hover:bg-muted/40"
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
  );
}
