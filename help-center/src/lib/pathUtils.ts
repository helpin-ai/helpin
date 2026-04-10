export function prefixBasepath(basepath: string, path: string): string {
  if (!path) {
    return basepath || '/'
  }

  if (/^[a-z]+:/i.test(path) || path.startsWith('//')) {
    return path
  }

  const normalizedBasepath = basepath ? basepath.replace(/\/+$/, '') : ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`

  if (!normalizedBasepath) {
    return normalizedPath
  }

  if (normalizedPath === '/') {
    return `${normalizedBasepath}/`
  }

  if (normalizedPath === normalizedBasepath || normalizedPath.startsWith(`${normalizedBasepath}/`)) {
    return normalizedPath
  }

  return `${normalizedBasepath}${normalizedPath}`
}

export function stripBasepath(pathname: string, basepath: string): string {
  if (!basepath) return pathname
  if (pathname === basepath) return '/'
  if (pathname.startsWith(`${basepath}/`)) {
    return pathname.slice(basepath.length) || '/'
  }
  return pathname
}
