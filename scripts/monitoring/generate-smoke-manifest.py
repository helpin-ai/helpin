#!/usr/bin/env python3
"""Generate the smoke ConfigMap from its reviewed source and npm lock.

The manifest lives in helpin-ai/gitops at helpin/prod/monitoring-smoke-config.yaml;
the production release writes it there. Pass an output path, or omit it to print.
"""
import json
import sys
from pathlib import Path
root = Path(__file__).resolve().parents[2]
files = {name: (root / 'ops/widget-smoke' / name).read_text() for name in ['smoke.cjs', 'package.json', 'package-lock.json']}
# JSON is valid YAML and avoids multiline quoting drift without a PyYAML dependency.
manifest = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'helpin-widget-smoke', 'namespace': 'helpin'}, 'data': files}
output = json.dumps(manifest, indent=2) + '\n'
if len(sys.argv) > 1:
    Path(sys.argv[1]).write_text(output)
else:
    sys.stdout.write(output)
