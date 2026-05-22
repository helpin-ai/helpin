import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel open-in-new-tab action', () => {
  it('uses the standard external-open icon instead of the maximize icon', () => {
    const source = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const tooltipIndex = source.indexOf('<QuickTooltip label="Open in new tab">');
    const buttonBlock = source.slice(tooltipIndex, source.indexOf('</QuickTooltip>', tooltipIndex));

    expect(tooltipIndex).toBeGreaterThan(-1);
    expect(buttonBlock).toContain('<ArrowUpRight01Icon className="h-3.5 w-3.5" />');
    expect(buttonBlock).not.toContain('Maximize01Icon');
  });
});
