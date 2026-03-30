import { useEffect } from 'react'
import { X } from 'lucide-react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { NavTree } from './NavTree'
import type { NavItem } from '@/lib/types'

interface MobileNavProps {
  locale: string
  navigation: NavItem[]
  onClose: () => void
}

export function MobileNav({ locale, navigation, onClose }: MobileNavProps) {
  useEffect(() => {
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
    }
  }, [])

  return (
    <div className="fixed inset-0 z-40 lg:hidden">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />

      <aside
        className="absolute left-0 top-0 bottom-0 bg-background border-r border-border"
        style={{ width: 'min(280px, 85vw)' }}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-border">
          <span className="text-[13px] font-semibold">Navigation</span>
          <button
            onClick={onClose}
            className="inline-flex items-center justify-center h-7 w-7 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
            aria-label="Close navigation"
          >
            <X size={16} />
          </button>
        </div>
        <ScrollArea className="h-[calc(100vh-49px)]">
          <NavTree
            locale={locale}
            navigation={navigation}
            onArticleClick={onClose}
          />
        </ScrollArea>
      </aside>
    </div>
  )
}
