import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const flowsSource = readFileSync(resolve(__dirname, '../AutomationFlows.tsx'), 'utf8');
const skillsSource = readFileSync(resolve(__dirname, '../SkillCatalog.tsx'), 'utf8');
const toolsSource = readFileSync(resolve(__dirname, '../ToolCatalog.tsx'), 'utf8');
const triggersSource = readFileSync(resolve(__dirname, '../../../components/automation/AutomationOverviewPanel.tsx'), 'utf8');
const quietSource = readFileSync(resolve(__dirname, '../../../components/design-system/quiet.tsx'), 'utf8');

describe('automation search field design', () => {
  it('centralizes the Skill Catalog treatment instead of using underline controls', () => {
    for (const source of [flowsSource, skillsSource, toolsSource, triggersSource]) {
      expect(source).not.toContain('<QuietUnderlineInput');
      expect(source).toContain('<QuietSearchInput');
    }

    expect(quietSource).toContain('export function QuietSearchInput');
    expect(quietSource).toContain('h-9 w-full border-border/70 bg-muted/30 pl-9 text-sm');
  });
});
