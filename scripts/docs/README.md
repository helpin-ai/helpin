# Check documentation names and links

This guide is for contributors editing Markdown. It explains the naming and link
checks that CI runs and how to run them locally. From the repository root:

```sh
python3 scripts/docs/check_names.py
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
```

The naming check covers all tracked Markdown under `docs/` plus new non-ignored
documents. It enforces lowercase hyphenated names (except `README.md`), rejects
redundant `prd-` and `todo-` prefixes, and validates date prefixes. See the
[writing and naming guide](../../docs/documentation-guide.md) for the full convention.

CI runs these in the always-required `workflow-checks` job, including on
documentation-only PRs. The checker validates all documents listed in
[maintained-docs.json](maintained-docs.json) every run, so a deleted or renamed
linked source file can fail the check even if the Markdown did not change.
Add new maintained operator/contributor guides to this list. This list controls
link-check coverage, not authorization to publish a document.

It checks inline links, images, reference definitions, explicit reference links,
local file existence, and Markdown heading/HTML anchors. Fenced examples, inline
code, and HTML comments are ignored. Repository escapes are rejected. External
URLs are not fetched, and fragments within non-Markdown files are not validated.
This lightweight checker is not a complete CommonMark renderer; use ordinary
Markdown links in maintained docs. Raw HTML href/src attributes are outside its
scope. Historical plans/specifications are not checked by default.

To check additional documents, supply repository-relative paths:

```sh
python3 scripts/docs/check_links.py README.md ARCHITECTURE.md
```

For changes to operator docs, also run the existing release packaging test. It
checks the actual bundle's relative links with mocked registry operations:

```sh
python3 -m unittest discover -s community/tests -p 'package_release_test.py'
```
