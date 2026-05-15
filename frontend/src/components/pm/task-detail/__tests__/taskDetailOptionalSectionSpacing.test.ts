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
    expect(panelSource).toContain('const hasRelationshipContent = hasVisibleTaskAssociations(associationsRes.data);');
    expect(panelSource).toContain('if (hasRelationshipContent) setShowRelationships(true);');
    expect(relationshipsSource).toContain('className={className}');
    expect(checklistIndex).toBeGreaterThan(-1);
    expect(relationshipsIndex).toBeGreaterThan(checklistIndex);
    expect(checklistBlock).not.toContain('className="mt-8"');
    expect(checklistBlock).not.toContain('className="mt-6"');
    expect(externalLinksBlock).not.toContain('className="mt-8"');
    expect(externalLinksBlock).not.toContain('className="mt-6"');
  });

  it('uses shared subtle action pills and locks populated optional sections', () => {
    const panelSource = readFileSync(resolve(__dirname, '../../TaskDetailPanel.tsx'), 'utf8');
    const createTaskSource = readFileSync(resolve(__dirname, '../../CreateTaskModal.tsx'), 'utf8');
    const globalCreateSource = readFileSync(resolve(__dirname, '../../GlobalCreateModals.tsx'), 'utf8');
    const pillSource = readFileSync(resolve(__dirname, '../../optionalSectionActionPill.ts'), 'utf8');

    expect(pillSource).toContain("export type OptionalSectionActionState = 'available' | 'open' | 'locked';");
    expect(pillSource).toContain('border-primary/20 bg-primary/[0.025] text-primary/75');
    expect(pillSource).toContain('cursor-default border-border/60 bg-muted/35 text-muted-foreground/80');
    expect(panelSource).toContain('disabled={hasChecklistItems}');
    expect(panelSource).toContain('disabled={hasRelationshipItems}');
    expect(panelSource).toContain('disabled={hasExternalLinkItems}');
    expect(createTaskSource).toContain('getOptionalSectionActionClass(form.checklist_items.length > 0 ?');
    expect(createTaskSource).toContain('disabled={form.external_links.length > 0}');
    expect(globalCreateSource).toContain('getOptionalSectionActionClass(epicExternalLinks.length > 0 ?');
    expect(globalCreateSource).toContain('disabled={pendingFiles.length > 0}');
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
