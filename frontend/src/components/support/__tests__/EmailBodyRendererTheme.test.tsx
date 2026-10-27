// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { EmailBodyRenderer } from '../EmailBodyRenderer';

vi.mock('next-themes', () => ({ useTheme: () => ({ resolvedTheme: 'dark' }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const html = '<style>.branded{background:#fff;color:#111}</style><div class="branded" style="color: black !important; background: white !important; padding: 12px"><a href="https://example.com">Invoice</a><img src="https://example.com/logo.png"><blockquote data-helpin-quote="true">History</blockquote></div><script>alert(1)</script>';
let cleanup: (() => Promise<void>) | undefined;
afterEach(async () => { await cleanup?.(); cleanup = undefined; });

it('shows theme-aware emails in theme colors only, with no formatting toggle', async () => {
 const container = document.createElement('div'); document.body.appendChild(container);
 const root = createRoot(container);
 cleanup = async () => { await act(async()=>root.unmount()); container.remove(); };
 await act(async()=>root.render(<EmailBodyRenderer themeAware html={html} />));
 const source = container.querySelector('iframe')!.getAttribute('srcdoc')!;
 expect(source).not.toContain('color: black !important');
 expect(source).toContain('padding: 12px');
 expect(source).toContain('data-helpin-quote="true"');
 expect(source).toContain('https://example.com/logo.png');
 expect(source).not.toContain('<script>');
 expect(container.querySelector('button')).toBeNull();
 expect(container.textContent).not.toContain('Original formatting');
 expect(container.querySelector('iframe')!.getAttribute('sandbox')).not.toContain('allow-scripts');
});

it('preserves original formatting for callers outside support that do not opt in', async () => {
 const container = document.createElement('div'); document.body.appendChild(container);
 const root = createRoot(container);
 cleanup = async () => { await act(async()=>root.unmount()); container.remove(); };
 await act(async()=>root.render(<EmailBodyRenderer html={html} />));
 expect(container.querySelector('iframe')!.getAttribute('srcdoc')).toContain('color: black !important');
 expect(container.textContent).not.toContain('Original formatting');
});
