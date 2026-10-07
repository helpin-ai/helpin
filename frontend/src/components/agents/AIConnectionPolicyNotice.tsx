import { aiUsagePricingText } from "@edition/ai";
import type { AIConnectionPolicyView } from "@/lib/services/aiConnectionService";
import { AISetupHelp } from "./AISetupHelp";

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
  const pricing = aiUsagePricingText(policy.pricing);
  return pricing ? <AISetupHelp label={label ? `${label} usage details` : "Usage details"} description={pricing} /> : null;
}
