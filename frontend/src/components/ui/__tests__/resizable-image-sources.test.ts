import { describe, expect, it } from 'vitest';

import { selectResizableImageSources } from '../resizable-image-sources';

describe('selectResizableImageSources', () => {
  it('displays the flattened render while editing against the original legacy artifact', () => {
    expect(selectResizableImageSources({
      src: '/attachments/rendered/content',
      darkSrc: '/artifacts/original-dark',
      resolvedSrc: '/artifacts/original',
      isArtifactBacked: true,
      hasAnnotations: true,
      attachmentId: 'rendered-attachment',
      sourceAttachmentId: 'older-rendered-attachment',
    })).toEqual({
      displaySrc: '/attachments/rendered/content',
      displayDarkSrc: null,
      annotationSourceUrl: '/artifacts/original',
      annotationSourceAttachmentId: null,
    });
  });

  it('uses the resolved artifact before a legacy image has annotations', () => {
    expect(selectResizableImageSources({
      src: 'helpin://artifacts/artifact-1',
      darkSrc: null,
      resolvedSrc: '/artifacts/original',
      isArtifactBacked: true,
      hasAnnotations: false,
      attachmentId: null,
      sourceAttachmentId: null,
    }).displaySrc).toBe('/artifacts/original');
  });

  it('preserves the attachment-backed image behavior', () => {
    expect(selectResizableImageSources({
      src: '/attachments/rendered/content',
      darkSrc: null,
      resolvedSrc: '/attachments/rendered/content',
      isArtifactBacked: false,
      hasAnnotations: true,
      attachmentId: 'rendered-attachment',
      sourceAttachmentId: 'original-attachment',
    })).toEqual({
      displaySrc: '/attachments/rendered/content',
      displayDarkSrc: null,
      annotationSourceUrl: '/attachments/rendered/content',
      annotationSourceAttachmentId: 'original-attachment',
    });
  });
});
