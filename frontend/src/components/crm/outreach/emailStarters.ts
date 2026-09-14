import type { SequenceStep } from "@/lib/crmOutreachTypes";
export interface StarterField {
  key: string;
  label: string;
  hint: string;
}
export interface EmailStarter {
  id: string;
  name: string;
  category: "Prospecting" | "Follow-up" | "Customers";
  when: string;
  fields: StarterField[];
  emails: {
    label: string;
    delay: number;
    subject: string;
    paragraphs: string[];
  }[];
}
const field = (key: string, label: string, hint: string): StarterField => ({
  key,
  label,
  hint,
});
const topic = field(
  "topic",
  "Topic",
  "A specific priority, e.g. simplifying campaign approvals",
);
const product = field(
  "product",
  "Product or service",
  "Your actual product or service name",
);
const tip = field(
  "tip",
  "Useful follow-up insight",
  "One practical suggestion you can stand behind",
);
const emails = (
  first: string[],
  second: string[],
  last: string[],
  subjects: string[],
  delays = [0, 3, 5],
): EmailStarter["emails"] =>
  [first, second, last].map((paragraphs, i) => ({
    label: ["Opening email", "Helpful follow-up", "Close the loop"][i],
    delay: delays[i],
    subject: subjects[i],
    paragraphs,
  }));
