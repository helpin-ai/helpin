# Documentation link checks

From the repository root:

```sh
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
```

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
