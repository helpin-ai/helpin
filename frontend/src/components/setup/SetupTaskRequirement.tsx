import { useId, useState } from 'react';
import { ArrowDown01Icon } from '@/lib/icons';
import type { CapabilitiesResponse, Capability } from '@/lib/capabilityTypes';
import { cn } from '@/lib/utils';
import { setupTextActionClassName } from './CapabilityActions';
import { findCapability } from './capabilityPresentation';
import type { SetupTaskRequirementKind } from './setupTaskRequirements';
import { SetupAIStep } from './SetupAIStep';
import { SetupGitHubStep } from './SetupGitHubStep';

export type SetupTaskRequirementProps = {
  kind: SetupTaskRequirementKind;
  capabilities: CapabilitiesResponse;
  workspaceId: string;
  slug: string;
  /** Workspace settings access (`workspace.update`). */
  canManage: boolean;
  isOwner: boolean;
};

/** Compact prerequisite shown inside a Setup guide task. */
export function SetupTaskRequirement({ kind, capabilities, workspaceId, slug, canManage, isOwner }: SetupTaskRequirementProps) {
  const capability = findCapability(capabilities, kind === 'ai' ? 'ai_chat' : 'github');
  if (!capability) return null;
  if (kind === 'ai') return <SetupTaskAIRequirement capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />;
  return (
    <div className="mt-2 space-y-1.5" data-requirement="github">
      <p className="text-[12.5px] text-quiet-text-tertiary">GitHub needs to be connected before you can choose repositories.</p>
      <SetupGitHubStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} isOwner={isOwner} />
    </div>
  );
}

function SetupTaskAIRequirement({ capability, workspaceId, slug, canManage }: { capability: Capability; workspaceId: string; slug: string; canManage: boolean }) {
  const [open, setOpen] = useState(false);
  const panelId = useId();

  if (!canManage) {
    return (
      <div className="mt-2" data-requirement="ai">
        <p className="text-[12.5px] text-quiet-text-tertiary">This step needs AI. A workspace admin can connect AI.</p>
      </div>
    );
  }

  return (
    <div className="mt-2 space-y-3" data-requirement="ai">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <p className="text-[12.5px] text-quiet-text-tertiary">This step needs an AI provider.</p>
        <button type="button" className={setupTextActionClassName} aria-expanded={open} aria-controls={panelId} onClick={() => setOpen((value) => !value)}>
          Connect AI
          <ArrowDown01Icon className={cn('h-3.5 w-3.5 transition-transform motion-reduce:transition-none', open && 'rotate-180')} aria-hidden="true" />
        </button>
      </div>
      <div id={panelId} hidden={!open}>
        {open && <SetupAIStep capability={capability} workspaceId={workspaceId} slug={slug} canManage />}
      </div>
    </div>
  );
}
