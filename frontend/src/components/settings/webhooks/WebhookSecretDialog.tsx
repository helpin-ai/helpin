import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const VERIFY_EXAMPLE = `// Node.js: verify Helpin-Signature before trusting the body.
const [t, v1] = header.split(',').map((part) => part.split('=')[1])
const expected = crypto.createHmac('sha256', secret).update(\`\${t}.\${rawBody}\`).digest('hex')
const valid = v1?.length === expected.length
  && crypto.timingSafeEqual(Buffer.from(v1), Buffer.from(expected))
  && Math.abs(Date.now() / 1000 - Number(t)) < 300`

/** Shows a new signing secret once. */
export function WebhookSecretDialog({ secret, onClose }: { secret: string | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    if (!secret) return
    try {
      await navigator.clipboard.writeText(secret)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }
  return (
    <Dialog open={Boolean(secret)} onOpenChange={(open) => { if (!open) { setCopied(false); onClose() } }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Signing secret</DialogTitle>
          <DialogDescription>Copy it now; it is not shown again. Use it to check that each request comes from Helpin.</DialogDescription>
        </DialogHeader>
        <div className="min-w-0 space-y-1.5">
          <Label htmlFor="webhook-secret">Secret</Label>
          <div className="flex gap-2">
            <Input id="webhook-secret" readOnly value={secret ?? ''} className="min-w-0 font-mono text-xs" onFocus={(event) => event.currentTarget.select()} />
            <Button type="button" variant="outline" onClick={() => void copy()}>{copied ? 'Copied' : 'Copy'}</Button>
          </div>
        </div>
        <div className="min-w-0 space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Verify the signature</div>
          <pre className="overflow-x-auto rounded-md bg-muted px-3 py-2 text-[11px] leading-relaxed">{VERIFY_EXAMPLE}</pre>
        </div>
        <DialogFooter>
          <Button type="button" onClick={() => { setCopied(false); onClose() }}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
