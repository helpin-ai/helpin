# Demo provisioning completion plan

Status: implemented on `feat/demo-provisioning-completion`, November 16, 2026, except B1's root cause, B2 and B3. Where the build differs from this plan:

- The website source ID, index state, verify results and `issues` are stored in the build's `report` JSON, not in a new `content_source_id` column. `issues` is `report.issues`.
- Only `DEMO_PROVISIONING_WEBSITE_CRAWL_LIMIT` sets the crawl limit (no `website_crawl_limit` input). `DEMO_PROVISIONING_EXTRA_ORIGINS` is not built: allowed websites are the demo help center only.
- There is no `indexing` status; `step` is `index_wait` while the build waits.
- A queued build reports `status=queued` without a queue position.
- `refresh_demo` takes no `steps`: it reruns from `website` (website, widget, knowledge, index_wait, verify, crm), so it also applies the widget names and allowed websites to the existing demos. It reuses a website source added by hand when its host matches, with or without `www.`.
- Logo fallback: a header `<img>` logo first, then an inline header SVG (inside the home link, or named "logo") with `currentColor` replaced by the site's theme color, or near-black when there is none. `og:image` is not used: a social card is not a logo.
- `ready` also requires AI replies to be on; otherwise `ready_with_issues` with `ai_replies_off`.

