import { describe, expect, it } from 'vitest';
import { groupUpcomingMeetings } from '../upcomingCalendarMeetingGroups';
import type { CRMCalendarMeetingCandidate } from '@/lib/crmMeetingTypes';

function candidate(id: string, title: string, recurringSeriesId?: string): CRMCalendarMeetingCandidate {
  return {
    event: {
      id,
      workspace_id: 'workspace-1',
      email_account_id: 'account-1',
      external_event_id: id,
      recurring_series_id: recurringSeriesId,
      title,
      start_time: '2026-08-20T19:00:00Z',
      end_time: '2026-08-20T20:00:00Z',
      status: 'confirmed',
      visibility: 'default',
      all_day: false,
      attendees: [],
      contact_ids: [],
      created_at: '2026-08-17T00:00:00Z',
      updated_at: '2026-08-17T00:00:00Z',
    },
    eligible: true,
    effective_auto_join: false,
    auto_join_source: 'workspace',
  };
}

describe('groupUpcomingMeetings', () => {
  it('collapses provider occurrences from the same recurring series', () => {
    const groups = groupUpcomingMeetings([
      candidate('event-1', 'Weekly sync', 'series-1'),
      candidate('event-2', 'Weekly sync', 'series-1'),
      candidate('event-3', 'Weekly sync', 'series-1'),
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0].recurring).toBe(true);
    expect(groups[0].occurrences.map((item) => item.event.id)).toEqual(['event-1', 'event-2', 'event-3']);
  });

  it('does not group unrelated meetings just because their titles match', () => {
    const groups = groupUpcomingMeetings([
      candidate('event-1', 'Office hours'),
      candidate('event-2', 'Office hours'),
    ]);

    expect(groups).toHaveLength(2);
  });
});