// Original Helpin copy. Vendor research informs the scenario structure, not copied wording.
export const emailStarters: EmailStarter[] = [
  {
    id: "cold-introduction",
    name: "Relevant cold introduction",
    category: "Prospecting",
    when: "For a carefully selected prospect whose role makes this topic relevant. Personalize the reason before sending.",
    fields: [
      field("sender_company", "Your company", "The business you represent"),
      topic,
      field(
        "value",
        "How you help",
        "A specific, truthful description, e.g. reduce manual reporting",
      ),
      tip,
    ],
    emails: emails(
      [
        "Reaching out about [[topic]]. At [[sender_company]], we help teams [[value]].",
        "Is this something your team is working on, or should I leave it here?",
      ],
      [
        "One practical thought on [[topic]]: [[tip]]",
        "Would a short example be useful?",
      ],
      [
        "I’ll pause my outreach about [[topic]] here. If it becomes relevant later, you’re welcome to reply and we can pick it up then.",
      ],
      [
        "A question about [[topic]]",
        "An idea for [[topic]]",
        "Leaving this with you",
      ],
    ),
  },
  {
    id: "inbound-inquiry",
    name: "Inbound inquiry",
    category: "Prospecting",
    when: "For someone who explicitly requested information about your product or service.",
    fields: [product, topic, tip],
    emails: emails(
      [
        "Thanks for asking about [[product]]. I can help you explore [[topic]].",
        "What would you most like to understand first?",
      ],
      [
        "A useful starting point for [[topic]]: [[tip]]",
        "Would you prefer a brief walkthrough or an answer by email?",
      ],
      [
        "I’ll leave your inquiry about [[product]] with you for now. Reply whenever you’d like to continue; there’s no need to book time before we know what would help.",
      ],
      [
        "Your question about [[product]]",
        "A starting point for [[topic]]",
        "Whenever you’re ready",
      ],
      [0, 2, 4],
    ),
  },
  {
    id: "post-demo",
    name: "After a demo",
    category: "Follow-up",
    when: "After a completed demo. Use the customer’s actual priorities and the next step you discussed.",
    fields: [
      product,
      field("priority", "Customer priority", "What they told you matters most"),
      field(
        "next_step",
        "Proposed next step",
        "A concrete next step, without an invented deadline",
      ),
      tip,
    ],
    emails: emails(
      [
        "Thank you for the conversation about [[product]]. My understanding is that your priority is [[priority]].",
        "A useful next step would be [[next_step]]. Does that match what you have in mind?",
      ],
      [
        "I’ve been thinking about [[priority]]. One detail that may help your evaluation: [[tip]]",
        "What would you need to resolve before deciding on the next step?",
      ],
      [
        "I’ll pause the follow-up on [[product]] so you have space to evaluate. If the priority or timing has changed, a quick note is enough to help me adjust.",
      ],
      [
        "Next steps after our demo",
        "For your evaluation",
        "Should we revisit this later?",
      ],
      [0, 2, 5],
    ),
  },
  {
    id: "proposal",
    name: "Proposal follow-up",
    category: "Follow-up",
    when: "After a proposal has actually been shared. Include a real, accessible proposal reference.",
    fields: [
      field(
        "proposal",
        "Proposal reference",
        "The proposal title or its accessible URL",
      ),
      field("outcome", "Intended outcome", "The agreed business outcome"),
      tip,
    ],
    emails: emails(
      [
        "I’m following up on [[proposal]], which outlines our approach to [[outcome]].",
        "What questions would be most useful to work through before you decide?",
      ],
      [
        "One consideration for [[outcome]]: [[tip]]",
        "Is there anything in the scope or approach that needs adjusting?",
      ],
      [
        "I’ll pause my follow-up on [[proposal]]. If you’re still evaluating it, let me know what timing works for you and I’ll follow your lead.",
      ],
      [
        "Questions on the proposal",
        "A consideration for your decision",
        "Pausing proposal follow-up",
      ],
      [0, 3, 5],
    ),
  },
  {
    id: "missed-meeting",
    name: "Reschedule a missed meeting",
    category: "Follow-up",
    when: "For a meeting that did not take place. Keep the tone understanding and avoid assuming why.",
    fields: [
      topic,
      field(
        "scheduling",
        "How to reschedule",
        "An actual booking URL or a short scheduling instruction",
      ),
    ],
    emails: emails(
      [
        "It looks like we didn’t get a chance to connect about [[topic]]. These things happen.",
        "If another time would help, [[scheduling]].",
      ],
      [
        "We can also cover [[topic]] by email if that is easier.",
        "What is the main question you wanted to discuss?",
      ],
      [
        "I’ll leave rescheduling with you. If you’d still like to discuss [[topic]], reply whenever the timing works.",
      ],
      [
        "Another time to connect?",
        "Email may be easier",
        "Leaving rescheduling with you",
      ],
      [0, 2, 5],
    ),
  },
  {
    id: "event-follow-up",
    name: "After an event conversation",
    category: "Follow-up",
    when: "For someone you actually spoke with at an event, using the topic you discussed.",
    fields: [
      field("event", "Event name", "The event where you met"),
      topic,
      tip,
    ],
    emails: emails(
      [
        "I enjoyed our conversation at [[event]] about [[topic]].",
        "Would it be useful to continue that discussion by email?",
      ],
      [
        "A thought to add to our conversation: [[tip]]",
        "Does that fit what you’re working toward with [[topic]]?",
      ],
      [
        "I’ll pause here, but I’m glad we connected at [[event]]. You’re welcome to reach out if [[topic]] comes back into focus.",
      ],
      [
        "Our conversation at [[event]]",
        "One thought after [[event]]",
        "Good to have connected",
      ],
      [0, 4, 6],
    ),
  },
  {
    id: "reactivation",
    name: "Revisit an earlier conversation",
    category: "Prospecting",
    when: "For a genuine earlier conversation where timing was not right. Include a real reason to reconnect.",
    fields: [
      topic,
      field(
        "change",
        "What is new",
        "A relevant, verified change since your last discussion",
      ),
      tip,
    ],
    emails: emails(
      [
        "We previously discussed [[topic]]. Since then, [[change]]",
        "Is this worth revisiting, or is it still not a priority?",
      ],
      [
        "One useful consideration if [[topic]] is back on your list: [[tip]]",
        "Would a brief update help you assess whether anything has changed?",
      ],
      [
        "I’ll leave our earlier discussion about [[topic]] on pause. If your priorities shift, feel free to reply and we can start from what you need then.",
      ],
      [
        "Worth revisiting [[topic]]?",
        "A useful consideration",
        "Keeping this on pause",
      ],
      [0, 5, 7],
    ),
  },
  {
    id: "renewal",
    name: "Start a renewal conversation",
    category: "Customers",
    when: "For an existing customer approaching a confirmed renewal. These delays are relative to enrollment, not the renewal date.",
    fields: [
      product,
      field(
        "renewal_date",
        "Confirmed renewal date",
        "An actual date, e.g. 30 November 2026",
      ),
      field(
        "review_topic",
        "Review focus",
        "The customer outcome or usage you want to review",
      ),
    ],
    emails: emails(
      [
        "Your [[product]] renewal is approaching on [[renewal_date]]. I’d like to make sure the next term reflects what your team needs.",
        "Would it be helpful to review [[review_topic]] together?",
      ],
      [
        "Ahead of the [[product]] renewal, we can review what is working, what needs to change, and any questions about the next term.",
        "What should we prioritize in that conversation?",
      ],
      [
        "A final note from me in this sequence ahead of [[renewal_date]]. I’m available to help with questions about [[product]] whenever you’re ready.",
        "What would make the renewal decision easier for your team?",
      ],
      [
        "Planning your [[product]] renewal",
        "What should we review?",
        "Here to help with your renewal",
      ],
      [0, 5, 7],
    ),
  },
];
const escapeHTML = (value: string) =>
  value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
export function buildStarterSteps(
  starter: EmailStarter,
  values: Record<string, string>,
  preview = false,
): SequenceStep[] {
  const render = (text: string, html = false) =>
    text.replace(/\[\[([a-z_]+)\]\]/g, (_token, key: string) => {
      const field = starter.fields.find((f) => f.key === key);
      if (!field) throw new Error("Unknown starter field");
      const value = values[key]?.trim();
      if (!value && !preview)
        throw new Error(`Complete ${field.label.toLowerCase()}`);
      if (value && (/[\r\n]|\{\{|\[\[/.test(value) || value.length > 200))
        throw new Error(
          "Use plain text of 200 characters or fewer for each detail.",
        );
      const replacement = value || `[${field.label.toLowerCase()}]`;
      return html ? escapeHTML(replacement) : replacement;
    });
  return starter.emails.map((email) => ({
    kind: "email",
    mode: "review",
    delay_days: email.delay,
    subject: render(email.subject),
    body_html:
      "<p>Hi {{first_name|there}},</p>" +
      email.paragraphs.map((p) => `<p>${render(p, true)}</p>`).join(""),
  }));
}
