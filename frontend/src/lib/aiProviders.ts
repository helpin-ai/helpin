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
    description: "Sign in with your ChatGPT plan. Choose which models to share with workspace agents.",
    needsApiKey: false,
    supportsControls: { reasoningEffort: true, serviceTier: true, openrouterQuantizations: false },
    scopes: ["personal"],
  },
  openai_compatible: {
    key: "openai_compatible",
    label: "Compatible endpoint",
    shortLabel: "Compatible",
    description: "Connect to a compatible model endpoint available in Helpin.",
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
  unconfigured: { label: "Not configured", tone: "neutral" },
  pending: { label: "Awaiting login", tone: "attention" },
  reauthorization_required: { label: "Reconnect required", tone: "attention" },
  disconnected: { label: "Disconnected", tone: "neutral" },
};

export function chatGPTAccountLabel(connection: AIConnection) {
  if (connection.provider !== "openai_chatgpt") return undefined;
  return connection.account_id ? `ChatGPT account ···· ${connection.account_id.slice(-8)}` : "ChatGPT subscription";
}

export function connectionStatusInfo(status: string, verificationError?: string) {
  if (status === "connected" && verificationError) return { label: "Last test failed", tone: "attention" as const };
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

const GPT_6_2_ASTRA = { selectionModel: "gpt-6.2-astra", canonicalModel: "gpt-6.2-astra", label: "GPT-6.2 Astra" };
const GPT_6_2_SOL = { selectionModel: "gpt-6.2-sol", canonicalModel: "gpt-6.2-sol", label: "GPT-6.2 Sol" };

const CHATGPT_MODELS = [
  GPT_6_2_ASTRA,
  GPT_6_2_SOL,
  { selectionModel: "gpt-6.1-terra", canonicalModel: "gpt-6.1-terra", label: "GPT-6.1 Terra" },
  { selectionModel: "gpt-6-luna", canonicalModel: "gpt-6-luna", label: "GPT-6 Luna" },
];

// Profile suggestions are independent of Helpin-funded routing and pricing.
// Verified 2026-11-20 against the provider catalogs:
// https://developers.openai.com/api/docs/models
// https://platform.claude.com/docs/en/about-claude/models/overview
// https://openrouter.ai/api/v1/models
const LATEST_PROVIDER_MODELS: Record<string, AIModelSuggestionGroup["models"]> = {
  openai: [GPT_6_2_ASTRA, GPT_6_2_SOL],
  anthropic: [
    { selectionModel: "claude-fable-5-2", canonicalModel: "claude-fable-5-2", label: "Claude Fable 5.2" },
    { selectionModel: "claude-opus-5-5", canonicalModel: "claude-opus-5-5", label: "Claude Opus 5.5" },
  ],
  openrouter: [
    { selectionModel: "openai/gpt-6.2-astra", canonicalModel: "gpt-6.2-astra", label: "GPT-6.2 Astra" },
    { selectionModel: "openai/gpt-6.2-sol", canonicalModel: "gpt-6.2-sol", label: "GPT-6.2 Sol" },
    { selectionModel: "anthropic/claude-fable-5.2", canonicalModel: "claude-fable-5-2", label: "Claude Fable 5.2" },
    { selectionModel: "anthropic/claude-opus-5.5", canonicalModel: "claude-opus-5-5", label: "Claude Opus 5.5" },
  ],
};

/**
 * Suggested models for each connection type. ChatGPT suggestions are separate
 * from the API billing catalog; compatible endpoints rely on free text.
 */
export function modelCatalogFor(provider: string): AIModelSuggestionGroup[] {
  if (provider === "openai_chatgpt") return [{ key: "chatgpt", label: "", description: "", models: CHATGPT_MODELS }];
  const catalogProvider = provider === "openai_chatgpt" ? "openai" : provider;
  if (catalogProvider === "openai_compatible") return [];
  const latest = LATEST_PROVIDER_MODELS[provider] ?? [];
  const latestIds = new Set(latest.map(model => model.selectionModel));
  const catalogGroups = AI_MODELS.tiers
    .map((tier) => ({
      key: tier.key,
      label: tier.label,
      description: tier.description,
      models: AI_MODELS.models
        .filter((model) => model.enabled && !('legacy' in model && model.legacy) && model.provider === catalogProvider && model.tier === tier.key && !latestIds.has(model.selection_model))
        .map((model) => ({
          selectionModel: model.selection_model,
          canonicalModel: model.canonical_model,
          label: model.label,
        })),
    }))
    .filter((group) => group.models.length > 0);
  return latest.length ? [{ key: "latest", label: "Latest models", description: "", models: latest }, ...catalogGroups] : catalogGroups;
}

/** Catalog display name for a model identifier, when the catalog knows it. */
export function catalogLabel(provider: string, model: string): string | undefined {
  const latest = LATEST_PROVIDER_MODELS[provider]?.find(entry => entry.selectionModel === model);
  if (latest) return latest.label;
  if (provider === "openai_chatgpt") {
    const suggested = CHATGPT_MODELS.find(entry => entry.selectionModel === model);
    if (suggested) return suggested.label;
  }
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

/** Show an explicitly saved thinking level without inventing a provider default. */
export function modelNameWithThinking(name: string, effort?: string): string {
  if (!effort || effort === "default") return name;
  const label = effort === "xhigh" ? "Extra high" : effort.charAt(0).toUpperCase() + effort.slice(1);
  const suffix = ` - ${label}`;
  return name.toLowerCase().endsWith(suffix.toLowerCase()) ? name : `${name}${suffix}`;
}
