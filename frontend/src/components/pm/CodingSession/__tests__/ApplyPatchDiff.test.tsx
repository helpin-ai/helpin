// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { ApplyPatchDiff } from '../ApplyPatchDiff';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

function render(node: React.ReactNode) {
  act(() => {
    root.render(node);
  });
}

describe('ApplyPatchDiff', () => {
  it('renders the Modified badge, file path, and diff lines for a real Codex unified-diff payload', () => {
    // Regression guard for the hidden-diff bug: when the ActivityToolCallRow
    // accidentally short-circuited via PublishedToolPreviewCard, this test
    // would still pass in isolation but the transcript would render nothing.
    // Keeping this high-fidelity DOM assertion ensures the parser + renderer
    // pair keeps producing visible markup for the Codex output shape.
    const realPayload = [
      '--- a/server/internal/service/pm_checklist_item.go',
      '+++ b/server/internal/service/pm_checklist_item.go',
      '@@ -10,2 +10,3 @@',
      ' \t"github.com/helpin-ai/helpin/server/internal/repository"',
      '+\t"github.com/helpin-ai/helpin/server/internal/tiptap"',
      ' \t"github.com/helpin-ai/helpin/server/internal/websocket"',
    ].join('\n');

    render(<ApplyPatchDiff argsText={realPayload} />);

    expect(container.textContent).toContain('Modified');
    expect(container.textContent).toContain('server/internal/service/pm_checklist_item.go');
    expect(container.textContent).toContain('tiptap');
    expect(container.querySelectorAll('table')).toHaveLength(1);
    // + line + - line (none here) + 2 context lines = 3 rows total.
    expect(container.querySelectorAll('tbody tr').length).toBeGreaterThan(0);
    // At least one row is an "added" row and shows a + gutter.
    const gutterChars = Array.from(container.querySelectorAll('td:first-child')).map((c) => c.textContent?.trim());
    expect(gutterChars).toContain('+');
  });

  it('shows the Added badge and new file path when the diff creates a file', () => {
    const patch = [
      '--- /dev/null',
      '+++ b/src/brand-new.ts',
      '@@ -0,0 +1,1 @@',
      '+export const brandNew = true;',
    ].join('\n');

    render(<ApplyPatchDiff argsText={patch} />);

    expect(container.textContent).toContain('Added');
    expect(container.textContent).toContain('src/brand-new.ts');
    expect(container.textContent).toContain('brandNew');
  });

  it('shows the Deleted badge and a "File deleted" placeholder when the diff removes a file', () => {
    const patch = [
      '--- a/src/removed.ts',
      '+++ /dev/null',
      '@@ -1,1 +0,0 @@',
      '-gone',
    ].join('\n');

    render(<ApplyPatchDiff argsText={patch} />);

    expect(container.textContent).toContain('Deleted');
    expect(container.textContent).toContain('File deleted');
    // No diff table should render for a pure delete.
    expect(container.querySelectorAll('table')).toHaveLength(0);
  });

  it('renders rename arrow when the file was moved', () => {
    const patch = [
      '--- a/old-name.ts',
      '+++ b/new-name.ts',
      '@@ -1,1 +1,1 @@',
      ' unchanged',
    ].join('\n');

    render(<ApplyPatchDiff argsText={patch} />);

    expect(container.textContent).toContain('old-name.ts');
    expect(container.textContent).toContain('new-name.ts');
    expect(container.textContent).toMatch(/old-name\.ts\s*→\s*new-name\.ts/);
  });

  it('returns null when the patch is empty or unparseable', () => {
    render(<ApplyPatchDiff argsText="" />);
    expect(container.innerHTML).toBe('');

    render(<ApplyPatchDiff argsText="not a diff at all" />);
    expect(container.innerHTML).toBe('');
  });

  it('truncates very long diffs with a "more lines not shown" footer', () => {
    const manyLines = Array.from({ length: 80 }, (_, i) => `+line ${i}`).join('\n');
    const patch = [
      '--- a/big.ts',
      '+++ b/big.ts',
      '@@ -1,80 +1,80 @@',
      manyLines,
    ].join('\n');

    render(<ApplyPatchDiff argsText={patch} />);

    expect(container.textContent).toMatch(/\+\d+ more lines? not shown/);
  });
});
