import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { slackService } from '@/lib/services/slackService'

export const Route = createFileRoute('/_authenticated/oauth/slack/callback')({
  component: SlackCallback,
})
function SlackCallback() {
  const [message, setMessage] = useState('Connecting Slack…')
  const params = new URLSearchParams(location.search)
  const [pairing, setPairing] = useState(params.has('pair'))
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    const query = new URLSearchParams(location.search)
    async function finish() {
      if (query.has('pair')) return
      const pending = JSON.parse(
        sessionStorage.getItem('slack-install') || 'null',
      )
      if (!pending?.workspaceId || !query.get('code'))
        throw new Error(
          'Connection cancelled or expired. Return to External tools to try again.',
        )
      await slackService.complete(
        pending.workspaceId,
        query.get('code')!,
        query.get('state') ?? '',
      )
      sessionStorage.removeItem('slack-install')
      window.location.replace(
        '/w/' + encodeURIComponent(pending.slug) + '/settings/external-tools?tab=apps' + (pending.initialInbox ? '&slack_inbox=' + encodeURIComponent(pending.initialInbox) : ''),
      )
    }
    void finish().catch((e) =>
      setMessage(e instanceof Error ? e.message : 'Could not connect Slack'),
    )
  }, [])
  if (pairing)
    return (
      <main className="mx-auto max-w-sm space-y-4 py-20">
        <h1 className="text-lg font-semibold">Link your Slack account</h1>
        <form
          className="space-y-3"
          onSubmit={async (e) => {
            e.preventDefault()
            setBusy(true)
            try {
              await slackService.link(
                params.get('workspace') ?? '',
                code.trim(),
              )
              setPairing(false)
              setMessage(
                params.get('note') === '1'
                  ? 'Account linked. Helpin will save your original note. You can return to Slack.'
                  : 'Account linked. Return to Slack and repeat your request.',
              )
            } catch {
              setMessage(
                'This code expired or is invalid. Request a new code in Slack.',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          <label htmlFor="code" className="text-sm">
            Private code from @Helpin
          </label>
          <Input
            id="code"
            required
            autoComplete="off"
            value={code}
            onChange={(e) => setCode(e.target.value)}
          />
          <Button disabled={busy || !code.trim()}>Link accounts</Button>
        </form>
        {message.startsWith('This code') && <p role="alert">{message}</p>}
      </main>
    )
  return (
    <main className="grid min-h-screen place-content-center gap-3 text-center">
      <h1 className="font-semibold">Slack</h1>
      <p role="status">{message}</p>
      <a className="text-sm underline" href="/workspaces">
        Back to Helpin
      </a>
    </main>
  )
}
