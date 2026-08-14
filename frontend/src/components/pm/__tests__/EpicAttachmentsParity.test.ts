import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('epic attachment parity', () => {
  const epicSource = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');
  const taskSource = readFileSync(resolve(__dirname, '../TaskDetailPanel.tsx'), 'utf8');
  const attachmentsSource = readFileSync(resolve(__dirname, '../Attachments.tsx'), 'utf8');

  it('keeps both pages on the shared attachment component with the same compact empty presentation', () => {
    const sharedWrapper = 'className="mt-6 border-t border-border/60 pt-6" id="attachments-section"';

    expect(taskSource).toContain('id="attachments-section"');
    expect(taskSource).toContain('emptyPresentation="inline-action"');
    expect(taskSource).not.toContain(sharedWrapper);
    expect(epicSource).toContain('id="attachments-section"');
    expect(epicSource).toContain('emptyPresentation="inline-action"');
    expect(epicSource).not.toContain(sharedWrapper);
    expect(epicSource).toContain('entityType="epic"');
    expect(epicSource).toContain('editable={canEdit}');
    expect(epicSource).toContain('showAddAction');
    expect(epicSource).not.toContain('showEmptyState');
    expect(epicSource).not.toContain('Attach Files');
    expect(epicSource).not.toContain('onFilePickerReady');
    expect(epicSource).not.toContain('getOptionalSectionActionClass');
  });

  it('uses one shared standard-size ghost action in both attachment states', () => {
    expect(attachmentsSource).toContain('function AttachFilesButton');
    expect(attachmentsSource).toContain('variant="ghost" size="default"');
    expect(attachmentsSource.match(/<AttachFilesButton /g)).toHaveLength(2);
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
