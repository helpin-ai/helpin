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

const getServerRequestContext = createServerFn({ method: 'GET' }).handler(
  (): HelpCenterRequestContext => {
    const host = getRequestHost({ xForwardedHost: true })
    const protocol = getRequestProtocol({ xForwardedProto: true })
    const url = getRequestUrl({
      xForwardedHost: true,
      xForwardedProto: true,
    })

    const ctx = resolveHelpCenterContext(host, url.pathname)

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
