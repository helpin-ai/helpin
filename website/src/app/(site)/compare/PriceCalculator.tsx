'use client';

import { useId, useState, type CSSProperties } from 'react';
import { AI_ALLOWANCE, PLANS } from '../../pricing/pricing-data';
import type { Calculator, CalculatorPlan } from './compare-data';

type Billing = 'annual' | 'monthly';
type Mode = 'cloud' | 'self';
type Line = readonly [label: string, value: string];

const MAX_SEATS = 100;
const TICKS = [1, 25, 50, 75, 100];

const money = (value: number) => `$${Math.round(value).toLocaleString('en-US')}`;
const share = (value: number, total: number) => `${total > 0 ? (value / total) * 100 : 0}%`;
const unitPrice = (plan: CalculatorPlan, billing: Billing) => (billing === 'monthly' && plan.monthly !== undefined ? plan.monthly : plan.annual);
const count = (value: number, [one, many]: readonly [string, string]) => `${value.toLocaleString('en-US')} ${value === 1 ? one : many}`;

// Rounds a chart's top value up to 1, 2, 2.5 or 5 times a power of ten.
function niceCeiling(value: number) {
  if (value <= 0) return 100;
  const power = 10 ** Math.floor(Math.log10(value));
  return ([1, 2, 2.5, 5, 10].find(step => step * power >= value) ?? 10) * power;
}

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

function Slider({ id, label, value, min, max, step = 1, onChange }: { id: string; label: string; value: number; min: number; max: number; step?: number; onChange: (value: number) => void }) {
  return (
    <div className="cmp-calc-field">
      <label className="cmp-calc-label" htmlFor={id}>{label}<output htmlFor={id}>{value.toLocaleString('en-US')}</output></label>
      <input id={id} className="cmp-range" type="range" min={min} max={max} step={step} value={value} style={{ '--fill': share(value - min, max - min) } as CSSProperties} onChange={event => onChange(Number(event.target.value))} />
      <span className="cmp-range-scale" aria-hidden="true"><span>{min.toLocaleString('en-US')}</span><span>{max.toLocaleString('en-US')}</span></span>
    </div>
  );
}

