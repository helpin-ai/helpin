// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { EntitySummaryCard } from '../EntitySummaryCard';

const entitySummarySource = readFileSync(resolve(__dirname, '../EntitySummaryCard.tsx'), 'utf8');

const summaryState = vi.hoisted(() => ({
  contactSummary: null as null | { summary_markdown: string; highlights: unknown[]; status: string },
  contactPending: false,
  companyPending: false,
  refreshContact: vi.fn(),
  refreshCompany: vi.fn(),
  refetchContact: vi.fn(),
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

vi.mock('@/hooks/queries', () => ({
  useContactSummary: () => ({
    data: summaryState.contactSummary,
    isLoading: false,
    isFetching: false,
    refetch: summaryState.refetchContact,
  }),
  useDealSummary: () => ({ data: null, isLoading: false, isFetching: false, refetch: vi.fn() }),
  useCompanySummary: () => ({ data: null, isLoading: false, isFetching: false, refetch: vi.fn() }),
  useRefreshContactSummary: () => ({ mutate: summaryState.refreshContact, isPending: summaryState.contactPending }),
  useRefreshCompanySummary: () => ({ mutate: summaryState.refreshCompany, isPending: summaryState.companyPending }),
  useRefreshDealSummary: () => ({ mutate: vi.fn(), isPending: false }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  summaryState.contactSummary = null;
  summaryState.contactPending = false;
  summaryState.companyPending = false;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

describe('EntitySummaryCard', () => {
  it('uses the Deal-style dashed card for every overview summary state', () => {
    expect(entitySummarySource).toContain('rounded-md border border-dashed border-border/70 bg-muted/25');
    expect(entitySummarySource.match(/className=\{overviewSummaryCardClassName\}/g)).toHaveLength(3);
  });

  it('generates a contact summary instead of refetching the empty read model', () => {
    act(() => root.render(<EntitySummaryCard workspaceId="workspace-1" contactId="contact-1" />));

    const button = [...container.querySelectorAll('button')]
      .find((candidate) => candidate.textContent?.includes('Generate anyway'));
    expect(button).toBeDefined();

    act(() => button?.click());

    expect(summaryState.refreshContact).toHaveBeenCalledTimes(1);
    expect(summaryState.refetchContact).not.toHaveBeenCalled();
  });

  it('disables the action and shows generation progress while the request is active', () => {
    summaryState.contactPending = true;
    act(() => root.render(<EntitySummaryCard workspaceId="workspace-1" contactId="contact-1" />));

    const button = [...container.querySelectorAll('button')]
      .find((candidate) => candidate.textContent?.includes('Generating'));
    expect(button).toBeDefined();
    expect(button?.hasAttribute('disabled')).toBe(true);
  });

  it('routes company generation through the company refresh endpoint', () => {
    act(() => root.render(<EntitySummaryCard workspaceId="workspace-1" companyId="company-1" />));

    const button = [...container.querySelectorAll('button')]
      .find((candidate) => candidate.textContent?.includes('Generate anyway'));
    act(() => button?.click());

    expect(summaryState.refreshCompany).toHaveBeenCalledTimes(1);
    expect(summaryState.refreshContact).not.toHaveBeenCalled();
  });
});
