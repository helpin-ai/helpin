import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));
const detailEditorClassName = 'min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0';
const taskDetailBottomSpacer = '<div className="h-20 shrink-0 lg:h-40" aria-hidden="true" />';
const epicDetailBottomSpacer = '<div className="h-40 shrink-0" aria-hidden="true" />';

describe('description editor style parity', () => {
  it('uses the create-modal divider editor when editing task and epic descriptions', () => {
    const createTask = readFileSync(resolve(__dirname, '../CreateTaskModal.tsx'), 'utf8');
    const createEntities = readFileSync(resolve(__dirname, '../GlobalCreateModals.tsx'), 'utf8');
    const taskDetail = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');
    const epicDetail = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');
    const detailActions = readFileSync(resolve(__dirname, '../DetailDescriptionEditorActions.tsx'), 'utf8');
    const quietDesignSystem = readFileSync(resolve(__dirname, '../../design-system/quiet.tsx'), 'utf8');

    for (const source of [createTask, createEntities, taskDetail, epicDetail]) {
      expect(source).toContain('variant="divider"');
      expect(source).toContain('contentVariant="pm"');
    }

    for (const detailSource of [taskDetail, epicDetail]) {
      expect(detailSource).toContain('variant="pm"');
      expect(detailSource).toContain(detailEditorClassName);
      expect(detailSource).toContain('<DetailDescriptionEditorActions');
      expect(detailSource).toContain('className="group/description-editor"');
      expect(detailSource).toContain('onCancel={cancelDescriptionEditing}');
      expect(detailSource).toContain('descriptionEditStartRef.current');
      expect(detailSource).toContain('<QuietTitleInput');
      expect(detailSource).not.toContain('[&_.tiptap]:px-6');
      expect(detailSource).not.toContain('[&_.tiptap]:py-4');
      expect(detailSource).not.toContain('className="border-transparent shadow-none [&_.ProseMirror]:text-sm"');
    }

    expect(quietDesignSystem).toContain("'pb-2 text-[26px] leading-[1.15] tracking-[-0.02em] md:text-[26px]'");
    expect(quietDesignSystem).toContain("presentation?: 'entity' | 'header'");

    expect(taskDetail).toContain(taskDetailBottomSpacer);
    expect(epicDetail).toContain(epicDetailBottomSpacer);

    expect(taskDetail).toContain('px-4 pt-5 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:px-10">');
    expect(epicDetail).toContain('px-4 pt-5 sm:px-6 lg:overflow-y-auto lg:px-10">');

    expect(detailActions).toContain('sticky bottom-0');
    expect(detailActions).toContain('group-focus-within/description-editor:border-foreground/70');
    expect(detailActions).toContain('Cancel');
    expect(detailActions).toContain('Done');
    expect(detailActions).toContain('variant="default"');
    expect(detailActions).not.toContain('overflow-y-auto');
  });
});
