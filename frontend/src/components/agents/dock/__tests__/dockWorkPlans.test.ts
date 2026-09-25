import { describe, expect, it } from 'vitest';
import { dockPlanBoundary, dockWorkPlans, hasWorkPlanOrigin } from '../dockWorkPlans';
import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import type { CodingSessionEvent, CodingSessionStreamState, RunPlanArtifact } from '@/lib/pmTypes';
const at = '2026-09-21T12:00:00Z';
const plan: RunPlanArtifact = {origin:{event_id:'p1',turn_id:'t1',created_at:at},plan:[{step:'Inspect',status:'pending'}]};
const stream = {transcript_messages:[],current_plan:plan,work_plans:[plan]} as unknown as CodingSessionStreamState;
describe('dock work plan placement', () => {
 it('keeps the plan before the next user message and collapses it', () => {
  expect(dockPlanBoundary(plan,[{timestamp:Date.parse(at)-1000,user:true},{timestamp:Date.parse(at)+1000,user:false},{timestamp:Date.parse(at)+2000,user:true}])).toEqual({index:1,collapsed:true});
 });
 it('keeps a pending follow-up below the plan even with a skewed client clock', () => {
  expect(dockPlanBoundary(plan,[{timestamp:Date.parse(at)-1000,user:true,pending:true}])).toEqual({index:0,collapsed:true});
 });
 it('merges saved and live progress by stable origin', () => {
  const live = {...plan,plan:[{step:'Inspect',status:'completed' as const}]};
  expect(dockWorkPlans({...stream,current_plan:live},[plan])).toEqual([live]);
  expect(hasWorkPlanOrigin({plan:plan.plan})).toBe(false);
 });
 it('does not prepend old plans to a newer page of history', () => {
  expect(dockWorkPlans({...stream,transcript_messages:[{timestamp:'2026-09-22T12:00:00Z'} as never]})).toEqual([]);
 });
 it('retains an origin across progress events and restores it from a snapshot', () => {
  const event = (id:string,time:string,status:string): CodingSessionEvent => ({id,session_id:'run',run_id:'run',type:'plan.updated',timestamp:time,sequence_no:1,runtime_kind:'native',payload:{content:JSON.stringify({plan:[{step:'Inspect',status}]})}});
  const result=buildCodingSessionStreamState([event('p1',at,'pending'),{...event('p2','2026-09-21T12:00:01Z','completed'),sequence_no:2}]);
  expect(result.current_plan?.origin?.event_id).toBe('p1');
  expect(result.work_plans).toHaveLength(1);
  const restored=buildCodingSessionStreamState([], {current_plan:result.current_plan!,work_plans:result.work_plans});
  expect(restored.work_plans).toEqual(result.work_plans);
 });
});
