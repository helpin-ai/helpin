import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  PROVIDER_ORDER,
  providerMeta,
  type AIProviderKey,
} from "@/lib/aiProviders";
import type {
  AIConnection,
  AIConnectionModel,
} from "@/lib/services/aiConnectionService";
import { ProviderIcon } from "./ProviderIcon";
import { AIConnectionStatusBadge } from "./AIConnectionStatusBadge";
import { ArrowLeft01Icon, PlusSignIcon } from "@/lib/icons";

export function AIAddModelsDialog({
  connections,
  models,
  canManageWorkspace,
  onClose,
  onModels,
  onConnect,
  onReconnect,
}: {
  connections: AIConnection[];
  models: AIConnectionModel[];
  canManageWorkspace: boolean;
  onClose: () => void;
  onModels: (connection: AIConnection) => void;
  onConnect: (provider: AIProviderKey) => void;
  onReconnect: (connection: AIConnection) => void;
}) {
  const [provider, setProvider] = useState<AIProviderKey | null>(null);
  const available = PROVIDER_ORDER.filter(
    (key) =>
      key === "openai_compatible" ||
      models.some((m) => m.provider === key) ||
      connections.some((c) => c.provider === key),
  );
  const own = connections.filter((c) => c.provider === provider);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader className="text-left">
          <DialogTitle>
            {provider ? providerMeta[provider].shortLabel : "Add models"}
          </DialogTitle>
          <DialogDescription>
            {provider
              ? "Choose a connection to select models."
              : "Choose a provider."}
          </DialogDescription>
        </DialogHeader>
        {provider ? (
          <>
            <Button
              variant="ghost"
              size="sm"
              className="w-fit"
              onClick={() => setProvider(null)}
            >
              <ArrowLeft01Icon className="size-4" />
              Providers
            </Button>
            <div className="divide-y">
              {own.map((connection) => {
                const editable =
                  connection.scope === "personal" || canManageWorkspace;
                return (
                  <div
                    key={connection.id}
                    className="flex flex-wrap items-center gap-3 py-3"
                  >
                    <ProviderIcon
                      provider={connection.provider}
                      className="size-5 shrink-0"
                    />
                    <div className="min-w-[100px] flex-1">
                      <p className="truncate text-sm font-medium">
                        {connection.name}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {connection.scope === "personal"
                          ? "Personal"
                          : "Workspace-wide"}
                      </p>
                    </div>
                    <AIConnectionStatusBadge status={connection.status} />
                    <Button
                      size="sm"
                      variant="outline"
                      aria-label={`Models for ${connection.name}`}
                      onClick={() => onModels(connection)}
                    >
                      {editable ? "Models" : "View models"}
                    </Button>
                    {editable &&
                      connection.status !== "connected" &&
                      connection.funding !== "managed" && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => onReconnect(connection)}
                        >
                          {connection.status === "pending"
                            ? "Continue login"
                            : "Reconnect"}
                        </Button>
                      )}
                  </div>
                );
              })}
            </div>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => onConnect(provider)}
            >
              <PlusSignIcon className="size-4" />
              {own.length ? "Add another connection" : "Connect provider"}
            </Button>
          </>
        ) : (
          <div className="divide-y">
            {available.map((key) => {
              const connected = connections.some(
                (c) => c.provider === key && c.status === "connected",
              );
              return (
                <button
                  key={key}
                  type="button"
                  aria-label={`Choose ${providerMeta[key].shortLabel}`}
                  onClick={() =>
                    connections.some((c) => c.provider === key)
                      ? setProvider(key)
                      : onConnect(key)
                  }
                  className="flex w-full items-center gap-3 rounded-md px-2 py-3 text-left hover:bg-quiet-hover focus-visible:outline-2 focus-visible:outline-ring"
                >
                  <ProviderIcon provider={key} className="size-5 shrink-0" />
                  <span className="flex-1 text-sm font-medium">
                    {key === "openai_compatible"
                      ? "Custom endpoint"
                      : providerMeta[key].shortLabel}
                  </span>
                  {connected && (
                    <span className="text-xs text-quiet-positive">
                      Connected
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
