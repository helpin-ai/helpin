# Codex workspace-write node-pool proposal

> Historical infrastructure proposal, source-compared on 2026-09-17. This
> document records an April sandbox investigation. Its shared Codex worker topology
> and deployment/rollback commands are not current operating instructions.

## Current deployment boundary

The checked-in [staging](../../k8s/stage/temporal-worker.yaml) and
[production](../../k8s/prod/temporal-worker.yaml) manifests define
`helpin-temporal-automation` with the `automation-default` queue. They retain
non-root execution, a read-only root filesystem, dropped capabilities, and a
writable `/tmp`, but do not contain a Codex-specific deployment,
`CODEX_SANDBOX_MODE`, `hostUsers: false`, or the proposed Codex node selector.
This source comparison does not establish live node/kernel configuration or
values supplied through external secrets.

Helpin's [configuration](../../server/internal/config/config.go) now exposes
Agent Runtime connection and launch settings, and the
[runtime client](../../server/internal/service/agent_runtime_client.go) handles
remote execution requests. Use the current
[local setup guide](../agent-runtime-local-setup.md) and
the staging runbook (maintained privately) to identify the execution
service and its deployment owner before planning sandbox changes. Editing the
Helpin automation-worker manifest is not evidence that a separate runtime's
sandbox policy has changed.

The listed kernel/runtime minimums, old Bubblewrap failure, dedicated-pool
assumptions, and `danger-full-access` workaround below are historical proposal
material. They were not revalidated as current platform requirements and should
not be copied into an active rollout or rollback. No cluster inspection, node
provisioning, service restart, sandbox change, or deployment was performed during
this documentation review. A future migration needs a new plan based on the
actual runtime and cluster configuration.

## Original April proposal

## Status

Draft implementation plan for restoring Codex `workspace-write` sandboxing on Kubernetes while keeping the current shared Temporal worker architecture.

This plan assumes:

- Codex workers remain part of the current Temporal workflow model
- shared worker pods continue to exist
- `danger-full-access` is only a temporary unblocker
- some existing worker nodes run older kernels and should not be reused for Codex sandbox workloads

## Problem

Codex shell commands in Kubernetes worker pods are currently failing with Bubblewrap namespace errors similar to:

`bwrap: No permissions to create a new namespace, likely because the kernel does not allow non-privileged user namespaces`

The immediate unblocker is to run Codex with `danger-full-access`, but that is not the right steady-state security model for shared multi-tenant worker pods.

## Goal

Restore Codex `workspace-write` sandboxing for Kubernetes-hosted Codex workers by moving those workers onto a dedicated node pool with the kernel and runtime support required for user namespaces.

Keep the existing Temporal workflow architecture.

Do not move to per-run pods or VM-per-run unless later isolation requirements justify that complexity.

## Non-Goals

- redesigning the agent run workflow model
- replacing Temporal with direct pod orchestration
- introducing per-run Kubernetes Jobs for Codex in this phase
- running Codex workers on the existing mixed-age general-purpose node pool

## Why This Approach

This is the lowest-maintenance path that preserves:

- shared worker operations
- current run lifecycle behavior
- current backend control plane
- Codex sandboxing in `workspace-write`

Compared with alternatives:

- `danger-full-access` in shared multi-tenant pods is operationally simple but weakens the in-pod trust boundary too far
- per-run pods/jobs improve isolation but introduce significant platform complexity
- KubeVirt or VM-per-run improves isolation further but is substantially heavier to build and operate

## Required Platform Capabilities

Dedicated Codex worker nodes should meet all of the following:

- Linux kernel `6.3+`
- container runtime support compatible with Kubernetes user namespaces
- `containerd 2.0+` or `CRI-O 1.25+`
- `runc 1.2+` or `crun 1.13+`
- distro support for unprivileged user namespaces where applicable
- `bubblewrap` available in the worker image or host runtime path used by Codex

Kubernetes-side requirements:

- Pod user namespaces enabled on the cluster and node/runtime stack
- Codex worker deployments use `hostUsers: false`
- Codex workers scheduled only onto the new node pool

## Architecture

Keep the current shape:

