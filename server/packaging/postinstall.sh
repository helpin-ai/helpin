#!/usr/bin/env bash
set -euo pipefail

action="${1:-}"
previous_version="${2:-}"
is_upgrade=0

if [[ "${action}" == "configure" && -n "${previous_version}" ]]; then
  is_upgrade=1
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true

  if [[ "${is_upgrade}" -eq 1 ]]; then
    if systemctl is-active --quiet helpin-temporal-worker.service; then
      systemctl try-restart helpin-temporal-worker.service || true
    fi

    if systemctl is-enabled --quiet helpin-temporal-worker-updater.timer || systemctl is-active --quiet helpin-temporal-worker-updater.timer; then
      systemctl restart helpin-temporal-worker-updater.timer || true
    fi
  fi
fi

cat <<'EOF'
Helpin Temporal Worker package installed.

Next steps:
  1. Edit /etc/helpin/temporal-worker.conf and set DOPPLER_PROJECT, DOPPLER_CONFIG, and GH_TOKEN.
  2. Add DOPPLER_TOKEN via: systemctl edit helpin-temporal-worker
     Example:
       [Service]
       Environment=DOPPLER_TOKEN=dp.st.xxxxx
  3. Ensure both runtimes are installed when both queues are enabled:
       codex
       opencode
  4. Start the worker manually:
       systemctl enable --now helpin-temporal-worker
  5. Enable automatic updates only after GH_TOKEN and gh are configured:
       systemctl enable --now helpin-temporal-worker-updater.timer
EOF
