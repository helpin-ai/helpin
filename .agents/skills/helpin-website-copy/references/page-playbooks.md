# Page playbooks

Read the entry for the actual page. These priorities guide the story inside its existing structure; they are not a required route list, section plan, or feature catalog.

## 1. Homepage

Introduce what agents help accomplish and why shared history improves that work. Give the loop a concrete earlier event that changes the action. History cards explain what is known; product tabs explain what people can do. Keep those jobs distinct. Preserve the current approved hero rather than automatically reverting to an earlier slogan.

## 2. Customer support

Make better answers, useful handoffs, and connected follow-up tangible. Keep everyday inbox depth visible where supported: chat, email, ownership, routing, tags, views, and reply tools. Reuse an existing task when the issue is already tracked. A customer notification follows confirmed release, not merely a proposed fix.

## 3. Meetings

The call becomes history and reviewed commitments become work. Distinguish summaries, agreements, unresolved questions, and suggested actions. Source statements to speakers and timestamps in the fictional demo. Do not invent a deadline or turn an open question into a promise. Show capture settings and recording limits only as verified.

## 4. Projects

Present full project management, including internal work and maintenance. Explain priorities, dependencies, objectives, roadmap, sprints, and delivery where supported. Customer context informs relevant work; it does not define every project. Keep delivery progress separate from measured outcomes and code review separate from release.

## 5. CRM

Connect the commercial record with the conversation behind it. Account briefings, pipeline stages, signals, and playbooks should explain the next decision. A buying signal needs review; resolving a support issue does not automatically win a renewal. Preserve ordinary contact/company management as well as agent assistance.

## 6. Knowledge

Explain the reader experience, publishing options, developer docs, selected agent sources, and maintenance in their existing sections. Separate public publishing from internal documents and agent-source selection. Show draft, publication, and source refresh as different states. Verify performance, reverse-proxy, and API-reference details rather than inferring parity with another vendor.

## 7. AI Agents

Lead with useful work informed by history. Distinguish Ask Agent from focused specialists and validate current names/roles/counts. Demonstrate parallel independent investigations followed by dependent next steps. Keep permissions and action-specific approvals visible without claiming universal approval or unrestricted autonomy.

## 8. Developers

Make each interface's purpose and access boundary clear: in-product SDK, authenticated routes, AI-client access to Helpin, external tools used by agents, and event-triggered work. Preserve runnable snippets unless verified replacements are needed. Verify release versions and availability. Never present a widget key as an API session credential.

## 9. Open source and self-hosting

Explain the full offered product within the confirmed release scope, who operates it, and the services required. Keep deployment and external connections visually/conceptually distinct. The primary setup action should reach the intended installation instructions. Module inclusion, license, production readiness, and provider costs require separate evidence.

## 10. Pricing

Help visitors compare the actual offer: price, billing unit, included capacity, usage, plan gates, and deployment choice. Use platform language for positioning when appropriate while retaining the correct billing unit. Concision must not remove material conditions. Do not invent savings percentages or a comparison calculator's assumptions.

## 11. Comparison hub and “Helpin vs X” pages

The hub (`/compare`) lets a visitor weigh Helpin against the tools they use; each `/compare/<slug>` page is a long-form guide a buyer can trust. Honesty is the product here: a page that hides the other tool's strengths loses the reader.

- Lead with the difference the buyer will feel. For support tools, that's the four advantages in [positioning and voice](positioning-and-voice.md); for trackers such as Linear, Plane, and Jira, it's projects with the customer attached.
- Name both editions: open source on the Community edition, or Helpin Cloud.
- State competitor facts only from their own pricing and documentation, checked on a recorded date (`CHECKED` in `compare-data.ts`). Keep source URLs in the data's `sources` field; they are not shown on the page.
- Keep “Where X is stronger” honest and specific, and say when the other tool costs less. The calculator uses list prices only.
- Refer to other products by name only: no logos, marks, or letter badges.
- Keep the byline and trademark line: written by the Helpin team, verified date, “X is a trademark of its owner; Helpin is not affiliated with it.”
- Titles follow “Open-Source X Alternative: Helpin vs X (year)” when that fits in 60 characters; otherwise “Helpin vs X: Open-Source Alternative”.

Adding a competitor touches `compare-data.ts` (facts, table, calculator, FAQs), `article-data.ts` (the guide's prose), `matrix-data.ts` (hub table), the hero video and poster in `website/public/new/compare/`, and an OG card in `website/scripts/generate-og-images.mjs`. The sitemap and the footer's Compare column pick the new page up automatically.

## 12. Release announcements and changelog

When the team adds a changelog or release posts, write one post per public release, titled with the version and its two or three headline changes. Open with a one-line summary, give each headline change a short section with a product visual and a link to try it or read the docs, then list smaller improvements and fixes. Add upgrade notes for self-hosters (commands and breaking changes) and credit community contributors. Link to the full GitHub release notes rather than pasting the pull-request list.

## Future pages and “next page”

Read an explicit sequence and current progress first. Otherwise inspect real routes, identify an unhandled page, and explain the choice. Record source files, audience, intended action, neighboring overlap, and one distinct promise before drafting. Inventory existing sections and slots instead of applying another page's layout. Only a human approval record establishes approval; deployed code alone does not prove editorial sign-off.
