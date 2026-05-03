export type ExcalidrawScene = {
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
};

function jsonSafeRecord(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  try {
    return JSON.parse(JSON.stringify(value)) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function jsonSafeArray(value: unknown): unknown[] {
  if (!Array.isArray(value)) return [];
  try {
    return JSON.parse(JSON.stringify(value)) as unknown[];
  } catch {
    return [];
  }
}

export function normalizeExcalidrawScene(value: unknown): ExcalidrawScene {
  const raw = value && typeof value === 'object' ? value as Record<string, unknown> : {};
  const appState = jsonSafeRecord(raw.appState);
  delete appState.collaborators;

  return {
    elements: jsonSafeArray(raw.elements).filter((element) => {
      if (!element || typeof element !== 'object') return false;
      return (element as { type?: unknown }).type !== 'image';
    }),
    appState,
    files: {},
  };
}

export function isExcalidrawSceneEmpty(scene: unknown): boolean {
  return normalizeExcalidrawScene(scene).elements.length === 0;
}

export type ExcalidrawExportTheme = 'light' | 'dark';

export async function exportExcalidrawPngBlob(scene: unknown, theme: ExcalidrawExportTheme = 'light'): Promise<Blob> {
  const normalized = normalizeExcalidrawScene(scene);
  if (normalized.elements.length === 0) {
    throw new Error('Excalidraw drawing is empty');
  }

  const mod = await import('@excalidraw/excalidraw');
  const exportToBlob = (mod as { exportToBlob?: (opts: Record<string, unknown>) => Promise<Blob> }).exportToBlob;
  if (!exportToBlob) {
    throw new Error('Excalidraw export utilities are unavailable');
  }

  return exportToBlob({
    elements: normalized.elements,
    appState: {
      ...normalized.appState,
      exportBackground: false,
      exportWithDarkMode: theme === 'dark',
      theme,
      viewBackgroundColor: 'transparent',
    },
    files: normalized.files,
    mimeType: 'image/png',
  });
}

export async function excalidrawPngFile(
  scene: unknown,
  filename = 'excalidraw-drawing.png',
  theme: ExcalidrawExportTheme = 'light',
): Promise<File> {
  const blob = await exportExcalidrawPngBlob(scene, theme);
  return new File([blob], filename, { type: 'image/png' });
}
