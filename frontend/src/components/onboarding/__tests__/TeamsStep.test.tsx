// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { TeamsStep } from '../TeamsStep';
import { button, click, renderWithQuery, type Rendered } from '@/components/setup/__tests__/setupTestUtils';
const state=vi.hoisted(()=>({teams:[{id:'eng',handle:'engineering'},{id:'prod',handle:'product'},{id:'marketing',handle:'marketing'}],refetch:vi.fn(),create:vi.fn()}));
vi.mock('@/hooks/queries/useSettings',()=>({useWorkspaceSettings:()=>({data:{teams:state.teams},isPending:false,refetch:state.refetch})}));
vi.mock('@/lib/services/settingsService',()=>({settingsService:{createTeam:state.create}}));
vi.mock('sonner',()=>({toast:{error:vi.fn(),warning:vi.fn()}}));
let rendered:Rendered|undefined;
afterEach(async()=>{await rendered?.unmount();rendered=undefined;vi.clearAllMocks();});
globalThis.ResizeObserver??=class{observe(){}unobserve(){}disconnect(){}};
describe('resuming team setup',()=>{
 it('reuses teams created by an assistant and carries their IDs into invitations',async()=>{
  state.refetch.mockResolvedValue({data:{teams:state.teams},isError:false});const done=vi.fn();
  rendered=await renderWithQuery(<TooltipProvider><TeamsStep workspaceId="ws-1" createdHandles={[]} onDone={done}/></TooltipProvider>);
  await click(button(rendered.container,'Continue with 3 teams'));
  expect(state.create).not.toHaveBeenCalled();expect(done).toHaveBeenCalledWith(state.teams);
 });
 it('does not create duplicates when the latest teams cannot be checked',async()=>{
  state.refetch.mockResolvedValue({isError:true});const done=vi.fn();
  rendered=await renderWithQuery(<TooltipProvider><TeamsStep workspaceId="ws-1" createdHandles={[]} onDone={done}/></TooltipProvider>);
  await click(button(rendered.container,'Continue with 3 teams'));
  expect(state.create).not.toHaveBeenCalled();expect(done).not.toHaveBeenCalled();
 });
 it('does not carry an unselected existing team into invitation assignments',async()=>{
  state.refetch.mockResolvedValue({data:{teams:state.teams},isError:false});const done=vi.fn();
  rendered=await renderWithQuery(<TooltipProvider><TeamsStep workspaceId="ws-1" createdHandles={['engineering']} onDone={done}/></TooltipProvider>);
  await click(rendered.container.querySelector<HTMLElement>('[role="checkbox"]')!);
  await click(button(rendered.container,'Continue with 2 teams'));
  expect(done).toHaveBeenCalledWith(state.teams.filter(team=>team.id!=='eng'));
 });
});
