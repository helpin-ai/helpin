import type { ReactNode } from 'react'

/** Keep the details rail beside both the header and editor, with independent scrolling. */
export function DocsDocumentLayout({ content, children, sidebar }: { content: ReactNode; children?: ReactNode; sidebar: ReactNode }) {
  return <div className="relative flex h-full min-h-0 overflow-hidden" data-docs-document-layout>
    <div className="flex min-w-0 flex-1 flex-col overflow-y-auto lg:overflow-hidden" data-docs-document-main>
      {content}
    </div>
    {sidebar && <aside aria-label="Document details" className="absolute inset-y-0 right-0 z-40 flex w-[300px] max-w-full shrink-0 flex-col overflow-hidden border-l border-border/60 bg-background shadow-xl lg:static lg:shadow-none">
      {sidebar}
    </aside>}
    {children}
  </div>
}
