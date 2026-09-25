# Billing Usage Chart Implementation Plan

> Historical implementation plan, source-compared on 2026-09-17. The component
> and tests now live under `frontend/src/ee/components/billing/`, not the old
> `frontend/src/components/billing/` paths below. Current
> [UsageDetail](../../frontend/src/ee/components/billing/UsageDetail.tsx) uses a
> six-tick default including first/last dates, shared `minmax(8px, 32px)` grid
> columns for bars and labels, centered sparse points and horizontal overflow.
> [Unit tests](../../frontend/src/ee/components/billing/__tests__/UsageDetail.test.ts)
> and [layout tests](../../frontend/src/ee/components/billing/__tests__/UsageDetailLayout.test.tsx)
> cover those contracts.
>
> Values now represent charged micro-USD as a percentage of the allowance, not
> legacy usage units. Already-cumulative API responses are not summed again;
> a zero/missing allowance maps chart values to zero. Original test/compiler and
> screenshot checkboxes below are historical records, not fresh validation.

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make billing AI-usage periods readable on the x-axis and prevent sparse data from producing oversized bars.

**Architecture:** Keep the existing CSS chart and add a pure tick-selection helper plus a shared bounded chart track. Render capped-width columns and aligned adaptive date labels from the same points.

**Tech Stack:** React, TypeScript, Tailwind CSS, Day.js, Vitest.

---

### Task 1: Adaptive ticks and bounded bars

**Files:**
- Modify: `frontend/src/components/billing/UsageDetail.tsx`
- Modify: `frontend/src/components/billing/__tests__/UsageDetail.test.ts`
- Modify: `frontend/src/components/billing/__tests__/UsageDetailLayout.test.tsx`

- [x] Add failing unit tests for adaptive first/intermediate/last tick selection.
- [x] Add failing layout assertions for x-axis labels and a 32px maximum bar width.
- [x] Implement tick selection, bounded columns, sparse centering, and overflow behavior.
- [x] Run the focused tests and TypeScript compiler.
- [x] Start the frontend, capture the Billing page, and visually inspect spacing and labels.
