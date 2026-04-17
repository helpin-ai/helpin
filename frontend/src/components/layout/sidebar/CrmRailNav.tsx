import { Mail01Icon, SparklesIcon, WorkflowSquare01Icon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { StandardRailNav } from './StandardRailNav';
import type { NavGroup } from './types';

interface CrmRailNavProps {
  groups: NavGroup[];
  isActive: (link: string) => boolean;
  wsSlug: string;
  onNavigate: (link: string) => void;
  onNavigateTo: (to: string) => void;
}

export function CrmRailNav({
  groups,
  isActive,
  wsSlug,
  onNavigate,
  onNavigateTo,
}: CrmRailNavProps) {
  const goToSetting = (section: string) => onNavigateTo(`/w/${wsSlug}/settings/${section}`);

  return (
    <>
      <StandardRailNav groups={groups} isActive={isActive} onNavigate={onNavigate} />

      <div className="fixed inset-x-0 bottom-0 z-10 ml-16 w-[calc(var(--sidebar-width)-4rem)] border-t border-border/70 bg-[#fafafa] px-2 py-2 dark:bg-sidebar">
        <div className="flex items-center justify-around">
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => goToSetting('crm-pipelines')}
              >
                <WorkflowSquare01Icon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Pipelines</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => goToSetting('crm-email')}
              >
                <Mail01Icon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Email Accounts</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => goToSetting('crm-autonomy')}
              >
                <SparklesIcon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Autonomy</TooltipContent>
          </Tooltip>
        </div>
      </div>
    </>
  );
}
