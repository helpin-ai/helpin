// Same detection mechanism used by src/lib/haptics.ts — Tauri v2 injects this
// global into the webview, so its presence is a reliable runtime signal that
// we're running inside the native app shell rather than a plain browser tab.
export function isTauri(): boolean {
  return typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window
}
