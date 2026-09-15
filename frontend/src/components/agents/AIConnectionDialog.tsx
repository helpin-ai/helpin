import { useEffect, useId, useState } from "react";
import { toast } from "sonner";
import { AIConnectionPolicyNotice } from "./AIConnectionPolicyNotice";
import { ChatGPTDeviceLogin, useChatGPTDeviceLogin } from "./ChatGPTDeviceLogin";
import { ProviderIcon } from "./ProviderIcon";
import {
  useAIModelEndpoints,
  useCreateAIConnection,
  useReconnectAIConnection,
} from "@/hooks/queries/useAIConnections";
import {
  PROVIDER_ORDER,
  providerLabel,
  providerMeta,
  type AIProviderKey,
} from "@/lib/aiProviders";
import type {
  AIConnection,
  AIConnectionLogin,
  AIConnectionModel,
} from "@/lib/services/aiConnectionService";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { QuietUnderlineInput } from "@/components/design-system/quiet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";

export type AIConnectionDialogMode =
  | { kind: "add" }
  | { kind: "reconnect"; connection: AIConnection; autoPoll?: boolean };

export function AIConnectionDialog({
  workspaceId,
  scope,
  open,
  mode,
  models,
  onOpenChange,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  open: boolean;
  mode: AIConnectionDialogMode;
  models: AIConnectionModel[];
  onOpenChange: (open: boolean) => void;
}) {
  const id = useId();
  const reconnecting = mode.kind === "reconnect" ? mode.connection : undefined;
  const [name, setName] = useState("");
  const [provider, setProvider] = useState<string>("openai");
  const [apiKey, setApiKey] = useState("");
  const [endpointId, setEndpointId] = useState("");
  const [login, setLogin] = useState<AIConnectionLogin>();

  const create = useCreateAIConnection(workspaceId);
  const reconnect = useReconnectAIConnection(workspaceId);
  const endpoints = useAIModelEndpoints(
    workspaceId,
    open && provider === "openai_compatible" && !reconnecting,
  );

  const availableProviders = PROVIDER_ORDER.filter(
    (key) =>
      providerMeta[key].scopes.includes(scope) &&
      (key === "openai_compatible" || models.some((model) => model.provider === key)),
  );

  // Reset every field when the dialog opens so a previous key never lingers.
  useEffect(() => {
    if (!open) return;
    setLogin(undefined);
    setApiKey("");
    if (reconnecting) {
      setName(reconnecting.name);
      setProvider(reconnecting.provider);
      setEndpointId(reconnecting.endpoint?.id ?? "");
      return;
    }
    const first = availableProviders[0] ?? "openai";
    setProvider(first);
    setName(providerMeta[first as AIProviderKey]?.shortLabel ?? "");
    setEndpointId("");
    // availableProviders is derived from props that are stable while open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, reconnecting?.id]);

  const endpoint = reconnecting
    ? reconnecting.endpoint
    : endpoints.data?.find((entry) => entry.id === endpointId);
  const isChatGPT = provider === "openai_chatgpt";
  const needsKey =
    !isChatGPT && !(provider === "openai_compatible" && endpoint?.auth_mode === "none");

  const device = useChatGPTDeviceLogin({
    workspaceId,
    login,
    onLogin: setLogin,
  });

  async function startDeviceLoginAgain() {
    const target = login?.connection.id ?? reconnecting?.id;
    if (!target) return;
    try {
      setLogin(await reconnect.mutateAsync({ id: target }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not restart the login.");
    }
  }

  function validate(): string | null {
    if (!reconnecting) {
      if (!provider) return "Choose a provider";
      if (!name.trim()) return "Name is required";
      if (provider === "openai_compatible" && !endpoint)
        return endpoints.data?.length
          ? "Choose an approved endpoint"
          : "Your administrator must approve an endpoint before you can connect.";
    }
    if (needsKey && !apiKey.trim()) return "API key is required";
    return null;
  }

  async function submit() {
    const problem = validate();
    if (problem) {
      toast.error(problem);
      return;
    }
    try {
      const result = reconnecting
        ? await reconnect.mutateAsync({ id: reconnecting.id, apiKey: apiKey || undefined })
        : await create.mutateAsync({
            name: name.trim(),
            scope,
            provider,
            api_key: needsKey ? apiKey || undefined : undefined,
            endpoint_id: provider === "openai_compatible" ? endpointId : undefined,
          });
      setApiKey("");
      if (result.user_code || result.connection.status === "pending") {
        setLogin(result);
        return;
      }
      toast.success(reconnecting ? "Connection updated" : "Connection added");
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not save the connection.");
    }
  }

  const pending = create.isPending || reconnect.isPending;
  const showDeviceLogin = Boolean(login && (login.user_code || isChatGPT));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-lg">
        <DialogHeader className="space-y-1.5 border-b border-border/60 px-6 py-4 text-left">
          <DialogTitle>
            {reconnecting ? `Reconnect “${reconnecting.name}”` : "Add connection"}
          </DialogTitle>
          <DialogDescription>
            {reconnecting
              ? "Replace the stored credential. Runs already accepted keep their route."
              : scope === "personal"
                ? "Only you can use this connection, for your manual runs and chats."
                : "Members can select this connection. Shared profiles can use it for automation."}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-4 overflow-y-auto px-6 py-5">
          {reconnecting && (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <ProviderIcon provider={reconnecting.provider} className="h-3.5 w-3.5" />
              {providerLabel(reconnecting.provider)} ·{" "}
              {reconnecting.scope === "personal" ? "Personal" : "Workspace"}
            </p>
          )}

          {showDeviceLogin && login ? (
            <ChatGPTDeviceLogin
              login={login}
              phase={device.phase}
              failure={device.failure}
              countdown={device.countdown}
              isChecking={device.isChecking}
              onCheckNow={() => void device.checkNow()}
              onStartAgain={() => void startDeviceLoginAgain()}
            />
          ) : (
            <form
              className="space-y-4"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              {!reconnecting && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-provider`}>Provider</Label>
                  <Select
                    value={provider}
                    disabled={pending}
                    onValueChange={(next) => {
                      setProvider(next);
                      setApiKey("");
                      setEndpointId("");
                      const previous = providerMeta[provider as AIProviderKey]?.shortLabel;
                      const nextMeta = providerMeta[next as AIProviderKey];
                      if (nextMeta && (!name.trim() || name === previous))
                        setName(nextMeta.shortLabel);
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
                      {availableProviders.map((key) => (
                        <SelectItem key={key} value={key} textValue={providerMeta[key].label}>
                          <span className="flex items-center gap-2">
                            <ProviderIcon provider={key} className="h-3.5 w-3.5 shrink-0" />
                            <span>{providerMeta[key].label}</span>
                          </span>
                          <span className="block text-[11px] text-muted-foreground">
                            {providerMeta[key].description}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}

              {!reconnecting && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-name`}>Name</Label>
                  <QuietUnderlineInput
                    id={`${id}-name`}
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    maxLength={100}
                    placeholder="Team OpenAI key"
                    disabled={pending}
                  />
                </div>
              )}

              {provider === "openai_compatible" && !reconnecting && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-endpoint`}>Approved endpoint</Label>
                  {endpoints.isPending ? (
                    <Skeleton className="h-8 w-full" />
                  ) : (
                    <Select
                      value={endpointId}
                      onValueChange={(value) => {
                        setEndpointId(value);
                        setApiKey("");
                      }}
                      disabled={pending || endpoints.isError || !endpoints.data?.length}
                    >
                      <SelectTrigger
                        id={`${id}-endpoint`}
                        variant="underline"
                        className="w-full px-0.5"
                      >
                        <SelectValue placeholder="Choose an endpoint" />
                      </SelectTrigger>
                      <SelectContent>
                        {endpoints.data?.map((entry) => (
                          <SelectItem key={entry.id} value={entry.id} textValue={entry.id}>
                            <span>{entry.id}</span>
                            <span className="block text-[11px] text-muted-foreground">
                              {entry.base_url} ·{" "}
                              {entry.auth_mode === "none" ? "No authentication" : "API key"}
                            </span>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                  {endpoints.isError ? (
                    <p className="flex items-center gap-2 text-xs text-destructive">
                      Could not load approved endpoints.
                      <button
                        type="button"
                        className="underline"
                        onClick={() => void endpoints.refetch()}
                      >
                        Retry
                      </button>
                    </p>
                  ) : !endpoints.isPending && !endpoints.data?.length ? (
                    <p className="text-xs text-muted-foreground">
                      Your administrator must approve an endpoint before you can connect.
                    </p>
                  ) : null}
                </div>
              )}

              {provider === "openai_compatible" && endpoint && (
                <p className="break-all text-xs text-muted-foreground">
                  {endpoint.base_url}
                  {endpoint.auth_mode === "none" ? " · No API key is sent." : ""}
                </p>
              )}

              {needsKey && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-key`}>API key</Label>
                  <QuietUnderlineInput
                    id={`${id}-key`}
                    type="password"
                    autoComplete="off"
                    value={apiKey}
                    onChange={(event) => setApiKey(event.target.value)}
                    disabled={pending}
                  />
                  <p className="text-xs text-muted-foreground">
                    {reconnecting
                      ? "Paste a new key. The stored key is replaced."
                      : "Stored encrypted. It is never returned by the API."}
                  </p>
                </div>
              )}

              {isChatGPT && (
                <p className="text-xs text-muted-foreground">
                  You will sign in to ChatGPT with a one-time code.
                </p>
              )}

              {reconnecting?.policy && <AIConnectionPolicyNotice policy={reconnecting.policy} />}
              <button type="submit" className="sr-only">
                {isChatGPT ? "Start device login" : "Save connection"}
              </button>
            </form>
          )}
        </div>

        <DialogFooter className="border-t border-border/60 px-6 py-3">
          <Button type="button" size="sm" variant="outline" onClick={() => onOpenChange(false)}>
            {device.phase === "connected" ? "Done" : "Cancel"}
          </Button>
          {!showDeviceLogin && (
            <Button type="button" size="sm" disabled={pending} onClick={() => void submit()}>
              {pending
                ? "Connecting…"
                : isChatGPT
                  ? "Start device login"
                  : reconnecting
                    ? "Save changes"
                    : "Save connection"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
