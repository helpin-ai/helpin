import { useEffect } from 'react'
import {
  createRootRouteWithContext,
  useParams,
  useRouterState,
} from '@tanstack/react-router'
import { Eye } from 'lucide-react'
import { AppShell } from '@/components/layout/AppShell'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { DocsProvider } from '@/contexts/DocsContext'
import { useHelpCenterConfig, useSpaces } from '@/hooks/queries'
import { resolveActiveLocale } from '@/lib/locale'
import type { HelpCenterContext } from '@/lib/types'

export const Route = createRootRouteWithContext<HelpCenterContext>()({
  component: RootLayout,
})

function RootLayout() {
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const params = useParams({ strict: false }) as { locale?: string; spaceSlug?: string }
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const isPreview = pathname.startsWith('/preview/')

  const {
    data: config,
    isLoading: configLoading,
    error: configError,
  } = useHelpCenterConfig(subdomain)

  const activeLocale = resolveActiveLocale({
    paramsLocale: params.locale,
    paramsSpaceSlug: params.spaceSlug,
    enabledLocales: config?.enabled_locales,
    defaultLocale: config?.default_locale || 'en',
  })
  const { data: spaces, isLoading: spacesLoading } = useSpaces(
    subdomain,
    activeLocale,
  )

  // Inject brand color as CSS custom property overrides
  useEffect(() => {
    if (!config?.brand_color) return

    const hex = config.brand_color.replace('#', '')
    if (hex.length !== 6) return

    const r = parseInt(hex.substring(0, 2), 16)
    const g = parseInt(hex.substring(2, 4), 16)
    const b = parseInt(hex.substring(4, 6), 16)

    // Lighter version for dark mode
    const lighten = (c: number, amount: number) => Math.round(c + (255 - c) * amount)
    const lr = lighten(r, 0.35)
    const lg = lighten(g, 0.35)
    const lb = lighten(b, 0.35)
    const lightColor = `rgb(${lr}, ${lg}, ${lb})`

    const style = document.createElement('style')
    style.id = 'brand-color-override'
    style.textContent = `
      :root {
        --primary: ${config.brand_color};
        --ring: ${config.brand_color};
        --sidebar-active: rgba(${r}, ${g}, ${b}, 0.08);
        --sidebar-active-foreground: ${config.brand_color};
      }
      .dark {
        --primary: ${lightColor};
        --ring: ${lightColor};
        --sidebar-active: rgba(${lr}, ${lg}, ${lb}, 0.12);
        --sidebar-active-foreground: ${lightColor};
      }
    `
    document.getElementById('brand-color-override')?.remove()
    document.head.appendChild(style)

    return () => {
      document.getElementById('brand-color-override')?.remove()
    }
  }, [config?.brand_color])

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

  if (configLoading || spacesLoading) {
    return <LoadingState message="Loading help center..." fullScreen />
  }

  if (configError) {
    return (
      <ErrorState
        title="Help Center not found"
        message="This help center does not exist or is not currently available."
        statusCode={404}
        fullScreen
      />
    )
  }

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
    <DocsProvider
      subdomain={subdomain}
      locale={activeLocale}
      defaultLocale={config!.default_locale}
      enabledLocales={config!.enabled_locales ?? [config!.default_locale]}
      config={config!}
      spaces={spaces ?? []}
    >
      {isPreview && (
        <div className="sticky top-0 z-50 flex items-center justify-center gap-2 border-b bg-amber-50 dark:bg-amber-950/30 px-4 py-2 text-center">
          <Eye size={14} className="text-amber-600 dark:text-amber-400 shrink-0" />
          <span className="text-sm font-medium text-amber-700 dark:text-amber-300">
            Preview Mode
          </span>
          <span className="text-xs text-amber-600/70 dark:text-amber-400/60">
            — This is how your article will appear in the help center.
          </span>
        </div>
      )}
      <AppShell />
    </DocsProvider>
  )
}
