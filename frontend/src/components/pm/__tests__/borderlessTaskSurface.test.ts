import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('borderless task surfaces', () => {
  it('uses the flush dialog and divider editor without replacing the task sidebar', () => {
    const source = readFileSync(resolve(__dirname, '../CreateTaskModal.tsx'), 'utf8');

    expect(source).toContain('variant="flush"');
    expect(source).toContain('variant="plain"');
    expect(source).toContain('variant="divider"');
    expect(source).toContain('focus-within:border-foreground/70');
    expect(source).toContain("'borderless'");
    expect(source).toContain('Right sidebar — metadata');
    expect(source).toContain('grid-cols-[16px_80px_1fr]');
  });

  it('uses the update composer and line tabs for task activity', () => {
    const updatesSource = readFileSync(resolve(__dirname, '../task-detail/TaskUpdatesView.tsx'), 'utf8');
    const editorSource = readFileSync(resolve(__dirname, '../CommentEditor.tsx'), 'utf8');
    const threadSource = readFileSync(resolve(__dirname, '../CommentThread.tsx'), 'utf8');

    expect(updatesSource).toContain('composerVariant="update"');
    expect(updatesSource).toContain('<TabsList variant="line"');
    expect(editorSource).toContain("variant === 'update'");
    expect(editorSource).toContain('group/update-composer');
    expect(editorSource).toContain('border-t border-border/40');
    expect(editorSource).toContain('group-focus-within/update-composer:border-foreground/70');
    expect(editorSource).toContain('rounded-full bg-primary');
    expect(threadSource).toContain("variant: composerVariant === 'update' ? 'update' : 'reply'");
    expect(threadSource).toContain("variant={composerVariant === 'update' ? 'update' : undefined}");
  });
});
