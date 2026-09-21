import {
  ArrowRight,
  Cloud,
  KeyRound,
  UserRound,
  Server,
  RefreshCw,
  SlidersHorizontal,
  ReceiptText,
} from "lucide-react";
import { AI_PRICING } from "@/generated/aiPricing";
import { GITHUB_URL, SectionHead } from "../new/_components/ui";

const PROFILE_DESCRIPTIONS = {
  small: "Keep straightforward work lightweight.",
  medium: "Handle the everyday mix of questions and drafts.",
  large: "Give complex tasks more room for reasoning.",
  flagship: "Reserve the strongest option for the hardest work.",
};

const usd = (microusd: number) => new Intl.NumberFormat("en-US", {
  style: "currency", currency: "USD", maximumFractionDigits: 4,
}).format(microusd / 1_000_000);

export function AIUsage() {
  return (
    <section id="ai-usage" className="pricing-ai-section">
      <div className="wrap">
        <SectionHead
          eyebrow="Choose how your agents work"
          title={"Match the AI\nto the job."}
          lede="An account summary and a code review need different kinds of work. Choose the model and connection that fit the task, alongside the tools and approvals you give the agent."
        />
        <div
          className="pricing-ai-providers"
          aria-label="Supported provider connections"
        >
          {[
            { id: "openai", name: "OpenAI" },
            { id: "anthropic", name: "Anthropic" },
            { id: "openrouter", name: "OpenRouter" },
          ].map((provider) => (
            <span key={provider.id}>
              <img
                src={`/new/providers/${provider.id}.svg`}
                width={21}
                height={21}
                alt=""
              />
              {provider.name}
            </span>
          ))}
          <span>
            <Server size={20} strokeWidth={1.5} aria-hidden="true" />
            Approved compatible endpoints
          </span>
        </div>
        <div className="pricing-ai-routes">
          <article>
            <span className="pricing-ai-route-icon">
              <Cloud size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">MANAGED AI</span>
            <h3>Start with the included allowance.</h3>
            <p>
              Use the AI allocation in your Cloud plan.
            </p>
            <span className="pricing-ai-route-footer">
              Managed by Helpin
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <KeyRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">YOUR PROVIDER KEYS</span>
            <h3>Connect your provider account.</h3>
            <p>
              Use approved shared connections for team workflows.
            </p>
            <span className="pricing-ai-route-footer">
              Cloud enablement required
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <UserRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">PERSONAL CONNECTION</span>
            <h3>Keep personal access personal.</h3>
            <p>
              Where enabled, use it for manually started runs—not shared automation.
            </p>
            <span className="pricing-ai-route-footer">
              Availability-dependent.
            </span>
          </article>
        </div>
        <div className="pricing-ai-models">
          <div className="pricing-ai-models-heading">
            <div>
              <h3>Choose the level of reasoning the work needs.</h3>
            </div>
            <a href={`${GITHUB_URL}/blob/develop/docs/ai-connections.md`}>
              Connection guide
              <ArrowRight size={14} aria-hidden="true" />
            </a>
          </div>
          <div className="pricing-ai-tiers">
            {AI_PRICING.tiers.map((tier, index) => (
              <article key={tier.key}>
                <div>
                  <span className="pricing-ai-tier-bars" aria-hidden="true">
                    {[0, 1, 2, 3].map((i) => (
                      <i
                        key={i}
                        data-active={i <= index}
                        style={{ height: `${7 + i * 4}px` }}
                      />
                    ))}
                  </span>
                  <h4>{tier.label}</h4>
                </div>
                <p>{PROFILE_DESCRIPTIONS[tier.key]}</p>
              </article>
            ))}
          </div>
        </div>
        <p className="pricing-note">Choose deliberately. More demanding work does not always need the same model as an everyday question.</p>
        <div className="pricing-ai-billing">
          <h3>Understand what is included—and what costs extra.</h3>
          <div>
            {[
              {
                Icon: RefreshCw,
                title: "Monthly allowance",
                text: "Renews monthly, without carryover—even with annual billing.",
              },
              {
                Icon: SlidersHorizontal,
                title: "Additional usage",
                text: "Opt-in, metered overage. No prepaid blocks.",
              },
              {
                Icon: ReceiptText,
                title: "Connected-provider charges",
                text: "Provider billing is separate from Cloud platform and paid-tool fees. Community self-hosting has no Helpin token or tool fees.",
              },
            ].map(({ Icon, title, text }) => (
              <article key={title}>
                <Icon size={17} strokeWidth={1.5} aria-hidden="true" />
                <div>
                  <h4>{title}</h4>
                  <p>{text}</p>
                </div>
              </article>
            ))}
          </div>
        </div>
        <details className="pricing-ai-charges" id="ai-usage-charges">
          <summary>Review AI usage and charges →</summary>
          <div>
            <h3>Your included allowance</h3>
            <p>The allowance is a USD-valued budget for Helpin AI charges, not a fixed number of messages or runs. It is included in your subscription. Unlimited teammates does not mean unlimited AI usage.</p>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Monthly AI allowance by billing period">
              <table><caption>Included each month, by billing period</caption><thead><tr><th scope="col">Plan</th><th scope="col">Monthly billing</th><th scope="col">Annual billing</th></tr></thead><tbody>
                {(["starter", "growth"] as const).map(plan => <tr key={plan}><th scope="row">{plan === "starter" ? "Starter" : "Growth"}</th>{(["monthly", "annual"] as const).map(interval => <td key={interval}>{usd(AI_PRICING.plans.find(item => item.plan === plan && item.billing_interval === interval)!.allowance_microusd)}</td>)}</tr>)}
              </tbody></table>
            </div>
            <p>Annual subscriptions still renew their AI allowance monthly. Unused allowance does not carry forward. The 14-day Growth trial includes {usd(AI_PRICING.plans.find(item => item.billing_interval === "trial")!.allowance_microusd)} for the trial period. After the trial, your chosen plan’s capacity and agent capabilities apply.</p>
            <h3>How managed AI consumes the allowance</h3>
            <p>Tokens are the units of content a model processes and generates. Multiply each token count by its rate below, divide by one million, then add any paid-tool charges. Cached input uses its corresponding cache rate; reasoning tokens use the output rate. Charges reduce your remaining allowance.</p>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Managed AI token rates">
              <table><caption>USD per one million tokens · Pricing effective {AI_PRICING.effective_date}</caption><thead><tr><th scope="col">Profile</th><th scope="col">Input</th><th scope="col">Cache read</th><th scope="col">Cache write</th><th scope="col">Output / reasoning</th></tr></thead><tbody>
                {AI_PRICING.tiers.map(tier => <tr key={tier.key}><th scope="row">{tier.label}</th><td>{usd(tier.rates.input_microusd_per_million)}</td><td>{usd(tier.rates.cache_read_microusd_per_million)}</td><td>{usd(tier.rates.cache_write_microusd_per_million)}</td><td>{usd(tier.rates.output_microusd_per_million)}</td></tr>)}
              </tbody></table>
            </div>
            <p>For example, one million uncached input tokens and one million output tokens on Small consume {usd(AI_PRICING.tiers[0].rates.input_microusd_per_million + AI_PRICING.tiers[0].rates.output_microusd_per_million)} of the allowance, before any paid-tool charges.</p>
            <h3>When the allowance runs out</h3>
            <p>Without overage enabled, new paid AI work is blocked when the remaining allowance cannot cover it. Work already in progress reserves part of the allowance. Wait for the monthly renewal or enable optional overage to continue paid AI work.</p>
            <p>With overage enabled, the same usage rates apply. Only charges above the included allowance are billed, settled monthly and rounded to the nearest cent. No prepaid blocks. Applicable taxes are extra.</p>
            <h3>Connected providers and paid tools</h3>
            <p>Your provider bills its usage separately. Cloud provider-key connections use the platform’s configured flat rate per million tokens, with paid tools charged separately; review your workspace’s configured fees before use. Community self-hosting has no Helpin token or tool fees.</p>
          </div>
        </details>
      </div>
    </section>
  );
}
