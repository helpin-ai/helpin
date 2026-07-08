# Helpin Pricing Strategy

## Section 1: Recommended Pricing Strategy

**Core principle: Platform pricing by company stage, not per seat.**

Helpin replaces 4-6 tools. Pricing should reflect the value of consolidation + AI, not the number of humans typing. Per-seat pricing punishes growth and makes buyers do math — both kill conversion.

**The right model for Helpin:**
- Flat monthly platform fee per plan
- Generous user allowances (not per-seat billing)
- AI credits included in every plan, with clear overage pricing
- 3 plans + Enterprise
- Annual discount (20%) to improve retention and cash flow

**Why this works for Helpin specifically:**
1. Buyers are comparing against 4-6 separate tool subscriptions — Helpin needs to feel like one price for everything
2. AI usage is variable — some months heavy, some light. Credits with included allowance handles this cleanly
3. The "platform fee" framing positions Helpin as infrastructure, not another SaaS tool
4. Generous user limits remove the "let me count my team" friction from buying decisions

---

## Section 2: Three Pricing Model Options

### Option A: Platform Tiers (Recommended)

| | Starter | Growth | Business | Enterprise |
|---|---------|--------|----------|------------|
| Price | $299/mo | $799/mo | $1,999/mo | Custom |
| Users | Up to 10 | Up to 30 | Up to 100 | Unlimited |
| AI Credits | 5,000/mo | 25,000/mo | 100,000/mo | Custom |
| Workspaces | 1 | 3 | Unlimited | Unlimited |

**Pros:**
- Dead simple to understand — pick your company size
- No per-seat math
- AI included, not gated
- Easy to upsell (more users, more credits)
- Feels premium but fair

**Cons:**
- Revenue doesn't scale linearly with team size (mitigated by plan tiers)
- Some large teams on small plans (mitigated by user caps)

### Option B: Per-Seat + Platform Fee

| | Starter | Growth | Business |
|---|---------|--------|----------|
| Platform fee | $99/mo | $299/mo | $799/mo |
| Per seat | $15/seat/mo | $12/seat/mo | $10/seat/mo |
| AI Credits | 2,000/mo | 10,000/mo | 50,000/mo |

**Pros:**
- Revenue scales with team size
- Lower entry point

**Cons:**
- Adds complexity ("what's my total?")
- Per-seat feels like what they're leaving behind (Jira, HubSpot)
- Undermines the "one price for everything" message
- Buyers hate per-seat — it's the #1 complaint about SaaS pricing

### Option C: Usage-Based (AI-First)

| | Base | Pro | Scale |
|---|------|-----|-------|
| Price | $199/mo | $599/mo | $1,499/mo |
| Users | Unlimited | Unlimited | Unlimited |
| AI Credits | 3,000/mo | 15,000/mo | 75,000/mo |
| Overage | $0.05/credit | $0.04/credit | $0.03/credit |

**Pros:**
- Unlimited users is a powerful headline
- AI usage drives revenue
- Low entry, scales with value

**Cons:**
- Unpredictable bills (buyers hate this)
- Hard to budget for
- AI credits become the thing buyers worry about instead of using
- Feels like a cloud infrastructure bill, not a product

---

## Section 3: Recommended Final Pricing Model

**Go with Option A: Platform Tiers.**

Here's the refined structure:

### Founder — internal, non-Stripe plan
*For organizations that existed before the billing launch cutoff.*

- Hidden from public pricing and checkout.
- Applies to all workspaces in organizations that existed at the cutoff.
- New workspaces created later inside those Founder organizations also receive Founder.
- All modules and all Growth-gated features.
- Unlimited teams, workspaces, CRM contacts, and documents.
- 100,000 AI credits/month.
- Credits reset monthly with no rollover.
- Managed by Helpin, not Stripe.
- Billing area should show "Founder plan"; Stripe checkout, plan changes, customer portal, payment method management, and on-demand billing controls should be disabled or hidden.
- Implementation uses `plan = founder` on `workspace_billing` plus `organization_billing.founder_plan_enabled` for inheritance by future workspaces in Founder organizations.

### Starter — $299/month
*For small teams getting started with one connected system.*

- Up to 10 users
- 1 workspace
- All modules (PM, Support, Sales, Docs)
- Built-in AI agents
- 5,000 AI credits/month
- Community support
- GitHub integration

### Growth — $799/month
*For growing companies replacing their tool stack.*

- Up to 30 users
- 3 workspaces
- Everything in Starter
- Custom AI agents
- 25,000 AI credits/month
- Priority support
- Advanced automations
- API access
- Import from Jira, Notion, Intercom, HubSpot

### Business — $1,999/month
*For teams that want full AI-powered operations.*

- Up to 100 users
- Unlimited workspaces
- Everything in Growth
- 100,000 AI credits/month
- Dedicated success manager
- SSO / SAML
- Advanced RBAC
- Audit logs
- Custom integrations
- SLA guarantee

### Enterprise — Custom
*For organizations with complex requirements.*

- Unlimited users
- Custom AI credit volume
- Dedicated infrastructure
- Custom SLAs
- Onboarding & migration support
- MSA / custom contracts
- Volume discounts

