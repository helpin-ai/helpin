#!/usr/bin/env bash
set -euo pipefail

action="${1:-}"

case "${action}" in
  remove|purge|deconfigure)
    if command -v systemctl >/dev/null 2>&1; then
      systemctl stop helpin-temporal-worker.service 2>/dev/null || true
      systemctl disable helpin-temporal-worker.service 2>/dev/null || true
      systemctl stop helpin-temporal-worker-updater.timer 2>/dev/null || true
      systemctl disable helpin-temporal-worker-updater.timer 2>/dev/null || true
      systemctl stop helpin-temporal-worker-updater.service 2>/dev/null || true
      systemctl daemon-reload 2>/dev/null || true
    fi
    ;;
esac
