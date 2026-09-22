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
                <span className="pricing-ai-billing-icon"><Icon size={20} strokeWidth={1.5} aria-hidden="true" /></span>
                <div>
                  <h4>{title}</h4>
                  <p>{text}</p>
                </div>
              </article>
            ))}
          </div>
        </div>
        <details className="pricing-ai-charges" id="ai-usage-charges">
          <summary><span><strong>Review AI usage and charges</strong><small>Monthly allowances, token rates, and overage.</small></span></summary>
          <div>
            <h3>Monthly AI allowance</h3>
            <p>Your plan includes the monthly AI budget below, in USD. Unlimited teammates does not mean unlimited AI usage.</p>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Monthly AI allowance by billing period">
              <table><caption>Included each month, by billing period</caption><thead><tr><th scope="col">Plan</th><th scope="col">Monthly billing</th><th scope="col">Annual billing</th></tr></thead><tbody>
                {(["starter", "growth"] as const).map(plan => <tr key={plan}><th scope="row">{plan === "starter" ? "Starter" : "Growth"}</th>{(["monthly", "annual"] as const).map(interval => <td key={interval}>{usd(AI_PRICING.plans.find(item => item.plan === plan && item.billing_interval === interval)!.allowance_microusd)}</td>)}</tr>)}
              </tbody></table>
            </div>
            <p>Renews monthly on all plans, without carryover. The 14-day Growth trial includes {usd(AI_PRICING.plans.find(item => item.billing_interval === "trial")!.allowance_microusd)} total; your chosen plan’s limits and capabilities apply afterward.</p>
            <h3>Managed AI rates</h3>
            <p>Usage charges = tokens × the applicable rate ÷ 1,000,000, plus paid-tool charges. These charges reduce your allowance.</p>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Managed AI token rates">
              <table><caption>USD per one million tokens · Pricing effective {AI_PRICING.effective_date}</caption><thead><tr><th scope="col">Profile</th><th scope="col">Input</th><th scope="col">Cache read</th><th scope="col">Cache write</th><th scope="col">Output / reasoning</th></tr></thead><tbody>
                {AI_PRICING.tiers.map(tier => <tr key={tier.key}><th scope="row">{tier.label}</th><td>{usd(tier.rates.input_microusd_per_million)}</td><td>{usd(tier.rates.cache_read_microusd_per_million)}</td><td>{usd(tier.rates.cache_write_microusd_per_million)}</td><td>{usd(tier.rates.output_microusd_per_million)}</td></tr>)}
              </tbody></table>
            </div>
            <h3>When the allowance runs out</h3>
            <p><strong>Overage off:</strong> New paid AI work stops when the available allowance is insufficient. Active work reserves allowance. Resume after renewal or enable overage.</p>
            <p><strong>Overage on:</strong> Excess usage is billed monthly at the same rates, rounded to cents. No prepaid blocks; taxes extra.</p>
            <h3>Connected providers and paid tools</h3>
            <p>Your provider bills separately. Cloud adds its configured per-million-token platform fee and paid-tool charges—check workspace rates. Community self-hosting has no Helpin token or tool fees.</p>
          </div>
        </details>
      </div>
    </section>
  );
}
