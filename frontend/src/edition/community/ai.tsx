import type { AIExecutionPolicySnapshot } from "@/lib/services/aiConnectionService";

export function aiUsagePricingText(
  policy?: AIExecutionPolicySnapshot,
): string | null {
  if (policy?.mode !== "community") return null;
  return "Usage is billed by your provider account. Helpin adds no token fee.";
}

export function AIUsagePricing({
  policy,
  label,
}: {
  policy?: AIExecutionPolicySnapshot;
  label?: string;
}) {
  const text = aiUsagePricingText(policy);
  return text ? (
    <span className="block text-xs text-quiet-text-secondary">
      {label ? `${label}: ` : ""}
      {text}
    </span>
  ) : null;
}