**Why this is the right model:**

1. **Simple** — 4 options, clear user/credit limits, no math
2. **Competitive** — $299/mo for 10 users replaces $500-1500/mo in separate tools
3. **Scalable** — revenue grows with company size naturally
4. **AI-native** — credits are included, not an afterthought
5. **Premium** — pricing says "operating system" not "another tool"

---

## Section 4: Feature Packaging by Plan

### What should NOT be feature-gated aggressively:
- **All core modules** (PM, Support, Sales, Docs) — available on every plan. Gating modules kills the "one system" story.
- **Built-in AI agents** — available on every plan. AI is the differentiator, not a premium add-on.
- **Basic automations** — available on every plan.
- **Mobile access** — available on every plan.
- **Import tools** — available on every plan. Don't make switching harder.

### What SHOULD be gated by plan:

| Feature | Starter | Growth | Business | Enterprise |
|---------|---------|--------|----------|------------|
| Core modules (PM, Support, Sales, Docs) | ✓ | ✓ | ✓ | ✓ |
| Built-in AI agents | ✓ | ✓ | ✓ | ✓ |
| Custom AI agents | — | ✓ | ✓ | ✓ |
| Agent scheduling & cron | — | ✓ | ✓ | ✓ |
| Workspaces | 1 | 3 | Unlimited | Unlimited |
| Users | 10 | 30 | 100 | Unlimited |
| AI credits/month | 5,000 | 25,000 | 100,000 | Custom |
| GitHub integration | ✓ | ✓ | ✓ | ✓ |
| API access | — | ✓ | ✓ | ✓ |
| Advanced automations | — | ✓ | ✓ | ✓ |
| Help center (public) | ✓ | ✓ | ✓ | ✓ |
| Custom domain (help center) | — | ✓ | ✓ | ✓ |
| Import tools | ✓ | ✓ | ✓ | ✓ |
| Priority support | — | ✓ | ✓ | ✓ |
| Dedicated success manager | — | — | ✓ | ✓ |
| SSO / SAML | — | — | ✓ | ✓ |
| Audit logs | — | — | ✓ | ✓ |
| Advanced RBAC | — | — | ✓ | ✓ |
| Custom SLA | — | — | — | ✓ |
| Dedicated infrastructure | — | — | — | ✓ |

### Internal Founder packaging

Founder is not part of the public pricing table, but in product behavior it should be treated as equal to or better than Growth:

- Custom AI agents: enabled
- Automation flows: enabled
- Agent scheduling and cron: enabled
- Teams: unlimited
- Workspaces inside Founder organizations: unlimited by plan
- CRM contacts: unlimited
- Documents: unlimited
- AI credits/month: 100,000
- Stripe billing actions: disabled/hidden because Founder is managed by Helpin
- Migration: `202606230001_founder_plan.sql` marks current organizations as Founder-enabled and backfills current workspace billing rows.

---

## Section 5: AI Credits Model

### What is an AI credit?

**Commercially:** 1 AI credit = 1 unit of AI work. Each agent action consumes credits based on complexity.

**Technically:** Credits map to token usage across the AI provider (Claude, GPT, etc.). But never expose this to customers — they don't care about tokens.

### Credit consumption examples (for the pricing page):

| Action | Approximate credits |
|--------|-------------------|
| Triage a support ticket | ~5 credits |
| Plan an epic from a brief | ~50 credits |
| Generate a document draft | ~30 credits |
| Code review (single PR) | ~20 credits |
| Search knowledge base | ~2 credits |
| Auto-reply to customer | ~10 credits |
| Full sprint planning session | ~100 credits |

### How to present credits:

**On the pricing page:**
> "Every plan includes AI credits. Credits are consumed when agents work — triaging tickets, planning features, drafting docs, and more. Most teams never exceed their included credits."

**Key messaging:**
- Credits are INCLUDED, not extra
- Show what 5,000 credits means in real terms: "~1,000 support triages or ~100 epic plans per month"
- Overage is available, not punitive: "$0.05/credit beyond your plan"
- Credits reset monthly — no rollover (keeps it simple)

### BYO Keys consideration:
Since users bring their own API keys, the "AI credits" are really about Helpin's orchestration, context loading, and tool execution — not raw model costs. This is important: you're charging for the AGENT WORK, not the API call. Frame it that way.

---

## Section 6: Website Pricing Page Copy

### Hero
**Headline:** "One platform. One price. No per-seat surprises."

**Subheadline:** "Replace your project management, support, CRM, and docs tools with one AI-powered system. Every plan includes AI agents that do the work."

### Plan Names & Descriptions

**Starter — $299/mo**
"For small teams replacing their first set of disconnected tools."

**Growth — $799/mo** ← MOST POPULAR badge
"For growing companies that want AI agents across every team."

**Business — $1,999/mo**
"For organizations running their entire operation on Helpin."

**Enterprise — Custom**
"For companies with complex compliance, security, or scale requirements."

### Top Features Per Plan (for the pricing cards)

