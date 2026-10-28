import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useSetCompanySLATier } from '@/hooks/queries/useSupportSLATiers'
import { Cancel01Icon, Loading01Icon } from '@/lib/icons'
import type { SupportSLATier } from '@/lib/pmTypes'

const NO_TIER = '__none__'

/** Floating actions for selected companies: set or clear their SLA tier. */
export function CompanySLATierBulkBar({
  workspaceId,
  companyIds,
  tiers,
  onClearSelection,
}: {
  workspaceId: string
  companyIds: string[]
  tiers: SupportSLATier[]
  onClearSelection: () => void
}) {
  const setTier = useSetCompanySLATier(workspaceId)
  const count = companyIds.length
  const noun = count === 1 ? 'company' : 'companies'

  const apply = (value: string) => {
    const tier = tiers.find((candidate) => candidate.id === value)
    setTier.mutate({ companyIds, tierId: tier?.id ?? null }, {
      onSuccess: ({ updated }) => {
        const what = tier ? `set to ${tier.name}` : 'cleared'
        toast.success(updated === 0 ? `SLA tier already ${tier ? tier.name : 'empty'}` : `SLA tier ${what} for ${updated} ${updated === 1 ? 'company' : 'companies'}`)
        onClearSelection()
      },
    })
  }

  return (
    <div
      role="toolbar"
      aria-label={`Actions for ${count} selected ${noun}`}
      className="pointer-events-auto flex max-w-full flex-wrap items-center gap-1 rounded-lg border border-border bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-lg"
    >
      <span className="mr-1 inline-flex shrink-0 items-center gap-1 px-1 font-medium">
        {setTier.isPending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : null}
        {count} selected
      </span>
      <Select value="" disabled={setTier.isPending} onValueChange={apply}>
        <SelectTrigger size="sm" className="h-7 min-w-[120px] gap-1 border-0 bg-transparent px-2 text-xs shadow-none hover:bg-accent" aria-label="Set SLA tier">
          <SelectValue placeholder="Set SLA tier" />
        </SelectTrigger>
        <SelectContent>
          {tiers.map((tier) => <SelectItem key={tier.id} value={tier.id}>{tier.name}</SelectItem>)}
          <SelectItem value={NO_TIER}>No tier</SelectItem>
        </SelectContent>
      </Select>
      <Button variant="ghost" size="icon" className="h-7 w-7" aria-label="Clear selection" onClick={onClearSelection}>
        <Cancel01Icon className="h-3.5 w-3.5" />
      </Button>
    </div>
  )
}
