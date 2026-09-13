import { defaultStageColor } from '@/lib/crmStageColors';
import { revenueSuffix } from './dealCreationDefaults';
import { useMemo } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Calendar03Icon, UserAdd01Icon } from '@/lib/icons';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember } from '@/lib/assignableMembers';
import { useDealDisplayStore } from '@/stores/dealDisplayStore';
import type { CRMDeal } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

const pillBase = 'flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium';

interface DealCardProps {
  deal: CRMDeal;
  onOpen: (deal: CRMDeal) => void;
  workspaceId: string;
  assignableMembers?: AssignableMember[];
  ownerNameMap?: Map<string, string>;
  onOwnerChange?: (id: string, ownerId: string) => void;
  isOverlay?: boolean;
  pending?: boolean;
}

export function DealCard({
  deal,
  onOpen,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  onOwnerChange,
  isOverlay = false,
  pending = false,
}: DealCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    // The overlay must not replace the source card in dnd-kit's node registry.
    id: isOverlay ? `deal-overlay:${deal.id}` : deal.id,
    disabled: pending || isOverlay,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  const displayProps = useDealDisplayStore((s) => s.properties);

  const closeDate = useMemo(() => {
    if (!deal.close_date) return null;
    const date = parseISO(deal.close_date);
    const today = startOfDay(new Date());
    const overdue = isBefore(date, today) && deal.stage?.stage_type === 'open';
    const daysAway = differenceInDays(date, today);
    const approaching = deal.stage?.stage_type === 'open' && !overdue && daysAway <= 7;
    return {
      label: format(date, 'MMM d'),
      overdue,
      approaching,
    };
  }, [deal.close_date, deal.stage?.stage_type]);

  const currentOwnerName = useMemo(() => {
    const ownerKey = deal.owner_member_id;
    if (!ownerKey) return null;
    return ownerNameMap?.get(ownerKey) ?? null;
  }, [deal.owner_member_id, ownerNameMap]);


  return (
    <article
      ref={setNodeRef}
      data-deal-id={deal.id}
      aria-label={deal.name}
      aria-busy={pending}
      inert={pending || isOverlay}
      style={style}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={() => onOpen(deal)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' && !isDragging) {
          event.preventDefault();
          onOpen(deal);
        } else { listeners?.onKeyDown?.(event); }
      }}
      className={cn(
        'group cursor-pointer rounded-lg border border-border/60 bg-card p-3 shadow-sm transition-all',
        'hover:border-border hover:shadow-md',
        isDragging && 'opacity-30',
        isOverlay && 'rotate-2 shadow-lg',
      )}
    >
      {/* Row 1: display_id + stage type icon */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {deal.stage && (
          <span title={deal.stage.name} className="h-3 w-3 shrink-0 rounded-full border border-border/50" style={{backgroundColor: deal.stage.color || defaultStageColor(deal.stage.stage_type, deal.stage.position)}} />
        )}
        <span className="font-mono text-[11px]">{deal.display_id}</span>
        <span className="flex-1" />
      </div>

      {/* Row 2: Deal name */}
      <h4 className="mt-1.5 line-clamp-2 text-sm font-medium leading-snug text-foreground">
        {deal.name}
      </h4>

      {/* Row 3: Amount + Probability */}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {displayProps.amount && deal.amount != null && (
          <span className={cn(pillBase, 'border-green-300 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-950/50 dark:text-green-400')}>
            {deal.currency} {new Intl.NumberFormat().format(deal.amount)}{revenueSuffix(deal.revenue_type)}
          </span>
        )}

        {displayProps.probability && deal.probability != null && (
          <div className="flex items-center gap-1.5">
            <div className="h-1 w-12 overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary/60"
                style={{ width: `${deal.probability}%` }}
              />
            </div>
            <span className="text-[10px] text-muted-foreground">{deal.probability}%</span>
          </div>
        )}
      </div>

      {/* Row 4: Footer - close date + owner */}
      <div className="mt-2 flex items-center gap-1.5">
        {displayProps.close_date && closeDate && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={cn(
                pillBase,
                closeDate.overdue
                  ? 'border-red-300 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400'
                  : closeDate.approaching
                    ? 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400'
                    : 'border-border bg-muted/50 text-muted-foreground',
              )}>
                <Calendar03Icon className="h-3 w-3 shrink-0" />
                {closeDate.label}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              {closeDate.overdue ? 'Overdue' : closeDate.approaching ? 'Closing soon' : 'Close date'}: {closeDate.label}
            </TooltipContent>
          </Tooltip>
        )}

        <span className="flex-1" />

        {/* Owner avatar / assign button */}
        {displayProps.owner && (assignableMembers && workspaceId && onOwnerChange ? (
          <MemberPickerPopover
            value={deal.owner_member_id || '__none__'}
            members={assignableMembers}
            noneLabel="Unassigned"
            triggerLabel="Owner"
            onChange={(value) => {
              onOwnerChange(deal.id, value === '__none__' ? '' : value);
            }}
            align="end"
            triggerClassName="shrink-0 rounded-full transition-opacity hover:opacity-80"
            contentClassName="w-[220px]"
            renderTrigger={() => {
              const selectedMember = findAssignableMember(assignableMembers, deal.owner_member_id);
              return selectedMember ? (
                <UserAvatar
                  name={selectedMember.display_name || selectedMember.email}
                  avatarUrl={selectedMember.avatar_url}
                  avatarStyle={selectedMember.avatar_style}
                  avatarSeed={selectedMember.avatar_seed}
                  avatarBackgroundMode={selectedMember.avatar_background_mode}
                  avatarBackgroundColor={selectedMember.avatar_background_color}
                  className="h-7 w-7"
                />
              ) : (
                <span className="flex h-7 w-7 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary">
                  <UserAdd01Icon className="h-3.5 w-3.5" />
                </span>
              );
            }}
          />
        ) : currentOwnerName ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="shrink-0">
                <UserAvatar name={currentOwnerName} className="h-7 w-7" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{currentOwnerName}</TooltipContent>
          </Tooltip>
        ) : null)}
      </div>
    </article>
  );
}
