import { createContext } from "react";
import { aiUsagePricingText } from "@edition/ai";
import type { AIConnectionPolicyView } from "@/lib/services/aiConnectionService";

// Suppress only pricing that is already stated on the surrounding settings page.
export const AIModelsPricingContext = createContext<string | null>(null);

export function sharedAIModelsPricing(policies: (AIConnectionPolicyView | undefined)[]): string | null {
  if (!policies.length) return null;
  const texts = policies.map(policy => policy?.allowed ? aiUsagePricingText(policy.pricing) : null);
  return texts[0] && texts.every(text => text === texts[0]) ? texts[0] : null;
}
