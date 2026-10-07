import type { AIProfile } from "@/lib/services/aiProfileService";
import { catalogLabel } from "@/lib/aiProviders";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import {
  PencilEdit01Icon,
  Delete01Icon,
  MoreHorizontalIcon,
} from "@/lib/icons";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";

export function AIConfiguredModelRow({
  profile,
  isDefault,
  notListed,
  editable,
  busy,
  onEdit,
  onRemove,
  onVisibility,
}: {
  profile: AIProfile;
  isDefault: boolean;
  notListed: boolean;
  editable: boolean;
  busy: boolean;
  onEdit: () => void;
  onRemove: () => void;
  onVisibility: (shown: boolean) => void;
}) {
  const modelID = profile.primary.model.model;
  const modelName =
    catalogLabel(profile.primary.model.provider, modelID) ?? modelID;
  const detail = [
    profile.name !== modelName ? modelName : null,
    profile.fallback ? "Fallback configured" : null,
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <div className="flex items-center gap-3 border-b py-3 last:border-0">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="break-words text-sm font-medium">
            {profile.name}
          </span>
          {isDefault && <Badge variant="secondary">Default</Badge>}
          {notListed && (
            <QuickTooltip label="This model is absent from the provider's latest list. It may be unavailable or require different access. Edit it to choose a replacement; saved agents are never switched automatically.">
              <span
                tabIndex={0}
                className="shrink-0 text-xs text-amber-700 dark:text-amber-400"
              >
                Not listed
              </span>
            </QuickTooltip>
          )}
        </div>
        {detail && (
          <p className="truncate text-xs text-muted-foreground">{detail}</p>
        )}
        <AIConnectionPolicyNotice policy={profile.primary_policy} />
      </div>
      {editable && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7 shrink-0"
              aria-label={`Actions for ${profile.name}`}
              disabled={busy}
            >
              <MoreHorizontalIcon className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={onEdit}>
              <PencilEdit01Icon className="size-4" />
              Model settings
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onSelect={onRemove}>
              <Delete01Icon className="size-4" />
              Remove configuration
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <Switch
        aria-label={`Show ${profile.name} in model pickers`}
        checked={!profile.hidden_from_ask_agent}
        disabled={!editable || busy}
        onCheckedChange={onVisibility}
      />
    </div>
  );
}