`create_demo` (server/ee/demo, released in #807) builds a demo help center from a prospect's public Intercom help center. We ran it on production for 11 companies on November 15 and 16 (Ticket Tailor, Rebrandly, RotaCloud, Spectora, Recruit CRM, CourtReserve, Nutshell, Lawmatics, Pirate Ship, Dubsado, Carepatron). Every build imported cleanly. But for every demo an operator then had to finish the setup by hand through the REST API before the demo was usable, and some of those steps fix silent failures. This plan moves those steps into `create_demo`, fixes the two server bugs we found, and makes "ready" mean that a prospect can open the link and get answers.

## What happened on production

| Finding | Effect | Seen on |
|---|---|---|
| Help articles were not indexed for the AI after parallel builds. The agent's knowledge search returned 0 results (later, only website pages), hours after the build. Setting the help space as an explicit agent knowledge source and reindexing fixed it within about 15 minutes. | Every question was handed off ("cannot_answer"). | All 10 demos built in parallel (batches of 4 and 5). Not Ticket Tailor, which was built alone. |
| `create_demo` reports `ready` before any AI index exists. | A prospect who opens the link early gets only handoffs. | All |
| The workspace website (`website_url`) is not set, and the company website is not a knowledge source. | Pricing, plan and feature questions that the help center doesn't cover go unanswered. | All |
| The widget is named "<Brand>" and the assistant "<Brand> Assistant". | It reads like the prospect's official bot. We renamed every one to "<Brand> demo support" / "<Brand> demo assistant". | All |
| No light-background logo found, so the build switches to the dark theme. | The header shows only text, or the demo doesn't look like the prospect's site. Fixed by hand by taking the inline SVG logo from the website header. | Ticket Tailor, Pirate Ship |
| `theme_mode` is left as `system`. | The demo looks different for dark-mode visitors. We set `light` by hand. | Most |
| Allowed websites include the prospect's real domain (`https://tickettailor.com`, `https://*.tickettailor.com`). | Our demo widget key can be embedded on the prospect's real site. | Ticket Tailor |
| The number check blocks correct pricing answers (`answer_validation_numeric_mismatch`). | 8 of 9 handoffs in QA, e.g. "$27/mo", "$109/month", "100% free". It gets worse once the website is a knowledge source. | Rebrandly, Spectora, Recruit CRM, Nutshell, Pirate Ship, Dubsado, Carepatron |
| A help center whose home page is public but where the importer detects sign-in gating is refused. | No demo possible. | Kit |
| Only 5 builds can run at once (`MaxUnfinished = 5`). | The sixth `create_demo` call fails, and the caller has to retry. | Batch 02 |

QA results after the manual fixes: 21 of 30 test questions were answered. Of the 9 handoffs, 8 were the number check and 1 was a genuine `cannot_answer` (Lawmatics intake forms).

## Goal

One `create_demo` call produces a demo that needs no manual steps:

- Help center copied, branded, in demo mode (unchanged).
- Workspace website set, and the company website crawled as an AI knowledge source.
- Help center and website both indexed for the support agent before the build finishes.
- Widget named and themed as a demo.
- A verify step has asked real questions, and the results are in the report and the CRM note.
- `status` is `ready` only when the verify step passes. Otherwise it's `ready_with_issues`, with the reasons listed.

Contact research stays agent work (LinkedIn and email patterns) and is out of scope.

## New build steps

Current order: organization → workspace → branding → import → homepage → widget → crm → done.

New order: organization → workspace → branding → import → homepage → **website** → widget → **knowledge** → **index_wait** → **verify** → crm → done.

Each new step is idempotent and safe to resume, like the existing steps. The build stores the step it is on.

### Step `website` (new)

- Input: `website_url` on `create_demo`. The default is `https://<company domain>` from the CRM company. Also accept the company's `website` custom property if present.
- Check that it's HTTPS with no credentials (same rule as `official_docs_url`).
- Save it on the demo workspace with `WorkspaceService.Update(..., UpdateWorkspaceRequest{WebsiteURL})`.
- Create the website content source with `SupportContentSourceService.Create`: `name="<Brand> website"`, `start_url=<website_url>`, `crawl_limit=100`, `crawl_depth=2`, `crawl_source="all"`, `formats=["markdown"]`, `render=true`, `crawl_purposes=["search","ai-input"]`, `max_age_seconds=86400`.
- Store the content source ID on the demo record (new column `content_source_id`) so a resumed build doesn't create a second one.
- Make the crawl limit configurable (`DEMO_PROVISIONING_WEBSITE_CRAWL_LIMIT`, default 100).
- If the crawl can't start (blocked, robots, Cloudflare), don't fail the build. Add a report note, and the verify step reflects it.
- Order matters for allowed websites. `websiteOriginDefaults` adds the workspace website to the widget's allowed websites when the installation is created. Either set `website_url` *after* the installation exists, or have the `widget` step overwrite `allowed_origins` explicitly (see below).

### Step `widget` (changed)

- Names: `widget_name="<Brand> demo support"`, `ai_display_name="<Brand> demo assistant"`. Visitors should be able to tell it's a demo.
- `allowed_origins`: set explicitly to the demo help center origin only, plus the demo landing page origin once that exists (configurable list, `DEMO_PROVISIONING_EXTRA_ORIGINS`). Never the prospect's real domain.
- Link the website content source to the support agent with `AgentContentSourceService.Set(ws, agentID, [content_source_id])`. This step already binds the default agent (`BindDefaultSupportAgent`), so it's the natural place.
- Color scheme: copy the help center theme (already done), and keep it in sync with the theme change in `branding`.

### Step `branding` (changed)

- Never leave `theme_mode=system`: use `light` unless only a dark-background logo exists *and* the logo fallback below also fails.
- **Logo fallback:** when `ProposeBrand` finds no light-background logo, look on the company website (not the help center) for:
  1. the header or home-link `<svg>` (inline): serialize it, replace `currentColor` with the computed color, and upload it as `logo`;
  2. a header `<img>` whose src or alt contains "logo" or the brand name;
  3. `og:image` only as a last resort.
  Upload with the existing helpcenter asset upload (type `logo`) and set the light theme.
- This belongs in `HelpcenterBrandService.ProposeBrand` / a new `ProposeBrandFromWebsite`, so the Settings → Brand import screen benefits too.

### Step `knowledge` (new)

- Set the imported help space as an explicit knowledge source of the support agent with `AgentKnowledgeSourceService.Set(ws, agentID, [spaceID])`, then `Reindex(ws, agentID, spaceID)`.
- This is the workaround for bug B1 below. Keep it after B1 is fixed: an explicit source makes the agent's knowledge visible in Settings and gives an indexing status to wait on.

### Step `index_wait` (new)

- Poll `AgentKnowledgeSourceService.List` and the content source until:
  - the help space source has `sync_status=ready` and `indexed_documents >= articles_imported` (allowing a small tolerance, e.g. 98%);
  - the website source has `sync_status` `ready` or `failed` (a failed website crawl doesn't block the build; it's recorded).
- Release and requeue the build between polls (`release(ctx, demo, delay)`) instead of holding the lease. The 15-minute lease must not run out while waiting.
- Time out after `DEMO_PROVISIONING_INDEX_TIMEOUT` (default 60 minutes). On timeout, continue to `verify` and record "index incomplete".
- Record `indexed_documents`, `indexed_pages` and any sync errors in the report.

### Step `verify` (new)

- Ask 3–5 questions through `SupportPreviewService.Start` with `include_answer=true`, polling the run until it's done (re-check in later step runs, don't hold the lease):
  - 2–3 questions generated from the most important collections, e.g. "How do I <top article title, lowercased and turned into a question>?". Use the article title, not its content, so nothing is leaked.
  - 1 pricing question ("How much does <Brand> cost?") when the website source indexed anything.
  - Optional: caller-supplied questions (`verify_questions` on `create_demo`).
- Pass rule: at least 2 of the generated questions get `final_decision=answer`, and retrieval for those returned `source_type=docs` results.
- Write each question's decision, reason, grounded confidence and source types to the report. Never write the answer text into the CRM note (it can be long), only a short summary.
- Outcome:
  - `ready` when it passes;
  - `ready_with_issues` (new status) otherwise, with `issues[]` such as `index_incomplete`, `website_crawl_failed`, `verify_handoffs:numeric_mismatch`.
