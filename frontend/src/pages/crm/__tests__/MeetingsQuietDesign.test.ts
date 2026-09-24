import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const meetingsSource = readFileSync(resolve(__dirname, '../Meetings.tsx'), 'utf8');
const meetingDetailSource = readFileSync(resolve(__dirname, '../MeetingDetail.tsx'), 'utf8');
const upcomingSource = readFileSync(resolve(__dirname, '../../../components/crm/UpcomingCalendarMeetings.tsx'), 'utf8');
const createDialogSource = readFileSync(resolve(__dirname, '../../../components/crm/CreateMeetingDialog.tsx'), 'utf8');
const recordingSource = readFileSync(resolve(__dirname, '../../../components/crm/MeetingRecordingPlayer.tsx'), 'utf8');
const processingSource = readFileSync(resolve(__dirname, '../../../components/crm/MeetingProcessingState.tsx'), 'utf8');

describe('CRM meetings Quiet Hairline composition', () => {
  it('uses centralized page, section, list, and empty-state primitives', () => {
    expect(meetingsSource).toContain('<QuietPageViewport className="min-h-0 flex-1">');
    expect(meetingsSource).toContain('<QuietPageHeader');
    expect(meetingsSource).toContain('variant="shell"');
    expect(meetingsSource.indexOf('<QuietPageHeader')).toBeLessThan(meetingsSource.indexOf('<QuietPageViewport'));
    expect(meetingsSource).toContain('<QuietListRow');
    expect(meetingsSource).toContain('<QuietEmptyState');
    expect(upcomingSource).toContain('<QuietSection');
    expect(upcomingSource).toContain('<QuietListRow');
    expect(meetingsSource).toContain('<QuietSearchInput');
    expect(meetingsSource).not.toContain('<QuietUnderlineInput');
  });

  it('uses an editable Quiet detail header, detail rail, and shared tabs for meeting detail', () => {
    expect(meetingDetailSource).toContain('<QuietDetailHeader');
    expect(meetingDetailSource).toContain('<QuietBreadcrumbs');
    expect(meetingDetailSource).toContain('<QuietDetailAction');
    expect(meetingDetailSource).toContain('<QuietTitleInput');
    expect(meetingDetailSource).toContain('presentation="header"');
    expect(meetingDetailSource).toContain('useUpdateCRMMeeting');
    expect(meetingDetailSource).toContain('<QuietDetailLayout');
    expect(meetingDetailSource).toContain('<TabsList variant="quiet"');
    expect(meetingDetailSource).toContain('Based on transcript');
    expect(meetingDetailSource).toContain('formatMeetingDate');
    expect(meetingDetailSource).not.toContain('title="Meeting notes"');
  });

  it('keeps follow-up work prominent and progressively discloses task routing', () => {
    expect(meetingDetailSource.indexOf('>Action items<')).toBeLessThan(meetingDetailSource.indexOf('title="Decisions made"'));
    expect(meetingDetailSource).not.toContain('Action items and next steps');
    expect(meetingDetailSource).toContain('Change destination');
    expect(meetingDetailSource).toContain('View transcript evidence');
    expect(meetingDetailSource).toContain('rounded-lg border border-border/70 bg-card');
    expect(meetingDetailSource).toContain('className="mt-4 grid gap-3 md:grid-cols-2"');
  });

  it('explains and disables capture when the server has no capture provider', () => {
    for (const source of [meetingsSource, meetingDetailSource]) {
      expect(source).toContain("capture_configured === false");
      expect(source).toContain('<ServerSetupNotice');
      expect(source).toContain('Meeting capture isn’t set up on this server.');
    }
    expect(meetingDetailSource).toContain('disabled={startCapture.isPending || captureUnavailable}');
    expect(meetingDetailSource).toContain('aria-describedby={captureUnavailable ? CAPTURE_UNAVAILABLE_ID : undefined}');
    expect(meetingsSource).toContain('calendarConnectUnavailable={googleConnectUnavailable}');
  });

  it('retains the compact speaker-and-transcript row design', () => {
    expect(meetingDetailSource).toContain("sm:grid-cols-[120px_minmax(0,1fr)]");
    expect(meetingDetailSource).toContain("active && 'bg-primary/5 ring-1 ring-primary/15'");
    expect(meetingDetailSource).toContain('text-sm leading-6 text-muted-foreground');
  });

  it('keeps forms, recording, and processing free of card and gradient chrome', () => {
    expect(createDialogSource).toContain('variant="flush"');
    expect(createDialogSource).toContain('<QuietUnderlineInput');
    expect(recordingSource).toContain('<QuietSection');
    expect(processingSource).toContain('border-l border-quiet-divider-strong');

    for (const source of [meetingsSource, meetingDetailSource, upcomingSource, recordingSource, processingSource]) {
      expect(source).not.toContain('<Card');
      expect(source).not.toContain('<Badge');
      expect(source).not.toContain('bg-gradient');
      expect(source).not.toContain('MeetingStatusBadge');
    }
  });
});
