// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { SupportAttachmentGallery } from '../SupportAttachmentGallery';
import { TooltipProvider } from '@/components/ui/tooltip';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

it('keeps failed files retryable across processing attempts without broken download links', async () => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  let retried = '';
  async function render(status: 'processing' | 'failed') {
    await act(async () => root.render(
      <TooltipProvider>
        <SupportAttachmentGallery
          attachments={[{
            id: 'attachment', file_key: '', file_name: 'report.pdf', file_type: 'application/pdf',
            file_size: 12, url: '', processing_status: status,
          }]}
          onRetry={async (id) => { retried = id; }}
        />
      </TooltipProvider>,
    ));
  }
  try {
    await render('failed');
    expect(container.textContent).toContain('Unavailable');
    expect(container.querySelector('a')).toBeNull();
    const retry = Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Retry');
    expect(retry).toBeTruthy();
    await act(async () => retry!.click());
    expect(retried).toBe('attachment');

    await render('processing');
    expect(container.textContent).toContain('Processing…');
    expect(container.querySelector('button')).toBeNull();
    await render('failed');
    expect(container.textContent).toContain('Unavailable');
    expect(container.querySelector('button')?.textContent).toBe('Retry');
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});
