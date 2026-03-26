import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'

export const Route = createFileRoute('/')({
  component: RootLocaleRedirect,
})

function RootLocaleRedirect() {
  const { defaultLocale } = useDocsContext()
  const navigate = useNavigate()

  useEffect(() => {
    navigate({
      to: '/$locale',
      params: { locale: defaultLocale },
      replace: true,
    })
  }, [defaultLocale, navigate])

  return <LoadingState message="Redirecting..." />
}
