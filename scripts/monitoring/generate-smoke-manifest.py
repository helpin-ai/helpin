#!/usr/bin/env python3
"""Regenerate the committed smoke ConfigMap from its reviewed source and npm lock."""
import json
from pathlib import Path
root = Path(__file__).resolve().parents[2]
files = {name: (root / 'ops/widget-smoke' / name).read_text() for name in ['smoke.cjs', 'package.json', 'package-lock.json']}
# JSON is valid YAML and avoids multiline quoting drift without a PyYAML dependency.
manifest = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'helpin-widget-smoke', 'namespace': 'helpin'}, 'data': files}
(root / 'k8s/prod/monitoring-smoke-config.yaml').write_text(json.dumps(manifest, indent=2) + '\n')
