import type { ReactNode } from 'react';

import { QuietIconAction } from '@/components/design-system/quiet';
import { SidebarTrigger, useSidebar } from '@/components/ui/sidebar';
import { ArrowLeft01Icon, ArrowRight01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

function SidebarCaretButton({
  direction,
  label,
  expanded,
  className,
}: {
  direction: 'left' | 'right';
  label: string;
  expanded: boolean;
  className?: string;
}) {
  const { toggleSidebar } = useSidebar();
  const Icon = direction === 'left' ? ArrowLeft01Icon : ArrowRight01Icon;

  return (
    <QuietIconAction
      type="button"
      aria-label={label}
      aria-expanded={expanded}
      title={label}
      onClick={toggleSidebar}
      className={cn(
        'group/caret size-11 rounded-none p-0 hover:bg-transparent focus-visible:bg-transparent',
        className,
      )}
    >
      <span className="flex size-7 items-center justify-center rounded-[6px] text-quiet-text-tertiary transition-colors duration-150 group-hover/caret:bg-quiet-hover group-hover/caret:text-quiet-text-primary group-focus-visible/caret:bg-quiet-hover group-focus-visible/caret:text-quiet-text-primary group-focus-visible/caret:ring-2 group-focus-visible/caret:ring-quiet-text-primary motion-reduce:transition-none">
        <Icon className="h-[15px] w-[15px]" />
      </span>
    </QuietIconAction>
  );
}

export function SidebarHeaderToggle() {
  return (
    <SidebarCaretButton
      direction="left"
      label="Collapse sidebar"
      expanded
      className="size-11 md:size-8"
    />
  );
}

export function WorkspaceSidebarToggle() {
  const { isMobile, openMobile, state } = useSidebar();
  const isVisible = isMobile ? !openMobile : state === 'collapsed';

  if (!isVisible) return null;

  return (
    <div className="absolute left-1 top-1.5 z-40 pl-[env(safe-area-inset-left)] md:left-3 md:top-3 md:pl-0">
      <SidebarTrigger
        aria-label="Open navigation"
        aria-expanded={false}
        title="Open navigation"
        className="size-11 rounded-[6px] text-quiet-text-tertiary hover:bg-quiet-hover hover:text-quiet-text-primary focus-visible:ring-2 focus-visible:ring-quiet-text-primary md:size-8"
      />
    </div>
  );
}

export function WorkspaceMainContent({ children }: { children: ReactNode }) {
  const { isMobile, openMobile, state } = useSidebar();
  const showSidebarToggle = isMobile ? !openMobile : state === 'collapsed';

  return (
    <main
      data-sidebar-toggle-visible={showSidebarToggle ? 'true' : 'false'}
      className="group/workspace-main relative min-h-0 flex-1 overflow-hidden"
    >
      <WorkspaceSidebarToggle />
      {children}
    </main>
  );
}
