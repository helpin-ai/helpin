import { useCallback, useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  aiConnectionService,
  type AIConnectionSelection,
} from "@/lib/services/aiConnectionService";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import { QuietTextAction } from "@/components/design-system/quiet";
import { Label } from "@/components/ui/label";
import { AIConnectionsDialog } from "./AIConnectionsDialog";

export function AIConnectionPicker({
  workspaceId,
  value,
  onChange,
  disabled = false,
  locked = false,
}: {
  workspaceId: string;
  value: AIConnectionSelection;
  onChange: (value: AIConnectionSelection) => void;
  disabled?: boolean;
  locked?: boolean;
}) {
  const id = useId();
  const [manage, setManage] = useState(false);
  const query = useQuery({
    queryKey: ["ai-connections", workspaceId],
    enabled: !!workspaceId,
    queryFn: async () => {
      const res = await aiConnectionService.list(workspaceId);
      if (res.error || !res.data)
        throw new Error("Unable to load AI connections");
      return res.data;
    },
  });
  const onChanged = useCallback(() => {
    void query.refetch();
  }, [query.refetch]);
  if (query.isPending)
    return (
      <p className="text-xs text-muted-foreground">Loading AI connections…</p>
    );
  if (query.isError)
    return (
      <QuietTextAction type="button" onClick={() => void query.refetch()}>
        Retry loading AI connections
      </QuietTextAction>
    );
  if (!query.data.enabled) return null;
  const { connections, models } = query.data;
  const connection = connections.find(
    (item) => item.id === value.model_connection_id,
  );
  const availableModels = models.filter(
    (item) => item.provider === connection?.provider,
  );
  return (
    <div className="min-w-0 space-y-2">
      <Label htmlFor={id}>AI connection</Label>
      <Select
        value={value.model_connection_id || "runtime"}
        disabled={disabled || locked}
        onValueChange={(next) => {
          const selected = connections.find((item) => item.id === next);
          onChange(
            selected
              ? {
                  model_connection_id: selected.id,
                  model_name: models.find(
                    (item) => item.provider === selected.provider,
                  )?.selection_model,
                }
              : {},
          );
        }}
      >
        <SelectTrigger
          aria-label="AI connection"
          id={id}
          variant="underline"
          className="w-full px-0.5"
        >
          <SelectValue placeholder="Runtime default" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="runtime">Runtime default</SelectItem>
          {connections.map((item) => (
            <SelectItem
              key={item.id}
              value={item.id}
              disabled={
                item.status !== "connected" ||
                !models.some((m) => m.provider === item.provider)
              }
            >
              {item.name}
              {item.status !== "connected" ? " · Reconnect required" : ""}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {value.model_connection_id && (
        <>
          <Label htmlFor={`${id}-model`}>Model</Label>
          <Select
            value={value.model_name || ""}
            disabled={disabled || locked}
            onValueChange={(model_name) => onChange({ ...value, model_name })}
          >
            <SelectTrigger
              aria-label="Model"
              id={`${id}-model`}
              variant="underline"
              className="w-full px-0.5"
            >
              <SelectValue placeholder="Choose a model" />
            </SelectTrigger>
            <SelectContent>
              {availableModels.map((item) => (
                <SelectItem
                  key={item.selection_model}
                  value={item.selection_model}
                >
                  {item.label} · {item.tier}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {connection?.status !== "connected" && (
            <p role="alert" className="text-xs text-destructive">
              Reconnect this connection before starting a run.
            </p>
          )}
          <p className="text-xs text-muted-foreground">
            Helpin AI credits still apply at the usual rate when using your own
            API key or ChatGPT subscription.
          </p>
        </>
      )}
      <QuietTextAction
        type="button"
        disabled={disabled}
        onClick={() => setManage(true)}
      >
        Manage my AI connections
      </QuietTextAction>
      <AIConnectionsDialog
        workspaceId={workspaceId}
        open={manage}
        onOpenChange={setManage}
        connections={connections}
        models={models}
        onChanged={onChanged}
      />
    </div>
  );
}
