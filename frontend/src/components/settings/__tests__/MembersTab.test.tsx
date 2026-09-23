// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import { MembersTab } from '../MembersTab';
import type { WorkspaceTeam, TeamUserMembership } from '@/lib/types';

vi.mock('@/hooks/queries', () => ({ useOrganizationMembers: () => ({ data: [] }), useWorkspaceMemberPresenceMap: () => ({ data: new Map() }) }));
vi.mock('@/hooks/queries/useSettings', () => ({ useWorkspaceModuleAccess: () => ({ data: { grants: [{ id: 'grant', module: 'support', subject_type: 'team', subject_id: 'team-1' }] }, isLoading: false, isError: false }) }));
vi.mock('@/lib/services/workspacesService', () => ({ workspacesService: { removeMember: vi.fn().mockResolvedValue({ data: null, error: null }), listMembers: vi.fn().mockResolvedValue({ data: [
  { id: 'owner-member', user_id: 'owner', full_name: 'Workspace Owner', email: 'owner@example.com', role: 'owner' },
  { id: 'member-1', user_id: 'ada', full_name: 'Ada Lovelace', email: 'ada@example.com', role: 'member' },
], error: null }) } }));
vi.mock('@/lib/services/inviteService', () => ({ inviteService: { list: vi.fn().mockResolvedValue({ data: [], error: null }), send: vi.fn() } }));
const copy = vi.hoisted(() => vi.fn());
vi.mock('@/hooks/useCopyToClipboard', () => ({ useCopyToClipboard: () => ({ copy }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
afterEach(() => { document.body.innerHTML = ''; });

describe('MembersTab', () => {
  it('shows effective modules and email under the name with row actions and confirms removal', async () => {
    useAuthStore.setState({ user: { id: 'owner', full_name: 'Owner', email: 'owner@example.com' } });
    const container = document.createElement('div'); document.body.appendChild(container); const root = createRoot(container);
    await act(async () => root.render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><MembersTab workspaceId="ws-1" editable canManageTeams canManageModuleAccess teams={[{ id: 'team-1', name: 'Support team' }] as WorkspaceTeam[]} userMemberships={[{ user_id: 'ada', team_id: 'team-1' }] as TeamUserMembership[]} /></TooltipProvider></QueryClientProvider>));
    const row = Array.from(container.querySelectorAll('tbody tr')).find(item => item.textContent?.includes('Ada Lovelace'))!;
    const cells = row.querySelectorAll('td');
    expect(cells[0].textContent).toContain('Ada Lovelace');
    expect(cells[0].textContent).toContain('ada@example.com');
    expect(Array.from(container.querySelectorAll('th')).map(item => item.textContent)).not.toContain('Email');
    expect(cells[3].textContent).toContain('Projects');
    expect(cells[3].textContent).toContain('Support');
    expect(cells[3].textContent).not.toContain('CRM');
    expect(row.querySelector('[role="combobox"]')).toBeNull();
    expect(Array.from(row.querySelectorAll('button')).map(item => item.getAttribute('aria-label'))).toEqual(['Edit Ada Lovelace', 'Remove Ada Lovelace']);
    const remove = row.querySelector<HTMLButtonElement>('button[aria-label="Remove Ada Lovelace"]')!;
    await act(async () => remove.click());
    expect(workspacesService.removeMember).not.toHaveBeenCalled();
    expect(document.querySelector('[role="alertdialog"]')?.textContent).toContain('Ada Lovelace');
    await act(async () => Array.from(document.querySelectorAll<HTMLButtonElement>('[role="alertdialog"] button')).find(button => button.textContent === 'Cancel')!.click());
    expect(workspacesService.removeMember).not.toHaveBeenCalled();
    await act(async () => remove.click());
    await act(async () => Array.from(document.querySelectorAll<HTMLButtonElement>('[role="alertdialog"] button')).find(button => button.textContent === 'Remove member')!.click());
    expect(workspacesService.removeMember).toHaveBeenCalledWith('ws-1', 'member-1');
    expect(container.textContent).not.toContain('Ada Lovelace');
    act(() => root.unmount());
  });

  it('offers copyable invite links when the server has no email', async () => {
    useAuthStore.setState({
      user: { id: 'owner', full_name: 'Owner', email: 'owner@example.com' },
      configuration: { email_verification_required: false, app_email_configured: false, google_login_enabled: false },
    } as never);
    const joinUrl = 'https://helpin.example.com/join/abc123';
    vi.mocked(inviteService.list).mockResolvedValueOnce({ data: [
      { id: 'inv-1', workspace_id: 'ws-1', email: 'grace@example.com', role: 'member', status: 'pending', invited_by: 'owner', expires_at: '2026-10-01T00:00:00Z', created_at: '2026-09-23T00:00:00Z', join_url: joinUrl },
    ], error: null } as never);
    const container = document.createElement('div'); document.body.appendChild(container); const root = createRoot(container);
    await act(async () => root.render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><MembersTab workspaceId="ws-1" editable teams={[]} userMemberships={[]} /></TooltipProvider></QueryClientProvider>));

    expect(container.textContent).toContain('Email isn’t set up on this server');
    const copyButton = container.querySelector<HTMLButtonElement>('button[aria-label="Copy invite link for grace@example.com"]')!;
    expect(copyButton).not.toBeNull();
    await act(async () => copyButton.click());
    expect(copy).toHaveBeenCalledWith(joinUrl);

    // Managing the invitation offers the link but not an email resend.
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Manage invitation for grace@example.com"]')!.click());
    const dialogText = document.querySelector('[role="dialog"]')?.textContent ?? '';
    expect(dialogText).toContain('Copy invite link');
    expect(dialogText).not.toContain('Resend invitation');
    act(() => root.unmount());
  });
});
