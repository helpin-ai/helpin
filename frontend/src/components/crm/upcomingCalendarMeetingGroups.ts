import type { CRMCalendarMeetingCandidate } from '@/lib/crmMeetingTypes';

export interface UpcomingMeetingGroup {
  key: string;
  recurring: boolean;
  occurrences: CRMCalendarMeetingCandidate[];
}

export function groupUpcomingMeetings(candidates: CRMCalendarMeetingCandidate[]): UpcomingMeetingGroup[] {
  const groups = new Map<string, UpcomingMeetingGroup>();
  candidates.forEach((candidate) => {
    const seriesId = candidate.event.recurring_series_id;
    const key = seriesId
      ? `series:${candidate.event.email_account_id}:${seriesId}`
      : `event:${candidate.event.id}`;
    const group = groups.get(key) ?? { key, recurring: Boolean(seriesId), occurrences: [] };
    group.occurrences.push(candidate);
    groups.set(key, group);
  });
  return Array.from(groups.values());
}
