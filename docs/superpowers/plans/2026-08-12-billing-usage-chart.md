# Billing Usage Chart Implementation Plan

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
