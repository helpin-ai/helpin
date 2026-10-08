import { OnboardingActions, OnboardingTextButton } from './OnboardingShell';

export function SetupMethodStep({ onUI, onAssistant, onLater }: { onUI: () => void; onAssistant: () => void; onLater: () => void }) {
  return <div className="space-y-7">
    <div className="space-y-3">
      {[
        { title: 'Set up in Helpin', description: 'Follow the setup steps here, at your own pace.', action: onUI },
        { title: 'Use my AI assistant', description: 'Connect an assistant you already use to help with teams, invitations and your selected goals.', action: onAssistant },
      ].map(option => <button key={option.title} type="button" onClick={option.action} className="flex w-full items-center gap-4 rounded-lg border p-5 text-left transition-colors hover:border-foreground/25 hover:bg-muted/40">
        <span className="min-w-0 flex-1"><span className="block text-sm font-semibold">{option.title}</span><span className="mt-1 block text-[13px] leading-relaxed text-muted-foreground">{option.description}</span></span>
        <span aria-hidden="true" className="text-muted-foreground">→</span>
      </button>)}
    </div>
    <OnboardingActions><OnboardingTextButton onClick={onLater}>I’ll do this later</OnboardingTextButton></OnboardingActions>
  </div>;
}
