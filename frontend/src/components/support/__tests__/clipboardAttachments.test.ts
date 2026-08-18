import { describe, expect, it } from 'vitest';
import { getClipboardImageFiles } from '@/lib/clipboardAttachments';

describe('getClipboardImageFiles', () => {
  it('returns only image files from clipboard data', () => {
    const image = new File(['image'], 'screenshot.png', { type: 'image/png' });
    const text = new File(['text'], 'notes.txt', { type: 'text/plain' });

    expect(getClipboardImageFiles({ files: [image, text] })).toEqual([image]);
  });

  it('returns an empty list when the clipboard has no files', () => {
    expect(getClipboardImageFiles(null)).toEqual([]);
    expect(getClipboardImageFiles({ files: [] })).toEqual([]);
  });
});
