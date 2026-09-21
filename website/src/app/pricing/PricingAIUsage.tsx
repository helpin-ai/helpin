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

export function AIUsage() {
  return (
    <section id="ai-usage" className="pricing-ai-section">
      <div className="wrap">
        <SectionHead
          eyebrow="AI, on your terms"
          title="Choose your models and accounts."
          lede="Choose frontier models for complex work or efficient models for everyday tasks. Use Cloud AI, your own provider accounts, or both."
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
            <span className="pricing-ai-route-label">MANAGED BY HELPIN</span>
            <h3>Start with included AI.</h3>
            <p>
              A monthly allowance for your agents. Routine work uses less;
              advanced work uses more. Track the percentage used in settings.
            </p>
            <span className="pricing-ai-route-footer">
              Included in Starter and Growth
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <KeyRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">MANAGED BY YOUR TEAM</span>
            <h3>Bring your own API keys.</h3>
            <p>
              Use your keys with the providers above. Shared connections power
              team workflows and unattended agent runs.
            </p>
            <span className="pricing-ai-route-footer">
              Requires workspace enablement on Cloud
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <UserRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">PERSONAL TO YOU</span>
            <h3>Use your subscription.</h3>
            <p>
              Connect ChatGPT for runs you start yourself. Personal connections
              cannot power shared workspace automation.
            </p>
            <span className="pricing-ai-route-footer">
              Where enabled · Personal use
            </span>
          </article>
        </div>
        <div className="pricing-ai-models">
          <div className="pricing-ai-models-heading">
            <div>
              <h3>Four profiles for your work.</h3>
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
                <p>{tier.description}.</p>
              </article>
            ))}
          </div>
        </div>
        <div className="pricing-ai-billing">
          <h3>Usage and billing</h3>
          <div>
            {[
              {
                Icon: RefreshCw,
                title: "Monthly reset",
                text: "Resets on your monthly renewal date—even on annual plans. No rollover.",
              },
              {
                Icon: SlidersHorizontal,
                title: "Optional extra usage",
                text: "Opt in to metered usage beyond your allowance. No prepaid blocks.",
              },
              {
                Icon: ReceiptText,
                title: "Provider and platform fees",
                text: "Your provider bills model usage. Cloud adds a flat per-token platform fee and paid-tool charges. Self-hosted Community has no Helpin token or tool fees.",
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
      </div>
    </section>
  );
}