function Receipt({ name, plan, lines, monthly, note, helpin }: { name: string; plan: string; lines: Line[]; monthly: number; note?: string; helpin?: boolean }) {
  return (
    <div className="cmp-calc-receipt" data-product={helpin ? 'helpin' : 'rival'}>
      <h4>{name}<span>{plan}</span></h4>
      <dl>
        {lines.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
        <div className="cmp-calc-receipt-total"><dt>Per month</dt><dd>{money(monthly)}</dd></div>
        <div className="cmp-calc-receipt-total"><dt>Per year</dt><dd>{money(monthly * 12)}</dd></div>
      </dl>
      {note ? <p>{note}</p> : null}
    </div>
  );
}

// Monthly cost for every team size on the slider. Per-seat pricing climbs; Helpin's
// workspace price stays flat, so the chart shows where, if anywhere, the two cross.
function Growth({ name, unit, seats, rival, helpin }: { name: string; unit: readonly [string, string]; seats: number; rival: (seats: number) => number; helpin: number }) {
  const sizes = Array.from({ length: MAX_SEATS }, (_, index) => index + 1);
  const top = niceCeiling(Math.max(rival(MAX_SEATS), helpin) * 1.04);
  const x = (size: number) => ((size - 1) / (MAX_SEATS - 1)) * 100;
  const y = (cost: number) => 100 - (cost / top) * 100;
  const rivalPath = sizes.map((size, index) => `${index ? 'L' : 'M'}${x(size).toFixed(2)} ${y(rival(size)).toFixed(2)}`).join(' ');
  const rivalCheaperUpTo = sizes.filter(size => rival(size) < helpin).at(-1);
  const helpinCheaperFrom = sizes.find(size => rival(size) > helpin);
  const summary = helpinCheaperFrom === 1
    ? `Helpin costs less at every team size up to ${count(MAX_SEATS, unit)}.`
    : helpinCheaperFrom === undefined
      ? rivalCheaperUpTo === undefined ? `Both cost the same at every team size.` : `${name} costs less at every team size up to ${count(rivalCheaperUpTo, unit)}.`
      : rivalCheaperUpTo === undefined
        ? `Helpin costs less from ${count(helpinCheaperFrom, unit)} up.`
        : `${name} costs less up to ${count(rivalCheaperUpTo, unit)}. From ${helpinCheaperFrom}, Helpin costs less.`;
  // Each line is labelled above its right end. When the lines finish too close
  // together, the lower line's label moves to its left end instead.
  const rivalOnTop = rival(MAX_SEATS) >= helpin;
  const crowded = Math.abs(y(helpin) - y(rival(MAX_SEATS))) < 14;
  const tag = (product: 'rival' | 'helpin') => {
    const start = crowded && (product === 'rival') !== rivalOnTop;
    const cost = product === 'helpin' ? helpin : rival(start ? 1 : MAX_SEATS);
    return { 'data-product': product, 'data-side': start ? 'start' : 'end', style: { top: `${y(cost)}%` } };
  };

  return (
    <figure className="cmp-growth">
      <figcaption><strong>As your team grows</strong><span>{summary}</span></figcaption>
      <div className="cmp-growth-plot" role="img" aria-label={`Monthly cost for 1 to ${MAX_SEATS} ${unit[1]}. ${summary}`}>
        {[top, top / 2, 0].map(tick => <span key={tick} className="cmp-growth-grid" style={{ top: `${y(tick)}%` }}><span>{money(tick)}</span></span>)}
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          {helpinCheaperFrom && helpinCheaperFrom > 1 ? <rect className="cmp-growth-zone" x={x(helpinCheaperFrom)} y="0" width={100 - x(helpinCheaperFrom)} height="100" /> : null}
          <path className="cmp-growth-rival" d={rivalPath} vectorEffect="non-scaling-stroke" />
          <path className="cmp-growth-helpin" d={`M0 ${y(helpin)} L100 ${y(helpin)}`} vectorEffect="non-scaling-stroke" />
        </svg>
        <span className="cmp-growth-now" style={{ left: `${x(seats)}%` }} aria-hidden="true" />
        <span className="cmp-growth-dot" data-product="rival" style={{ left: `${x(seats)}%`, top: `${y(rival(seats))}%` }} aria-hidden="true" />
        <span className="cmp-growth-dot" data-product="helpin" style={{ left: `${x(seats)}%`, top: `${y(helpin)}%` }} aria-hidden="true" />
        <span className="cmp-growth-tag" {...tag('rival')} aria-hidden="true">{name}</span>
        <span className="cmp-growth-tag" {...tag('helpin')} aria-hidden="true">Helpin</span>
      </div>
      <div className="cmp-growth-axis" aria-hidden="true">{TICKS.map(tick => <span key={tick} style={{ left: `${x(tick)}%` }}>{tick}</span>)}<em>{unit[1]}</em></div>
    </figure>
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
  const perSeat = unitPrice(plan, billing);
  const ai = !selfHosted ? calculator.ai : undefined;
  const aiCost = ai ? volume * ai.price : 0;
  const aiName = ai?.label.replace(' a month', '');
  const rivalAt = (size: number) => Math.max(size, plan.minSeats ?? 1) * perSeat + aiCost;
  const billedSeats = Math.max(seats, plan.minSeats ?? 1);
  const seatCost = billedSeats * perSeat;
  const rivalMonthly = seatCost + aiCost;
  const rivalLines: Line[] = [[`${billedSeats} × ${plan.name} at ${money(perSeat)}`, money(seatCost)]];
  if (ai) rivalLines.push([`${volume.toLocaleString('en-US')} ${aiName} × $${ai.price.toFixed(2)}`, money(aiCost)]);
  const rivalNote = [
    plan.minSeats && seats < plan.minSeats ? `${plan.name} has a ${plan.minSeats}-seat minimum.` : '',
    billing === 'monthly' && !selfHosted && plan.monthly === undefined ? 'Monthly price not published; annual rate shown.' : '',
  ].filter(Boolean).join(' ');

  const helpinPlan = PLANS.find(item => item.key === helpinKey)!;
  const helpinMonthly = selfHosted ? 0 : billing === 'annual' ? helpinPlan.annual : helpinPlan.price;
  const allowance = AI_ALLOWANCE[helpinKey][billing];
  const helpinLines: Line[] = selfHosted
    ? [['Community edition license', '$0'], [count(seats, calculator.seatsUnit), 'Included'], ['AI agents', 'Your AI provider']]
    : [[`${helpinPlan.name} plan, one workspace`, money(helpinMonthly)], [count(seats, calculator.seatsUnit), 'Included'], ['AI usage allowance', `${money(allowance)} included`]];
  const helpinNote = selfHosted ? '' : `${helpinKey === 'starter' ? 'Starter includes 10 teams, 5,000 contacts and 500 documents. ' : ''}Heavy AI use can go past the allowance; overage is metered.`;

  const billed = selfHosted ? 'self-hosted' : `billed ${billing === 'annual' ? 'annually' : 'monthly'}`;
  const scale = Math.max(rivalMonthly, helpinMonthly);
  const yearlyGap = Math.abs(rivalMonthly - helpinMonthly) * 12;
  const leader = rivalMonthly > helpinMonthly ? 'helpin' : rivalMonthly < helpinMonthly ? 'rival' : 'even';
  const verdict = leader === 'helpin'
    ? <>Helpin costs <em>{money(yearlyGap)} less</em> a year than {name}.</>
    : leader === 'rival'
      ? <>{name} costs <em>{money(yearlyGap)} less</em> a year than Helpin.</>
      : <>Helpin and {name} cost the same at these list prices.</>;

  return (
    <div className="cmp-calc">
      <div className="cmp-calc-controls">
        {calculator.selfHosted ? (
          <Segmented label="Hosting" value={mode} options={[['cloud', 'Cloud'], ['self', 'Self-hosted']]} onChange={value => { setMode(value); setPlanIndex(value === 'self' ? calculator.selfHosted!.plan : calculator.plan); }} />
        ) : null}
        <div className="cmp-calc-sliders">
          <Slider id={`${id}-seats`} label={calculator.seatsLabel} value={seats} min={1} max={MAX_SEATS} onChange={setSeats} />
          {ai ? <Slider id={`${id}-volume`} label={ai.label} value={volume} min={0} max={ai.max} step={ai.step} onChange={setVolume} /> : null}
        </div>
        <div className="cmp-calc-options">
          <Segmented label={`${name} plan`} value={Math.min(planIndex, plans.length - 1)} options={plans.map((item, index) => [index, item.name] as const)} onChange={setPlanIndex} />
          {!selfHosted ? <Segmented label="Helpin plan" value={helpinKey} options={[['starter', 'Starter'], ['growth', 'Growth']]} onChange={setHelpinKey} /> : null}
          {!selfHosted ? <Segmented label="Billing" value={billing} options={[['annual', 'Annual'], ['monthly', 'Monthly']]} onChange={setBilling} /> : null}
        </div>
      </div>

      <div className="cmp-calc-result" data-leader={leader}>
        <p className="cmp-calc-context">Estimated at list prices · {count(seats, calculator.seatsUnit)}{ai ? ` · ${volume.toLocaleString('en-US')} ${aiName} a month` : ''} · {billed}</p>
        <p className="cmp-calc-verdict" aria-live="polite">{verdict}</p>
        <div className="cmp-calc-bars">
          <div className="cmp-calc-bar" data-product="helpin">
            <span className="cmp-calc-bar-name"><img src="/brand/helpin-icon-white.svg" width={18} height={18} alt="" />Helpin<small>{selfHosted ? 'Community edition' : helpinPlan.name}</small></span>
            <span className="cmp-calc-bar-track" aria-hidden="true"><span style={{ width: share(helpinMonthly, scale) }} /></span>
            <strong>{money(helpinMonthly)}<small>/mo</small></strong>
          </div>
          <div className="cmp-calc-bar" data-product="rival">
            <span className="cmp-calc-bar-name"><span aria-hidden="true" />{name}<small>{plan.name}</small></span>
            <span className="cmp-calc-bar-track" aria-hidden="true"><span style={{ width: share(seatCost, scale) }} />{ai ? <span data-part="ai" style={{ width: share(aiCost, scale) }} /> : null}</span>
            <strong>{money(rivalMonthly)}<small>/mo</small></strong>
          </div>
          <p className="cmp-calc-legend" aria-hidden="true">
            <span data-part="helpin">Helpin{selfHosted ? ' license' : ' plan, AI allowance included'}</span>
            <span data-part="rival">{name} seats</span>
            {ai ? <span data-part="ai">{aiName}</span> : null}
          </p>
        </div>
        <Growth name={name} unit={calculator.seatsUnit} seats={seats} rival={rivalAt} helpin={helpinMonthly} />
      </div>

      <div className="cmp-calc-math">
        <Receipt name="Helpin" helpin plan={selfHosted ? 'Community edition, self-hosted' : `${helpinPlan.name}, ${billed}`} lines={helpinLines} monthly={helpinMonthly} note={helpinNote} />
        <Receipt name={name} plan={`${plan.name}, ${billed}`} lines={rivalLines} monthly={rivalMonthly} note={rivalNote} />
      </div>

      <ul className="cmp-calc-notes">
        {(selfHosted ? [calculator.selfHosted!.note] : calculator.notes).map(note => <li key={note}>{note}</li>)}
        {!selfHosted ? <li>Helpin measures AI usage in tokens, not resolutions. Paid plans can turn on metered overage beyond the allowance.</li> : null}
      </ul>
    </div>
  );
}
