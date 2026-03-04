import { createFileRoute } from '@tanstack/react-router'
import { FileText } from 'lucide-react'
import { useTitle } from '@/hooks/useTitle'

export const Route = createFileRoute('/_authenticated/w/$slug/docs')({
  component: Docs,
})

function Docs() {
  return (
    <div className="flex h-full flex-col items-center justify-center p-4 text-center md:p-6">
      <FileText className="h-12 w-12 text-muted-foreground/40" />
      <h2 className="mt-4 text-xl font-semibold">Documentation</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Coming Soon — Documentation features are being built.
      </p>
    </div>
  )
}
