// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';
import { SupportAttachmentGallery } from '../SupportAttachmentGallery';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('support video attachments', () => {
  it('renders native playback and an independent download fallback beside ordinary files', async () => {
    const container = document.createElement('div');
    const root = createRoot(container);
    try {
      await act(async () => root.render(<SupportAttachmentGallery attachments={[
        { id: 'video', file_key: 'recording.mov', file_name: 'recording.mov', file_type: 'video/quicktime', file_size: 104857600, url: 'https://cdn.example.com/recording.mov' },
        { id: 'pdf', file_key: 'notes.pdf', file_name: 'notes.pdf', file_type: 'application/pdf', file_size: 512, url: 'https://cdn.example.com/notes.pdf' },
      ]} />));
      const video = container.querySelector('video');
      expect(video?.controls).toBe(true);
      expect(video?.playsInline).toBe(true);
      expect(video?.preload).toBe('metadata');
      expect(video?.querySelector('source')?.getAttribute('type')).toBe('video/quicktime');
      expect(video?.autoplay).toBe(false);
      const fallback = container.querySelector('a[download="recording.mov"]');
      expect(fallback?.getAttribute('href')).toBe('https://cdn.example.com/recording.mov');
      expect(video?.contains(fallback)).toBe(false);
      expect(container.querySelectorAll('a[href="https://cdn.example.com/recording.mov"]')).toHaveLength(1);
      expect(container.querySelector('a[href="https://cdn.example.com/notes.pdf"]')).not.toBeNull();
      await act(async () => { video?.querySelector('source')?.dispatchEvent(new Event('error')); });
      expect(container.querySelector('[role="status"]')?.textContent).toContain('Download it to watch');
      expect(container.querySelector('a[download="recording.mov"]')).not.toBeNull();
    } finally {
      await act(async () => root.unmount());
    }
  });
});
