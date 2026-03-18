import { useEffect, useState } from 'react';
import { ShortcutImportWizard } from '@/components/pm/ShortcutImportWizard';
import { HelpCenterImportSection } from '@/components/settings/HelpCenterImportSection';
import { workspacesService } from '@/lib/services/workspacesService';
import type { MemberWithUser } from '@/lib/types';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ChevronRight, Import } from 'lucide-react';

const IMPORT_SOURCES = [
  {
    key: 'shortcut' as const,
    title: 'Shortcut',
    description: 'Import stories, epics, workflows, and members from Shortcut.',
    icon: Import,
    comingSoon: false,
  },
  {
    key: 'jira' as const,
    title: 'Jira',
    description: 'Import issues, projects, and workflows from Jira.',
    icon: Import,
    comingSoon: true,
  },
  {
    key: 'linear' as const,
    title: 'Linear',
    description: 'Import issues, projects, and cycles from Linear.',
    icon: Import,
    comingSoon: true,
  },
];

export function ImportTab({ workspaceId, editable = true }: { workspaceId: string; editable?: boolean }) {
  const [selected, setSelected] = useState<string | null>(null);
  const [members, setMembers] = useState<MemberWithUser[]>([]);

  useEffect(() => {
    workspacesService.listMembers(workspaceId).then(({ data }) => {
      if (data) setMembers(data);
    });
  }, [workspaceId]);

  if (selected === 'shortcut') {
    return (
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="gap-1.5 text-muted-foreground" onClick={() => setSelected(null)}>
          <ChevronRight className="h-4 w-4 rotate-180" />
          Back to sources
        </Button>
        <ShortcutImportWizard workspaceId={workspaceId} members={members} />
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
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <source.icon className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{source.title}</p>
                <p className="text-sm text-muted-foreground">{source.description}</p>
              </div>
              {source.comingSoon ? (
                <Badge variant="secondary" className="text-xs">Coming Soon</Badge>
              ) : (
                <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Help Center */}
      <div className="space-y-4">
        <h3 className="text-sm font-semibold">Help Center</h3>
        <HelpCenterImportSection workspaceId={workspaceId} editable={editable} />
      </div>
    </div>
  );
}