- Preview runs count toward usage. Budget at most 5 per build.

### Step `crm` (changed)

- Write `demo_status=ready` or `ready_with_issues`.
- Extend the note: website source pages, indexed documents, verify results (question, answered or handed off, reason), plus the existing import notes.
- Set `demo_website_url` (new custom property) alongside `demo_workspace_url`.

## Tool changes (`create_demo` / `get_demo`)

- `create_demo` input: add `website_url` (optional; defaults from CRM) and `verify_questions` (optional, at most 5).
- `get_demo` output: add `report.website` (`url`, `source_id`, `sync_status`, `indexed_pages`), `report.knowledge` (`indexed_documents`), `report.verify[]`, `issues[]`, and the new statuses `indexing` (for the step) and `ready_with_issues`.
- Build cap: when the cap is reached, queue the build instead of failing. Return `status=queued` with its queue position. Keep `MaxUnfinished` as the concurrency limit for running builds only.
- Add `refresh_demo(demo_id, steps?)`: re-runs `knowledge`, `index_wait` and `verify` on an existing demo without re-importing. Use it to repair the 11 existing demos and to re-check after reindex or validator fixes.
- Optional `website_crawl_limit` input for large marketing sites.

## Server bugs outside the tool

### B1. Imported help centers not indexed for the AI when no explicit knowledge source exists

- Symptom: after `create_demo` (4–5 builds in parallel), `support-preview` retrieval returned no `docs` results for hours. The same check on Ticket Tailor (built alone, also with no explicit knowledge source) returned `docs` results.
- Investigate:
  - how the implicit "all published external spaces" knowledge is indexed after a bulk public import (publish events vs. import completion);
  - whether the indexing job is dropped, deduplicated or starved when several imports finish together;
  - whether the importer publishes through a path that skips the indexing hook.
- Fix the cause, and add a regression test that imports and publishes a space with no explicit knowledge source, then asserts that agent retrieval returns its documents.
- Until it's fixed, the `knowledge` step works around it.

### B2. Number check blocks correct pricing answers

- Covered by `docs/plans/2026-11-11-support-answer-evidence-review.md` (Jev adjudication of flagged claims). Add the 8 production cases to that plan's evaluation set: Rebrandly $27/mo, Spectora $109/month, Nutshell $13/$25/$42/$59 per user, Dubsado $335/year, Pirate Ship "100% free", Carepatron plans, Recruit CRM plans, and Recruit CRM Gmail setup.
- Check specifically that numbers from website content sources (`source_type=content`) count as evidence the same way as docs. Several drafts cited website pages.

### B3. Sign-in-gated detection (Kit)

- `CheckPublicHelpCenter` refused help.kit.com ("asks visitors to sign in"), although the home page and articles are public. Look at what triggered it; it may be an audience-restricted collection or a login link in the Intercom data. Possible changes:
  - import the public articles and skip the gated ones, with a report note;
  - or keep refusing, but allow an Intercom access token as an alternative source.

## Repair of existing demos

After `refresh_demo` ships, run it on all 11 demos to record verify results. The manual fixes already applied (website, content source, knowledge source, widget names, theme, logos) match the new steps, so the steps should find them in place and skip. Check Ticket Tailor's allowed websites by hand.

## Tests

- Unit tests for each new step, using fake services: idempotency on resume, the content source isn't created twice, index_wait releases instead of blocking, verify pass/fail rules, the `ready_with_issues` path.
- Logo fallback: inline SVG with `currentColor`, `<img>` logo, no logo.
- Allowed websites: never contain the company domain.
- Integration test (Postgres): a full build against a fixture Intercom help center and a fixture website, reaching `ready` with indexed counts in the report.
- B1 regression test as above.

## Rollout

1. B1 investigation and fix (or confirm the `knowledge` step as the permanent path).
2. Steps `website`, `widget` changes, `knowledge`, `index_wait`, plus the tool output fields. Deploy and run one build end to end on production.
3. Logo fallback and the `theme_mode` rule.
4. `verify` step, `ready_with_issues`, and the build queue instead of failing on the cap.
5. `refresh_demo`, then run it on the 11 existing demos.
6. B2 through the evidence review plan; then re-run `refresh_demo` and expect pricing questions to pass.
7. B3 when a gated help center matters.

## Done means

- A fresh `create_demo` reaches `ready` with no manual API calls, and 3 test questions answered in the report.
- `get_demo` shows website and knowledge index counts.
- No demo has the prospect's real domain in allowed websites.
- The outreach runbook (`claude/demo-runbook.md` in the project) shrinks to: qualify, research contacts, `create_demo`, read the report.
