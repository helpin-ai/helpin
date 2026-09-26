'use client';

import { useId, useState } from 'react';
import { AI_ALLOWANCE, PLANS } from '../../pricing/pricing-data';
import type { Calculator, CalculatorPlan } from './compare-data';

type Billing = 'annual' | 'monthly';
type Mode = 'cloud' | 'self';
type Line = readonly [label: string, value: string];

const money = (value: number) => `$${Math.round(value).toLocaleString('en-US')}`;
const unitPrice = (plan: CalculatorPlan, billing: Billing) => (billing === 'monthly' && plan.monthly !== undefined ? plan.monthly : plan.annual);

function Segmented<T extends string | number>({ label, value, options, onChange }: { label: string; value: T; options: readonly (readonly [T, string])[]; onChange: (value: T) => void }) {
  return (
    <div className="cmp-calc-field">
      <span className="cmp-calc-label">{label}</span>
      <div className="cmp-calc-segmented" role="group" aria-label={label}>
        {options.map(([option, text]) => <button key={String(option)} type="button" aria-pressed={option === value} onClick={() => onChange(option)}>{text}</button>)}
      </div>
    </div>
  );
}

function Estimate({ name, plan, lines, monthly, note, helpin }: { name: string; plan: string; lines: Line[]; monthly: number; note?: string; helpin?: boolean }) {
  return (
    <article className={helpin ? 'cmp-bill cmp-bill-helpin' : 'cmp-bill'}>
      <header>
        {helpin ? <img className="cmp-mark" src="/brand/helpin-icon-ink.svg" width={28} height={28} alt="" /> : <span className="cmp-monogram" aria-hidden="true">{name.charAt(0)}</span>}
        <div><h3>{name}</h3><span>{plan}</span></div>
      </header>
      <dl>{lines.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
      <div className="cmp-bill-total"><span>Estimated monthly</span><strong>{money(monthly)}<small>/mo</small></strong></div>
      <p>{money(monthly * 12)} a year.{note ? ` ${note}` : ''}</p>
    </article>
  );
}

// List-price arithmetic only: every input and exclusion is visible, and the result
// says plainly when the other tool costs less.
export function PriceCalculator({ name, calculator }: { name: string; calculator: Calculator }) {
  const id = useId();
  const [mode, setMode] = useState<Mode>('cloud');
  const [billing, setBilling] = useState<Billing>('annual');
  const [seats, setSeats] = useState(calculator.seats);
  const [planIndex, setPlanIndex] = useState(calculator.plan);
  const [helpinKey, setHelpinKey] = useState(calculator.helpinPlan);
  const [volume, setVolume] = useState(calculator.ai?.volume ?? 0);

  const selfHosted = mode === 'self' && calculator.selfHosted;
  const plans = selfHosted ? calculator.selfHosted!.plans : calculator.plans;
  const plan = plans[Math.min(planIndex, plans.length - 1)];
  const billedSeats = Math.max(seats, plan.minSeats ?? 1);
  const perSeat = unitPrice(plan, billing);
  const seatCost = billedSeats * perSeat;
  const ai = !selfHosted ? calculator.ai : undefined;
  const aiCost = ai ? volume * ai.price : 0;
  const competitorMonthly = seatCost + aiCost;
  const competitorLines: Line[] = [[`${billedSeats} × ${plan.name} at ${money(perSeat)}`, money(seatCost)]];
  if (ai) competitorLines.push([`${volume.toLocaleString('en-US')} ${ai.label.replace(' a month', '')} × $${ai.price.toFixed(2)}`, money(aiCost)]);
  const competitorNotes = [
    plan.minSeats && seats < plan.minSeats ? `${plan.name} has a ${plan.minSeats}-seat minimum.` : '',
    billing === 'monthly' && !selfHosted && plan.monthly === undefined ? 'Monthly price not published; annual rate shown.' : '',
  ].filter(Boolean).join(' ');

  const helpinPlan = PLANS.find(item => item.key === helpinKey)!;
  const helpinMonthly = selfHosted ? 0 : billing === 'annual' ? helpinPlan.annual : helpinPlan.price;
  const allowance = AI_ALLOWANCE[helpinKey][billing];
  const helpinLines: Line[] = selfHosted
    ? [['Community edition license', '$0'], [`${seats} teammates`, 'Included'], ['AI agents', 'Your AI provider']]
    : [[`${helpinPlan.name} plan, one workspace`, money(helpinMonthly)], [`${seats} teammates`, 'Included'], ['AI usage', `${money(allowance)} allowance included`]];
  const helpinNote = selfHosted ? '' : `${helpinKey === 'starter' ? 'Starter includes 10 teams, 5,000 contacts and 500 documents. ' : ''}Heavy AI use can go past the allowance; overage is metered.`;

  const yearlyGap = Math.abs(competitorMonthly - helpinMonthly) * 12;
  const verdict = competitorMonthly > helpinMonthly
    ? <>At these list prices, Helpin costs <strong>{money(yearlyGap)} less a year</strong> than {name}.</>
    : competitorMonthly < helpinMonthly
      ? <>At these list prices, {name} costs <strong>{money(yearlyGap)} less a year</strong> than Helpin.</>
      : <>At these list prices, both cost the same.</>;

  return (
    <div className="cmp-calc">
      <div className="cmp-calc-controls">
        {calculator.selfHosted ? (
          <Segmented label="Hosting" value={mode} options={[['cloud', 'Cloud'], ['self', 'Self-hosted']]} onChange={value => { setMode(value); setPlanIndex(value === 'self' ? calculator.selfHosted!.plan : calculator.plan); }} />
        ) : null}
        {!selfHosted ? <Segmented label="Billing" value={billing} options={[['annual', 'Annual'], ['monthly', 'Monthly']]} onChange={setBilling} /> : null}
        <div className="cmp-calc-field">
          <label className="cmp-calc-label" htmlFor={`${id}-seats`}>{calculator.seatsLabel}<output htmlFor={`${id}-seats`}>{seats}</output></label>
          <input id={`${id}-seats`} type="range" min={1} max={100} value={seats} onChange={event => setSeats(Number(event.target.value))} />
        </div>
        {ai ? (
          <div className="cmp-calc-field">
            <label className="cmp-calc-label" htmlFor={`${id}-volume`}>{ai.label}<output htmlFor={`${id}-volume`}>{volume.toLocaleString('en-US')}</output></label>
            <input id={`${id}-volume`} type="range" min={0} max={ai.max} step={ai.step} value={volume} onChange={event => setVolume(Number(event.target.value))} />
          </div>
        ) : null}
        <Segmented label={`${name} plan`} value={Math.min(planIndex, plans.length - 1)} options={plans.map((item, index) => [index, item.name] as const)} onChange={setPlanIndex} />
        {!selfHosted ? <Segmented label="Helpin plan" value={helpinKey} options={[['starter', 'Starter'], ['growth', 'Growth']]} onChange={setHelpinKey} /> : null}
      </div>
      <div className="cmp-calc-results">
        <div className="cmp-bills">
          <Estimate name="Helpin" helpin plan={selfHosted ? 'Community edition, self-hosted' : `${helpinPlan.name}, billed ${billing === 'annual' ? 'annually' : 'monthly'}`} lines={helpinLines} monthly={helpinMonthly} note={helpinNote} />
          <Estimate name={name} plan={selfHosted ? `${plan.name}, self-hosted` : `${plan.name}, billed ${billing === 'annual' ? 'annually' : 'monthly'}`} lines={competitorLines} monthly={competitorMonthly} note={competitorNotes} />
        </div>
        <p className="cmp-calc-verdict" aria-live="polite">{verdict}</p>
        <ul className="cmp-calc-notes">
          {(selfHosted ? [calculator.selfHosted!.note] : calculator.notes).map(note => <li key={note}>{note}</li>)}
          {!selfHosted ? <li>Helpin measures AI usage in tokens, not resolutions. Paid plans can turn on metered overage beyond the allowance.</li> : null}
        </ul>
      </div>
    </div>
  );
}
