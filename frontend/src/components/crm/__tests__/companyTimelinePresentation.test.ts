import { describe, expect, it } from 'vitest';

import type { CRMCompanyTimelineItem } from '@/lib/crmTypes';
import { companyTimelinePresentation, dedupeCompanyTimelineItems } from '../companyTimelinePresentation';

function timelineItem(patch: Partial<CRMCompanyTimelineItem> = {}): CRMCompanyTimelineItem {
  return {
    id: 'activity:1',
    kind: 'note',
    event_type: 'activity.note',
    source_type: 'crm_activity',
    source_id: '1',
    title: 'Renewal discussion',
    occurred_at: '2026-08-21T10:00:00Z',
    can_edit: true,
    can_delete: true,
    ...patch,
  };
}

describe('companyTimelinePresentation', () => {
  it('turns manual notes into authored content without repeating their subject in the action', () => {
    const presentation = companyTimelinePresentation(timelineItem({
      actor: { type: 'actor', id: 'member-1', name: 'Waqar' },
      description: 'Send the revised pricing on Monday.',
    }));

    expect(presentation).toMatchObject({
      mode: 'content',
      actorKind: 'human',
      label: 'Note added',
      attribution: 'Waqar',
      contentTitle: 'Renewal discussion',
      contentBody: 'Send the revised pricing on Monday.',
      contentFormat: 'plain',
    });
  });

  it('presents captured meeting intelligence as system-authored Markdown', () => {
    const presentation = companyTimelinePresentation(timelineItem({
      kind: 'meeting',
      event_type: 'meeting.captured',
      title: 'Quarterly planning',
      description: '**Decision:** renew for one year.',
      actor: { type: 'actor', id: 'member-1', name: 'Meeting owner' },
      can_edit: false,
      can_delete: false,
    }));

    expect(presentation).toMatchObject({
      mode: 'content',
      actorKind: 'system',
      label: 'Meeting · Quarterly planning notes added',
      attribution: 'System',
      contentFormat: 'markdown',
    });
  });

  it('uses contact-aware natural language for email previews', () => {
    const presentation = companyTimelinePresentation(timelineItem({
      kind: 'email',
      event_type: 'email.inbound',
      source_type: 'crm_email_message',
      title: 'Re: renewal',
      description: 'Looks good to us.',
      contact: { type: 'contact', id: 'contact-1', name: 'Sarah Chen' },
      can_edit: false,
      can_delete: false,
    }));

    expect(presentation.label).toBe('Email · Re: renewal received from Sarah Chen');
    expect(presentation.mode).toBe('content');
    expect(presentation.actorKind).toBe('unknown');
  });

  it('keeps task changes compact and replaces “this task” with its linked identity', () => {
    const presentation = companyTimelinePresentation(timelineItem({
      kind: 'task',
      event_type: 'task.state_changed',
      source_type: 'pm_activity',
      title: 'Prepare renewal plan',
      description: 'moved this task to Done',
      actor: { type: 'actor', id: 'user-1', name: 'Amad' },
      entity: { type: 'task', id: 'task-1', name: 'Prepare renewal plan', display_id: 'HLP-42' },
      can_edit: false,
      can_delete: false,
    }));

    expect(presentation.mode).toBe('compact');
    expect(presentation.label).toBe('Task HLP-42 · Prepare renewal plan moved to Done');
    expect(presentation.attribution).toBe('Amad');
    expect(presentation.emphasizedValues).toEqual(expect.arrayContaining(['HLP-42', 'Prepare renewal plan']));
  });

  it('does not attribute legacy manual activity to System when its actor is missing', () => {
    const call = companyTimelinePresentation(timelineItem({
      kind: 'call',
      event_type: 'activity.call',
      title: 'Meeting with Waqar',
      description: 'Everything that was needed to be discussed was discussed.',
      actor: undefined,
    }));

    expect(call.label).toBe('Call · Meeting with Waqar logged');
    expect(call.label).not.toContain('System');
    expect(call.attribution).toBeUndefined();
    expect(call.actorKind).toBe('unknown');
  });

  it('puts a deal before its action and actor attribution', () => {
    const deal = companyTimelinePresentation(timelineItem({
      kind: 'deal',
      event_type: 'deal.stage_changed',
      source_type: 'pm_activity',
      title: 'Pro plan',
      description: 'moved this deal from Presentation Scheduled to Decision Maker Bought-In',
      actor: { type: 'actor', id: 'user-1', name: 'Amad Ali' },
      entity: { type: 'deal', id: 'deal-2', name: 'Pro plan', display_id: 'DEAL-2' },
      can_edit: false,
      can_delete: false,
    }));

    expect(deal.label).toBe('DEAL-2 · Pro plan moved from Presentation Scheduled to Decision Maker Bought-In');
    expect(deal.attribution).toBe('Amad Ali');
  });
});

describe('dedupeCompanyTimelineItems', () => {
  it('keeps the detailed deal stage event when its legacy event arrives at the same time', () => {
    const detailed = timelineItem({
      id: 'deal:detailed',
      kind: 'deal',
      event_type: 'deal.stage_changed',
      source_type: 'pm_activity',
      title: 'Pro plan',
      description: 'moved this deal from Qualified to Buy to Presentation Scheduled',
      occurred_at: '2026-08-21T10:58:01.714993Z',
      entity: { type: 'deal', id: 'deal-2', name: 'Pro plan', display_id: 'DEAL-2' },
      can_edit: false,
      can_delete: false,
    });
    const legacy = {
      ...detailed,
      id: 'deal:legacy',
      description: 'moved this deal to Presentation Scheduled',
      occurred_at: '2026-08-21T10:58:01.709965Z',
    };

    expect(dedupeCompanyTimelineItems([detailed, legacy])).toEqual([detailed]);
  });

  it('retains a separate transition outside the duplicate window', () => {
    const detailed = timelineItem({
      id: 'deal:detailed',
      kind: 'deal',
      event_type: 'deal.stage_changed',
      source_type: 'pm_activity',
      description: 'moved this deal from Qualified to Buy to Presentation Scheduled',
      occurred_at: '2026-08-21T10:58:01Z',
      entity: { type: 'deal', id: 'deal-2', name: 'Pro plan', display_id: 'DEAL-2' },
    });
    const later = {
      ...detailed,
      id: 'deal:later',
      description: 'moved this deal to Presentation Scheduled',
      occurred_at: '2026-08-21T11:05:00Z',
    };

    expect(dedupeCompanyTimelineItems([detailed, later])).toHaveLength(2);
  });
});
