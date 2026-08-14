import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));
const detailEditorClassName = 'min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0';
const detailTitleClassName = 'border-b border-border/60 bg-transparent pb-2 text-2xl font-bold text-foreground transition-colors placeholder:text-muted-foreground/50 focus:border-foreground/70 focus:outline-none';

describe('description editor style parity', () => {
  it('uses the create-modal divider editor when editing task and epic descriptions', () => {
    const createTask = readFileSync(resolve(__dirname, '../CreateTaskModal.tsx'), 'utf8');
    const createEntities = readFileSync(resolve(__dirname, '../GlobalCreateModals.tsx'), 'utf8');
    const taskDetail = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');
    const epicDetail = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');

    for (const source of [createTask, createEntities, taskDetail, epicDetail]) {
      expect(source).toContain('variant="divider"');
    }

    for (const detailSource of [taskDetail, epicDetail]) {
      expect(detailSource).toContain(detailEditorClassName);
      expect(detailSource).toContain(detailTitleClassName);
      expect(detailSource).not.toContain('[&_.tiptap]:px-6');
      expect(detailSource).not.toContain('[&_.tiptap]:py-4');
      expect(detailSource).not.toContain('className="border-transparent shadow-none [&_.ProseMirror]:text-sm"');
    }
  });
});
