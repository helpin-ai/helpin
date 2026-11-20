# README visuals

The README screenshots are captures of the live product previews on the Helpin
website ([helpin.ai](https://helpin.ai)). The previews are React mockups of the
Helpin workspace filled with fictional data: the OrbitDesk workspace, Maya Chen
at Northstar Labs, and the EXP-142 export task. They are not captures of production conversations or
evidence of release availability.

| File | Source on the website |
| --- | --- |
| `helpin-support-inbox.webp` | Homepage hero inbox scene, last stage: Sam’s reply sent, NRT met |
| `helpin-support-ai-research.webp` | Homepage hero inbox scene, stage 4: research result and the AI’s Excel answer |
| `helpin-support-sla-warning.webp` | Homepage hero inbox scene, stage 8: the “SLA due soon” notification |
| `helpin-support-ask-agent.webp` | Homepage hero inbox scene, stage 10: the Ask Agent answer |
| `helpin-knowledge-help-center.webp` | Homepage product walkthrough, **Knowledge** tab |
| `helpin-ask-agent.webp` | Homepage **Ask Agent** section |

## Updating the screenshots

Capture each preview element with a 1440 × 1000 viewport at a device scale
factor of 2. The hero inbox scene (`.hs-canvas`) is animated: wait for its
`data-stage` attribute to reach the stage in the table, then let the stage
finish typing before capturing. For the other previews, use
`prefers-reduced-motion: reduce` so they show their final state. Hide the site navigation and any play/pause
controls. Resize the captures to 1920 pixels wide and save them as WebP at
quality 84. Keep each file under about 150 KB.

The README uses copies in this folder so documentation does not depend on
website asset paths.
