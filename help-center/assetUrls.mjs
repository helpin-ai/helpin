export function prefixAssetUrls(content, basepath) {
  if (!basepath) return content
  return content
    .replaceAll(/([("'=])\/assets\//g, `$1${basepath}/assets/`)
    // Bare "assets/…" entries (Vite's preload dep map) get a leading "/" from
    // the preload helper, so keep them relative: "docs/assets/…", not "//docs/…".
    .replaceAll(/(["'])assets\//g, `$1${basepath.replace(/^\/+/, '')}/assets/`)
}
