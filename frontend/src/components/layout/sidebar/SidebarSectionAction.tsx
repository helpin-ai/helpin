import { PlusSignIcon } from '@/lib/icons';

type SidebarSectionActionProps = {
  label: string;
  onClick: () => void;
  dataSlot?: string;
};

export function SidebarSectionAction({ label, onClick, dataSlot }: SidebarSectionActionProps) {
  return (
    <button
      type="button"
      data-slot={dataSlot}
      aria-label={label}
      className="group/plus -mr-0.5 flex h-5 w-5 items-center justify-center rounded transition-colors hover:bg-muted"
      onClick={onClick}
    >
      <PlusSignIcon className="h-3 w-3 text-muted-foreground/70 group-hover/plus:text-foreground" />
    </button>
  );
}
