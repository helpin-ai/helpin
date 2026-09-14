import { AIUsagePricing } from "@edition/ai";
import type { AIConnectionPolicyView } from "@/lib/services/aiConnectionService";

export function AIConnectionPolicyNotice({
  policy,
  label,
}: {
  policy?: AIConnectionPolicyView;
  label?: string;
}) {
  if (!policy) return null;
  if (!policy.allowed)
    return (
      <span className="block text-xs text-quiet-text-secondary" role="status">
        {label ? `${label}: ` : ""}
        {policy.message || "Unavailable for new runs."}
      </span>
    );
  return <AIUsagePricing policy={policy.pricing} label={label} />;
}
