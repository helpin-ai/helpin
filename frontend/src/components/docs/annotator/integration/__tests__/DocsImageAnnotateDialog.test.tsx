// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { AnnotationState } from '../../core/annotationTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const testState = vi.hoisted(() => ({
  renderAnnotationsToFile: vi.fn(),
  uploadEditorImage: vi.fn(),
  removeAttachment: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock('sonner', () => ({ toast: { error: testState.toastError } }));

vi.mock('@/hooks/useEditorImageUpload', () => ({
  uploadEditorImage: testState.uploadEditorImage,
}));

vi.mock('@/lib/services/pmAttachmentService', () => ({
  pmAttachmentService: { remove: testState.removeAttachment },
}));

vi.mock('@/components/docs/annotator/core/renderAnnotations', () => ({
  renderAnnotationsToFile: testState.renderAnnotationsToFile,
}));

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
}));

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, variant, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: string }) => {
    void variant;
    return <button {...props}>{children}</button>;
  },
}));

vi.mock('@/components/ui/checkbox', () => ({
  Checkbox: ({
    checked,
    onCheckedChange,
    ...props
  }: React.InputHTMLAttributes<HTMLInputElement> & { onCheckedChange?: (checked: boolean) => void }) => (
    <input
      type="checkbox"
      checked={Boolean(checked)}
      onChange={(event) => onCheckedChange?.(event.target.checked)}
      {...props}
    />
  ),
}));

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('@/components/docs/annotator/core/ImageAnnotator', async () => {
  const ReactModule = await import('react');
  return {
    ImageAnnotator: ReactModule.forwardRef(function MockImageAnnotator(
      {
        shapes,
        onImageLoaded,
        onSelectedIdChange,
      }: {
        shapes: AnnotationState['shapes'];
        onImageLoaded?: (size: { width: number; height: number }) => void;
        onSelectedIdChange: (id: string | null) => void;
      },
      ref: React.ForwardedRef<{ getShapesForSave: () => AnnotationState['shapes'] }>,
    ) {
      ReactModule.useImperativeHandle(ref, () => ({ getShapesForSave: () => shapes }), [shapes]);
      ReactModule.useEffect(() => onImageLoaded?.({ width: 800, height: 600 }), [onImageLoaded]);
      return (
        <button type="button" data-testid="select-first" onClick={() => onSelectedIdChange(shapes[0]?.id ?? null)}>
          Select first shape
        </button>
      );
    }),
  };
});

const { DocsImageAnnotateDialog } = await import('../DocsImageAnnotateDialog');

const emptyState: AnnotationState = {
  version: 1,
  baseWidth: 800,
  baseHeight: 600,
  shapes: [],
};

const coverState: AnnotationState = {
  ...emptyState,
  shapes: [{ id: 'cover-1', type: 'cover', x: 10, y: 20, width: 100, height: 80, color: '#111827' }],
};

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  vi.clearAllMocks();
  testState.renderAnnotationsToFile.mockResolvedValue(new File(['flattened'], 'annotated.png', { type: 'image/png' }));
  testState.uploadEditorImage.mockResolvedValue({ attachmentId: 'flattened-attachment', publicUrl: '/flattened.png' });
  testState.removeAttachment.mockResolvedValue({ error: null });
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

function dialogProps(overrides: Partial<React.ComponentProps<typeof DocsImageAnnotateDialog>> = {}) {
  return {
    open: true,
    onOpenChange: vi.fn(),
    workspaceId: 'workspace-1',
    uploadConfig: { workspaceId: 'workspace-1', entityType: 'editor_upload' as const, entityId: 'document-1' },
    sourceUrl: '/original.png',
    sourceAttachmentId: null,
    initialState: emptyState,
    onSave: vi.fn(),
    ...overrides,
  };
}

async function renderDialog(props = dialogProps()) {
  await act(async () => root.render(<DocsImageAnnotateDialog {...props} />));
  return props;
}

async function click(element: Element | null) {
  if (!element) throw new Error('Expected element to exist');
  await act(async () => element.dispatchEvent(new MouseEvent('click', { bubbles: true })));
}

function button(label: string): HTMLButtonElement | null {
  return container.querySelector(`button[aria-label="${label}"]`);
}

describe('DocsImageAnnotateDialog permanent flattening', () => {
  it('disables redaction and permanent flattening without a deletable original', async () => {
    await renderDialog();

    expect(button('Redact')?.disabled).toBe(true);
    expect(container.querySelector<HTMLInputElement>('#annotation-flatten')?.disabled).toBe(true);
    expect(container.textContent).toContain('Permanent flattening is unavailable because the original source cannot be deleted');
  });

  it('blocks permanent operations when an alternate original cannot be deleted atomically', async () => {
    await renderDialog(dialogProps({
      sourceAttachmentId: 'original-attachment',
      hasAlternateSource: true,
    }));

    expect(button('Redact')?.disabled).toBe(true);
    expect(container.querySelector<HTMLInputElement>('#annotation-flatten')?.disabled).toBe(true);
  });

  it('fails closed before rendering or uploading a legacy cover without a deletable original', async () => {
    const props = await renderDialog(dialogProps({ initialState: coverState }));

    await click([...container.querySelectorAll('button')].find((item) => item.textContent?.includes('Save annotated image')) ?? null);

    expect(testState.toastError).toHaveBeenCalledWith(expect.stringContaining('original image cannot be securely deleted'));
    expect(testState.renderAnnotationsToFile).not.toHaveBeenCalled();
    expect(testState.uploadEditorImage).not.toHaveBeenCalled();
    expect(testState.removeAttachment).not.toHaveBeenCalled();
    expect(props.onSave).not.toHaveBeenCalled();
  });

  it('allows an editable save after the last unsupported cover is removed', async () => {
    const props = await renderDialog(dialogProps({ initialState: coverState }));

    await click(container.querySelector('[data-testid="select-first"]'));
    await click(button('Delete selected'));
    await click([...container.querySelectorAll('button')].find((item) => item.textContent?.includes('Save annotated image')) ?? null);

    expect(testState.removeAttachment).not.toHaveBeenCalled();
    expect(props.onSave).toHaveBeenCalledWith(expect.objectContaining({
      permanent: false,
      annotationState: expect.objectContaining({ shapes: [] }),
      sourceAttachmentId: null,
    }));
  });

  it('deletes an attachment-backed original before applying a permanent redaction', async () => {
    const props = await renderDialog(dialogProps({
      initialState: coverState,
      sourceAttachmentId: 'original-attachment',
    }));

    await click([...container.querySelectorAll('button')].find((item) => item.textContent?.includes('Save annotated image')) ?? null);

    expect(testState.removeAttachment).toHaveBeenCalledWith('workspace-1', 'original-attachment');
    expect(props.onSave).toHaveBeenCalledWith({
      attachmentId: 'flattened-attachment',
      url: '/flattened.png',
      permanent: true,
      annotationState: null,
      sourceAttachmentId: null,
    });
  });

  it('never applies the flattened image when deleting the original fails', async () => {
    testState.removeAttachment.mockResolvedValue({ error: 'delete denied' });
    const props = await renderDialog(dialogProps({
      initialState: coverState,
      sourceAttachmentId: 'original-attachment',
    }));

    await click([...container.querySelectorAll('button')].find((item) => item.textContent?.includes('Save annotated image')) ?? null);

    expect(testState.uploadEditorImage).toHaveBeenCalledOnce();
    expect(testState.toastError).toHaveBeenCalledWith(expect.stringContaining('redaction was not applied'));
    expect(props.onSave).not.toHaveBeenCalled();
  });
});
