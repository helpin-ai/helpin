import { differenceInCalendarDays, isValid, parseISO } from 'date-fns';
import type { Task } from '@/lib/pmTypes';
import { pmTaskService } from '@/lib/services/pmTaskService';

export type MyWorkFilter = 'All' | 'Overdue' | 'Due soon' | 'Blocked' | 'In progress';
export type MyWorkPreferences = {
  search: string;
  filter: MyWorkFilter;
  team: string;
  priority: string;
  group: 'focus' | 'stage' | 'team';
  compact: boolean;
  showCompleted: boolean;
};
export const defaultMyWorkPreferences: MyWorkPreferences = { search: '', filter: 'All', team: '', priority: '', group: 'focus', compact: false, showCompleted: true };
export const myWorkFilters: MyWorkFilter[] = ['All', 'Overdue', 'Due soon', 'Blocked', 'In progress'];

// Task deadlines are calendar dates, serialized as midnight UTC by the task API.
export function deadlineDays(deadline?: string, now = new Date()): number | null {
  if (!deadline) return null;
  const date = parseISO(deadline.slice(0, 10));
  return isValid(date) ? differenceInCalendarDays(date, now) : null;
}
export function matchesMyWorkFilter(task: Task, filter: MyWorkFilter, now = new Date()) {
  if (filter === 'All') return true;
  if (task.completed) return false;
  const days = deadlineDays(task.deadline, now);
  if (filter === 'Overdue') return days !== null && days < 0;
  if (filter === 'Due soon') return days !== null && days >= 0 && days <= 3;
  if (filter === 'Blocked') return task.blocked;
  return task.state_type === 'started';
}
export function filterMyWork(tasks: Task[], prefs: MyWorkPreferences, ownerNames: (task: Task) => string, now = new Date()) {
  const query = prefs.search.trim().toLowerCase();
  return tasks.filter(task => (!task.completed || prefs.showCompleted)
    && (!prefs.team || prefs.team === task.team_id)
    && (!prefs.priority || prefs.priority === task.priority)
    && matchesMyWorkFilter(task, prefs.filter, now)
    && (!query || `${task.name} ${task.task_key} ${ownerNames(task)}`.toLowerCase().includes(query)));
}
function focusScore(task: Task, now: Date) {
  const days = deadlineDays(task.deadline, now);
  let score = days !== null && days < 0 ? 1000 + Math.abs(days) : days !== null && days <= 3 ? 500 : 0;
  if (task.blocked) score += 400;
  if (task.priority === 'urgent') score += 300;
  else if (task.priority === 'high') score += 200;
  if (task.state_type === 'started') score += 100;
  if (differenceInCalendarDays(now, parseISO(task.updated_at)) <= 1) score += 50;
  return score;
}
export function groupMyWork(tasks: Task[], prefs: MyWorkPreferences, assigned: boolean, teamName: (id?: string) => string | undefined, now = new Date(), memberId?: string) {
  const active = tasks.filter(task => !task.completed).sort((a,b) => focusScore(b,now)-focusScore(a,now) || b.updated_at.localeCompare(a.updated_at));
  const done = tasks.filter(task => task.completed).sort((a,b) => b.updated_at.localeCompare(a.updated_at));
  // Search includes every completion; the default list preserves the latest ten.
  const shownDone = prefs.search.trim() ? done : done.slice(0,10);
  const groups = new Map<string, { id: string; label: string; tasks: Task[]; completed?: boolean }>();
  if (prefs.group === 'focus') {
    for (const label of ['Needs your input','Focus now','Blocked','Other tasks','Recently completed']) groups.set(label,{id:label,label,tasks:[],completed:label==='Recently completed'});
    for (const task of active) {
      const ownsTask = assigned || Boolean(memberId && task.owner_member_ids?.includes(memberId));
      const label = task.latest_run_status === 'paused' && ownsTask ? 'Needs your input' : task.blocked ? 'Blocked' : focusScore(task,now) >= 100 ? 'Focus now' : 'Other tasks';
      groups.get(label)!.tasks.push(task);
    }
    groups.get('Recently completed')!.tasks = shownDone;
  } else {
    for (const task of [...active,...shownDone]) {
      const id = prefs.group === 'team' ? task.team_id || 'no-team' : task.workflow_state_id || task.state_name || 'no-stage';
      const label = prefs.group === 'team' ? teamName(task.team_id) || task.team_name || 'No team' : task.state_name || 'No stage';
      if (!groups.has(id)) groups.set(id,{id,label,tasks:[]});
      groups.get(id)!.tasks.push(task);
    }
  }
  return [...groups.values()].filter(group => group.tasks.length);
}
export async function loadMyWorkTasks(workspaceId: string, filters: Parameters<typeof pmTaskService.list>[1], cancelled: () => boolean, list = pmTaskService.list): Promise<Task[] | null> {
  const tasks = new Map<string,Task>();
  let totalPages = 1;
  for (let page = 1; page <= totalPages; page++) {
    const response = await list(workspaceId, {...filters,page,per_page:200});
    if (cancelled()) return null;
    if (response.error) throw new Error(response.error);
    if (!response.data) throw new Error('Unable to load your work.');
    for (const task of response.data.data) tasks.set(task.id,task);
    totalPages = response.data.total_pages || 1;
  }
  return [...tasks.values()];
}
