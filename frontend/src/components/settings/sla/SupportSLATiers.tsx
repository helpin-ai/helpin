import { useState } from 'react'
import { QuietIconAction, QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  useCreateSLATier,
  useDeleteSLATier,
  useReorderSLATiers,
  useSLATiers,
  useUpdateSLATier,
} from '@/hooks/queries/useSupportSLATiers'
import { ArrowDown01Icon, ArrowUp01Icon, Delete01Icon, PencilEdit01Icon, PlusSignIcon } from '@/lib/icons'
import type { SupportSLATier } from '@/lib/pmTypes'

function plural(count: number, singular: string, pluralForm: string) {
  return `${count} ${count === 1 ? singular : pluralForm}`
}

/** Tier name and description editor. A null tier creates a new one. */
function SLATierDialog({
  open,
  tier,
  saving,
  onOpenChange,
  onSave,
}: {
  open: boolean
  tier: SupportSLATier | null
  saving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (input: { name: string; description: string }) => Promise<boolean>
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{tier ? 'Edit tier' : 'New SLA tier'}</DialogTitle>
          <DialogDescription>Give companies a tier, then add SLA policies for it.</DialogDescription>
        </DialogHeader>
        {open ? <SLATierForm key={tier?.id ?? 'new'} tier={tier} saving={saving} onCancel={() => onOpenChange(false)} onSave={onSave} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function SLATierForm({ tier, saving, onCancel, onSave }: { tier: SupportSLATier | null; saving: boolean; onCancel: () => void; onSave: (input: { name: string; description: string }) => Promise<boolean> }) {
  const [name, setName] = useState(tier?.name ?? '')
  const [description, setDescription] = useState(tier?.description ?? '')
  const trimmed = name.trim()
  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        if (trimmed) void onSave({ name: trimmed, description: description.trim() }).then((saved) => { if (saved) onCancel() })
      }}
    >
      <div className="space-y-1">
        <Label htmlFor="sla-tier-name" className="text-sm font-medium text-quiet-text-secondary">Name</Label>
        <QuietUnderlineInput id="sla-tier-name" value={name} maxLength={60} placeholder="For example: Gold" onChange={(event) => setName(event.target.value)} autoFocus />
      </div>
      <div className="space-y-1">
        <Label htmlFor="sla-tier-description" className="text-sm font-medium text-quiet-text-secondary">Description</Label>
        <QuietUnderlineInput id="sla-tier-description" value={description} maxLength={300} placeholder="Optional, for example: Enterprise plan" onChange={(event) => setDescription(event.target.value)} />
      </div>
      <DialogFooter className="border-t border-quiet-divider-strong pt-3">
        <QuietTextAction type="button" disabled={saving} onClick={onCancel}>Cancel</QuietTextAction>
        <QuietPrimaryAction type="submit" disabled={saving || !trimmed}>{saving ? 'Saving…' : tier ? 'Save tier' : 'Create tier'}</QuietPrimaryAction>
      </DialogFooter>
    </form>
  )
}

/** Workspace SLA tiers, such as Gold and Standard, that companies carry. */
export function SupportSLATiers({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: tiers, isLoading, error } = useSLATiers(workspaceId)
  const createTier = useCreateSLATier(workspaceId)
  const updateTier = useUpdateSLATier(workspaceId)
  const deleteTier = useDeleteSLATier(workspaceId)
  const reorder = useReorderSLATiers(workspaceId)
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<SupportSLATier | null>(null)

  const openEditor = (tier: SupportSLATier | null) => {
    setEditing(tier)
    setDialogOpen(true)
  }

  const save = async (input: { name: string; description: string }) => {
    try {
      if (editing) await updateTier.mutateAsync({ id: editing.id, input })
      else await createTier.mutateAsync(input)
      return true
    } catch {
      return false
    }
  }

  const move = (index: number, direction: -1 | 1) => {
    if (!tiers) return
    const ids = tiers.map((tier) => tier.id)
    const target = index + direction
    if (target < 0 || target >= ids.length) return
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    reorder.mutate(ids)
  }

  const remove = async (tier: SupportSLATier) => {
    const usage = [
      tier.company_count ? `${plural(tier.company_count, 'company', 'companies')} will have no tier` : null,
      tier.policy_count ? `${plural(tier.policy_count, 'policy', 'policies')} will stop matching it, and a policy for this tier alone is turned off` : null,
    ].filter(Boolean)
    const confirmed = await confirm({
      title: `Delete the ${tier.name} tier?`,
      description: usage.length
        ? `${usage.join(', and ')}. Their open conversations are checked again, and a newly matching policy counts from now.`
        : 'No company or policy uses it.',
      confirmText: 'Delete tier',
      variant: 'destructive',
    })
    if (confirmed) deleteTier.mutate(tier.id)
  }

  if (isLoading) return <Skeleton className="h-24 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load SLA tiers: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <section className="space-y-3" aria-labelledby="sla-tiers-title">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 max-w-2xl">
          <h2 id="sla-tiers-title" className="text-sm font-semibold text-quiet-text-primary">Tiers</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Give companies a tier on their page, then add policies that match it. A company or tier change counts new targets from the change.
          </p>
        </div>
        {editable ? (
          <QuietTextAction onClick={() => openEditor(null)}>
            <PlusSignIcon className="h-4 w-4" />
            New tier
          </QuietTextAction>
        ) : null}
      </div>

      {!tiers || tiers.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-3 text-sm text-muted-foreground">
          No tiers yet. Without tiers, policies apply to every company alike.
        </p>
      ) : (
        <ol className="divide-y divide-border rounded-lg border border-border" aria-label="SLA tiers">
          {tiers.map((tier, index) => (
            <li key={tier.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5" data-testid="sla-tier">
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium">{tier.name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {[tier.description || null, plural(tier.company_count, 'company', 'companies'), plural(tier.policy_count, 'policy', 'policies')].filter(Boolean).join(' · ')}
                </div>
              </div>
              {editable ? (
                <div className="flex items-center gap-0.5">
                  <QuietIconAction aria-label={`Move ${tier.name} up`} disabled={index === 0 || reorder.isPending} onClick={() => move(index, -1)}>
                    <ArrowUp01Icon className="h-4 w-4" />
                  </QuietIconAction>
                  <QuietIconAction aria-label={`Move ${tier.name} down`} disabled={index === tiers.length - 1 || reorder.isPending} onClick={() => move(index, 1)}>
                    <ArrowDown01Icon className="h-4 w-4" />
                  </QuietIconAction>
                  <QuietIconAction aria-label={`Edit ${tier.name}`} onClick={() => openEditor(tier)}>
                    <PencilEdit01Icon className="h-4 w-4" />
                  </QuietIconAction>
                  <QuietIconAction aria-label={`Delete ${tier.name}`} className="hover:text-destructive" onClick={() => void remove(tier)}>
                    <Delete01Icon className="h-4 w-4" />
                  </QuietIconAction>
                </div>
              ) : null}
            </li>
          ))}
        </ol>
      )}

      <SLATierDialog
        open={dialogOpen}
        tier={editing}
        saving={createTier.isPending || updateTier.isPending}
        onOpenChange={setDialogOpen}
        onSave={save}
      />
    </section>
  )
}
