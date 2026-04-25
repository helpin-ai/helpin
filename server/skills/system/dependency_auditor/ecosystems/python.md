# Python Dependency Audits

## Parse
- Discover direct dependencies from `pyproject.toml`, `requirements.txt`, `requirements/*.txt`, `setup.py`, `setup.cfg`, and `Pipfile`.
- Support PEP 621 `project.dependencies` and `project.optional-dependencies`.
- Support Poetry-style `tool.poetry.dependencies`, `tool.poetry.group.*.dependencies`, and extras.
- Support PDM/uv declaration sections when present in `pyproject.toml`.
- Parse requirements lines for package name, extras, version specifier, environment marker, index options, constraints includes, editable installs, VCS URLs, paths, and comments.
- Use `poetry.lock`, `uv.lock`, PDM lockfiles, and `Pipfile.lock` only as supporting context. Do not create tasks for lockfile-only transitive packages.

## Verify Versions
- Preferred registry source for public packages: PyPI JSON API `https://pypi.org/pypi/{normalized-name}/json`.
- Respect the current Python version requirement and package `requires_python`. Do not propose a latest version that is incompatible with the declared Python range.
- Treat prereleases as candidates only when the current declaration is prerelease or the specifier explicitly allows prereleases.
- Do not propose updates for editable installs, local paths, private indexes, or VCS dependencies unless a verified comparable version source exists.
- For constraints files, record the constraint and mark the task as requiring human review if the manifest and constraint disagree.

## Classify And Describe
- Normalize package names per PEP 503 for dedupe. Dependency identity is normalized package name plus source/index identity.
- For non-semver versions, classify conservatively: first numeric segment change is `major`, second is `minor`, later segment is `patch`, and incomparable schemes are `unknown`.
- Include affected manifests, requirement/declaration text, group/extra, extras, markers, index/source, current specifier, supporting resolved lockfile version when found, latest verified version, Python compatibility notes, and advisory/changelog URLs in each task.
