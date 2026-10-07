import { useEffect, useId, useState } from "react";
import { helpinClient } from "@/lib/helpin";
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
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { InformationCircleIcon } from "@/lib/icons";
import { Skeleton } from "@/components/ui/skeleton";

function ConnectionHelp({ label, description }: { label: string; description: string }) {
  return <QuickTooltip label={description}><button type="button" aria-label={label} className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"><InformationCircleIcon className="size-3.5" /></button></QuickTooltip>;
}

export type AIConnectionDialogMode =
  | { kind: "add"; provider?: AIProviderKey }
  | { kind: "reconnect"; connection: AIConnection; autoPoll?: boolean };

export function AIConnectionDialog({
  workspaceId,
  scope: initialScope,
  canChooseScope = false,
  open,
  mode,
  models,
  onOpenChange,
  onConnected,
}: {
  workspaceId: string;
  scope: "personal" | "workspace";
  canChooseScope?: boolean;
  open: boolean;
  mode: AIConnectionDialogMode;
  models: AIConnectionModel[];
  onOpenChange: (open: boolean) => void;
  onConnected?: (connection: AIConnection) => void;
}) {
  const id = useId();
  const [scope, setScope] = useState(initialScope);
  const chosenProvider = mode.kind === "add" ? mode.provider : undefined;
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
    setScope(initialScope);
    setLogin(undefined);
    setApiKey("");
    if (reconnecting) {
      setName(reconnecting.name);
      setProvider(reconnecting.provider);
      setEndpointId(reconnecting.endpoint?.id ?? "");
      return;
    }
    const first = chosenProvider ?? availableProviders[0] ?? "openai";
    setProvider(first);
    setName(providerMeta[first as AIProviderKey]?.shortLabel ?? "");
    setEndpointId("");
    // availableProviders is derived from props that are stable while open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, reconnecting?.id, chosenProvider, initialScope]);

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
          ? "Choose an endpoint"
          : "No compatible endpoints are available. Contact Helpin support to request one.";
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
      onConnected?.(result.connection);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Could not save the connection.");
    }
  }

  const pending = create.isPending || reconnect.isPending;
  const showDeviceLogin = Boolean(login && (login.user_code || isChatGPT));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-lg" onOpenAutoFocus={(event) => {
        const firstField = document.getElementById(`${id}-${reconnecting ? 'key' : 'provider'}`) ?? document.getElementById(`${id}-cancel`);
        if (firstField) { event.preventDefault(); firstField.focus(); }
      }}>
        <DialogHeader className="space-y-1.5 border-b border-border/60 px-6 py-4 text-left">
          <DialogTitle>
            {reconnecting ? `Reconnect “${reconnecting.name}”` : "Add connection"}
          </DialogTitle>
          <div className="flex items-center gap-1">
            <DialogDescription>
              {scope === "personal" ? "Personal connection · Only you" : "Workspace connection · Shared with members"}
            </DialogDescription>
            <ConnectionHelp label={reconnecting ? "About reconnecting" : "Who can use this connection"} description={reconnecting
              ? "Replace the stored credential. Runs already accepted keep their route."
              : scope === "personal"
                ? "Only you can use this connection, for your manual runs and chats."
                : "Members can select this connection. Shared models can be used for automation."} />
          </div>
        </DialogHeader>

        <div className="min-h-0 max-h-[calc(88vh-8.5rem)] space-y-4 overflow-y-auto px-6 py-5">
          {reconnecting && (
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <ProviderIcon provider={reconnecting.provider} className="h-3.5 w-3.5" />
              {providerLabel(reconnecting.provider)}
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
              {!reconnecting && !chosenProvider && (
                <div className="space-y-1.5">
                  <div className="flex items-center gap-1">
                    <Label htmlFor={`${id}-provider`}>Provider</Label>
                    <ConnectionHelp label="About this provider" description={providerMeta[provider as AIProviderKey]?.description ?? providerLabel(provider)} />
                  </div>
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
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}

              {chosenProvider && <p className="flex items-center gap-2 text-sm"><ProviderIcon provider={provider} className="size-4" />{providerLabel(provider)}</p>}
              {!reconnecting && canChooseScope && providerMeta[provider as AIProviderKey]?.scopes.includes("workspace") && (
                <div className="space-y-1.5">
                  <div className="flex items-center gap-1"><Label htmlFor={`${id}-access`}>Access</Label><ConnectionHelp label="About connection access" description="Personal models are available only to you. Workspace-wide models are shared with workspace members. Access is fixed when the connection is created." /></div>
                  <Select value={scope} onValueChange={(value) => setScope(value as "personal" | "workspace")} disabled={pending}>
                    <SelectTrigger id={`${id}-access`} aria-label="Access" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
                    <SelectContent><SelectItem value="personal">Personal</SelectItem><SelectItem value="workspace">Workspace-wide</SelectItem></SelectContent>
                  </Select>
                </div>
              )}
              {!reconnecting && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-name`}>Connection name</Label>
                  <QuietUnderlineInput
                    id={`${id}-name`}
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    maxLength={100}
                    placeholder={scope === "personal" ? "My OpenAI connection" : "Team OpenAI connection"}
                    disabled={pending}
                  />
                </div>
              )}

              {provider === "openai_compatible" && !reconnecting && (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-endpoint`}>Endpoint</Label>
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
                            <span className="min-w-0 flex-1">
                            <span className="block">{entry.id}</span>
                            <span className="block truncate text-xs text-muted-foreground">
                              {entry.base_url} ·{" "}
                              {entry.auth_mode === "none" ? "No authentication" : "API key"}
                            </span>
                            </span>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                  {endpoints.isError ? (
                    <p className="flex items-center gap-2 text-xs text-destructive">
                      Could not load endpoints.
                      <button
                        type="button"
                        className="underline"
                        onClick={() => void endpoints.refetch()}
                      >
                        Retry
                      </button>
                    </p>
                  ) : !endpoints.isPending && !endpoints.data?.length ? (
                    <div className="space-y-1 text-xs">
                      <p className="text-muted-foreground">No compatible endpoints are available.</p>
                      <button type="button" className="underline underline-offset-4" onClick={() => {
                        onOpenChange(false);
                        helpinClient?.show();
                        helpinClient?.open();
                      }}>Contact Helpin support to request one</button>
                    </div>
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
                  <div className="flex items-center gap-1">
                    <Label htmlFor={`${id}-key`}>API key</Label>
                    <ConnectionHelp label="About API key storage" description={reconnecting ? "Paste a new key to replace the stored credential. The key is stored encrypted and never returned by the API." : "Stored encrypted. It is never returned by the API."} />
                  </div>
                  <QuietUnderlineInput
                    id={`${id}-key`}
                    type="password"
                    autoComplete="off"
                    value={apiKey}
                    onChange={(event) => setApiKey(event.target.value)}
                    disabled={pending}
                  />

                </div>
              )}

              {isChatGPT && (
                <p className="text-xs text-muted-foreground">
                  You will sign in to ChatGPT with a one-time code.
                </p>
              )}

              {reconnecting?.policy && <AIConnectionPolicyNotice policy={reconnecting.policy} />}
              <button type="submit" className="sr-only">
                {isChatGPT ? "Continue to ChatGPT" : "Add connection"}
              </button>
            </form>
          )}
        </div>

        <DialogFooter className="border-t border-border/60 px-6 py-3">
          <Button id={`${id}-cancel`} type="button" size="sm" variant="ghost" onClick={() => { onOpenChange(false); if (device.phase === "connected" && login) onConnected?.(login.connection); }}>
            {device.phase === "connected" ? onConnected ? "Choose models" : "Done" : "Cancel"}
          </Button>
          {!showDeviceLogin && (
            <Button type="button" size="sm" disabled={pending} onClick={() => void submit()}>
              {pending
                ? "Connecting…"
                : isChatGPT
                  ? "Continue to ChatGPT"
                  : reconnecting
                    ? "Save changes"
                    : "Add connection"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
