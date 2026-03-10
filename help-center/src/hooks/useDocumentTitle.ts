import { useEffect } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'

/**
 * Sets the document title with the help center brand name suffix.
 * Pass null/undefined to reset to the brand name only.
 */
export function useDocumentTitle(title?: string | null) {
  const { config } = useDocsContext()
  const brandName = config.brand_name || 'Help Center'

  useEffect(() => {
    document.title = title ? `${title} — ${brandName}` : brandName
    return () => {
      document.title = brandName
    }
  }, [title, brandName])
}