```text
Helpin UI/API
  -> Temporal workflow
    -> shared Codex Temporal worker pods
      -> Codex app-server
        -> workspace-write sandbox
```

Change only the execution environment for Codex worker pods:

- dedicated node pool
- user namespace support
- manifest scheduling constraints
- revert temporary `danger-full-access` override after validation

## Rollout Phases

## Phase 1: Temporary Unblocker

Current temporary state:

- Codex workers can run with `CODEX_SANDBOX_MODE=danger-full-access`
- this is acceptable only as a short-term unblocker

Exit criteria:

- Codex runs complete again in Kubernetes
- operators can continue feature work while the dedicated node pool is prepared

## Phase 2: Dedicated Codex Node Pool

Provision a new Kubernetes node pool only for Codex workers.

Requirements:

- newer kernel meeting the minimum requirements
- current container runtime and OCI runtime versions
- node labels such as `helpin.ai/codex-sandbox=true`
- optional taint such as `helpin.ai/codex-sandbox=true:NoSchedule`

Do not colocate general worker queues on this pool initially.

Exit criteria:

- a node in the new pool can run a test pod with `hostUsers: false`
- Codex prerequisites are installed and verified on the new pool

## Phase 3: Pod User Namespace Enablement

Update Codex worker deployments to use pod user namespaces:

- add `hostUsers: false`
- add node selector or affinity for the dedicated Codex pool
- add tolerations if the pool is tainted

Keep:

- `readOnlyRootFilesystem: true`
- writable `emptyDir` at `/tmp`
- `allowPrivilegeEscalation: false`
- dropped capabilities

Exit criteria:

- Codex worker pods schedule only onto the new pool
- worker pods start successfully with `hostUsers: false`

## Phase 4: Restore Workspace-Write

After the dedicated pool is validated:

- remove `CODEX_SANDBOX_MODE=danger-full-access` from Codex worker deployments
- allow the runtime to return to its default sandbox selection
- confirm Codex uses `workspace-write` for repo-writing runs

Exit criteria:

- `run_command` no longer fails with Bubblewrap namespace errors
- repo mutation flows work under `workspace-write`
- interactive and autonomous Codex runs behave normally

## Phase 5: Hardening and Validation

Add operational checks for:

- node kernel version drift
- runtime version drift
- Codex worker pods accidentally scheduled outside the dedicated pool
- Codex runs unexpectedly falling back to `danger-full-access`

Recommended validation:

- smoke test `pwd`
- smoke test `git status`
- smoke test `rg`
- smoke test repo edit + diff generation
- smoke test interactive approval flow

## Manifest Changes Required

When Phase 3 begins, update:

- `k8s/stage/temporal-worker.yaml`
- `k8s/prod/temporal-worker.yaml`

For the Codex worker deployments only:

- add `hostUsers: false`
- add node selectors / affinity for the Codex pool
- add tolerations for Codex taints
- later remove the temporary `CODEX_SANDBOX_MODE=danger-full-access`

## Operational Checklist

Before rollout:

- verify kernel version on dedicated Codex nodes
- verify container runtime and OCI runtime versions
- verify user namespace support is enabled
- verify Codex binary and Bubblewrap behavior on those nodes

During rollout:

- deploy stage Codex workers to dedicated pool first
- validate autonomous and interactive stage runs
- promote the same manifest pattern to prod

After rollout:

- remove the temporary sandbox override
- monitor failure rate for Codex `run_command`
- monitor unexpected pod scheduling drift

## Risks

- older kernels in the general node pool create inconsistent behavior if scheduling constraints are incomplete
- `hostUsers: false` may expose runtime-specific incompatibilities not seen on current pods
- a partial rollout may leave stage and prod on different sandbox semantics

## Rollback

If `workspace-write` remains unstable on the dedicated pool:

- restore `CODEX_SANDBOX_MODE=danger-full-access`
- keep Codex workers isolated to the dedicated node pool
- continue investigating node/runtime prerequisites before another attempt

## Recommendation

Proceed with:

1. dedicated Codex node pool
2. user-namespace validation on that pool
3. `hostUsers: false` for Codex workers
4. removal of the temporary `danger-full-access` override only after validation

Do not attempt to make the existing mixed-age shared worker node pool support this uniformly.
