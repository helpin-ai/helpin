# README visuals

The README screenshots are captures of the live product previews on the Helpin
website ([helpin.ai](https://helpin.ai)). The previews are interactive React
mockups of the Helpin workspace filled with fictional data: the OrbitDesk
workspace, Maya Chen at Northstar Labs, the EXP-142 export task, and the
PRJ-214 SSO pilot. They are not captures of production conversations or
evidence of release availability.

| File | Source on the website |
| --- | --- |
| `helpin-support-inbox.webp` | Homepage product walkthrough, **Support** tab |
| `helpin-projects-delivery.webp` | Homepage product walkthrough, **Projects** tab, while the teammate reviews the change |
| `helpin-crm-contacts.webp` | Homepage product walkthrough, **CRM** tab |
| `helpin-meetings-review.webp` | Homepage product walkthrough, **Meetings** tab |
| `helpin-knowledge-help-center.webp` | Homepage product walkthrough, **Knowledge** tab |
| `helpin-ask-agent.webp` | Homepage **Ask Agent** section |

## Updating the screenshots

Capture each preview element with a 1440 × 1000 viewport at a device scale
factor of 2. Use `prefers-reduced-motion: reduce` so the previews show their
final state, except for Projects, whose final state stacks the Ask Agent card
over the task; capture that one with motion enabled once the delivery timeline
reaches the teammate review. Hide the site navigation and any play/pause
controls. Resize the captures to 1920 pixels wide and save them as WebP at
quality 84. Keep each file under about 150 KB.

The README uses copies in this folder so documentation does not depend on
website asset paths.
