# Node / JavaScript Dependency Audits

Node covers JavaScript and TypeScript package manifests, including React, Vite, Next.js, Express, frontend apps, backend Node services, and package workspaces. React itself is not a separate ecosystem; audit it as npm packages declared in Node manifests.

## Parse
- Discover `package.json` files. If workspaces are configured, parse the workspace root and each member package manifest.
- Direct dependency sections are `dependencies`, `devDependencies`, `optionalDependencies`, `peerDependencies`, and package-manager-specific dependency metadata. Ignore transitive lockfile-only packages unless `include_indirect` is true.
- Record package manager and workspace context from `packageManager`, `workspaces`, `.npmrc`, `npm-shrinkwrap.json`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`, and `bun.lockb`.
- Record version declaration shape: exact, range (`^`, `~`, comparison ranges), dist-tag, workspace protocol, npm alias, file/link path, git URL, tarball URL, registry scope, peer dependency, optional dependency, and overrides/resolutions.
- Treat `overrides`, `resolutions`, `pnpm.overrides`, and workspace protocol declarations as risk context. Do not propose a version that conflicts with them.

## Verify Versions
- Preferred registry source for public packages: npm registry metadata at `https://registry.npmjs.org/<encoded-package-name>`.
- Use `dist-tags.latest` as the normal latest version. Use prerelease dist-tags only when the current declaration is prerelease or explicitly points at that dist-tag.
- Batch-friendly CLI options may be used when package-manager context is clear: `npm outdated --json`, `pnpm outdated --format json`, or `yarn npm info <pkg> --json`.
- Respect package `engines.node` when comparing versions. Do not propose a latest version that is incompatible with the repository's declared Node runtime.
- Mark private registries, local `file:`/`link:` dependencies, workspace-only dependencies, ambiguous aliases, and unverifiable git/tarball dependencies unresolvable unless a verified comparable registry version exists.

## Classify And Describe
- Dependency identity is package name plus registry/source identity. Preserve npm scopes and aliases; when an alias points to another package, cite both alias and real package.
- For `0.x.y` packages, treat minor changes as potentially breaking unless official notes show compatibility.
- Include affected manifests, dependency section, current declaration, lockfile resolved version when found, package manager, workspace package path, registry/source, `engines.node`, peer dependency constraints, overrides/resolutions, latest verified version, and advisory/changelog URLs in each task.
