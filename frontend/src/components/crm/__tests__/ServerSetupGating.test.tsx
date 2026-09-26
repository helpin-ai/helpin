// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerSetupNotice } from '../ServerSetupNotice';
import { UpcomingCalendarMeetings } from '../UpcomingCalendarMeetings';
import { button, renderWithQuery, type Rendered } from '@/components/setup/__tests__/setupTestUtils';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
  useNavigate: () => vi.fn(),
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
});

describe('ServerSetupNotice', () => {
  it('links server admins to System status', async () => {
    rendered = await renderWithQuery(<ServerSetupNotice id="reason" title="Meeting capture isn’t set up on this server." slug="acme" isServerAdmin />);
    const link = rendered.container.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/w/acme/settings/system-status');
    expect(rendered.container.querySelector('#reason')?.textContent).toBe('Meeting capture isn’t set up on this server.');
  });

  it('tells everyone else who can fix it', async () => {
    rendered = await renderWithQuery(<ServerSetupNotice id="reason" title="Meeting capture isn’t set up on this server." slug="acme" isServerAdmin={false} />);
    expect(rendered.container.querySelector('a')).toBeNull();
    expect(rendered.container.textContent).toContain('Ask your server admin');
  });
});

describe('UpcomingCalendarMeetings', () => {
  const props = {
    workspaceId: 'ws-1',
    workspaceSlug: 'acme',
    candidates: [],
    canEdit: true,
    canManageSettings: true,
    loading: false,
    calendarConnected: false,
    calendarLoading: false,
    connectingCalendar: false,
    meetingNotesEnabled: true,
    searching: false,
    onConnectCalendar: vi.fn(),
    onConfigureSettings: vi.fn(),
    onAddMeeting: vi.fn(),
  };

  it('disables connecting Google Calendar with the reason when the server cannot connect Google', async () => {
    rendered = await renderWithQuery(<UpcomingCalendarMeetings {...props} calendarConnectUnavailable isServerAdmin={false} />);
    const connect = button(rendered.container, 'Connect Google Calendar');
    expect(connect?.disabled).toBe(true);
    const reasonId = connect?.getAttribute('aria-describedby');
    expect(reasonId).toBeTruthy();
    expect(rendered.container.querySelector(`#${reasonId}`)?.textContent).toContain('isn’t set up on this server');
    expect(rendered.container.textContent).toContain('Ask your server admin');
  });

  it('keeps connecting available when Google is configured', async () => {
    rendered = await renderWithQuery(<UpcomingCalendarMeetings {...props} />);
    const connect = button(rendered.container, 'Connect Google Calendar');
    expect(connect?.disabled).toBe(false);
    expect(connect?.hasAttribute('aria-describedby')).toBe(false);
  });
});
