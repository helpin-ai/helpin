import { describe, expect, it } from 'vitest';
import type { SupportConversation, SupportMessage } from '@/lib/pmTypes';
import { formatTimestamp } from './helpers';
import {
  formatFollowUpTime,
  getSupportFollowUpStage,
  isFollowUpStatusNote,
} from './supportFollowUpState';

const conversation = {
  id: 'c',
  workspace_id: 'ws',
  status: 'open',
  flow_state: 'ai_handling',
  ai_state: 'pending',
  last_public_message_id: 'source',
  last_public_sender_type: 'ai',
  last_public_message_at: '2026-10-10T10:00:00Z',
  ai_follow_up: {
    id: 'episode',
    source_message_id: 'source',
    status: 'scheduled',
    sequence_version: 2,
    due_at: '2026-10-11T10:00:00Z',
    created_at: '2026-10-10T10:01:00Z',
    updated_at: '2026-10-10T10:01:00Z',
  },
} as SupportConversation;
function withEpisode(change: object, conv: object = {}) {
  return {
    ...conversation,
    ...conv,
    ai_follow_up: { ...conversation.ai_follow_up, ...change },
  } as SupportConversation;
}

describe('current follow-up stage', () => {
  it('replaces each stage rather than accumulating statuses', () => {
    expect(getSupportFollowUpStage(conversation)?.kind).toBe('scheduled');
    expect(getSupportFollowUpStage(withEpisode({ status: 'assessing' }))?.kind).toBe('reviewing');
    expect(
      getSupportFollowUpStage(
        withEpisode(
          { status: 'waiting', sent_message_id: 'first' },
          { last_public_message_id: 'first' },
        ),
      )?.kind,
    ).toBe('final');
    expect(
      getSupportFollowUpStage(
        withEpisode(
          {
            status: 'waiting',
            second_message_id: 'second',
            second_sent_at: '2026-10-11T10:00:00Z',
            close_at: '2026-10-11T11:00:00Z',
          },
          { last_public_message_id: 'second' },
        ),
      )?.kind,
    ).toBe('closing');
  });
  it.each([1, undefined])('preserves legacy closure for version %s', (sequence_version) => {
    expect(
      getSupportFollowUpStage(
        withEpisode({ status: 'waiting', sequence_version, close_at: '2026-10-11T11:00:00Z' }),
      )?.kind,
    ).toBe('closing');
  });
  it.each(['resolved', 'skipped', 'handoff'])('hides terminal %s events', (status) =>
    expect(getSupportFollowUpStage(withEpisode({ status }))).toBeNull(),
  );
  it('only shows a manual stop while the same sequence is current', () => {
    expect(
      getSupportFollowUpStage(withEpisode({ status: 'cancelled', reason: 'cancelled_by_teammate' }))
        ?.kind,
    ).toBe('stopped');
    expect(
      getSupportFollowUpStage(withEpisode({ status: 'cancelled', reason: 'settings_changed' })),
    ).toBeNull();
    expect(
      getSupportFollowUpStage(
        withEpisode(
          { status: 'cancelled', reason: 'cancelled_by_teammate' },
          { last_public_message_id: 'new-ai-answer' },
        ),
      ),
    ).toBeNull();
  });
  it.each([
    { status: 'resolved' },
    { human_takeover: true },
    { assigned_user_id: 'human' },
    { flow_state: 'waiting_for_human' },
    { last_public_sender_type: 'customer' },
    { customer_awaiting_response: true },
    { ai_resumed_at: '2026-10-10T10:02:00Z' },
  ])('hides superseded context %j', (change) => {
    expect(
      getSupportFollowUpStage({ ...conversation, ...change } as SupportConversation),
    ).toBeNull();
  });
  it('hides immediately when a newer reply arrives before conversation refetch', () => {
    expect(
      getSupportFollowUpStage(conversation, {
        id: 'new-reply',
        created_at: '2026-10-10T10:03:00Z',
      }),
    ).toBeNull();
    expect(
      getSupportFollowUpStage(conversation, {
        id: 'older-reply',
        created_at: '2026-10-09T10:03:00Z',
      })?.kind,
    ).toBe('scheduled');
  });
  it('only hides legacy internal failure notices, preserving real messages and other notes', () => {
    const failure = {
      is_internal: true,
      sender_type: 'ai',
      metadata: JSON.stringify({
        support_follow_up_id: 'episode',
        follow_up_failure_reason: 'assessment_timeout',
      }),
    } as SupportMessage;
    expect(isFollowUpStatusNote(failure)).toBe(true);
    expect(isFollowUpStatusNote({ ...failure, is_internal: false })).toBe(false);
    expect(
      isFollowUpStatusNote({ ...failure, metadata: '{"support_follow_up_id":"episode"}' }),
    ).toBe(false);
    expect(isFollowUpStatusNote({ ...failure, metadata: 'not-json' })).toBe(false);
  });
});

describe('viewer-local follow-up times', () => {
  const now = new Date(2026, 9, 10, 10, 0);
  it('uses viewer calendar days and time', () => {
    const tomorrow = new Date(2026, 9, 11, 9, 30);
    expect(formatFollowUpTime(tomorrow.toISOString(), now)).toBe(
      `Tomorrow, ${tomorrow.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}`,
    );
    expect(formatFollowUpTime(now.toISOString(), now)).toContain('Today, ');
  });
  it('omits the current year and includes a different year, including tooltips', () => {
    expect(formatFollowUpTime(new Date(2026, 11, 12, 9).toISOString(), now)).not.toContain('2026');
    expect(formatTimestamp(new Date(2026, 11, 12, 9).toISOString(), now)).not.toContain('2026');
    expect(formatTimestamp(new Date(2027, 0, 1, 9).toISOString(), now)).toContain('2027');
    expect(formatFollowUpTime(new Date(2027, 0, 1, 9).toISOString(), now)).toContain('2027');
    expect(
      formatFollowUpTime(new Date(2027, 0, 1, 9).toISOString(), new Date(2026, 11, 31, 9)),
    ).toContain('2027');
  });
  it('compares local years at a UTC year boundary', () => {
    const instant = new Date('2027-01-01T00:30:00Z');
    const label = formatFollowUpTime(instant.toISOString(), new Date(2026, 6, 10));
    expect(label.includes('2027')).toBe(instant.getFullYear() === 2027);
  });
  it('handles the next calendar day across daylight saving changes', () => {
    const before = new Date(2026, 2, 7, 12),
      after = new Date(2026, 2, 8, 12);
    expect(formatFollowUpTime(after.toISOString(), before)).toContain('Tomorrow, ');
  });
});
