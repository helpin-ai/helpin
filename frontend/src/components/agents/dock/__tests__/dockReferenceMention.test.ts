import { describe, expect, it } from 'vitest';
import { dockReferenceMention } from '../dockReferenceMention';

describe('dockReferenceMention', () => {
  it.each(['@', '@fix', 'Review @fix', 'Review\n@fix', 'Review\t@fix'])('recognizes a standalone token in %j', (value) => {
    const start = value.indexOf('@');
    expect(dockReferenceMention(value, value.length, value.length)).toEqual({ start, end: value.length, query: value.slice(start + 1) });
  });

  it.each(['name@example.com', 'pkg@version', '@@fix', '@fix ', 'hello@', 'plain text'])('leaves ordinary text %j alone', (value) => {
    expect(dockReferenceMention(value, value.length, value.length)).toBeNull();
  });

  it('captures the whole token when editing its middle, preserving surrounding text', () => {
    expect(dockReferenceMention('Review @fix tomorrow', 9, 9)).toEqual({ start: 7, end: 11, query: 'fix' });
  });

  it('does not autocomplete a text selection or inside an email-like token', () => {
    expect(dockReferenceMention('@fix', 0, 4)).toBeNull();
    expect(dockReferenceMention('@name@example.com', 3, 3)).toBeNull();
  });
});
