import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { useId, useState } from "react";
import type { AIConnection } from "@/lib/services/aiConnectionService";
import {
  aiProfileService,
  type AIProfile,
  type AIProfileRoute,
} from "@/lib/services/aiProfileService";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  QuietPrimaryAction,
  QuietTextAction,
  QuietUnderlineInput,
  QuietSection,
} from "@/components/design-system/quiet";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/design-system/quiet-dropdown-select";

const emptyRoute = (): AIProfileRoute => ({
  connection_id: "",
  model: { provider: "", model: "", controls: {} },
});

export function AIProfileEditor({
  workspaceId,
  scope,
  profile,
  connections,
  onClose,
  onSaved,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  profile?: AIProfile;
  connections: AIConnection[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const id = useId();
  const [name, setName] = useState(profile?.name ?? "");
  const [primary, setPrimary] = useState(profile?.primary ?? emptyRoute());
  const [fallback, setFallback] = useState<AIProfileRoute | null>(
    profile?.fallback ?? null,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const available = connections.filter(
    (c) => scope === "personal" || c.scope === "workspace",
  );
  async function save() {
    setBusy(true);
    setError("");
    try {
      const response = await aiProfileService.save(
        workspaceId,
        { name, scope, primary, fallback, revision: profile?.revision },
        profile?.id,
      );
      if (response.error) {
        setError(response.error);
        return;
      }
      onSaved();
      onClose();
    } catch {
      setError(
        "Unable to save this profile. Your edits are still here; please retry.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
    >
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {profile ? "Edit AI profile" : "Create AI profile"}
          </DialogTitle>
          <DialogDescription>
            {scope === "personal"
              ? "For your manual runs in this workspace."
              : "For workspace members and automation."}{" "}
            Changes apply to new runs.
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <div>
            <Label htmlFor={`${id}-name`}>Name</Label>
            <QuietUnderlineInput
              id={`${id}-name`}
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              required
              disabled={busy}
            />
          </div>
          <QuietSection title="Primary" className="px-0 sm:px-0 lg:px-0">
            <ProfileRouteFields
              route={primary}
              onChange={setPrimary}
              connections={available}
              disabled={busy}
            />
          </QuietSection>
          <QuietSection
            title="Fallback"
            className="px-0 sm:px-0 lg:px-0"
            action={
              <QuietTextAction
                type="button"
                disabled={busy}
                onClick={() => setFallback(fallback ? null : emptyRoute())}
              >
                {fallback ? "Remove fallback" : "Add fallback"}
              </QuietTextAction>
            }
          >
            <p className="mb-3 text-sm text-quiet-text-secondary">
              Used only if the primary connection is unavailable before a run
              starts. An accepted run keeps its selected route.
            </p>
            {fallback && (
              <ProfileRouteFields
                route={fallback}
                onChange={setFallback}
                connections={available.filter(
                  (c) => c.id !== primary.connection_id,
                )}
                disabled={busy}
              />
            )}
          </QuietSection>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <QuietTextAction type="button" onClick={onClose} disabled={busy}>
              Cancel
            </QuietTextAction>
            <QuietPrimaryAction
              type="submit"
              disabled={
                busy ||
                !name.trim() ||
                !primary.connection_id ||
                !primary.model.model.trim() ||
                !!(
                  fallback &&
                  (!fallback.connection_id || !fallback.model.model.trim())
                )
              }
            >
              {busy ? "Saving…" : "Save profile"}
            </QuietPrimaryAction>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ProfileRouteFields({
  route,
  onChange,
  connections,
  disabled,
}: {
  route: AIProfileRoute;
  onChange: (route: AIProfileRoute) => void;
  connections: AIConnection[];
  disabled: boolean;
}) {
  const id = useId();
  const connection = connections.find((c) => c.id === route.connection_id);
  const supportsReasoning = ["openai", "openai_chatgpt", "openrouter"].includes(
    route.model.provider,
  );
  return (
    <div className="space-y-3">
      <div>
        <Label htmlFor={`${id}-connection`}>Connection</Label>
        <Select
          value={route.connection_id}
          disabled={disabled}
          onValueChange={(value) => {
            const connection = connections.find((c) => c.id === value);
            if (connection)
              onChange({
                connection_id: value,
                model: {
                  provider: connection.provider,
                  model: "",
                  controls: {},
                },
              });
          }}
        >
          <SelectTrigger
            id={`${id}-connection`}
            variant="underline"
            className="w-full px-0.5"
          >
            <SelectValue placeholder="Choose a connection" />
          </SelectTrigger>
          <SelectContent>
            {connections.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name} · {c.scope === "workspace" ? "Workspace" : "Personal"}
                {c.status !== "connected" ? " · Reconnect required" : ""}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div>
        <AIConnectionPolicyNotice policy={connection?.policy} />
        <Label htmlFor={`${id}-model`}>Model identifier</Label>
        <QuietUnderlineInput
          id={`${id}-model`}
          value={route.model.model}
          onChange={(e) =>
            onChange({
              ...route,
              model: { ...route.model, model: e.target.value },
            })
          }
          disabled={disabled}
          required
          maxLength={256}
          placeholder="Exact model name from your provider"
        />
      </div>
      {supportsReasoning && (
        <div>
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
              {[
                "default",
                "none",
                "minimal",
                "low",
                "medium",
                "high",
                "xhigh",
              ].map((v) => (
                <SelectItem key={v} value={v}>
                  {v === "default" ? "Provider default" : v}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      {["openai", "openai_chatgpt"].includes(route.model.provider) && (
        <div>
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
            <SelectTrigger
              id={`${id}-service`}
              variant="underline"
              className="w-full px-0.5"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[
                "unset",
                "standard",
                "fast",
                "flex",
                "auto",
                "default",
                "priority",
              ].map((v) => (
                <SelectItem key={v} value={v}>
                  {v === "unset" ? "Provider default" : v}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      {route.model.provider === "openrouter" && (
        <div>
          <Label htmlFor={`${id}-quantizations`}>
            Provider quantizations (optional)
          </Label>
          <QuantizationsInput
            key={route.connection_id}
            id={`${id}-quantizations`}
            disabled={disabled}
            initialValue={
              route.model.controls.openrouter?.provider?.quantizations?.join(
                ", ",
              ) ?? ""
            }
            onChange={(quantizations) => {
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
              });
            }}
          />
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
