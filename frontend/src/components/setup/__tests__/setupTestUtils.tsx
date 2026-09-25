import { act, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot, type Root } from 'react-dom/client';
import type { Capability } from '@/lib/capabilityTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

export type Rendered = { container: HTMLDivElement; root: Root; client: QueryClient; unmount: () => Promise<void> };

/** Renders with a fresh query client and flushes the initial queries. */
export async function renderWithQuery(node: ReactNode): Promise<Rendered> {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => { root.render(<QueryClientProvider client={client}>{node}</QueryClientProvider>); });
  await flush();
  return {
    container,
    root,
    client,
    unmount: async () => {
      await act(async () => root.unmount());
      container.remove();
      client.clear();
    },
  };
}

export async function flush() {
  for (let i = 0; i < 4; i += 1) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

export function button(container: ParentNode, name: string) {
  return Array.from(container.querySelectorAll('button')).find((element) =>
    element.getAttribute('aria-label') === name || element.textContent?.trim() === name,
  ) as HTMLButtonElement | undefined;
}

export async function click(element: HTMLElement | undefined) {
  if (!element) throw new Error('element not found');
  await act(async () => { element.click(); });
  await flush();
}

export async function type(input: HTMLInputElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

export function capability(overrides: Partial<Capability> & Pick<Capability, 'key'>): Capability {
  return { status: 'needs_setup', detail: `${overrides.key} detail`, required: false, ...overrides };
}
