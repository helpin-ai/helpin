import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(__dirname, '../..');

const readSource = (path: string) => readFileSync(resolve(srcRoot, path), 'utf8');

describe('maximize and expand icon consistency', () => {
  it('exposes the shared Hugeicons expand and collapse pair', () => {
    const icons = readSource('lib/icons.tsx');

    expect(icons).toContain('ExpandIcon as _ExpandIcon');
    expect(icons).toContain('CollapseIcon as _CollapseIcon');
    expect(icons).toContain('export const ExpandIcon = hi(_ExpandIcon);');
    expect(icons).toContain('export const CollapseIcon = hi(_CollapseIcon);');
  });

  it.each([
    'components/agents/AskAgentsDock.tsx',
    'components/crm/DealBoard.tsx',
    'components/pm/KanbanBoard.tsx',
    'components/ui/resizable-image-component.tsx',
    'components/pm/CodingSession/CodingInteractionCard.tsx',
    'components/pm/CodingSession/PublishedToolPreviewCard.tsx',
    'components/pm/CodingSession/CodingPreviewPanels.tsx',
    'pages/automation/Agents.tsx',
  ])('uses the shared expand/collapse vocabulary in %s', (path) => {
    const source = readSource(path);

    expect(source).toContain('ExpandIcon');
    expect(source).not.toMatch(/\b(?:Maximize01|Minimize01|ArrowExpand|ArrowShrink)Icon\b/);
  });

  it('keeps the Ask dock overflow action before expand and expand next to close', () => {
    const source = readSource('components/agents/AskAgentsDock.tsx');
    const headerStart = source.indexOf('<header data-dock-header');
    const header = source.slice(headerStart, source.indexOf('</header>', headerStart));
    const actionRail = header.match(/<div data-dock-actions className="([^"]+)">/)?.[1];
    const overflowLabel = "aria-label={tab === 'agents' ? 'Agent run actions' : 'Conversation actions'}";
    const overflowIndex = header.indexOf(overflowLabel);
    const expandLabel = "aria-label={maximized ? 'Restore agent dock' : 'Maximize agent dock'}";
    const expandIndex = header.indexOf(expandLabel);
    const closeIndex = header.indexOf('aria-label={closeLabel}');

    expect(headerStart).toBeGreaterThan(-1);
    expect(actionRail).toBe('flex items-center gap-0.5');
    expect(overflowIndex).toBeGreaterThan(-1);
    expect(expandIndex).toBeGreaterThan(overflowIndex);
    expect(closeIndex).toBeGreaterThan(expandIndex);
    expect(header.slice(expandIndex + expandLabel.length, closeIndex)).not.toContain('aria-label=');
  });
});
