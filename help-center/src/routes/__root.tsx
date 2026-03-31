import type { ReactNode } from 'react'
import { useEffect } from 'react'
import {
  HeadContent,
  Scripts,
  createRootRouteWithContext,
  useRouterState,
} from '@tanstack/react-router'
import { Eye } from 'lucide-react'
import { AppShell } from '@/components/layout/AppShell'
import { ErrorState } from '@/components/ErrorState'
import { DocsProvider } from '@/contexts/DocsContext'
import appCss from '@/app.css?url'
import { loadRootRouteData } from '@/lib/rootLoader'
import type { RootRouteData } from '@/lib/rootLoader'
import { buildRootHead } from '@/lib/seo'
import type { HelpCenterContext } from '@/lib/types'

function buildBrandColorStyle(hex: string | undefined | null): string {
  if (!hex) return ''
  const clean = hex.replace('#', '')
  if (clean.length !== 6) return ''

  const r = parseInt(clean.substring(0, 2), 16)
  const g = parseInt(clean.substring(2, 4), 16)
  const b = parseInt(clean.substring(4, 6), 16)

  const lighten = (c: number, amount: number) => Math.round(c + (255 - c) * amount)
  const lr = lighten(r, 0.35)
  const lg = lighten(g, 0.35)
  const lb = lighten(b, 0.35)
  const lightColor = `rgb(${lr}, ${lg}, ${lb})`

  return `:root{--primary:${hex};--ring:${hex};--sidebar-active:rgba(${r},${g},${b},0.08);--sidebar-active-foreground:${hex}}.dark{--primary:${lightColor};--ring:${lightColor};--sidebar-active:rgba(${lr},${lg},${lb},0.12);--sidebar-active-foreground:${lightColor}}`
}

export const Route = createRootRouteWithContext<HelpCenterContext>()({
  head: ({ loaderData }) => {
    const rootHead = buildRootHead(loaderData)
    const brandColor = loaderData?.config?.brand_color
    const brandStyle = buildBrandColorStyle(brandColor)

    return {
      links: [
        { rel: 'stylesheet', href: appCss },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossOrigin: 'anonymous' },
        {
          rel: 'preload',
          href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap',
          as: 'style',
        },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap',
        },
        ...(rootHead.links ?? []),
      ],
      meta: [
        { charSet: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1.0' },
        ...(rootHead.meta ?? []),
      ],
      scripts: brandStyle
        ? [{ tag: 'style', attrs: { id: 'brand-color-override' }, children: brandStyle }]
        : [],
    }
  },
  loader: ({ context, location }) =>
    loadRootRouteData(context.queryClient, location.pathname),
  component: RootLayout,
  errorComponent: RootErrorBoundary,
})

function RootLayout() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const isPreview = pathname.startsWith('/preview/')
  const {
    activeLocale,
    config,
    multilingualEnabled,
    spaces,
    subdomain,
  } = Route.useLoaderData() as RootRouteData

  // Set favicon from config
  useEffect(() => {
    if (!config?.favicon_url) return
    let link = document.querySelector<HTMLLinkElement>("link[rel~='icon']")
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.href = config.favicon_url
  }, [config?.favicon_url])

  // Allow preview routes even when help center is not published
  if (config && !config.is_published && !isPreview) {
    return (
      <ErrorState
        title="Help Center unavailable"
        message="This help center is not currently published."
        fullScreen
      />
    )
  }

  return (
    <RootDocument lang={activeLocale || config.default_locale || 'en'}>
      <DocsProvider
        subdomain={subdomain}
        locale={activeLocale}
        defaultLocale={config.default_locale}
        enabledLocales={config.enabled_locales ?? [config.default_locale]}
        multilingualEnabled={multilingualEnabled}
        config={config}
        spaces={spaces}
      >
        {isPreview && (
          <div className="sticky top-0 z-50 flex items-center justify-center gap-2 border-b bg-amber-50 dark:bg-amber-950/30 px-4 py-2 text-center">
            <Eye size={14} className="shrink-0 text-amber-600 dark:text-amber-400" />
            <span className="text-sm font-medium text-amber-700 dark:text-amber-300">
              Preview Mode
            </span>
            <span className="text-xs text-amber-600/70 dark:text-amber-400/60">
              - This is how your article will appear in the help center.
            </span>
          </div>
        )}
        <AppShell />
      </DocsProvider>
    </RootDocument>
  )
}

const THEME_INIT_SCRIPT = `(function(){try{var t=localStorage.getItem('hc-theme');if(t==='dark'||(t!=='light'&&matchMedia('(prefers-color-scheme:dark)').matches))document.documentElement.classList.add('dark')}catch(e){}})()`

function RootDocument({
  children,
  lang = 'en',
}: Readonly<{ children: ReactNode; lang?: string }>) {
  return (
    <html lang={lang}>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  )
}

function RootErrorBoundary({ error }: { error: Error }) {
  return (
    <RootDocument>
      <ErrorState
        title="Help Center unavailable"
        message={error.message || 'This help center is not currently available.'}
        statusCode={404}
        fullScreen
      />
    </RootDocument>
  )
}
