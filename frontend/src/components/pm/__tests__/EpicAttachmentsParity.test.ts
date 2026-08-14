import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('epic attachment parity', () => {
  const epicSource = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');
  const taskSource = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');

  it('uses the same shared attachment section configuration as the task sheet', () => {
    const sharedWrapper = 'className="mt-6 border-t border-border/60 pt-6" id="attachments-section"';

    expect(taskSource).toContain(sharedWrapper);
    expect(epicSource).toContain(sharedWrapper);
    expect(epicSource).toContain('entityType="epic"');
    expect(epicSource).toContain('editable={canEdit}');
    expect(epicSource).toContain('showAddAction');
    expect(epicSource).toContain('showEmptyState');
    expect(epicSource).not.toContain('Attach Files');
    expect(epicSource).not.toContain('onFilePickerReady');
    expect(epicSource).not.toContain('getOptionalSectionActionClass');
  });

  it('matches the task description drop-to-insert flow', () => {
    for (const behavior of [
      'descriptionDragging',
      'descriptionUploadRef',
      'queuedDescriptionDropRef',
      'descriptionDragCounterRef',
      'handleDescriptionUploadReady',
      'handleDescriptionDragEnter',
      'handleDescriptionDragOver',
      'handleDescriptionDragLeave',
      'handleDescriptionDrop',
      'Drop to insert here',
      'onUploadReady={handleDescriptionUploadReady}',
    ]) {
      expect(taskSource).toContain(behavior);
      expect(epicSource).toContain(behavior);
    }
  });

  it('removes the former epic-wide attachment drop overlay', () => {
    expect(epicSource).not.toContain('panelDragging');
    expect(epicSource).not.toContain('uploadFilesRef');
    expect(epicSource).not.toContain('openFilePickerRef');
    expect(epicSource).not.toContain('dragCounterRef');
    expect(epicSource).not.toContain('Drop files to attach');
    expect(epicSource).not.toContain('Upload01Icon');
  });
});
