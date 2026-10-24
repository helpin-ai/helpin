import { describe, expect, it } from 'vitest';
import { getSupportInvestigationReport } from '../supportInvestigationReport';

const report = "## What's happening\n\nThe post was saved as a draft.\n\n## Suggested reply\n\nOpen the post and click Schedule.\n\n## For engineering\n\n`ApprovalBuilder.php:233` skips drafts.";

describe('getSupportInvestigationReport', () => {
  it('splits engineering detail from the support summary', () => {
    const metadata = JSON.stringify({ report_format: 'support_v1', suggested_reply: 'Open the post and click Schedule.' });
    expect(getSupportInvestigationReport({ metadata }, report)).toEqual({
      summary: "## What's happening\n\nThe post was saved as a draft.\n\n## Suggested reply\n\nOpen the post and click Schedule.",
      engineering: '`ApprovalBuilder.php:233` skips drafts.',
      suggestedReply: 'Open the post and click Schedule.',
    });
  });

  it('keeps a note without the engineering heading whole', () => {
    const metadata = JSON.stringify({ report_format: 'support_v1' });
    expect(getSupportInvestigationReport({ metadata }, 'Translated summary')).toEqual({ summary: 'Translated summary', engineering: '', suggestedReply: '' });
  });

  it.each([
    ['older code check notes', JSON.stringify({ investigation_id: 'job' })],
    ['notes without metadata', undefined],
    ['unreadable metadata', '{'],
  ])('ignores %s', (_label, metadata) => {
    expect(getSupportInvestigationReport({ metadata }, report)).toBeNull();
  });
});
