export interface ResizableImageSourceSelection {
  displaySrc: string | null;
  displayDarkSrc: string | null;
  annotationSourceUrl: string | null;
  annotationSourceAttachmentId: string | null;
}

interface SelectResizableImageSourcesInput {
  src: unknown;
  darkSrc: unknown;
  resolvedSrc: string | null;
  isArtifactBacked: boolean;
  hasAnnotations: boolean;
  attachmentId: unknown;
  sourceAttachmentId: unknown;
}

function optionalString(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null;
}

/**
 * Separates the image rendered in the document from the original used by the annotator.
 *
 * Artifact-backed legacy nodes keep resolving their artifact as the original. Once annotated,
 * their normal `src` is the flattened render and must take precedence in the document view.
 */
export function selectResizableImageSources({
  src,
  darkSrc,
  resolvedSrc,
  isArtifactBacked,
  hasAnnotations,
  attachmentId,
  sourceAttachmentId,
}: SelectResizableImageSourcesInput): ResizableImageSourceSelection {
  const storedSrc = optionalString(src);
  const storedDarkSrc = optionalString(darkSrc);

  if (isArtifactBacked) {
    return {
      displaySrc: hasAnnotations ? storedSrc ?? resolvedSrc : resolvedSrc,
      // A legacy alternate source is also unannotated. Once flattened, use the same rendered
      // annotation in both themes instead of allowing dark mode to hide it.
      displayDarkSrc: hasAnnotations ? null : storedDarkSrc,
      annotationSourceUrl: resolvedSrc,
      // Artifact provenance is not a PM attachment. In particular, never promote a previous
      // flattened render to the original when the annotation is edited a second time.
      annotationSourceAttachmentId: null,
    };
  }

  const originalAttachmentId = optionalString(sourceAttachmentId) ?? optionalString(attachmentId);
  return {
    displaySrc: resolvedSrc ?? storedSrc,
    displayDarkSrc: storedDarkSrc,
    annotationSourceUrl: resolvedSrc,
    annotationSourceAttachmentId: originalAttachmentId,
  };
}
