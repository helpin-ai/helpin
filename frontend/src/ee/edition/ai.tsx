import type { AIExecutionPolicySnapshot } from "@/lib/services/aiConnectionService";

export function aiUsagePricingText(
  policy?: AIExecutionPolicySnapshot,
): string | null {
  if (policy?.mode !== "ee") return null;
  if (policy.funding_mode === "helpin_hosted")
    return "Managed model pricing. Paid tools are billed separately.";
  if (policy.funding_mode !== "customer_funded_flat") return null;
  const tariff = policy.flat_tariff;
  if (
    !tariff ||
    tariff.currency !== "USD" ||
    !Number.isFinite(tariff.microusd_per_million) ||
    tariff.microusd_per_million < 0
  )
    return "Token fee information is unavailable.";
  const rate = tariff.microusd_per_million;
  if (rate === 0)
    return "No Helpin token fee. Paid tools are billed separately.";
  const price = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 6,
  }).format(rate / 1_000_000);
  return `${price} per million tokens, including input, output, cache, and reasoning tokens counted once. Paid tools are billed separately.`;
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
