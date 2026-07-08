import { useState } from 'react'
import { BellRing } from 'lucide-react'
import { toast } from 'sonner'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { setPushPrimingPref } from '@mobile/lib/prefs'
import { registerForPush } from '@mobile/push/push-registration'

export interface PermissionPrimingSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * First-run "priming" sheet shown before the OS notification permission
 * prompt — explains the value up front so the OS prompt (triggered by
 * `registerForPush()`, which calls the plugin's `getPushToken()`) doesn't
 * cold-prompt the user out of nowhere. Mount/visibility gating
 * (signed-in + Tauri + `shouldShowPriming`) lives in the caller
 * (`inbox-screen.tsx`); this component only renders the sheet body and
 * persists the user's decision.
 */
export function PermissionPrimingSheet({ open, onOpenChange }: PermissionPrimingSheetProps) {
  const [enabling, setEnabling] = useState(false)

  const handleEnable = async () => {
    setEnabling(true)
    try {
      const result = await registerForPush()
      // 'enabled' is persisted ONLY on a confirmed registration (token
      // obtained + backend POST succeeded). A denied prompt / simulator /
      // failed POST persists 'later' instead, so the natural 7-day cooldown
      // becomes the retry path rather than being locked into "enabled"
      // forever with no recovery.
      await setPushPrimingPref({
        decision: result === 'registered' ? 'enabled' : 'later',
        at: new Date().toISOString(),
      })
    } catch (error) {
      // `registerForPush()` can reject (e.g. the native plugin's
      // `getPushToken()` throwing) rather than resolving to 'unavailable' —
      // without this catch the rejection escaped as an unhandled promise
      // rejection (this handler is invoked via `void handleEnable()`) and
      // the user never saw any feedback. Surface the same error toast as
      // the ordinary 'unavailable' path would from the You-screen row.
      console.debug('[push] registerForPush rejected', error)
      toast.error("Couldn't enable notifications")
    } finally {
      setEnabling(false)
      onOpenChange(false)
    }
  }

  const handleNotNow = () => {
    void setPushPrimingPref({ decision: 'later', at: new Date().toISOString() })
    onOpenChange(false)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Enable notifications">
      <div className="flex flex-col items-center gap-3 px-6 pb-8 pt-2 text-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 text-primary">
          <BellRing className="h-7 w-7" />
        </div>
        <h2 className="text-title">Never miss a customer</h2>
        <p className="text-footnote text-muted-foreground">
          Get notified the moment a customer replies, even when the app is in the background.
        </p>

        <Pressable
          haptic="impactLight"
          disabled={enabling}
          onPress={() => void handleEnable()}
          className="mt-3 flex h-[52px] w-full items-center justify-center rounded-xl bg-primary text-body font-medium text-primary-foreground"
        >
          Enable notifications
        </Pressable>
        <Pressable
          onPress={handleNotNow}
          disabled={enabling}
          className="flex h-auto min-h-0 items-center justify-center py-1 text-footnote text-muted-foreground"
        >
          Not now
        </Pressable>
      </div>
    </Sheet>
  )
}
