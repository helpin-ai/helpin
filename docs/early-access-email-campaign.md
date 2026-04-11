# Early Access Welcome Sequence — Customer.io Campaign

## Setup Instructions

### Step 1: Create Campaign
- Go to Customer.io → Campaigns → Create Campaign
- Name: **Early Access Welcome Sequence**
- Trigger: **Event-triggered**
- Event name: **early_access_signup**
- Segment filter: None (all signups)

### Step 2: Add 3 emails with delays

---

## Email 1: Welcome (Immediately)

**Subject:** You're on the list — here's what we're building

**From:** Waqar from Helpin <hello@helpin.ai>

**Body:**

```
Hey {{customer.first_name | default: "there"}},

Waqar here, one of the founders of Helpin. Thanks for signing up for early access — it means a lot.

Here's the short version of what we're building:

Helpin is a single system that replaces your project management, support, CRM, and docs tools — with AI agents built in that actually do the work.

Not another AI wrapper. Not another tool to add to the stack. One connected platform where agents plan features, triage support tickets, follow up on deals, and keep docs updated — across every team, automatically.

We're building this because we lived the problem. Our team was drowning in Jira, Intercom, Notion, HubSpot, Slack, and ChatGPT — all disconnected, all losing context. We decided to fix it.

You'll be among the first to try it. I'll personally reach out when your access is ready.

In the meantime, reply to this email if you have questions or if there's a specific problem you're hoping Helpin solves. I read every reply.

— Waqar
Co-founder, Helpin
```

---

## Email 2: The Problem (Day 3)

**Subject:** The real cost of disconnected tools

**Delay:** 3 days after Email 1

**From:** Waqar from Helpin <hello@helpin.ai>

**Body:**

```
Hey {{customer.first_name | default: "there"}},

Quick question — how many tools does your team use daily?

Most teams I talk to are running 5-8 separate tools. Jira for tasks, Notion for docs, Intercom for support, HubSpot for sales, Slack for everything else.

Each tool works fine on its own. The problem is what happens between them:

- A support ticket comes in, but the engineer doesn't know the customer context
- A feature ships, but the docs don't get updated
- A decision gets made in Slack, but nobody can find it two weeks later
- AI tools help individually, but they can't see across your systems

That's the gap we're closing with Helpin.

Instead of connecting 6 tools with integrations and duct tape, you get one system where everything — tasks, tickets, deals, docs, and AI agents — shares the same context.

It's not about adding another tool. It's about removing five.

More soon.

— Waqar
```

---

## Email 3: The Invite (Day 7)

**Subject:** Ready to try Helpin?

**Delay:** 7 days after Email 1

**From:** Waqar from Helpin <hello@helpin.ai>

**Body:**

```
Hey {{customer.first_name | default: "there"}},

We're getting close to opening up early access, and your spot is reserved.

Here's what you'll get when you're in:

→ Full platform: PM, Support, Sales/CRM, and Docs — all connected
→ AI agents that plan epics, triage tickets, draft docs, and follow up on deals
→ Import your existing data from Jira, Notion, Intercom, or HubSpot
→ Unlimited seats for your team
→ Free to start — no credit card needed

If you want to get started right away, you can create your workspace now:

👉 https://app.helpin.ai

Or if you'd rather I walk you through it personally, just reply to this email and we'll set up a quick call. No sales pitch — I genuinely want to hear what problems you're trying to solve.

Either way, I'm here if you need anything.

— Waqar
Co-founder, Helpin

P.S. If you know someone else who's frustrated with their tool stack, feel free to forward this. Early access is open.
```

---

## Campaign Settings

| Setting | Value |
|---------|-------|
| Sender | Waqar Azeem <hello@helpin.ai> (ID: 1) |
| Reply-to | hello@helpin.ai |
| Trigger event | early_access_signup |
| Email format | Plain text (no HTML template) |
| Unsubscribe | Include Customer.io default unsubscribe link |
| Conversion goal | User creates account (track `signed_up` event) |
| Exit condition | User triggers `signed_up` event (stop sending if they already signed up) |

## Campaign Flow

```
[early_access_signup event]
    ↓
[Email 1: Welcome] — immediately
    ↓ (3 day delay)
[Email 2: The Problem] — day 3
    ↓ (4 day delay)  
[Email 3: The Invite] — day 7
    ↓
[End]
```

## Exit Conditions
- If user signs up for Helpin (triggers `signed_up` event) → exit campaign
- If user unsubscribes → exit campaign

## How to Set Up in Customer.io Dashboard

1. **Campaigns → Create Campaign**
2. Select **Event Triggered** → event name: `early_access_signup`
3. Add **Email 1** → paste subject + body above → set sender to "Waqar Azeem"
4. Add **Delay: 3 days**
5. Add **Email 2** → paste subject + body
6. Add **Delay: 4 days**
7. Add **Email 3** → paste subject + body
8. Set **Conversion goal** → event: `signed_up`
9. Set **Exit conditions** → event: `signed_up`
10. **Start campaign**
