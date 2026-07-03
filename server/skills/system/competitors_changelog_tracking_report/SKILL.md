---
name: competitors_changelog_tracking_report
description: Researches recent competitor changelog and product updates and produces a concise source-backed tracking report.
metadata:
  title: Competitors Changelog Tracking Report
  supported_runtimes:
    - native_sdk
  required_tools:
    - update_plan
    - web_search_exa
    - fetch_url
    - crawl_url
    - create_task
---

- Use `update_plan` first and keep it current as you research, synthesize, and create the configured output.
- Follow any configured target company, target domain, competitor list, lookback window, cadence, output destination, and follow-up-task behavior in the agent's own instructions. Treat those values as already resolved and authoritative.
- Do not plan or perform a step to discover configuration variables, workspace context, teams, stages, cadence, lookback settings, or output destination. Use the configured values directly.
- Use configured competitors when provided. If competitors are configured, do not discover additional competitors. If no competitors are configured, discover competitors with `web_search_exa`; do not invent competitors without sources.
- Use `web_search_exa` to understand the target company and find official competitor changelogs, release notes, product-update pages, product blogs, help-center "What's New" sections, and public roadmap tools.
- Use `fetch_url` on each exact source URL before citing it. Do not cite a search result unless the fetched page content supports the claim.
- If search results are thin or a competitor's update page is likely under a docs/blog subdomain, use `crawl_url` on the official site or docs host with changelog/update keywords before marking `no_public_changelog`.
- For each competitor, extract notable changes shipped within the configured lookback window. Convert all relative dates to absolute `YYYY-MM-DD` dates.
- Skip routine bug fixes and cosmetic tweaks unless they are strategically notable. Mark thin or contradictory evidence as `low_confidence` instead of guessing.
- Write a concise markdown report with: TL;DR, notable changes by competitor, source links, relevance to the target company, customer-facing implications, and recommended messaging or product follow-ups. Do not start the body by repeating the document title.
- Use the output/storage tool requested by the agent or flow instructions. This skill does not assume the report belongs in Docs, a task, or any other destination unless the current run config says so.
- When the configured destination is Docs, title the report `Competitors changelog tracking report - <scope/context> - YYYY-MM-DD`, call `create_document` exactly once with the complete report content, and do not call a separate write-content tool.
- If asked to create a task, use task type `chore`. Set priority `high` if any competitor shipped something directly overlapping the target's core positioning, `medium` for notable changes, and `low` if no notable changes were found.
- If asked to create follow-up tasks, keep them focused on concrete action from notable competitor changes; do not create tasks for every source you checked.
- Include an appendix in the report listing every competitor considered and any `no_public_changelog` or `low_confidence` findings.
- After creating or updating the configured output, include the created object ID, URL, or ref in the final response when the tool returns one.
