import { useCallback, useMemo } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { CalendarDays, UserPlus } from 'lucide-react';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { StageTypeIcon } from '@/lib/crmConstants';
import { crmDealService } from '@/lib/services/crmService';
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
  onOwnerChanged?: (deal: CRMDeal) => void;
  isOverlay?: boolean;
}

export function DealCard({
  deal,
  onOpen,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  onOwnerChanged,
  isOverlay = false,
}: DealCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: deal.id });

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

  const handleAssignOwner = useCallback(
    async (value: string) => {
      const newOwnerId = value === '__none__' ? '' : value;
      try {
        const result = await crmDealService.update(workspaceId, deal.id, { owner_member_id: newOwnerId });
        if (result.data) {
          onOwnerChanged?.(result.data);
        }
      } catch {}
    },
    [workspaceId, deal.id, onOwnerChanged],
  );

  return (
    <article
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      role="button"
      tabIndex={0}
      onClick={() => onOpen(deal)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen(deal);
        }
      }}
      className={cn(
        'group cursor-pointer rounded-lg border border-border/60 bg-background p-3 shadow-sm transition-all',
        'hover:border-border hover:shadow-md',
        isDragging && 'opacity-30',
        isOverlay && 'rotate-2 shadow-lg',
      )}
    >
      {/* Row 1: display_id + stage type icon */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {deal.stage && (
          <StageTypeIcon stageType={deal.stage.stage_type} className="h-3.5 w-3.5 shrink-0" />
        )}
        <span className="font-mono text-[11px]">{deal.display_id}</span>
        <span className="flex-1" />
      </div>

      {/* Row 2: Deal name */}
      <h4 className="mt-1.5 line-clamp-2 text-[13px] font-medium leading-snug text-foreground">
        {deal.name}
      </h4>

      {/* Row 3: Amount + Probability */}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {displayProps.amount && deal.amount != null && (
          <span className={cn(pillBase, 'border-green-300 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-950/50 dark:text-green-400')}>
            {deal.currency} {new Intl.NumberFormat().format(deal.amount)}
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
                <CalendarDays className="h-3 w-3 shrink-0" />
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
        {displayProps.owner && (assignableMembers && workspaceId ? (
          <MemberPickerPopover
            value={deal.owner_member_id || '__none__'}
            members={assignableMembers}
            noneLabel="Unassigned"
            onChange={(value) => {
              void handleAssignOwner(value);
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
                  className="h-5 w-5"
                />
              ) : (
                <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted/40 text-muted-foreground transition-colors hover:border-primary/40 hover:text-primary">
                  <UserPlus className="h-2.5 w-2.5" />
                </span>
              );
            }}
          />
        ) : currentOwnerName ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="shrink-0">
                <UserAvatar name={currentOwnerName} className="h-5 w-5" />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">{currentOwnerName}</TooltipContent>
          </Tooltip>
        ) : null)}
      </div>
    </article>
  );
}