**Starter:**
- Up to 10 users
- All modules included
- Built-in AI agents
- 5,000 AI credits/month
- GitHub integration
- 1 workspace

**Growth:**
- Up to 30 users
- Custom AI agents
- 25,000 AI credits/month
- Advanced automations
- API access
- 3 workspaces
- Priority support

**Business:**
- Up to 100 users
- 100,000 AI credits/month
- SSO / SAML
- Dedicated success manager
- Audit logs
- Unlimited workspaces
- SLA guarantee

**Enterprise:**
- Unlimited everything
- Custom AI credit volume
- Dedicated infrastructure
- Custom contracts
- Migration support

### AI Credits Section

**Headline:** "AI credits, explained simply."

**Copy:**
"Every plan includes AI credits that power your agents. When an agent triages a ticket, plans a sprint, or drafts a document — it uses credits. Most teams never hit their limit.

Need more? Add credit packs anytime, or upgrade your plan. No surprises, no throttling, no hidden costs."

**Visual:** Show a simple table of "what X credits gets you" with real examples.

### Comparison Section

**Headline:** "Everything you need. Nothing you don't."

**Subheadline:** "All plans include every module — PM, Support, Sales, and Docs. No feature walls between your teams."

### Final CTA

**Headline:** "Start with Starter. Scale when you're ready."

**Subheadline:** "Start with a 14-day Helpin trial. Upgrade when you're ready to keep access."

**Button:** "Start trial"
**Secondary:** "Talk to sales"

---

## Section 7: FAQs

**Q: Do I need to buy separate modules?**
A: No. Every plan includes all modules — PM, Support, Sales, and Docs. We don't sell features separately.

**Q: What happens if I exceed my AI credit limit?**
A: Your agents don't stop working. We'll notify you and any overage is billed at a simple per-credit rate. You can also add credit packs anytime.

**Q: Can I change plans anytime?**
A: Yes. Upgrade or downgrade at any time. Changes take effect on your next billing cycle.

**Q: Do you offer annual billing?**
A: Yes. Annual plans save 20% compared to monthly billing.

**Q: What AI models does Helpin use?**
A: You bring your own API keys. Helpin supports Claude, GPT, and other providers. You choose the model per agent.

**Q: Is there a free trial?**
A: Yes. New workspaces start on a 14-day Helpin-managed trial with no card required. Upgrade through Stripe Checkout during or after the trial to keep access.

**Q: How do AI credits work with BYO API keys?**
A: AI credits cover Helpin's agent orchestration — the planning, context loading, tool execution, and coordination across modules. Your API key covers the model inference cost. Credits represent the value of the agent doing the work, not just the API call.

**Q: Can I add more users without upgrading?**
A: User limits are per plan. If you need more users, upgrade to the next plan or contact us for a custom arrangement.

**Q: What's included in Priority Support?**
A: Priority support includes faster response times, direct access to the engineering team, and a dedicated Slack channel (Growth and above).

**Q: Do unused credits roll over?**
A: Credits reset monthly. We keep it simple — your included credits refresh at the start of each billing cycle.

---

## Section 8: Risks & Things to Watch Out For

### 1. Starter price may feel high for very early-stage teams
**Risk:** $299/mo is a real commitment for a 3-person startup.
**Mitigation:** Offer a generous free trial (14 days). Consider a "Solo" plan at $99/mo for 1-3 users if you see drop-off at signup. Don't launch with it — add it later if data shows you need it.

### 2. AI credits can create anxiety
**Risk:** Users worry about running out and stop using agents.
**Mitigation:** Make the included allowance generous enough that 80%+ of customers never exceed it. Show credit usage in-app but don't make it scary. Use "soft limits" — agents keep working, you notify and bill later.

### 3. BYO keys + credits = double cost perception
**Risk:** "I'm paying for my API keys AND your credits?"
**Mitigation:** Frame credits as "agent orchestration" not "AI usage." The credit pays for Helpin doing the work — reading your tickets, checking your roadmap, updating your docs. The API key pays for the model. Two different things. Be clear on the pricing page.

### 4. Enterprise deals may stall without a sales team
**Risk:** "Custom pricing" with no one to talk to = lost deals.
**Mitigation:** Add a Calendly link on the Enterprise card. Respond to Enterprise inquiries within 24 hours. Even if it's just the founder on the call.

### 5. Don't discount too early or too aggressively
**Risk:** First customers will push for discounts. Discounting trains the market to wait.
**Mitigation:** Offer "founding member" pricing (e.g., 30% off for the first year) instead of permanent discounts. This rewards early adopters without setting a low anchor.

### 6. Watch the Growth → Business gap
**Risk:** $799 → $1,999 is a big jump. Some companies with 30-50 users may feel stuck.
**Mitigation:** Consider a "Growth Plus" at $1,299 if you see clustering at the Growth cap. Don't launch with it — add it based on data.

### 7. Don't launch with all plans day one
**Risk:** Over-engineering pricing before you have customers.
**Recommendation:** Launch with just **Starter + Growth** and "Contact us" for larger teams. Add Business and Enterprise when you have real demand signals. This reduces complexity and lets you learn what buyers actually want before committing to a full pricing structure.
