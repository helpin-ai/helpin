import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  PROVIDER_ORDER,
  providerMeta,
  providerShortLabel,
  chatGPTAccountLabel,
  type AIProviderKey,
} from "@/lib/aiProviders";
import type {
  AIConnection,
  AIConnectionModel,
} from "@/lib/services/aiConnectionService";
import { ProviderIcon } from "./ProviderIcon";
import { AIConnectionStatusBadge } from "./AIConnectionStatusBadge";
import { ArrowLeft01Icon, MoreHorizontalIcon, PlusSignIcon } from "@/lib/icons";

export function AIAddModelsDialog({
  connections,
  models,
  canManageWorkspace,
  startWithProviders,
  onClose,
  onModels,
  onConnect,
  onReconnect,
  onDisconnect,
  onManage,
}: {
  connections: AIConnection[];
  models: AIConnectionModel[];
  canManageWorkspace: boolean;
  startWithProviders: boolean;
  onManage: () => void;
  onClose: () => void;
  onModels: (connection: AIConnection) => void;
  onConnect: (provider: AIProviderKey) => void;
  onReconnect: (connection: AIConnection) => void;
  onDisconnect: (connection: AIConnection) => void;
}) {
  const [chooseProvider, setChooseProvider] = useState(
    startWithProviders || !connections.length,
  );
  const providers = PROVIDER_ORDER.filter(
    (key) =>
      key === "openai_compatible" ||
      models.some((m) => m.provider === key) ||
      connections.some((c) => c.provider === key),
  );

  function connectionRow(connection: AIConnection) {
    const managed = connection.funding === "managed";
    const editable = connection.scope === "personal" || canManageWorkspace;
    const repair = editable && !managed && connection.status !== "connected";
    const action = repair
      ? connection.status === "pending"
        ? "Continue login"
        : "Reconnect"
      : editable && connection.status === "connected"
        ? "Choose models"
        : "View models";
    const name =
      managed && connection.name === connection.provider + " (managed)"
        ? providerShortLabel(connection.provider)
        : connection.name;
    const detail = chatGPTAccountLabel(connection);
    return (
      <div
        key={connection.id}
        className="flex items-center border-b last:border-b-0"
      >
        <button
          type="button"
          aria-label={repair ? action + " for " + name : "Models for " + name}
          className="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1 rounded-md px-2 py-3 text-left hover:bg-muted focus-visible:outline-ring sm:grid-cols-[minmax(0,1fr)_9rem_7rem]"
          onClick={() =>
            repair ? onReconnect(connection) : onModels(connection)
          }
        >
          <span className="flex min-w-0 items-center gap-2">
            <ProviderIcon
              provider={connection.provider}
              className="size-5 shrink-0"
            />
            <span className="min-w-0">
              <span className="block truncate text-sm font-medium" title={name}>
                {name}
              </span>
              {detail && (
                <span className="block text-xs text-muted-foreground">
                  {detail}
                </span>
              )}
            </span>
          </span>
          <AIConnectionStatusBadge
            status={connection.status}
            verificationError={connection.last_verification_error}
            className="col-start-1 row-start-2 justify-self-start sm:col-auto sm:row-auto"
          />
          <span className="col-start-2 row-start-1 whitespace-nowrap text-right text-sm sm:col-auto sm:row-auto">
            {action} ›
          </span>
        </button>
        <div className="w-8 shrink-0">
          {editable && !managed && connection.status !== "disconnected" && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  aria-label={`Connection actions for ${name}`}
                >
                  <MoreHorizontalIcon className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={() => onDisconnect(connection)}
                >
                  Disconnect
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>
    );
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        aria-describedby={undefined}
        className="max-h-[85vh] overflow-y-auto sm:max-w-xl"
      >
        <DialogHeader className="text-left">
          <DialogTitle>
            {chooseProvider ? "Connect a provider" : "Add models"}
          </DialogTitle>
        </DialogHeader>
        {chooseProvider ? (
          <>
            {!!connections.length && (
              <Button
                size="sm"
                variant="ghost"
                className="w-fit"
                onClick={() => setChooseProvider(false)}
              >
                <ArrowLeft01Icon className="size-4" />
                Connections
              </Button>
            )}
            <div className="divide-y">
              {providers.map((key) => (
                <button
                  type="button"
                  key={key}
                  className="flex w-full items-center gap-3 rounded-md px-2 py-3 text-left hover:bg-muted"
                  onClick={() => onConnect(key)}
                >
                  <ProviderIcon provider={key} className="size-5" />
                  {key === "openai_compatible"
                    ? "Custom endpoint"
                    : providerMeta[key].label}
                </button>
              ))}
            </div>
          </>
        ) : (
          <>
            {[true, false].map((managed) => {
              const group = connections.filter(
                (c) => (c.funding === "managed") === managed,
              );
              return (
                group.length > 0 && (
                  <section key={String(managed)}>
                    <h3 className="bg-muted/50 px-2 py-1.5 text-xs font-medium text-muted-foreground">
                      {managed ? "Helpin-managed" : "Your connections"}
                    </h3>
                    {group.map(connectionRow)}
                  </section>
                )
              );
            })}
          </>
        )}
        {(!chooseProvider || !connections.length) && (
          <div className="flex flex-wrap items-center justify-between gap-2">
            {!chooseProvider && (
              <Button variant="outline" size="sm" onClick={() => setChooseProvider(true)}>
                <PlusSignIcon className="size-4" />
                Connect a provider
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={onManage}>Manage connections</Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
