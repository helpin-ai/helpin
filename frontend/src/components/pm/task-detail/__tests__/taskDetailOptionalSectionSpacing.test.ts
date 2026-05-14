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
    const relationshipsIndex = panelSource.indexOf('<TaskRelationshipsSection');
    const externalLinksIndex = panelSource.indexOf('{/* External Links */}');
    const checklistBlock = panelSource.slice(checklistIndex, externalLinksIndex);
    const externalLinksBlock = panelSource.slice(externalLinksIndex, panelSource.indexOf('{/* Attachments */}', externalLinksIndex));

    expect(panelSource).toContain('const hasOptionalTaskSections = relationshipsToggleActive || showChecklist || showExternalLinks;');
    expect(panelSource).toContain('{hasOptionalTaskSections && (');
    expect(panelSource).toContain('className="mt-8 space-y-8"');
    expect(panelSource).toContain('associationsService.listByTask(workspaceId, taskDetail.task.id)');
    expect(panelSource).toContain('if (hasVisibleTaskAssociations(associationsRes.data)) setShowRelationships(true);');
    expect(relationshipsSource).toContain('className={className}');
    expect(checklistIndex).toBeGreaterThan(-1);
    expect(relationshipsIndex).toBeGreaterThan(checklistIndex);
    expect(checklistBlock).not.toContain('className="mt-8"');
    expect(checklistBlock).not.toContain('className="mt-6"');
    expect(externalLinksBlock).not.toContain('className="mt-8"');
    expect(externalLinksBlock).not.toContain('className="mt-6"');
  });

  it('keeps comments separated after removing the standalone comments divider', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const commentsIndex = panelSource.indexOf('{/* Comments + Activity */}');
    const commentsBlock = panelSource.slice(commentsIndex, panelSource.indexOf('{/* Comments card */}', commentsIndex));

    expect(commentsIndex).toBeGreaterThan(-1);
    expect(panelSource).not.toContain('<Separator className="my-6 bg-border/60" />');
    expect(commentsBlock).toContain('className="mt-6"');
  });
});
