// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { EditMemberDialog } from '../EditMemberDialog';
import type { MemberWithUser, WorkspaceTeam, WorkspaceModuleGrant } from '@/lib/types';

const api = vi.hoisted(() => ({ addTeamMember: vi.fn(), removeTeamMember: vi.fn(), createModuleGrant: vi.fn(), deleteModuleGrant: vi.fn(), updateMemberRole: vi.fn(), removeMember: vi.fn() }));
vi.mock('@/lib/services/settingsService', () => ({ settingsService: api }));
vi.mock('@/lib/services/workspacesService', () => ({ workspacesService: api }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });

const member = { id: 'member-1', user_id: 'user-1', full_name: 'Ada Lovelace', email: 'ada@example.com', role: 'member' } as MemberWithUser;
const teams = [{ id: 'team-1', name: 'Support team' }] as WorkspaceTeam[];
const grants = [{ id: 'team-grant', module: 'support', subject_type: 'team', subject_id: 'team-1' }] as WorkspaceModuleGrant[];

describe('EditMemberDialog', () => {
  let root: Root;
  let container: HTMLDivElement;
  let onClose: ReturnType<typeof vi.fn>;
  beforeEach(async () => {
    vi.clearAllMocks();
    for (const mock of Object.values(api)) mock.mockResolvedValue({ data: {}, error: null });
    api.createModuleGrant.mockResolvedValue({ data: { id: 'new-grant', module: 'crm', subject_type: 'workspace_member', subject_id: 'member-1' }, error: null });
    onClose = vi.fn();
    container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
    await act(async () => root.render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><TooltipProvider><EditMemberDialog workspaceId="ws-1" member={member} teams={teams} initialTeamIds={[]} grants={grants} canEditRole canEditTeams canEditModules roleOptions={[{ value: 'member', label: 'Workspace member' }]} onClose={onClose} onApplied={vi.fn()} /></TooltipProvider></QueryClientProvider>));
  });
  afterEach(() => { act(() => root.unmount()); document.body.innerHTML = ''; });
  const click = async (element: HTMLElement | undefined | null) => { expect(element).toBeTruthy(); await act(async () => element?.click()); };
  const button = (name: string) => Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(item => item.textContent === name);
  const check = (name: string) => Array.from(document.querySelectorAll('label')).find(label => label.textContent?.includes(name))?.querySelector<HTMLElement>('[role="checkbox"]');

  it('stages team and module changes until Save and previews inherited access', async () => {
    await click(check('Support team'));
    expect(check('Via Support team')?.getAttribute('data-state')).toBe('checked');
    expect(document.body.textContent).toContain('Via Support team');
    await click(check('CRM'));
    expect(api.addTeamMember).not.toHaveBeenCalled();
    expect(api.createModuleGrant).not.toHaveBeenCalled();
    await click(button('Save changes'));
    expect(api.addTeamMember).toHaveBeenCalledWith('ws-1', 'team-1', { user_id: 'user-1', role: 'member' });
    expect(api.createModuleGrant).toHaveBeenCalledWith('ws-1', { module: 'crm', subject_type: 'workspace_member', subject_id: 'member-1' });
    expect(onClose).toHaveBeenCalledOnce();
  });
  it('discards unsaved changes on Cancel', async () => {
    await click(check('CRM'));
    await click(button('Cancel'));
    expect(api.createModuleGrant).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledOnce();
  });
  it('retries a failed grant without repeating a successful team addition', async () => {
    api.createModuleGrant.mockResolvedValueOnce({ data: null, error: 'Grant failed' });
    await click(check('Support team')); await click(check('CRM')); await click(button('Save changes'));
    expect(onClose).not.toHaveBeenCalled();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Grant failed');
    await click(button('Save changes'));
    expect(api.addTeamMember).toHaveBeenCalledTimes(1);
    expect(api.createModuleGrant).toHaveBeenCalledTimes(2);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
