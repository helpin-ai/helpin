import { AI_MODELS } from "@/generated/aiModels";
import type { AIConnection } from "@/lib/services/aiConnectionService";

export type AIProviderKey =
  | "openai"
  | "anthropic"
  | "openrouter"
  | "openai_chatgpt"
  | "openai_compatible";

export type AIProviderControls = {
  reasoningEffort: boolean;
  serviceTier: boolean;
  openrouterQuantizations: boolean;
};

export type AIProviderMeta = {
  key: AIProviderKey;
  /** Full name used in pickers and dialogs. */
  label: string;
  /** Compact name used in dense rows and summaries. */
  shortLabel: string;
  description: string;
  needsApiKey: boolean;
  supportsControls: AIProviderControls;
  scopes: Array<"personal" | "workspace">;
};

const noControls: AIProviderControls = {
  reasoningEffort: false,
  serviceTier: false,
  openrouterQuantizations: false,
};

export const providerMeta: Record<AIProviderKey, AIProviderMeta> = {
  openai: {
    key: "openai",
    label: "OpenAI API key",
    shortLabel: "OpenAI",
    description: "Use an OpenAI platform key billed to your OpenAI account.",
    needsApiKey: true,
    supportsControls: { reasoningEffort: true, serviceTier: true, openrouterQuantizations: false },
    scopes: ["personal", "workspace"],
  },
  anthropic: {
    key: "anthropic",
    label: "Anthropic API key",
    shortLabel: "Anthropic",
    description: "Use an Anthropic console key billed to your Anthropic account.",
    needsApiKey: true,
    supportsControls: noControls,
    scopes: ["personal", "workspace"],
  },
  openrouter: {
    key: "openrouter",
    label: "OpenRouter API key",
    shortLabel: "OpenRouter",
    description: "Reach many providers through one OpenRouter key.",
    needsApiKey: true,
    supportsControls: { reasoningEffort: true, serviceTier: false, openrouterQuantizations: true },
    scopes: ["personal", "workspace"],
  },
  openai_chatgpt: {
    key: "openai_chatgpt",
    label: "ChatGPT subscription",
    shortLabel: "ChatGPT",
    description: "Sign in with your ChatGPT plan. Available for your own manual runs.",
    needsApiKey: false,
    supportsControls: { reasoningEffort: true, serviceTier: true, openrouterQuantizations: false },
    scopes: ["personal"],
  },
  openai_compatible: {
    key: "openai_compatible",
    label: "Compatible endpoint",
    shortLabel: "Compatible",
    description: "Use a local or self-hosted server your administrator approved.",
    needsApiKey: true,
    supportsControls: noControls,
    scopes: ["personal", "workspace"],
  },
};

export const PROVIDER_ORDER: AIProviderKey[] = [
  "openai",
  "anthropic",
  "openrouter",
  "openai_chatgpt",
  "openai_compatible",
];

export function isAIProviderKey(provider: string): provider is AIProviderKey {
  return provider in providerMeta;
}

export function providerInfo(provider: string): AIProviderMeta | undefined {
  return isAIProviderKey(provider) ? providerMeta[provider] : undefined;
}

/** Full provider name, falling back to the raw identifier for unknown providers. */
export function providerLabel(provider: string): string {
  return providerInfo(provider)?.label ?? provider;
}

/** Compact provider name, falling back to the raw identifier. */
export function providerShortLabel(provider: string): string {
  return providerInfo(provider)?.shortLabel ?? provider;
}

export function providerControls(provider: string): AIProviderControls {
  return providerInfo(provider)?.supportsControls ?? noControls;
}

export type ConnectionStatusTone = "positive" | "attention" | "neutral";

export const connectionStatusMeta: Record<
  AIConnection["status"],
  { label: string; tone: ConnectionStatusTone }
> = {
  connected: { label: "Connected", tone: "positive" },
  pending: { label: "Awaiting login", tone: "attention" },
  reauthorization_required: { label: "Reconnect required", tone: "attention" },
  disconnected: { label: "Disconnected", tone: "neutral" },
};

export function connectionStatusInfo(status: string) {
  return (
    connectionStatusMeta[status as AIConnection["status"]] ?? {
      label: status.replaceAll("_", " "),
      tone: "neutral" as ConnectionStatusTone,
    }
  );
}

export type AIModelSuggestionGroup = {
  key: string;
  label: string;
  description: string;
  models: Array<{ selectionModel: string; canonicalModel: string; label: string }>;
};

/**
 * Suggested models for a provider, grouped in catalog tier order. ChatGPT runs on
 * the OpenAI catalog; compatible endpoints have no catalog and rely on free text.
 */
export function modelCatalogFor(provider: string): AIModelSuggestionGroup[] {
  const catalogProvider = provider === "openai_chatgpt" ? "openai" : provider;
  if (catalogProvider === "openai_compatible") return [];
  return AI_MODELS.tiers
    .map((tier) => ({
      key: tier.key,
      label: tier.label,
      description: tier.description,
      models: AI_MODELS.models
        .filter((model) => model.enabled && model.provider === catalogProvider && model.tier === tier.key)
        .map((model) => ({
          selectionModel: model.selection_model,
          canonicalModel: model.canonical_model,
          label: model.label,
        })),
    }))
    .filter((group) => group.models.length > 0);
}

/** Catalog display name for a model identifier, when the catalog knows it. */
export function catalogLabel(provider: string, model: string): string | undefined {
  const catalogProvider = provider === "openai_chatgpt" ? "openai" : provider;
  return AI_MODELS.models.find(
    (entry) => entry.provider === catalogProvider && entry.selection_model === model,
  )?.label;
}

export function catalogTier(provider: string, model: string) {
  const catalogProvider = provider === "openai_chatgpt" ? "openai" : provider;
  const entry = AI_MODELS.models.find(
    (item) => item.provider === catalogProvider && item.selection_model === model,
  );
  if (!entry) return undefined;
  return AI_MODELS.tiers.find((tier) => tier.key === entry.tier);
}
