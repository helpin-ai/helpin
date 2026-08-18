import type { CRMDeal, CRMDealHealthScore } from '@/lib/crmTypes';
import type { Task } from '@/lib/pmTypes';

export type TodayTaskTiming = 'overdue' | 'today';

export interface TodayTaskItem {
  task: Task;
  timing: TodayTaskTiming;
  deadline: Date;
}

export interface DealAttentionItem {
  deal: CRMDeal;
  reasons: string[];
  severity: number;
  healthScore?: number;
  staleDays?: number;
}

function startOfLocalDay(value: Date): Date {
  const result = new Date(value);
  result.setHours(0, 0, 0, 0);
  return result;
}

function endOfLocalDay(value: Date): Date {
  const result = new Date(value);
  result.setHours(23, 59, 59, 999);
  return result;
}

function parsedDate(value?: string): Date | null {
  if (!value) return null;
  const dateOnlyMatch = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:T00:00:00(?:\.000)?Z)?$/);
  if (dateOnlyMatch) {
    const [, year, month, day] = dateOnlyMatch;
    return new Date(Number(year), Number(month) - 1, Number(day));
  }

  const result = new Date(value);
  return Number.isNaN(result.getTime()) ? null : result;
}

export function buildTodayTaskItems(tasks: Task[], now = new Date()): TodayTaskItem[] {
  const todayStart = startOfLocalDay(now);
  const todayEnd = endOfLocalDay(now);

  return tasks
    .filter((task) => (
      !task.archived
      && !task.completed
      && task.state_type !== 'done'
      && Boolean(task.contacts?.length || task.companies?.length || task.deals?.length)
    ))
    .map((task) => {
      const deadline = parsedDate(task.deadline);
      if (!deadline || deadline > todayEnd) return null;
      return {
        task,
        deadline,
        timing: deadline < todayStart ? 'overdue' as const : 'today' as const,
      };
    })
    .filter((item): item is TodayTaskItem => item !== null)
    .sort((left, right) => left.deadline.getTime() - right.deadline.getTime());
}

export function buildDealAttentionItems(
  deals: CRMDeal[],
  healthScores: CRMDealHealthScore[],
  now = new Date(),
): DealAttentionItem[] {
  const todayStart = startOfLocalDay(now);
  const healthByDealId = new Map(healthScores.map((score) => [score.deal_id, score.score]));

  return deals
    .map((deal): DealAttentionItem | null => {
      if (deal.stage?.stage_type === 'won' || deal.stage?.stage_type === 'lost') {
        return null;
      }

      const reasons: string[] = [];
      let severity = 0;
      const closeDate = parsedDate(deal.close_date);
      if (closeDate && closeDate < todayStart) {
        reasons.push('Close date overdue');
        severity = Math.max(severity, 3);
      }

      const healthScore = healthByDealId.get(deal.id);
      if (healthScore !== undefined && healthScore < 40) {
        reasons.push(`Health score ${healthScore}`);
        severity = Math.max(severity, 2);
      }

      const updatedAt = parsedDate(deal.updated_at);
      const staleDays = updatedAt
        ? Math.max(0, Math.floor((todayStart.getTime() - startOfLocalDay(updatedAt).getTime()) / 86_400_000))
        : undefined;
      if (staleDays !== undefined && staleDays >= 7) {
        reasons.push(`Not updated for ${staleDays} days`);
        severity = Math.max(severity, 1);
      }

      if (reasons.length === 0) return null;
      return { deal, reasons, severity, healthScore, staleDays };
    })
    .filter((item): item is DealAttentionItem => item !== null)
    .sort((left, right) => right.severity - left.severity || left.deal.name.localeCompare(right.deal.name));
}
