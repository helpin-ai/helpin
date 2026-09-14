import { useQuery } from "@tanstack/react-query";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { useEffect, useId, useState } from "react";
import {
  aiConnectionService,
  type AIConnection,
  type AIConnectionLogin,
  type AIConnectionModel,
} from "@/lib/services/aiConnectionService";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  QuietPrimaryAction,
  QuietTextAction,
  QuietUnderlineInput,
} from "@/components/design-system/quiet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import { Label } from "@/components/ui/label";

const providerLabels: Record<string, string> = {
  openai: "OpenAI API key",
  anthropic: "Anthropic API key",
  openrouter: "OpenRouter API key",
  openai_chatgpt: "ChatGPT subscription",
  openai_compatible: "Compatible endpoint",
};
export function AIConnectionsDialog({
  workspaceId,
  scope = "personal",
  open,
  onOpenChange,
  connections,
  models,
  onChanged,
}: {
  workspaceId: string;
  scope?: "personal" | "workspace";
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connections: AIConnection[];
  models: AIConnectionModel[];
  onChanged: () => void;
}) {
  const id = useId();
  const [name, setName] = useState("");
  const [provider, setProvider] = useState("openai");
  const [apiKey, setApiKey] = useState("");
  const [endpointId, setEndpointId] = useState("");
  const [reconnecting, setReconnecting] = useState<string>();
  const [login, setLogin] = useState<AIConnectionLogin>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const endpoints = useQuery({
    queryKey: ["ai-model-endpoints", workspaceId],
    enabled: open && provider === "openai_compatible" && !reconnecting,
    queryFn: async () => {
      const response = await aiConnectionService.endpoints(workspaceId);
      if (response.error) throw new Error(response.error);
      return response.data ?? [];
    },
    retry: false,
  });
  const endpoint = reconnecting
    ? connections.find((c) => c.id === reconnecting)?.endpoint
    : endpoints.data?.find((e) => e.id === endpointId);
  const needsKey =
    provider !== "openai_chatgpt" &&
    !(provider === "openai_compatible" && endpoint?.auth_mode === "none");
  useEffect(() => {
    if (!open) {
      setApiKey("");
      setLogin(undefined);
      setReconnecting(undefined);
    }
  }, [open]);
  useEffect(() => {
    if (!open || login?.connection.status !== "pending") return;
    let cancelled = false;
    const timer = window.setTimeout(
      async () => {
        try {
          const res = await aiConnectionService.poll(
            workspaceId,
            login.connection.id,
          );
          if (cancelled) return;
          if (res.error || !res.data) {
            setError(res.error || "Login failed. Reconnect to try again.");
            setLogin(undefined);
            return;
          }
          setLogin(res.data);
          onChanged();
        } catch {
          if (!cancelled) {
            setError("Unable to check login. Select Continue login to retry.");
            setLogin(undefined);
          }
        }
      },
      Math.max(5, login.interval_seconds || 5) * 1000,
    );
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [login, open, workspaceId, onChanged]);
  async function save() {
    setBusy(true);
    setError("");
    try {
      const res = reconnecting
        ? await aiConnectionService.reconnect(
            workspaceId,
            reconnecting,
            apiKey || undefined,
          )
        : await aiConnectionService.create(workspaceId, {
            name,
            scope,
            provider,
            api_key: needsKey ? apiKey || undefined : undefined,
            endpoint_id:
              provider === "openai_compatible" ? endpointId : undefined,
          });
      setApiKey("");
      if (res.error || !res.data) {
        setError(res.error || "Unable to save connection");
        return;
      }
      setLogin(res.data);
      setReconnecting(undefined);
      setName("");
      onChanged();
    } catch {
      setError("Unable to save connection. Please retry.");
    } finally {
      setBusy(false);
    }
  }
  async function disconnect(connection: AIConnection) {
    setBusy(true);
    setError("");
    try {
      const res = await aiConnectionService.disconnect(
        workspaceId,
        connection.id,
      );
      if (res.error) setError(res.error);
      else {
        if (login?.connection.id === connection.id) setLogin(undefined);
        onChanged();
      }
    } catch {
      setError("Unable to disconnect. Retry to stop affected runs.");
    } finally {
      setBusy(false);
    }
  }
  async function continueLogin(connection: AIConnection) {
    setBusy(true);
    setError("");
    try {
      const res = await aiConnectionService.poll(workspaceId, connection.id);
      if (res.error || !res.data)
        setError(res.error || "Unable to check login");
      else {
        setLogin(res.data);
        onChanged();
      }
    } catch {
      setError("Unable to check login. Please retry.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {scope === "personal"
              ? "My AI connections"
              : "Workspace AI connections"}
          </DialogTitle>
          <DialogDescription>
            {scope === "personal"
              ? "Only you can use these connections in this workspace, for manual runs and chats."
              : "Workspace members can select these connections. Shared profiles can also use them for automation."}
          </DialogDescription>
        </DialogHeader>
        <div className="divide-y divide-border">
          {connections.length === 0 && (
            <p className="py-3 text-sm text-muted-foreground">
              No connections yet. Add one below, then create an AI profile.
            </p>
          )}
          {connections.map((connection) => (
            <div
              key={connection.id}
              className="flex min-w-0 flex-wrap items-center justify-between gap-2 py-3"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">
                  {connection.name}
                </p>
                <p className="text-xs text-muted-foreground">
                  {providerLabels[connection.provider]} ·{" "}
                  {connection.status.replaceAll("_", " ")}
                  {connection.funding === "managed"
                    ? " · Managed by your administrator"
                    : ""}
                </p>
                <AIConnectionPolicyNotice policy={connection.policy} />
              </div>
              {connection.funding !== "managed" && (
                <div className="flex gap-2">
                  {connection.status === "pending" ? (
                    <QuietTextAction
                      disabled={busy}
                      onClick={() => void continueLogin(connection)}
                    >
                      Continue login
                    </QuietTextAction>
                  ) : (
                    <QuietTextAction
                      disabled={busy}
                      onClick={() => {
                        setReconnecting(connection.id);
                        setProvider(connection.provider);
                        setName(connection.name);
                        setApiKey("");
                        setLogin(undefined);
                      }}
                    >
                      Reconnect
                    </QuietTextAction>
                  )}
                  <QuietTextAction
                    disabled={busy}
                    onClick={() => void disconnect(connection)}
                  >
                    Disconnect
                  </QuietTextAction>
                </div>
              )}
            </div>
          ))}
        </div>
        {login?.user_code && (
          <div className="space-y-2 py-2" role="status">
            <p className="text-sm">
              Enter this code to connect your ChatGPT account:
            </p>
            <p className="select-all font-mono text-lg">{login.user_code}</p>
            <a
              className="text-sm underline"
              href={login.verification_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              Open ChatGPT device login
            </a>
            <p className="text-xs text-muted-foreground">
              Waiting for approval. The code expires at{" "}
              {new Date(login.expires_at!).toLocaleTimeString()}.
            </p>
          </div>
        )}
        {login?.connection.status === "connected" && (
          <p role="status" className="text-sm">
            Connection saved. New runs use the workspace’s current access
            policy.
          </p>
        )}
        <form
          className="space-y-3 border-t border-border pt-4"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <p className="text-sm font-medium">
            {reconnecting ? "Reconnect" : "Add connection"}
          </p>
          <Label htmlFor={`${id}-name`}>Name</Label>
          <QuietUnderlineInput
            id={`${id}-name`}
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={100}
            required
            disabled={busy || !!reconnecting}
          />
          <Label htmlFor={`${id}-provider`}>Provider</Label>
          <Select
            value={provider}
            disabled={busy || !!reconnecting}
            onValueChange={(next) => {
              setProvider(next);
              setApiKey("");
            }}
          >
            <SelectTrigger
              aria-label="Provider"
              id={`${id}-provider`}
              variant="underline"
              className="w-full px-0.5"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {Object.entries(providerLabels)
                .filter(
                  ([key]) =>
                    (scope === "personal" || key !== "openai_chatgpt") &&
                    (key === "openai_compatible" ||
                      models.some((m) => m.provider === key)),
                )
                .map(([key, label]) => (
                  <SelectItem key={key} value={key}>
                    {label}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
          {provider === "openai_compatible" && !reconnecting && (
            <div className="space-y-2">
              <Label htmlFor={`${id}-endpoint`}>Approved endpoint</Label>
              <Select
                value={endpointId}
                onValueChange={(value) => {
                  setEndpointId(value);
                  setApiKey("");
                }}
                disabled={busy || endpoints.isPending || endpoints.isError}
              >
                <SelectTrigger
                  id={`${id}-endpoint`}
                  variant="underline"
                  className="w-full px-0.5"
                >
                  <SelectValue placeholder="Choose an endpoint" />
                </SelectTrigger>
                <SelectContent>
                  {endpoints.data?.map((e) => (
                    <SelectItem key={e.id} value={e.id}>
                      {e.id} ·{" "}
                      {e.auth_mode === "none" ? "No authentication" : "API key"}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {endpoints.isError ? (
                <QuietTextAction
                  type="button"
                  onClick={() => void endpoints.refetch()}
                >
                  Retry loading endpoints
                </QuietTextAction>
              ) : !endpoints.isPending && !endpoints.data?.length ? (
                <p className="text-xs text-quiet-text-secondary">
                  Your administrator must approve an endpoint before you can
                  connect.
                </p>
              ) : null}
            </div>
          )}
          {provider === "openai_compatible" && endpoint && (
            <p className="break-all text-xs text-quiet-text-secondary">
              {endpoint.base_url}
              {endpoint.auth_mode === "none" ? " · No API key is sent." : ""}
            </p>
          )}
          {needsKey && (
            <>
              <Label htmlFor={`${id}-key`}>API key</Label>
              <QuietUnderlineInput
                id={`${id}-key`}
                type="password"
                autoComplete="off"
                value={apiKey}
                onChange={(event) => setApiKey(event.target.value)}
                required
                disabled={busy}
              />
            </>
          )}
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex gap-3">
            <QuietPrimaryAction
              type="submit"
              disabled={
                busy ||
                !name.trim() ||
                (needsKey && !apiKey.trim()) ||
                (provider === "openai_compatible" && !endpoint)
              }
            >
              {busy
                ? "Connecting…"
                : provider === "openai_chatgpt"
                  ? "Start device login"
                  : "Save connection"}
            </QuietPrimaryAction>
            {reconnecting && (
              <QuietTextAction
                type="button"
                onClick={() => {
                  setReconnecting(undefined);
                  setName("");
                  setApiKey("");
                }}
              >
                Cancel
              </QuietTextAction>
            )}
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
