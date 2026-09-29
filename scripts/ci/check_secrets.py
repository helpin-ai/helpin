#!/usr/bin/env python3
"""Fail when tracked files contain credentials or committed .env files.

Deliberately high-signal: provider token formats, private keys, database URLs
with a password on a non-local host, and filled-in MaxMind license keys. Fixtures
that are not real credentials go in ALLOWED_LINES with a reason.
"""
import re
import subprocess
import sys
from pathlib import Path

PATTERNS = {
    'Doppler token': r'\bdp\.(?:st|pt|sa|ct|scim|audit)\.[A-Za-z0-9_.]{20,}',
    'AWS access key': r'\bAKIA[0-9A-Z]{16}\b',
    'GitHub token': r'\b(?:ghp|gho|ghs|ghu)_[A-Za-z0-9]{30,}|\bgithub_pat_[A-Za-z0-9_]{30,}',
    'GitLab token': r'\bglpat-[A-Za-z0-9_-]{20}',
    'Anthropic/OpenAI key': r'\bsk-ant-[A-Za-z0-9_-]{20,}|\bsk-proj-[A-Za-z0-9_-]{20,}|\bsk-[A-Za-z0-9]{40,}',
    'Google API key': r'\bAIza[0-9A-Za-z_-]{35}',
    'Slack token or webhook': r'\bxox[baprs]-[A-Za-z0-9-]{10,}|hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+',
    'Stripe live key': r'\b(?:sk|rk)_live_[A-Za-z0-9]{16,}',
    'SendGrid key': r'\bSG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}',
    'Private key': r'-----BEGIN [A-Z ]*PRIVATE KEY-----',
    'MaxMind license key': r'MAXMIND_LICENSE_KEY\s*[=:]\s*["\']?[A-Za-z0-9_]{16,}',
    'Database URL with password': r'\b(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqp|clickhouse)://[^:@/\s"\']+:(?!\$|<|\{|password\b|postgres\b|user\b|pass\b|secret\b|changeme\b|example\b)[^@/\s"\'$]{8,}@(?!localhost\b|127\.0\.0\.1\b|postgres\b|db\b|redis\b|host\b|example)',
}
COMPILED = {name: re.compile(pattern) for name, pattern in PATTERNS.items()}
ENV_FILE = re.compile(r'(^|/)\.env(\..+)?$')
ENV_ALLOWED = re.compile(r'\.(example|sample|template)$')
SKIP_SUFFIXES = ('.lock', '.svg', '.png', '.jpg', '.jpeg', '.webp', '.gif', '.ico', '.woff', '.woff2', '.pdf', '.mp4', '.mp3', '.wav')
SKIP_FILES = {'pnpm-lock.yaml'}
ALLOWED_LINES = ()  # (path, substring) pairs for known non-secret fixtures


def scan_text(path, text):
    findings = []
    for number, line in enumerate(text.splitlines(), 1):
        if any(path == allowed and marker in line for allowed, marker in ALLOWED_LINES):
            continue
        for name, pattern in COMPILED.items():
            if pattern.search(line):
                findings.append((path, number, name))
    return findings


def check_path(path):
    if ENV_FILE.search(path) and not ENV_ALLOWED.search(path):
        return [(path, 0, 'Committed .env file')]
    return []


def tracked_files(root):
    output = subprocess.check_output(['git', 'ls-files', '-z'], cwd=root)
    return [item for item in output.decode().split('\0') if item]


def main():
    root = Path(__file__).resolve().parents[2]
    findings = []
    for path in tracked_files(root):
        findings.extend(check_path(path))
        target = root / path
        if path in SKIP_FILES or path.endswith(SKIP_SUFFIXES) or not target.is_file():
            continue
        try:
            text = target.read_text(encoding='utf-8')
        except (UnicodeDecodeError, OSError):
            continue
        findings.extend(scan_text(path, text))
    for path, number, name in findings:
        location = f'{path}:{number}' if number else path
        print(f'{location}: {name} (value not shown)')
    if findings:
        print(f'{len(findings)} potential secret(s). Rotate anything real, remove it from the tree, and use environment variables or a secret manager.', file=sys.stderr)
        return 1
    print('No committed secrets found.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
