"use client";

import { useId, useState } from "react";
import { ArrowRight, Check, SlidersHorizontal } from "lucide-react";
import { COMPARISON_FEATURES, PLANS } from "./pricing-data";
import { SectionHead } from "../new/_components/ui";

type Feature = {
  name: string;
  starter?: boolean | string;
  growth?: boolean | string;
};
const GROUPS: { name: string; rows: Feature[] }[] = [];
for (const row of COMPARISON_FEATURES) {
  if ("category" in row && row.category)
    GROUPS.push({ name: row.name, rows: [] });
  else GROUPS.at(-1)?.rows.push(row);
}
function CellValue({ value }: { value: Feature["starter"] }) {
  if (value === true)
    return (
      <span className="pricing-included">
        <Check size={17} strokeWidth={1.7} aria-hidden="true" />
        <span className="sr-only">Included</span>
      </span>
    );
  if (value === false || value === undefined)
    return (
      <span className="pricing-not-included">
        <span aria-hidden="true">—</span>
        <span className="sr-only">Not included</span>
      </span>
    );
  return <span>{value}</span>;
}
export function CloudComparison() {
  const [category, setCategory] = useState("all");
  const [differences, setDifferences] = useState(false);
  const id = useId();
  const groups = GROUPS.filter(
    (group) => category === "all" || category === group.name,
  )
    .map((group) => ({
      ...group,
      rows: group.rows.filter(
        (row) => !differences || row.starter !== row.growth,
      ),
    }))
    .filter((group) => group.rows.length);
  const count = groups.reduce((total, group) => total + group.rows.length, 0);
  return (
    <section id="compare-plans" className="pricing-comparison">
      <div className="wrap">
        <SectionHead
          eyebrow="Cloud plans, side by side"
          title="Find the right fit for your team."
          lede="Both plans include the product modules and unlimited teammates. Compare the capacity, automation, and agent controls that change as your team grows."
        />
        <div className="pricing-comparison-shell">
          <div className="pricing-comparison-tools">
            <div>
              <SlidersHorizontal size={16} aria-hidden="true" />
              <label htmlFor={`${id}-category`}>Compare</label>
              <select
                id={`${id}-category`}
                value={category}
                onChange={(event) => setCategory(event.target.value)}
              >
                <option value="all">All categories</option>
                {GROUPS.map((group) => (
                  <option key={group.name}>{group.name}</option>
                ))}
              </select>
            </div>
            <label className="pricing-difference-toggle">
              <input
                type="checkbox"
                checked={differences}
                onChange={(event) => setDifferences(event.target.checked)}
              />
              Only differences
            </label>
          </div>
          <div
            className="pricing-table-scroll"
            role="region"
            aria-label="Cloud plan feature comparison"
            tabIndex={0}
          >
            <table className="pricing-table">
              <caption className="sr-only">
                Starter and Growth Cloud features and limits. Checkmarks mean
                included and dashes mean not included.
              </caption>
              <colgroup>
                <col className="pricing-feature-column" />
                <col />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th scope="col">
                    <span className="pricing-table-header-label">
                      Your workspace
                    </span>
                    <span className="pricing-table-header-detail">
                      One plan for your whole team
                    </span>
                  </th>
                  {PLANS.map((plan) => (
                    <th scope="col" key={plan.name}>
                      <strong>{plan.name}</strong>
                      <span>
                        {plan.popular
                          ? "More capacity & control"
                          : "The essentials, connected"}
                      </span>
                      <a href={plan.href}>
                        Try {plan.name}
                        <ArrowRight size={12} aria-hidden="true" />
                      </a>
                    </th>
                  ))}
                </tr>
              </thead>
              {groups.map((group) => (
                <tbody key={group.name}>
                  <tr className="pricing-table-group">
                    <th scope="rowgroup" colSpan={3}>
                      {group.name}
                      <span>
                        {group.rows.length}{" "}
                        {group.rows.length === 1 ? "feature" : "features"}
                      </span>
                    </th>
                  </tr>
                  {group.rows.map((row) => (
                    <tr
                      key={row.name}
                      data-difference={row.starter !== row.growth}
                    >
                      <th scope="row">{row.name}</th>
                      <td>
                        <CellValue value={row.starter} />
                      </td>
                      <td>
                        <CellValue value={row.growth} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              ))}
              {!count && (
                <tbody>
                  <tr>
                    <td colSpan={3} className="pricing-comparison-empty">
                      Both plans include the same features in this category.
                      <button
                        type="button"
                        onClick={() => setDifferences(false)}
                      >
                        Show included features
                      </button>
                    </td>
                  </tr>
                </tbody>
              )}
            </table>
          </div>
          <div className="pricing-comparison-footer">
            <span role="status">
              {count} {count === 1 ? "feature" : "features"}
              {differences ? " with different plan limits or availability" : ""}
            </span>
            <span>Cloud plans · Per workspace</span>
          </div>
        </div>
        <p className="pricing-note">
          These limits apply to Helpin Cloud.{" "}
          <a href="#self-hosted">Explore the self-hosted product →</a>
        </p>
      </div>
    </section>
  );
}
