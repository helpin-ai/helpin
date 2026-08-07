export function prefixAssetUrls(content, basepath) {
  if (!basepath) return content
  return content.replaceAll(/([("'=])\/assets\//g, `$1${basepath}/assets/`)
}
