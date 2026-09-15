import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('agent dock vs. support widget layering', () => {
  it('flags the open floating dock on the body', () => {
    const dock = readFileSync(resolve(__dirname, '../AskAgentsDock.tsx'), 'utf8');
    expect(dock).toContain("document.body.dataset.helpinDockOpen = 'true'");
    expect(dock).toContain('delete document.body.dataset.helpinDockOpen');
    expect(dock).toContain('if (embedded || collapsed || hiddenByModal) return;');
  });

  it('drops the widget host under the dock root on phones', () => {
    const css = readFileSync(resolve(__dirname, '../../../index.css'), 'utf8');
    const rule = css.slice(css.indexOf('body[data-helpin-dock-open] #helpin-widget-container'));
    expect(rule).toContain('z-index: 55 !important');
    // Must stay below the dock root's own layer.
    const dockRootLayer = Number(/z-\[(\d+)\]/.exec(readFileSync(resolve(__dirname, '../AskAgentsDock.tsx'), 'utf8').split('agent-dock-root')[1])?.[1]);
    expect(dockRootLayer).toBeGreaterThan(55);
  });
});
