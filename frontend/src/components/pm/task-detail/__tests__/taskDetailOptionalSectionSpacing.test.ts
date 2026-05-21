import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('TaskDetailPanel optional section spacing', () => {
  it('uses one conditional stack gap for relationships, checklist, and external links', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const relationshipsSource = readFileSync(resolve(__dirname, '../../TaskRelationshipsSection.tsx'), 'utf8');
    const checklistIndex = panelSource.indexOf('{/* Checklist */}');
    const externalLinksIndex = panelSource.indexOf('{/* External Links */}');
    const checklistBlock = panelSource.slice(checklistIndex, externalLinksIndex);
    const externalLinksBlock = panelSource.slice(externalLinksIndex, panelSource.indexOf('{/* Attachments */}', externalLinksIndex));

    expect(panelSource).toContain('const hasOptionalTaskSections = relationshipsToggleActive || showChecklist || showExternalLinks;');
    expect(panelSource).toContain('{hasOptionalTaskSections && (');
    expect(panelSource).toContain('className="mt-8 space-y-8"');
    expect(relationshipsSource).toContain('className={className}');
    expect(checklistBlock).not.toContain('className="mt-8"');
    expect(checklistBlock).not.toContain('className="mt-6"');
    expect(externalLinksBlock).not.toContain('className="mt-8"');
    expect(externalLinksBlock).not.toContain('className="mt-6"');
  });
});
