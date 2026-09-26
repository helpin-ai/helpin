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

/**
 * Router location rewrite that mounts the app under `basepath`.
 *
 * Used instead of the router `basepath` option because TanStack Start resets
 * that option on every SSR request to its build-time value, which would render
 * unprefixed links for reverse-proxied help centers.
 */
export function createBasepathRewrite(basepath: string) {
  return {
    input: ({ url }: { url: URL }) => {
      url.pathname = stripBasepath(url.pathname, basepath)
      return url
    },
    output: ({ url }: { url: URL }) => {
      url.pathname = url.pathname === '/' ? basepath : `${basepath}${url.pathname}`
      return url
    },
  }
}
