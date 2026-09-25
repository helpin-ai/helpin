// @vitest-environment jsdom
import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { aiUsagePricingText } from "./ai";
import { AIExecutionDetails } from "@/components/agents/AIExecutionDetails";
import type { AIExecutionPolicySnapshot } from "@/lib/services/aiConnectionService";

const policy = (rate: number): AIExecutionPolicySnapshot => ({
  mode: "ee", funding_mode: "customer_funded_flat",
  flat_tariff: { version: "v1", currency: "USD", microusd_per_million: rate, accounting_version: "v1" },
});

it("discloses a provider-independent fee and separate paid tools", () => {
  expect(aiUsagePricingText(policy(1250000))).toContain("$1.25 per million tokens");
  expect(aiUsagePricingText(policy(1250000))).toContain("counted once");
  expect(aiUsagePricingText(policy(1250000))).toContain("Paid tools are billed separately");
  expect(aiUsagePricingText({ mode: "ee", funding_mode: "helpin_hosted" })).toContain("Managed model pricing");
  expect(aiUsagePricingText({ mode: "community", funding_mode: "customer_unbilled" })).toBeNull();
});

it("distinguishes explicit zero from missing and invalid rates", () => {
  expect(aiUsagePricingText(policy(0))).toContain("No Helpin token fee");
  for (const rate of [-1, NaN, Infinity]) expect(aiUsagePricingText(policy(rate))).toContain("unavailable");
  expect(aiUsagePricingText({ mode: "ee", funding_mode: "customer_funded_flat" })).toContain("unavailable");
});

it("shows the actual accepted fallback and frozen fee without consulting a profile", () => {
  const input = { ai_selection: { route: { model: { provider: "openai", model: "custom-unpriced" } },
    connection_scope: "workspace", fallback_reason: "primary_connection_unavailable", policy: policy(1250000) } };
  const html = renderToStaticMarkup(<AIExecutionDetails input={input} />);
  expect(html).toContain("custom-unpriced");
  expect(html).toContain("Workspace connection");
  expect(html).toContain("Fallback selected before execution");
  expect(html).toContain("$1.25 per million tokens");
  expect(renderToStaticMarkup(<AIExecutionDetails input={{}} />)).toBe("");
});
