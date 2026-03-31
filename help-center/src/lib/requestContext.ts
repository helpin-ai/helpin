import { createServerFn } from '@tanstack/react-start'
import { getRequestHost, getRequestProtocol } from '@tanstack/react-start/server'
import { resolveSubdomain } from '@/lib/utils'

interface HelpCenterRequestContext {
  host: string
  protocol: string
  subdomain: string
}

const getServerRequestContext = createServerFn({ method: 'GET' }).handler(
  (): HelpCenterRequestContext => {
    const host = getRequestHost()
    const protocol = getRequestProtocol()

    return {
      host,
      protocol,
      subdomain: resolveSubdomain(host),
    }
  },
)

export async function getHelpCenterRequestContext(): Promise<HelpCenterRequestContext> {
  if (typeof window !== 'undefined') {
    return {
      host: window.location.host,
      protocol: window.location.protocol.replace(/:$/, ''),
      subdomain: resolveSubdomain(),
    }
  }

  return getServerRequestContext()
}
