import { describe, expect, it, vi } from 'vitest';
import type { Task } from '@/lib/pmTypes';
import { deadlineDays, groupMyWork, filterMyWork, loadMyWorkTasks, defaultMyWorkPreferences } from './myWorkModel';
const task = (id: string, extra: Partial<Task> = {}) => ({ id, name: id, task_key: id, priority: 'none', completed: false, updated_at: '2026-09-17T12:00:00Z', ...extra }) as Task;
const now = new Date(2026, 8, 18, 9);
describe('My Work data', () => {
  it('uses calendar dates for yesterday, today, and the three-day boundary', () => {
    expect(deadlineDays('2026-09-17', now)).toBe(-1);
    expect(deadlineDays('2026-09-18', now)).toBe(0);
    expect(deadlineDays('2026-09-21', now)).toBe(3);
    expect(deadlineDays('invalid', now)).toBeNull();
  });
  it('keeps paused agents distinct from completed tasks and puts each task in one section', () => {
    const groups = groupMyWork([task('approval', {latest_run_status:'paused'}), task('finished agent', {latest_run_status:'completed', priority:'high'}), task('blocked', {blocked:true, priority:'high'})], defaultMyWorkPreferences, true, () => '', now);
    expect(groups.map(g => [g.label, g.tasks.map(t => t.id)])).toEqual([['Needs your input',['approval']],['Focus now',['finished agent']],['Blocked',['blocked']]]);
  });
  it('searches beyond the ten recent completions and includes assignee names', () => {
    const tasks = Array.from({length:12}, (_,i)=>task(`done-${i}`,{completed:true,owner_member_ids:['sara'],updated_at:`2026-09-${String(18-i).padStart(2,'0')}T12:00:00Z`}));
    expect(groupMyWork(tasks, defaultMyWorkPreferences, true, () => '', now)[0].tasks).toHaveLength(10);
    const prefs = {...defaultMyWorkPreferences, search:'done-11'};
    const matches=filterMyWork(tasks,prefs,()=> 'Sara');
    expect(groupMyWork(matches,prefs,true,()=>'',now)[0].tasks.map(t=>t.id)).toEqual(['done-11']);
    expect(filterMyWork(tasks,{...prefs,search:'Sara'},()=> 'Sara')).toHaveLength(12);
  });
  it('loads every page, keeps ownership filters, and deduplicates changing pages', async () => {
    const list=vi.fn().mockResolvedValueOnce({data:{data:[task('one')],total_pages:2}}).mockResolvedValueOnce({data:{data:[task('one'),task('two')],total_pages:2}});
    const result=await loadMyWorkTasks('ws',{owner_member_ids:'me',archived:false},()=>false,list);
    expect(result?.map(t=>t.id)).toEqual(['one','two']);
    expect(list).toHaveBeenLastCalledWith('ws',expect.objectContaining({page:2,owner_member_ids:'me',archived:false}));
  });
  it('does not return misleading partial totals if a later page fails', async () => {
    const list=vi.fn().mockResolvedValueOnce({data:{data:[task('one')],total_pages:2}}).mockResolvedValueOnce({error:'Unavailable'});
    await expect(loadMyWorkTasks('ws',{},()=>false,list)).rejects.toThrow('Unavailable');
  });
});
