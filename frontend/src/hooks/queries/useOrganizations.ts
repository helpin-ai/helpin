import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { organizationsService } from '@/lib/services/organizationsService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useOrganizations() {
  return useQuery({
    queryKey: queryKeys.organizations.all,
    queryFn: async () => unwrap(await organizationsService.list()),
    staleTime: 5 * 60_000,
  })
}

export function useCreateOrganization() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: { name: string; slug: string }) => unwrap(await organizationsService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.organizations.all })
    },
  })
}

export function useUpdateOrganization() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: { id: string; name?: string; slug?: string }) =>
      unwrap(await organizationsService.update(id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.organizations.all })
    },
  })
}

export function useDeleteOrganization() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await organizationsService.delete(id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.organizations.all })
    },
  })
}
