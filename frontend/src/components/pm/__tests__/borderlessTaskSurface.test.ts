import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('borderless task surfaces', () => {
  it('uses the flush dialog and divider editor without replacing the task sidebar', () => {
    const source = readFileSync(resolve(__dirname, '../CreateTaskModal.tsx'), 'utf8');
    const layoutSource = readFileSync(resolve(__dirname, '../CreateEntityModalLayout.tsx'), 'utf8');

    expect(source).toContain('<CreateEntityDialogContent>');
    expect(source).toContain('<CreateEntityTitleInput');
    expect(source).toContain('variant="divider"');
    expect(source).toContain("'borderless'");
    expect(source).toContain('Right sidebar — metadata');
    expect(source).toContain('className="[&>span]:text-ui"');

    expect(layoutSource).toContain('variant="flush"');
    expect(layoutSource).toContain('variant="plain"');
    expect(layoutSource).toContain('focus-within:border-foreground/70');
    expect(layoutSource).toContain('grid-cols-[16px_80px_1fr]');
    expect(layoutSource).toContain('self-center text-ui text-muted-foreground');
    expect(layoutSource).toContain('text-ui [&_button]:text-ui');
  });

  it('shares the task modal shell and PM control sizes with epic, sprint, and objective creation', () => {
    const source = readFileSync(resolve(__dirname, '../GlobalCreateModals.tsx'), 'utf8');

    ['epic', 'sprint', 'objective'].forEach((entity) => {
      expect(source).toContain(`title="Create ${entity}"`);
      expect(source).toContain(`aria-label="${entity[0].toUpperCase()}${entity.slice(1)} title"`);
    });

    expect(source.match(/<CreateEntityDialogContent>/g)).toHaveLength(3);
    expect(source.match(/<CreateEntityModalBody>/g)).toHaveLength(3);
    expect(source.match(/<CreateEntityModalSidebar/g)).toHaveLength(3);
    expect(source.match(/<CreateEntityModalFooter>/g)).toHaveLength(3);
    expect(source.match(/variant="divider"/g)).toHaveLength(3);
    expect(source).toContain('<Select size="ui"');
    expect(source).toContain('<SelectTrigger variant="ghost"');
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

  it('keeps shared PM picker triggers and dropdown options on the PM text-ui scale', () => {
    const pickerSources = [
      '../SidebarPopoverSelect.tsx',
      '../MemberPickerPopover.tsx',
      '../LabelPicker.tsx',
      '../EstimatePicker.tsx',
    ].map((path) => readFileSync(resolve(__dirname, path), 'utf8'));

    pickerSources.forEach((source) => {
      expect(source).toContain('text-ui');
    });
    expect(pickerSources.join('\n')).not.toContain('text-[12px]');
  });

  it('keeps task list metadata on the PM text-ui scale and names on the shared table-name scale', () => {
    const taskListSource = readFileSync(resolve(__dirname, '../TaskListView.tsx'), 'utf8');
    const displayMenuSource = readFileSync(resolve(__dirname, '../ListDisplayMenu.tsx'), 'utf8');
    const tableStylesSource = readFileSync(resolve(__dirname, '../../../lib/tableStyles.ts'), 'utf8');

    expect(taskListSource).toContain('font-mono text-ui text-muted-foreground');
    expect(taskListSource).toContain('text-left ${TABLE_NAME_TEXT} hover:text-primary');
    expect(tableStylesSource).toContain("TABLE_NAME_TEXT = 'text-sm text-foreground/90'");
    expect(taskListSource).toContain('<Select size="ui"');
    expect(taskListSource).toContain('className="h-8 text-ui"');
    expect(taskListSource).toContain('triggerClassName="text-[11px]"');
    expect(taskListSource).toContain('singleLine');
    expect(taskListSource).not.toContain('py-0.5 text-xs transition-colors hover:bg-accent');
    expect(displayMenuSource).toContain('size="icon-sm"');
    expect(displayMenuSource).toContain('px-2 py-1 text-ui font-medium');
  });
});
