import { expect, it } from "vitest";
import { sharedAIModelsPricing } from "@/components/agents/AIModelsPricingContext";
import type { AIConnectionPolicyView } from "@/lib/services/aiConnectionService";

const policy = (rate: number): AIConnectionPolicyView => ({
  allowed: true,
  pricing: { mode: "ee", funding_mode: "customer_funded_flat", flat_tariff: {
    version: "v1", currency: "USD", microusd_per_million: rate, accounting_version: "v1",
  } },
});

it("states the common zero tariff once, but does not generalize it across different or unknown policies", () => {
  expect(sharedAIModelsPricing([policy(0), policy(0)])).toBe("No Helpin token fee. Paid tools are billed separately.");
  expect(sharedAIModelsPricing([policy(0), policy(1000000)])).toBeNull();
  expect(sharedAIModelsPricing([policy(0), { allowed: true, pricing: { mode: "ee", funding_mode: "helpin_hosted" } }])).toBeNull();
  expect(sharedAIModelsPricing([policy(0), undefined])).toBeNull();
  expect(sharedAIModelsPricing([policy(0), { allowed: false }])).toBeNull();
  expect(sharedAIModelsPricing([])).toBeNull();
});

it("uses the actual common tariff instead of assuming customer connections are free", () => {
  expect(sharedAIModelsPricing([policy(1000000)])).toContain("$1.00 per million tokens");
  expect(sharedAIModelsPricing([{ allowed: true, pricing: { mode: "ee", funding_mode: "customer_funded_flat" } }])).not.toContain("No Helpin token fee");
});
