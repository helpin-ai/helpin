---
name: helpin-documentation
description: Write, explain, review, name, or reorganize Helpin repository documentation and update its navigation. Use for documentation work in this repository, including architecture and contributor/operator guides.
---

# Helpin documentation

Read [the documentation guide](../../../docs/documentation-guide.md) before
choosing a filename, page structure, or destination. It is the shared naming and
writing authority; do not duplicate its rules in another convention document.

Use [the documentation index](../../../docs/README.md) to find an existing home
for the topic. Product tutorials belong in the Helpin-hosted docs; current
technical instructions remain versioned in the repository. Editing a guide does
not imply publishing content or changing application behavior.

Ground setup instructions and architecture claims in the current code, manifests,
and release state. Label historical and proposed behavior, preserve limitations,
and give the reader a concrete task or explanation in the opening paragraph.
Improve the relevant passage; do not rewrite a large technical contract solely
for stylistic uniformity.

For renames, inspect working-tree changes and update incoming links, index labels,
agent instructions, coverage manifests, and any packaging references. Check
filename collisions and heading fragments. Preserve unrelated work. A document's
age alone is not a reason to delete it.

Run the naming/link checks and relevant documentation tests from the guide.
For bulk moves, compare link errors before and after so pre-existing historical
failures are distinguished from regressions. Report the changed names, validation,
and any external links or publication steps that remain unverified.

Before committing documentation or assets, apply the [Community/cloud boundary rules](../../../AGENTS.md#community-and-cloud-boundaries). Public repository branches expose tracked plans, screenshots, and operational details too. Ask the owner about unresolved publication scope and keep private evidence outside the repository.
