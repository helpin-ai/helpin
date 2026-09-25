import { describe, expect, it } from 'vitest';

import {
  helpinImageMarker,
  helpinReferenceMarker,
  helpinReferenceRoute,
  parseHelpinReference,
  parseHelpinImageMarker,
  parseHelpinReferenceMarker,
} from '@/lib/helpinReferences';

describe('Helpin references', () => {
  it('parses canonical entity and artifact references', () => {
    expect(parseHelpinReference('helpin://tasks/task-1')).toEqual({ type: 'tasks', id: 'task-1' });
    expect(parseHelpinReference('helpin://documents/doc-1')).toEqual({ type: 'documents', id: 'doc-1' });
    expect(parseHelpinReference('helpin://sprints/sprint-1')).toEqual({ type: 'sprints', id: 'sprint-1' });
    expect(parseHelpinReference('helpin://objectives/objective-1')).toEqual({ type: 'objectives', id: 'objective-1' });
    expect(parseHelpinReference('helpin://artifacts/asset-1')).toEqual({ type: 'artifacts', id: 'asset-1' });
  });

  it('accepts legacy references without emitting legacy routes', () => {
    expect(parseHelpinReference('helpin-artifact://asset-1')).toEqual({ type: 'artifacts', id: 'asset-1' });
    expect(parseHelpinReference('helpin://document/doc-1')).toEqual({ type: 'documents', id: 'doc-1' });
  });

  it('rejects unknown types and nested IDs', () => {
    expect(parseHelpinReference('helpin://unknown/item-1')).toBeNull();
    expect(parseHelpinReference('helpin://tasks/folder/item-1')).toBeNull();
    expect(parseHelpinReference('https://helpin.ai/tasks/task-1')).toBeNull();
  });

  it('marks only artifact images for inline rendering', () => {
    const marker = helpinImageMarker('helpin://artifacts/img-1');
    expect(parseHelpinImageMarker(marker ?? undefined)).toEqual({ type: 'artifacts', id: 'img-1' });
    expect(helpinImageMarker('helpin://tasks/task-1')).toBeNull();
    expect(helpinImageMarker('https://example.com/a.png')).toBeNull();
    expect(parseHelpinImageMarker(helpinReferenceMarker('helpin://artifacts/img-1') ?? undefined)).toBeNull();
  });

  it('round trips references through a sanitizer-safe marker', () => {
    const marker = helpinReferenceMarker('helpin://tasks/task-1');
    expect(marker).toBeTruthy();
    expect(parseHelpinReferenceMarker(marker ?? undefined)).toEqual({ type: 'tasks', id: 'task-1' });
  });

  it('builds current-workspace routes and keeps artifacts private', () => {
    expect(helpinReferenceRoute({ type: 'tasks', id: 'task-1' }, 'acme')).toBe('/w/acme/pm/tasks/task-1');
    expect(helpinReferenceRoute({ type: 'documents', id: 'doc-1' }, 'acme')).toBe('/w/acme/docs/documents/doc-1');
    expect(helpinReferenceRoute({ type: 'sprints', id: 'sprint-1' }, 'acme')).toBe('/w/acme/pm/sprints/sprint-1');
    expect(helpinReferenceRoute({ type: 'artifacts', id: 'asset-1' }, 'acme')).toBeNull();
  });
});
