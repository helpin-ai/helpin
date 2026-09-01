import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { MeetingStatusText } from '../MeetingStatusText';

describe('MeetingStatusText', () => {
  it('uses current, positive, and blocker Quiet status semantics', () => {
    const active = renderToStaticMarkup(<MeetingStatusText status="recording" />);
    const ready = renderToStaticMarkup(<MeetingStatusText status="ready" />);
    const failed = renderToStaticMarkup(<MeetingStatusText status="failed" />);

    expect(active).toContain('bg-quiet-text-primary');
    expect(active).toContain('motion-safe:animate-pulse');
    expect(ready).toContain('bg-quiet-positive');
    expect(failed).toContain('bg-quiet-accent');
    expect(active).not.toContain('data-slot="badge"');
  });

  it('uses the shared badge treatment in detail headers', () => {
    const ready = renderToStaticMarkup(<MeetingStatusText status="ready" presentation="badge" />);

    expect(ready).toContain('data-slot="badge"');
    expect(ready).toContain('bg-quiet-positive/10');
    expect(ready).not.toContain('size-1.5');
  });
});
