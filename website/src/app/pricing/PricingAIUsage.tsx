import {
  ArrowRight,
  Cloud,
  KeyRound,
  UserRound,
  Server,
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
          eyebrow="AI usage"
          title={"Choose how you\npay for AI."}
          lede="Use your plan’s allowance, workspace API keys, or a personal provider connection."
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
            <span className="pricing-ai-route-label">INCLUDED ALLOWANCE</span>
            <h3>Every Cloud plan includes AI.</h3>
            <p>
              A monthly AI allowance, billed and managed by Helpin. Nothing to configure.
            </p>
            <span className="pricing-ai-route-footer">
              Included with every Cloud plan
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <KeyRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">YOUR PROVIDER KEYS</span>
            <h3>Bring your own API keys.</h3>
            <p>
              Connect OpenAI, Anthropic, OpenRouter or an approved compatible endpoint. Keys are shared across the workspace and power team automations. The provider bills you directly.
            </p>
            <span className="pricing-ai-route-footer">
              Enabled by Helpin for Cloud workspaces
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <UserRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">PERSONAL CONNECTION</span>
            <h3>Use your own account for manual runs.</h3>
            <p>
              Sign in with ChatGPT or connect a personal provider key for runs you start yourself. Not used for shared automations.
            </p>
            <span className="pricing-ai-route-footer">
              Subject to your provider’s terms
            </span>
          </article>
        </div>
        <div className="pricing-usage-summary">
          <p><strong>Allowance resets monthly.</strong> No carryover, including on annual plans.</p>
          <p><strong>Overage is opt-in.</strong> Metered, pay for what you use. No prepaid blocks.</p>
          <p><strong>Your keys, your bill.</strong> With your own keys or personal connection, the provider charges you directly. Your Helpin plan, configured Cloud token fees, and any paid-tool fees still apply. Self-hosted Community edition has no Helpin AI or tool fees.</p>
        </div>
        <details className="pricing-ai-charges" id="ai-usage-charges">
          <summary><span><strong>Models, allowances, and usage rates</strong><small>Compare model profiles and check the billing details.</small></span></summary>
          <div>
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
