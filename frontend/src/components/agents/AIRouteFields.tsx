import { useId, useState } from "react";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { AIModelCombobox } from "./AIModelCombobox";
import { ProviderIcon } from "./ProviderIcon";
import { AI_MODELS } from "@/generated/aiModels";
import { connectionStatusInfo, providerControls, providerLabel } from "@/lib/aiProviders";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import type { AIProfileRoute } from "@/lib/services/aiProfileService";
import { Label } from "@/components/ui/label";
import { QuietUnderlineInput } from "@/components/design-system/quiet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";

const REASONING_EFFORTS = ["default", "none", "minimal", "low", "medium", "high", "xhigh"];

function unavailableReason(connection: AIConnection): string | null {
  if (connection.policy?.allowed === false)
    return connection.policy.message ?? "Unavailable for new runs";
  if (connection.status !== "connected") return connectionStatusInfo(connection.status).label;
  return null;
}

export function AIRouteFields({
  route,
  onChange,
  connections,
  disabled,
  emptyHint,
}: {
  route: AIProfileRoute;
  onChange: (route: AIProfileRoute) => void;
  connections: AIConnection[];
  disabled: boolean;
  emptyHint?: string;
}) {
  const id = useId();
  const connection = connections.find((entry) => entry.id === route.connection_id);
  const controls = providerControls(route.model.provider);
  const showTwoColumns =
    [controls.reasoningEffort, controls.serviceTier, controls.openrouterQuantizations].filter(
      Boolean,
    ).length > 1;

  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-connection`}>Connection</Label>
        <Select
          value={route.connection_id}
          disabled={disabled || connections.length === 0}
          onValueChange={(value) => {
            const next = connections.find((entry) => entry.id === value);
            if (!next) return;
            onChange({
              connection_id: value,
              model: {
                provider: next.provider,
                endpoint: next.endpoint,
                model: "",
                controls: {},
              },
            });
          }}
        >
          <SelectTrigger id={`${id}-connection`} variant="underline" className="w-full px-0.5">
            <SelectValue placeholder="Choose a connection" />
          </SelectTrigger>
          <SelectContent>
            {connections.map((entry) => {
              const reason = unavailableReason(entry);
              return (
                <SelectItem
                  key={entry.id}
                  value={entry.id}
                  textValue={entry.name}
                  disabled={entry.policy?.allowed === false}
                >
                  <span className="flex items-center gap-2">
                    <ProviderIcon provider={entry.provider} className="h-3.5 w-3.5 shrink-0" />
                    <span>{entry.name}</span>
                  </span>
                  <span className="block text-[11px] text-muted-foreground">
                    {providerLabel(entry.provider)}
                    {reason ? ` · ${reason}` : ""}
                  </span>
                </SelectItem>
              );
            })}
          </SelectContent>
        </Select>
        {connections.length === 0 && emptyHint && (
          <p className="text-xs text-muted-foreground">{emptyHint}</p>
        )}
        <AIConnectionPolicyNotice policy={connection?.policy} />
        {connection?.endpoint && (
          <p className="break-all text-xs text-muted-foreground">
            Endpoint: {connection.endpoint.id} ·{" "}
            {connection.endpoint.auth_mode === "none" ? "No authentication" : "API key"}
          </p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor={`${id}-model`}>Model</Label>
        <AIModelCombobox
          id={`${id}-model`}
          provider={route.model.provider}
          value={route.model.model}
          disabled={disabled || !route.connection_id}
          onChange={(model) => onChange({ ...route, model: { ...route.model, model } })}
        />
      </div>

      {(controls.reasoningEffort || controls.serviceTier || controls.openrouterQuantizations) && (
        <div className={showTwoColumns ? "grid gap-4 sm:grid-cols-2" : "space-y-4"}>
          {controls.reasoningEffort && (
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-reasoning`}>Reasoning effort</Label>
              <Select
                value={route.model.controls.reasoning_effort ?? "default"}
                disabled={disabled}
                onValueChange={(value) =>
                  onChange({
                    ...route,
                    model: {
                      ...route.model,
                      controls: {
                        ...route.model.controls,
                        reasoning_effort: value === "default" ? undefined : value,
                      },
                    },
                  })
                }
              >
                <SelectTrigger
                  id={`${id}-reasoning`}
                  variant="underline"
                  className="w-full px-0.5"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {REASONING_EFFORTS.map((value) => (
                    <SelectItem key={value} value={value}>
                      {value === "default" ? "Provider default" : value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {controls.serviceTier && (
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-service`}>Service tier</Label>
              <Select
                value={route.model.controls.service_tier ?? "unset"}
                disabled={disabled}
                onValueChange={(value) =>
                  onChange({
                    ...route,
                    model: {
                      ...route.model,
                      controls: {
                        ...route.model.controls,
                        service_tier: value === "unset" ? undefined : value,
                      },
                    },
                  })
                }
              >
                <SelectTrigger id={`${id}-service`} variant="underline" className="w-full px-0.5">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {[
                    "unset",
                    ...AI_MODELS.service_tiers,
                    ...(["default", "priority"].includes(route.model.controls.service_tier ?? "")
                      ? [route.model.controls.service_tier!]
                      : []),
                  ].map((value) => (
                    <SelectItem key={value} value={value}>
                      {value === "unset" ? "Provider default" : value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {controls.openrouterQuantizations && (
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-quantizations`}>Provider quantizations (optional)</Label>
              <QuantizationsInput
                key={route.connection_id}
                id={`${id}-quantizations`}
                disabled={disabled}
                initialValue={
                  route.model.controls.openrouter?.provider?.quantizations?.join(", ") ?? ""
                }
                onChange={(quantizations) =>
                  onChange({
                    ...route,
                    model: {
                      ...route.model,
                      controls: {
                        ...route.model.controls,
                        openrouter: quantizations.length
                          ? { provider: { quantizations } }
                          : undefined,
                      },
                    },
                  })
                }
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function QuantizationsInput({
  id,
  disabled,
  initialValue,
  onChange,
}: {
  id: string;
  disabled: boolean;
  initialValue: string;
  onChange: (values: string[]) => void;
}) {
  const [text, setText] = useState(initialValue);
  return (
    <QuietUnderlineInput
      id={id}
      disabled={disabled}
      value={text}
      placeholder="Comma-separated values"
      onChange={(event) => {
        setText(event.target.value);
        onChange(
          event.target.value
            .split(",")
            .map((value) => value.trim())
            .filter(Boolean),
        );
      }}
    />
  );
}
