#!/usr/bin/env bash
# Build a component's images, or promote existing ones when the same build
# inputs were already built by another release workflow.
#
# Images are tagged content-<key>, where <key> hashes the git tree objects of
# every build input. Staging and production build these images without
# environment-specific arguments, so equal keys mean identical images. A
# production release of code already built for staging then only retags it.
#
# Usage: build-or-promote-image.sh <checkout-dir> <env> <component> <release-tag>
#   env:       stage | prod (selects the <env>-latest tag and build cache)
#   component: server | helpcenter
#   key mode:  build-or-promote-image.sh --key <checkout-dir> <component>
#
# Environment: IMAGE_PREFIX (required), GITHUB_SHA (required in build mode),
# DRY_RUN=1 prints the docker commands instead of running them.
set -euo pipefail

# Bump to invalidate every content key, for example after changing how a
# component is built in a way its input trees do not capture.
readonly KEY_VERSION=1

component_inputs() {
  case "$1" in
    server) echo "server" ;;
    helpcenter) echo "help-center packages frontend package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json .dockerignore" ;;
    *) echo "unknown component: $1" >&2; return 1 ;;
  esac
}

# component_build_args prints arguments that change the image without being
# part of an input tree.
component_build_args() {
  case "$1" in
    server) echo "GO_BUILD_TAGS=ee" ;;
    helpcenter) echo "" ;;
  esac
}

content_key() {
  local checkout=$1 component=$2 input object
  {
    echo "version=$KEY_VERSION component=$component args=$(component_build_args "$component")"
    for input in $(component_inputs "$component"); do
      object=$(git -C "$checkout" rev-parse --verify --quiet "HEAD:$input" || echo missing)
      echo "$input $object"
    done
  } | sha256sum | cut -c1-32
}

run() {
  if [ "${DRY_RUN:-}" = "1" ]; then
    echo "+ $*"
  else
    "$@"
  fi
}

image_exists() {
  if [ "${DRY_RUN:-}" = "1" ]; then
    [ "${DRY_RUN_EXISTING:-}" = "1" ]
    return
  fi
  docker buildx imagetools inspect "$1" >/dev/null 2>&1
}

if [ "${1:-}" = "--key" ]; then
  content_key "$2" "$3"
  exit 0
fi

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <checkout-dir> <stage|prod> <server|helpcenter> <release-tag>" >&2
  exit 2
fi

checkout=$1
environment=$2
component=$3
release_tag=$4
: "${IMAGE_PREFIX:?IMAGE_PREFIX is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

case "$environment" in
  stage | prod) ;;
  *) echo "unknown environment: $environment" >&2; exit 2 ;;
esac

# Each entry is image:target; helpcenter has a single-stage default target.
case "$component" in
  server) images="server:api temporal-worker:temporal-worker server-migrate:migrate" ;;
  helpcenter) images="helpcenter:" ;;
  *) echo "unknown component: $component" >&2; exit 2 ;;
esac

key=$(content_key "$checkout" "$component")
echo "Content key for $component: $key"

promote=true
for entry in $images; do
  image=${entry%%:*}
  if ! image_exists "$IMAGE_PREFIX/$image:content-$key"; then
    promote=false
    break
  fi
done

for entry in $images; do
  image=${entry%%:*}
  target=${entry#*:}
  ref="$IMAGE_PREFIX/$image"
  tags=(-t "$ref:$release_tag" -t "$ref:sha-$GITHUB_SHA" -t "$ref:$environment-latest")

  if [ "$promote" = true ]; then
    echo "Promoting $ref:content-$key (already built from identical inputs)"
    run docker buildx imagetools create "${tags[@]}" "$ref:content-$key"
    continue
  fi

  echo "Building $ref"
  args=(docker buildx build "${tags[@]}" -t "$ref:content-$key" --push)
  case "$component" in
    server)
      cache="$IMAGE_PREFIX/server-buildcache:$environment-$target"
      args+=(-f "$checkout/server/Dockerfile" --target "$target")
      context="$checkout/server/"
      ;;
    helpcenter)
      cache="$IMAGE_PREFIX/helpcenter-buildcache:$environment"
      args+=(-f "$checkout/help-center/Dockerfile")
      context="$checkout/"
      ;;
  esac
  for build_arg in $(component_build_args "$component"); do
    args+=(--build-arg "$build_arg")
  done
  args+=(--cache-from "type=registry,ref=$cache" --cache-to "type=registry,ref=$cache,mode=max" "$context")
  run "${args[@]}"
done
