import { ArrowRight, Check } from "lucide-react";
import { COMPARISON_FEATURES, PLANS } from "./pricing-data";
import { SectionHead } from "../new/_components/ui";

type Value = boolean | string | undefined;
type Feature = { name: string; starter?: Value; growth?: Value };
type Group = { name: string; rows: Feature[] };

const ALL: Group[] = [];
for (const row of COMPARISON_FEATURES) {
  if ("category" in row && row.category) ALL.push({ name: row.name, rows: [] });
  else ALL.at(-1)?.rows.push(row);
}
// The table carries what changes between plans, plus stated limits.
// Everything both plans include identically moves to the list beneath it.
const isLimit = (row: Feature) =>
  typeof row.starter === "string" || typeof row.growth === "string";
const differs = (row: Feature) => row.starter !== row.growth;
const TABLE = ALL.map((group) => ({
  ...group,
  rows: group.rows.filter((row) => differs(row) || isLimit(row)),
})).filter((group) => group.rows.length);
const SHARED = ALL.map((group) => ({
  ...group,
  rows: group.rows.filter((row) => !differs(row) && !isLimit(row)),
})).filter((group) => group.rows.length);

function Cell({ value, plan }: { value: Value; plan: string }) {
  if (value === true)
    return (
      <td data-plan={plan}>
        <span className="pricing-included">
          <Check size={18} strokeWidth={2} aria-hidden="true" />
          <span className="sr-only">Included</span>
        </span>
      </td>
    );
  if (!value)
    return (
      <td data-plan={plan}>
        <span className="pricing-not-included">
          <span aria-hidden="true">—</span>
          <span className="sr-only">Not included</span>
        </span>
      </td>
    );
  return <td data-plan={plan}>{value}</td>;
}

export function CloudComparison() {
  return (
    <section id="compare-plans" className="pricing-comparison">
      <div className="wrap">
        <SectionHead eyebrow="Compare Cloud plans" title="What changes between Starter and Growth." />
        <table className="pricing-table">
          <caption className="sr-only">
            Starter and Growth Cloud limits and features. Checkmarks mean
            included and dashes mean not included.
          </caption>
          <colgroup>
            <col className="pricing-feature-column" />
            <col />
            <col />
          </colgroup>
          <thead>
            <tr>
              <th scope="col"><span className="sr-only">Feature</span></th>
              {PLANS.map((plan) => (
                <th scope="col" key={plan.key} data-featured={plan.popular}>
                  <span className="pricing-table-plan">{plan.name}</span>
                  <span className="pricing-table-price">${plan.price} / month</span>
                  <a
                    className={`btn ${plan.popular ? "btn-primary" : "btn-secondary"}`}
                    href={plan.href}
                    aria-label={`Start free trial with ${plan.name}`}
                  >
                    Start free
                    <ArrowRight size={14} aria-hidden="true" />
                  </a>
                </th>
              ))}
            </tr>
          </thead>
          {TABLE.map((group) => (
            <tbody key={group.name}>
              <tr className="pricing-table-group">
                <th scope="rowgroup" colSpan={3}>
                  {group.name}
                </th>
              </tr>
              {group.rows.map((row) => (
                <tr key={row.name}>
                  <th scope="row">{row.name}</th>
                  <Cell value={row.starter} plan="Starter" />
                  <Cell value={row.growth} plan="Growth" />
                </tr>
              ))}
            </tbody>
          ))}
        </table>
        <details className="pricing-disclosure pricing-shared">
          <summary>Included in both plans</summary>
          <dl>
            {SHARED.map((group) => (
              <div key={group.name}>
                <dt>{group.name}</dt>
                <dd>
                  <ul>
                    {group.rows.map((row) => (
                      <li key={row.name}>
                        <Check size={13} strokeWidth={2} aria-hidden="true" />
                        <span>{row.name}</span>
                      </li>
                    ))}
                  </ul>
                </dd>
              </div>
            ))}
          </dl>
        </details>
      </div>
    </section>
  );
}
