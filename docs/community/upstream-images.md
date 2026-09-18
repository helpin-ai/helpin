# Upstream infrastructure image maintenance

This page is for operators and maintainers reviewing the infrastructure images
the bundle pulls. It explains the pinning policy and the accepted scanner
findings at the time of writing; the authoritative list for a given bundle is
its `community/upstream-image-exceptions.json`.

Community uses unmodified, digest-pinned pgvector/PostgreSQL, NATS, Garage,
Redis, and Temporal images. Helpin maintains application images only. Updating
an upstream digest requires the clean-install and restore gates; do not rebuild
infrastructure merely to change a scan result.

The 2026-09-16 amd64 scan found **24 fixable high/critical findings in PostgreSQL,
2 in NATS, and 0 in Garage**. These are accepted beta exceptions through
2026-10-16, not a claim that these images contain no vulnerabilities. Exact CVE,
package, installed version and image digest are recorded in
`community/upstream-image-exceptions.json`. The image check fails on additional
findings, changed digests, or expired exceptions. Helpin and Runtime application
images retain the zero-fixable-high/critical gate. Native arm64 must pass its own
scan and acceptance run before inclusion in a release.

- PostgreSQL: 22 Go standard-library findings occur in the upstream `gosu`
  privilege-dropping executable. It runs during container startup, not as an
  application network server. This limits exposure to network-related Go flaws;
  it does not remove the findings. Two further findings affect Debian's
  `libpcre2-8-0`. The bundle exposes no PostgreSQL host port and gives applications
  separate non-superuser roles. Vulnerable packages remain installed.
- NATS: two findings are CVE-2026-14456 in Alpine `libcrypto3` and `libssl3`.
  The broker is reachable only inside the Compose network. It is not rebuilt or
  declared safe merely because the libraries may not be on a normal request path.
- Garage: upstream `dxflrs/garage:v2.3.0`, both amd64 and arm64 in its manifest.
  No fixable high/critical findings appeared in the amd64 scan. We do not build a
  storage server or storage-init client. Garage bootstraps a private bucket using
  its upstream single-node/default-bucket functionality.

Review upstream refreshes at each release and before exceptions expire. Prefer a
patched upstream pin, rerun scans and install/restore, then delete resolved
exceptions. Reassess any exception if ports, privileges or network exposure change.
Retain image SBOMs and the exception inventory in release evidence.

Garage's [quick start](https://garagehq.deuxfleurs.fr/documentation/quick-start/)
and [S3 compatibility list](https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/)
document the bootstrap options and unsupported ACL/policy APIs. The bundle uses
one private bucket, signed uploads/downloads, and Helpin's public-asset allowlist;
it enables neither anonymous S3 reads nor Garage website hosting. A one-node
store is not redundant; off-host backups remain required.
