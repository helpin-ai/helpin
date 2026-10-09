// @vitest-environment jsdom
import { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { TeamInboxAssignmentFields } from '../TeamInboxAssignmentFields';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { InboxAssignmentMode } from '../teamInboxAssignment';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

beforeAll(() => {
  globalThis.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.scrollIntoView = () => {};
});

let root: Root;
afterEach(() => {
  if (root) act(() => root.unmount());
  document.body.innerHTML = '';
});

function renderPicker(count: number, mode: InboxAssignmentMode = 'round_robin') {
  const container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  const members = Array.from({ length: count }, (_, i) => ({ id: `wm-${i}`, user_id: `user-${i}`, display_name: `Member ${i}`, email: `member${i}@example.com`, role: 'member', status: 'active' }));
  function Harness() {
    const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
    return <TeamInboxAssignmentFields mode={mode} members={members} selectedIDs={selectedIDs} loading={false} onModeChange={() => {}} onMembersChange={setSelectedIDs} />;
  }
  act(() => root.render(<TooltipProvider><Harness /></TooltipProvider>));
}

function click(element: Element | null | undefined) {
  expect(element).toBeTruthy();
  act(() => element!.dispatchEvent(new MouseEvent('click', { bubbles: true })));
}

function button(text: string) {
  return Array.from(document.querySelectorAll('button')).find(item => item.textContent === text);
}

describe('inbox assignment picker', () => {
  it('selects all eligible members and lets users clear the selection', () => {
    renderPicker(6);
    click(document.querySelector('#inbox-assignment-members'));
    click(button('Select all'));
    expect(document.querySelector('#inbox-assignment-members')?.textContent).toContain('6 members selected');
    click(button('Clear selection'));
    expect(document.querySelector('#inbox-assignment-members')?.textContent).toContain('Select members');
  });

  it('does not offer select all for five members', () => {
    renderPicker(5);
    click(document.querySelector('#inbox-assignment-members'));
    expect(button('Select all')).toBeUndefined();
    click(document.querySelector('[cmdk-item]'));
    expect(document.querySelector('#inbox-assignment-members')?.textContent).toContain('Member 0');
  });

  it('allows only one specific member and closes after selection', () => {
    renderPicker(6, 'specific_member');
    click(document.querySelector('#inbox-assignment-members'));
    expect(button('Select all')).toBeUndefined();
    click(document.querySelector('[cmdk-item]'));
    expect(document.querySelector('#inbox-assignment-members')?.textContent).toContain('Member 0');
    expect(document.querySelector('#inbox-assignment-members')?.getAttribute('aria-expanded')).toBe('false');
  });

  it('hides member selection for manual assignment', () => {
    renderPicker(6, 'manual');
    expect(document.querySelector('#inbox-assignment-members')).toBeNull();
  });
});
