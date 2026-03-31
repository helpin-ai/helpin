import { QueryClient } from '@tanstack/react-query'

const defaultQueryOptions = {
  queries: {
    staleTime: 300_000,
    gcTime: 1_800_000,
    retry: 1,
    refetchOnWindowFocus: false,
  },
} as const

export function createHelpCenterQueryClient() {
  return new QueryClient({
    defaultOptions: defaultQueryOptions,
  })
}
