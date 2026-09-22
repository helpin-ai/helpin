import { Cloud, KeyRound, UserRound } from 'lucide-react';
import { AI_PRICING } from '@/generated/aiPricing';
import { GITHUB_URL, SectionHead } from '../new/_components/ui';
import { AI_ALLOWANCE } from './pricing-data';

const PROVIDERS = [
  { id: 'openai', name: 'OpenAI' },
  { id: 'anthropic', name: 'Anthropic' },
  { id: 'openrouter', name: 'OpenRouter' },
];

const usd = (microusd: number) => new Intl.NumberFormat('en-US', {
  style: 'currency', currency: 'USD', maximumFractionDigits: 4,
}).format(microusd / 1_000_000);

const modelsFor = (tier: string) => [...new Set(AI_PRICING.models.filter(model => model.tier === tier && model.enabled).map(model => model.label))];

export function AIUsage() {
  return (
    <section id="ai-usage" className="pricing-ai-section">
      <div className="wrap">
        <SectionHead
          eyebrow="AI usage"
          title="AI is included. Your own keys work too."
          lede="Every Cloud plan comes with a monthly AI allowance. You can also connect your own provider and pay them directly."
        />
        <div className="pricing-ai-routes">
          <article>
            <Cloud size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>Included allowance</h3>
              <p>Starter includes ${AI_ALLOWANCE.starter.monthly} of AI usage a month and Growth includes ${AI_ALLOWANCE.growth.monthly} (${AI_ALLOWANCE.starter.annual} and ${AI_ALLOWANCE.growth.annual} on annual billing). Nothing to set up. The allowance resets monthly and doesn’t carry over.</p>
            </div>
            <span>Billed by Helpin</span>
          </article>
          <article>
            <KeyRound size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>Workspace API keys</h3>
              <p>Connect a key once and the whole workspace uses it, including automations.</p>
              <ul className="pricing-ai-providers" aria-label="Supported providers">
                {PROVIDERS.map(provider => <li key={provider.id}><img src={`/new/providers/${provider.id}.svg`} width={16} height={16} alt="" />{provider.name}</li>)}
                <li>Approved compatible endpoints</li>
              </ul>
            </div>
            <span>Billed by your provider · Enabled by Helpin on Cloud</span>
          </article>
          <article>
            <UserRound size={20} strokeWidth={1.75} aria-hidden="true" />
            <div>
              <h3>Personal connection</h3>
              <p>Sign in with ChatGPT or add a personal key for runs you start yourself. Shared automations never use it.</p>
            </div>
            <span>Billed by your provider</span>
          </article>
        </div>
        <p className="pricing-ai-note"><strong>If the allowance runs out:</strong> overage is off by default, so new AI work pauses until the allowance renews. Turn on metered overage to keep working at the same rates.</p>

        <details className="pricing-disclosure" id="ai-usage-charges">
          <summary>Models and token rates</summary>
          <div>
            <div className="pricing-rate-scroll" tabIndex={0} role="region" aria-label="Managed AI token rates">
              <table>
                <caption>USD per million tokens, effective {AI_PRICING.effective_date}</caption>
                <thead><tr><th scope="col">Profile</th><th scope="col">Models</th><th scope="col">Input</th><th scope="col">Cache read</th><th scope="col">Cache write</th><th scope="col">Output</th></tr></thead>
                <tbody>{AI_PRICING.tiers.map(tier => <tr key={tier.key}>
                  <th scope="row">{tier.label}<small>{tier.description}</small></th>
                  <td className="pricing-rate-models">{modelsFor(tier.key).join(', ')}</td>
                  <td>{usd(tier.rates.input_microusd_per_million)}</td>
                  <td>{usd(tier.rates.cache_read_microusd_per_million)}</td>
                  <td>{usd(tier.rates.cache_write_microusd_per_million)}</td>
                  <td>{usd(tier.rates.output_microusd_per_million)}</td>
                </tr>)}</tbody>
              </table>
            </div>
            <dl className="pricing-rate-notes">
              <div><dt>How usage is charged</dt><dd>Tokens × the profile’s rate ÷ 1,000,000, plus any paid-tool charges. Charges come out of your allowance first.</dd></div>
              <div><dt>Overage</dt><dd>Opt-in. Billed monthly at the same rates, rounded to the cent. No prepaid blocks.</dd></div>
              <div><dt>Your own keys</dt><dd>Your provider bills you directly. On Cloud, a per-million-token platform fee and paid-tool charges still apply; check your workspace rates. Self-hosted Community has no Helpin token or tool fees.</dd></div>
              <div><dt>Trial</dt><dd>The 14-day Growth trial includes ${AI_ALLOWANCE.trial} of AI usage in total.</dd></div>
            </dl>
            <a className="btn-link" href={`${GITHUB_URL}/blob/develop/docs/ai-connections.md`}>AI connection guide →</a>
          </div>
        </details>
      </div>
    </section>
  );
}
