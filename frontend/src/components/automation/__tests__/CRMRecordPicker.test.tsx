// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CRMRecordPicker } from '../CRMRecordPicker';
import { getCRMAgentRecord, listCRMAgentRecords } from '@/lib/services/crmAgentRecordService';

vi.mock('@/lib/services/crmAgentRecordService', () => ({ getCRMAgentRecord: vi.fn(), listCRMAgentRecords: vi.fn() }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
Element.prototype.scrollIntoView = vi.fn();

let container: HTMLDivElement;
let root: Root;
let client: QueryClient;

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  vi.mocked(listCRMAgentRecords).mockResolvedValue({ items: [], total: 0 });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  container.remove();
  vi.useRealTimers();
});

async function settle() {
  await act(async () => { await vi.advanceTimersByTimeAsync(300); });
}

async function render(props: Partial<React.ComponentProps<typeof CRMRecordPicker>> = {}) {
  const onChange = props.onChange ?? vi.fn();
  await act(async () => root.render(
    <QueryClientProvider client={client}>
      <CRMRecordPicker workspaceId="ws-1" targetType="crm_company" value="" onChange={onChange} {...props} />
    </QueryClientProvider>,
  ));
  await settle();
  return onChange;
}

async function openPicker() {
  act(() => container.querySelector('button')?.click());
  await settle();
}

describe('CRM record picker', () => {
  it('loads only when opened and selects an exact record with its familiar name', async () => {
    vi.mocked(listCRMAgentRecords).mockResolvedValue({ items: [
      { id: 'company-1', name: 'Acme', detail: 'one.test' },
      { id: 'company-2', name: 'Acme', detail: 'two.test' },
    ], total: 2 });
    const onChange = await render();
    expect(container.textContent).toContain('Choose a company');
    expect(listCRMAgentRecords).not.toHaveBeenCalled();
    await openPicker();
    expect(listCRMAgentRecords).toHaveBeenCalledWith('ws-1', 'crm_company', '');
    const item = document.querySelector('[cmdk-item][data-value="company-2"]');
    expect(item?.textContent).toContain('two.test');
    act(() => (item as HTMLElement).click());
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith('company-2');
  });

  it('searches on the server and makes a partial result list explicit', async () => {
    vi.mocked(listCRMAgentRecords).mockImplementation(async (_ws, _type, query) => ({
      items: [{ id: query ? 'company-900' : 'company-1', name: query || 'Acme', detail: '' }], total: query ? 1 : 900,
    }));
    await render();
    await openPicker();
    expect(document.body.textContent).toContain('Showing 1 of 900');
    const input = document.querySelector('[cmdk-input]') as HTMLInputElement;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(input, 'Zebra');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(document.querySelector('[data-value="company-1"]')).toBeNull();
    await settle();
    await settle();
    expect(listCRMAgentRecords).toHaveBeenLastCalledWith('ws-1', 'crm_company', 'Zebra');
    expect(document.querySelector('[data-value="company-900"]')?.textContent).toContain('Zebra');
  });

  it('resolves an existing selection without requiring it on the first page', async () => {
    vi.mocked(getCRMAgentRecord).mockResolvedValue({ id: 'company-900', name: 'Saved company', detail: '' });
    const onChange = await render({ value: 'company-900' });
    expect(container.textContent).toContain('Saved company');
    expect(getCRMAgentRecord).toHaveBeenCalledWith('ws-1', 'crm_company', 'company-900');
    expect(onChange).not.toHaveBeenCalled();
    expect(listCRMAgentRecords).not.toHaveBeenCalled();
  });

  it('does not expose raw errors or silently replace unavailable records', async () => {
    vi.mocked(getCRMAgentRecord).mockRejectedValue(new Error('private database error'));
    vi.mocked(listCRMAgentRecords).mockRejectedValue(new Error('private database error'));
    const onChange = await render({ value: 'missing' });
    expect(container.textContent).toContain('Company unavailable');
    await openPicker();
    expect(document.body.textContent).toContain('Check your CRM access or try again');
    expect(document.body.textContent).not.toContain('private database error');
    expect(document.body.textContent).not.toContain('No companies');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('clears the popup and scopes lookups when workspace or record type changes', async () => {
    vi.mocked(listCRMAgentRecords).mockResolvedValue({ items: [{ id: 'company-1', name: 'Old workspace company', detail: '' }], total: 1 });
    await render();
    await openPicker();
    expect(document.body.textContent).toContain('Old workspace company');
    vi.mocked(listCRMAgentRecords).mockResolvedValue({ items: [], total: 0 });
    await render({ workspaceId: 'ws-2', targetType: 'crm_contact' });
    expect(document.body.textContent).not.toContain('Old workspace company');
    expect(container.textContent).toContain('Choose a contact');
    await openPicker();
    expect(listCRMAgentRecords).toHaveBeenLastCalledWith('ws-2', 'crm_contact', '');
    expect(document.body.textContent).toContain('No contacts available');
  });

  it('does not open or search when disabled', async () => {
    await render({ disabled: true });
    await openPicker();
    expect(container.querySelector('button')?.disabled).toBe(true);
    expect(listCRMAgentRecords).not.toHaveBeenCalled();
  });
});
