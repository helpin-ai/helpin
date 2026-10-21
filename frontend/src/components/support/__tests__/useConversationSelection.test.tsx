// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';
import { loadConversationSelection } from '@/lib/supportBulkActions';
import { useConversationSelection } from '../useConversationSelection';

vi.mock('@/lib/supportBulkActions', () => ({ loadConversationSelection: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
const conversation = (id: string) => ({ id }) as SupportConversation;
let selection: ReturnType<typeof useConversationSelection>;
function Harness({ scope }: { scope: string }) {
  selection = useConversationSelection(scope);
  return <span>{selection.items.size}</span>;
}

describe('conversation selection', () => {
  let root: Root;
  let container: HTMLDivElement;
  beforeEach(() => {
    vi.resetAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => root.render(<Harness scope="inbox" />));
  });
  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('clears on view changes and does not restore a hidden selection when returning', () => {
    act(() => selection.toggle(conversation('1')));
    expect(selection.items.size).toBe(1);
    act(() => root.render(<Harness scope="resolved" />));
    expect(selection.items.size).toBe(0);
    act(() => root.render(<Harness scope="inbox" />));
    expect(selection.items.size).toBe(0);
  });

  it('keeps failed items selected and allows retry without the successful items', () => {
    act(() => selection.selectLoaded([conversation('1'), conversation('2')]));
    act(() => selection.removeCompleted(['1']));
    expect([...selection.items.keys()]).toEqual(['2']);
  });

  it('applies the same client-side filters when selecting unloaded matches', async () => {
    vi.mocked(loadConversationSelection).mockResolvedValue([conversation('1'), conversation('2')]);
    await act(async () => selection.selectAllMatches('ws', {}, (items) => items.filter((item) => item.id !== '2')));
    expect([...selection.items.keys()]).toEqual(['1']);
    expect(selection.allMatches).toBe(true);
  });

  it('ignores an old response if the user clears selection while matches load', async () => {
    let resolve!: (items: SupportConversation[]) => void;
    vi.mocked(loadConversationSelection).mockImplementation(() => new Promise((done) => { resolve = done; }));
    let pending!: Promise<void>;
    act(() => { selection.toggle(conversation('1')); });
    act(() => { pending = selection.selectAllMatches('ws', {}, (items) => items); });
    expect(selection.loading).toBe(true);
    act(() => selection.clear());
    await act(async () => { resolve([conversation('1'), conversation('2')]); await pending; });
    expect(selection.items.size).toBe(0);
    expect(selection.loading).toBe(false);
  });

  it('does not let an old action clear a new view selection', () => {
    act(() => selection.toggle(conversation('1')));
    const oldComplete = selection.removeCompleted;
    act(() => root.render(<Harness scope="another-workspace" />));
    act(() => selection.toggle(conversation('1')));
    act(() => oldComplete(['1']));
    expect([...selection.items.keys()]).toEqual(['1']);
  });
});
