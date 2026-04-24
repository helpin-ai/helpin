---
name: competitive_intelligence_digest
description: Researches recent competitor product updates and creates a concise marketing digest task.
metadata:
  title: Competitive Intelligence Digest
  supported_runtimes:
    - native_sdk
---

- Use `update_plan` first and keep it current as you research, synthesize, and create the task.
- Follow any configured target company, target domain, competitor list, lookback window, and task destination in the agent's own instructions. Treat those values as already resolved and authoritative.
- Do not plan or perform a step to discover configuration variables, workspace context, teams, stages, cadence, or lookback settings. Use the configured values directly.
- Use configured competitors when provided. If competitors are configured, do not discover additional competitors. If no competitors are configured, discover competitors with `web_search_exa`; do not invent competitors without sources.
- Use `web_search_exa` to understand the target company, locate competitor changelogs, release notes, product-update pages, product blogs, help-center "What's New" sections, and public roadmap tools.
- For each competitor, extract notable changes shipped within the configured lookback window. Convert all relative dates to absolute `YYYY-MM-DD` dates.
- Skip routine bug fixes and cosmetic tweaks unless they are strategically notable. Mark thin or contradictory evidence as `low_confidence` instead of guessing.
- Write a markdown brief under 500 words with TL;DR, themes, per-competitor highlights, and threats/opportunities for the target company. Cite a source URL for every claim.
- Create exactly one task with `create_task`. Pass the configured destination team directly as `team_id`; include the configured destination state directly as `state_id` only when it is present. Do not call team or workflow listing tools to resolve these IDs first.
- Use task type `chore`. Set priority `high` if any competitor shipped something directly overlapping the target's core positioning, `medium` for notable changes, and `low` if no notable changes were found.
- Include acceptance criteria in the task description because `create_task` does not accept a separate acceptance-criteria field.
- Include an appendix in the task description listing every competitor considered and any `no_public_changelog` or `low_confidence` findings.
- After `create_task` returns, include the created task ID or ref in the final response.
