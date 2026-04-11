import { createServerFn } from '@tanstack/react-start'
import {
  getRequestHost,
  getRequestProtocol,
  getRequestUrl,
} from '@tanstack/react-start/server'
import { resolveHelpCenterContext } from '@/lib/utils'

interface HelpCenterRequestContext {
  host: string
  protocol: string
  subdomain: string
  basepath: string
}

interface ServerRequestSnapshot {
  host?: string
  protocol?: string
  pathname?: string
  search?: string
  subdomain?: string
  basepath?: string
}

function readServerRequestSnapshot(): ServerRequestSnapshot | null {
  if (typeof window !== 'undefined') return null
  const getter = (
    globalThis as { __hcGetRequestContext__?: () => ServerRequestSnapshot | null }
  ).__hcGetRequestContext__
  if (typeof getter !== 'function') return null
  try {
    return getter() ?? null
  } catch {
    return null
  }
}

const getServerRequestContext = createServerFn({ method: 'GET' }).handler(
  (): HelpCenterRequestContext => {
    const snapshot = readServerRequestSnapshot()
    if (snapshot?.host && snapshot?.protocol) {
      const ctx =
        snapshot.subdomain != null && snapshot.basepath != null
          ? {
              subdomain: snapshot.subdomain,
              basepath: snapshot.basepath,
            }
          : resolveHelpCenterContext(
              snapshot.host,
              snapshot.pathname ?? '/',
              snapshot.search ?? '',
            )

      return {
        host: snapshot.host,
        protocol: snapshot.protocol,
        subdomain: ctx.subdomain,
        basepath: ctx.basepath,
      }
    }

    const host = getRequestHost({ xForwardedHost: true })
    const protocol = getRequestProtocol({ xForwardedProto: true })
    const url = getRequestUrl({
      xForwardedHost: true,
      xForwardedProto: true,
    })

    const ctx = resolveHelpCenterContext(host, url.pathname, url.search)

    return {
      host,
      protocol,
      subdomain: ctx.subdomain,
      basepath: ctx.basepath,
    }
  },
)

export async function getHelpCenterRequestContext(): Promise<HelpCenterRequestContext> {
  if (typeof window !== 'undefined') {
    const ctx = resolveHelpCenterContext(
      window.location.hostname,
      window.location.pathname,
      window.location.search,
    )
    return {
      host: window.location.host,
      protocol: window.location.protocol.replace(/:$/, ''),
      subdomain: ctx.subdomain,
      basepath: ctx.basepath,
    }
  }

  return getServerRequestContext()
}
