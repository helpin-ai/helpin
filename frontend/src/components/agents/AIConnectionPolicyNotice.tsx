import { useContext } from "react";
import { AIModelsPricingContext } from "./AIModelsPricingContext";
import { aiUsagePricingText } from "@edition/ai";
import type { AIConnectionPolicyView } from "@/lib/services/aiConnectionService";
import { AISetupHelp } from "./AISetupHelp";

export function AIConnectionPolicyNotice({
  policy,
  label,
  hideManagedPricing = false,
}: {
  policy?: AIConnectionPolicyView;
  label?: string;
  hideManagedPricing?: boolean;
}) {
  const sharedPricing = useContext(AIModelsPricingContext);
  if (!policy) return null;
  if (!policy.allowed)
    return (
      <span className="block text-xs text-quiet-text-secondary" role="status">
        {label ? `${label}: ` : ""}
        {policy.message || "Unavailable for new runs."}
      </span>
    );
  if (hideManagedPricing && policy.pricing?.funding_mode === "helpin_hosted") return null;
  const pricing = aiUsagePricingText(policy.pricing);
  return pricing && pricing !== sharedPricing ? <AISetupHelp label={label ? `${label} usage details` : "Usage details"} description={pricing} /> : null;
}
