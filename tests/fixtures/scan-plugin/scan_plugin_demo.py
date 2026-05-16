#!/usr/bin/env python3
"""
forge-scan-plugin: sample third-party scanner plugin (G-025).

Implements the scanner plugin stdin/stdout JSON contract:
  - Reads a JSON bundle from stdin.
  - Writes a JSON findings array to stdout.

Usage (direct):
    echo '{"files": ["main.go"], "root": "."}' | python3 scan_plugin_demo.py

Usage (via forge):
    forge plugin install tests/fixtures/scan-plugin --as scan-plugin-demo
    forge scan --plugin scan-plugin-demo
"""

import json
import sys
import os
import re


def scan_file(root: str, rel_path: str) -> list[dict]:
    """Run simple pattern-based checks on a single file."""
    findings = []
    abs_path = os.path.join(root, rel_path)
    try:
        with open(abs_path, encoding="utf-8", errors="ignore") as f:
            for i, line in enumerate(f, 1):
                # Example rule: flag TODO/FIXME comments as info.
                if re.search(r"\b(TODO|FIXME|HACK)\b", line):
                    findings.append({
                        "rule_id": "CUSTOM-001",
                        "file": rel_path,
                        "line": i,
                        "severity": "info",
                        "message": f"Action item comment: {line.strip()[:80]}",
                    })
                # Example rule: flag hardcoded localhost as warning.
                if "localhost" in line and "test" not in rel_path.lower():
                    findings.append({
                        "rule_id": "CUSTOM-002",
                        "file": rel_path,
                        "line": i,
                        "severity": "warning",
                        "message": "Hardcoded localhost reference — use configurable host",
                    })
    except OSError:
        pass
    return findings


def main():
    raw = sys.stdin.read()
    try:
        bundle = json.loads(raw) if raw.strip() else {}
    except json.JSONDecodeError:
        bundle = {}

    root = bundle.get("root", ".")
    files = bundle.get("files", [])

    all_findings = []
    for rel in files:
        all_findings.extend(scan_file(root, rel))

    result = {
        "scanner": "scan-plugin-demo/1.0.0",
        "findings": all_findings,
    }
    json.dump(result, sys.stdout, indent=2)
    print()  # trailing newline


if __name__ == "__main__":
    main()
