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
          title="Choose the model. Choose how you pay for it."
          lede="Frontier models for demanding work, efficient models for everyday tasks. Use the AI included in your Cloud plan, connect the provider accounts your team already pays for, or both."
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
            Compatible endpoints
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
              Cloud plans include a monthly AI allowance. Routine work uses a
              little, advanced agent work uses more, and workspace settings
              always show a simple percentage used.
            </p>
            <span className="pricing-ai-route-footer">
              Included with Starter and Growth
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <KeyRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">MANAGED BY YOUR TEAM</span>
            <h3>Bring your own API keys.</h3>
            <p>
              Connect the OpenAI, Anthropic or OpenRouter accounts your team
              manages, or an approved compatible endpoint. Shared connections
              power team workflows and unattended agent runs.
            </p>
            <span className="pricing-ai-route-footer">
              Enabled per workspace on Cloud
            </span>
          </article>
          <article>
            <span className="pricing-ai-route-icon">
              <UserRound size={21} strokeWidth={1.5} aria-hidden="true" />
            </span>
            <span className="pricing-ai-route-label">PERSONAL TO YOU</span>
            <h3>Use your own subscription.</h3>
            <p>
              Where enabled, connect your ChatGPT subscription for runs you
              start yourself. Personal connections never power shared workspace
              automation.
            </p>
            <span className="pricing-ai-route-footer">
              Personal use · ChatGPT
            </span>
          </article>
        </div>
        <div className="pricing-ai-models">
          <div className="pricing-ai-models-heading">
            <div>
              <h3>Match the model to the work.</h3>
              <p>
                Four standard profiles, from routine answers to complex agent
                work.
              </p>
            </div>
            <a href={`${GITHUB_URL}/blob/develop/docs/ai-connections.md`}>
              How model connections work
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
          <h3>Know what you’re paying for.</h3>
          <div>
            {[
              {
                Icon: RefreshCw,
                title: "A fresh allowance every month.",
                text: "Cloud allowances reset on your renewal date each month, on monthly and annual plans alike. Unused allowance does not roll over.",
              },
              {
                Icon: SlidersHorizontal,
                title: "Extra usage is your call.",
                text: "Turn on extra usage when you need it. Only the work beyond your allowance is metered, with no prepaid blocks.",
              },
              {
                Icon: ReceiptText,
                title: "Your keys, your provider\u2019s bill.",
                text: "With your own keys, the model provider bills you directly. On Helpin Cloud, a flat per-token platform fee and any paid tools are charged separately. Self-hosted Community has no Helpin token or tool fees.",
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
