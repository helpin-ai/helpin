import type { AgentRun, AgentRuntimeKind, CodingSession } from './pmTypes';

type TokenUsageShape = {
  runtime_kind: AgentRuntimeKind;
  tokens_used?: number;
  cached_input_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
};

function stripTrailingZero(value: string) {
  return value.endsWith('.0') ? value.slice(0, -2) : value;
}

export function formatCompactTokenCount(value: number) {
  if (value >= 1_000_000) {
    const digits = value >= 10_000_000 ? 0 : 1;
    return `${stripTrailingZero((value / 1_000_000).toFixed(digits))}m`;
  }
  if (value >= 1_000) {
    const digits = value >= 100_000 ? 0 : 1;
    return `${stripTrailingZero((value / 1_000).toFixed(digits))}k`;
  }
  return `${value}`;
}

export function formatAgentTokenUsage(
  usage: TokenUsageShape,
  options?: {
    emptyLabel?: string;
    includeUnit?: boolean;
  },
) {
  const emptyLabel = options?.emptyLabel ?? '-';
  const includeUnit = options?.includeUnit ?? false;
  const cachedInputTokens = usage.cached_input_tokens ?? 0;
  const inputTokens = usage.input_tokens ?? 0;
  const outputTokens = usage.output_tokens ?? 0;
  const totalTokens = usage.tokens_used ?? 0;
  const showsSplitUsage = (usage.runtime_kind === 'native_sdk' || usage.runtime_kind === 'codex')
    && (inputTokens > 0 || outputTokens > 0);

  if (showsSplitUsage) {
    if (cachedInputTokens > 0) {
      return `${formatCompactTokenCount(inputTokens)} input (${formatCompactTokenCount(cachedInputTokens)} cached) / ${formatCompactTokenCount(outputTokens)} output`;
    }
    return `${formatCompactTokenCount(inputTokens)} input / ${formatCompactTokenCount(outputTokens)} output`;
  }
  if (totalTokens > 0) {
    const compact = formatCompactTokenCount(totalTokens);
    return includeUnit ? `${compact} tokens` : compact;
  }
  return emptyLabel;
}

export function getAgentTokenUsageTotal(usage: TokenUsageShape) {
  const inputTokens = usage.input_tokens ?? 0;
  const outputTokens = usage.output_tokens ?? 0;
  if ((usage.runtime_kind === 'native_sdk' || usage.runtime_kind === 'codex')
    && (inputTokens > 0 || outputTokens > 0)) {
    return inputTokens + outputTokens;
  }
  const totalTokens = usage.tokens_used ?? 0;
  if (totalTokens > 0) return totalTokens;
  return inputTokens + outputTokens;
}

export function formatAgentTokenUsageTotal(
  usage: TokenUsageShape,
  options?: {
    emptyLabel?: string;
    includeUnit?: boolean;
  },
) {
  const totalTokens = getAgentTokenUsageTotal(usage);
  if (totalTokens <= 0) return options?.emptyLabel ?? '-';
  const compact = formatCompactTokenCount(totalTokens);
  return options?.includeUnit ? `${compact} tokens` : compact;
}

export function formatAgentTokenUsageBreakdown(usage: TokenUsageShape) {
  const lines: string[] = [];
  const totalTokens = getAgentTokenUsageTotal(usage);
  const inputTokens = usage.input_tokens ?? 0;
  const outputTokens = usage.output_tokens ?? 0;
  const cachedInputTokens = usage.cached_input_tokens ?? 0;

  if (totalTokens > 0) {
    lines.push(`Total: ${formatCompactTokenCount(totalTokens)} tokens`);
  }
  if (inputTokens > 0) {
    lines.push(`Input: ${formatCompactTokenCount(inputTokens)}`);
  }
  if (outputTokens > 0) {
    lines.push(`Output: ${formatCompactTokenCount(outputTokens)}`);
  }
  if (cachedInputTokens > 0) {
    lines.push(`Cached input: ${formatCompactTokenCount(cachedInputTokens)}`);
  }
  return lines;
}

export function formatRunTokenUsage(run: Pick<AgentRun, 'runtime_kind' | 'tokens_used' | 'cached_input_tokens' | 'input_tokens' | 'output_tokens'>, options?: {
  emptyLabel?: string;
  includeUnit?: boolean;
}) {
  return formatAgentTokenUsage(run, options);
}

export function formatRunTokenUsageTotal(run: Pick<AgentRun, 'runtime_kind' | 'tokens_used' | 'cached_input_tokens' | 'input_tokens' | 'output_tokens'>, options?: {
  emptyLabel?: string;
  includeUnit?: boolean;
}) {
  return formatAgentTokenUsageTotal(run, options);
}

export function formatRunTokenUsageBreakdown(run: Pick<AgentRun, 'runtime_kind' | 'tokens_used' | 'cached_input_tokens' | 'input_tokens' | 'output_tokens'>) {
  return formatAgentTokenUsageBreakdown(run);
}

export function formatSessionTokenUsage(
  session: Pick<CodingSession, 'runtime_kind' | 'tokens_used' | 'cached_input_tokens' | 'input_tokens' | 'output_tokens'>,
  options?: {
    emptyLabel?: string;
    includeUnit?: boolean;
  },
) {
  return formatAgentTokenUsage(session, options);
}

export function formatSessionTokenUsageTotal(
  session: Pick<CodingSession, 'runtime_kind' | 'tokens_used' | 'cached_input_tokens' | 'input_tokens' | 'output_tokens'>,
  options?: {
    emptyLabel?: string;
    includeUnit?: boolean;
  },
) {
  return formatAgentTokenUsageTotal(session, options);
}
