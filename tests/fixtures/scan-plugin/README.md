# forge-scan-plugin: sample third-party scanner plugin (G-025)
#
# This fixture demonstrates the scanner plugin contract for forge.
# A scanner plugin must expose a single HTTP endpoint (or be invoked via stdin/stdout)
# and return a JSON findings array on stdout.
#
# Contract:
#   Input (stdin): JSON object with keys:
#     - "files": []string — list of file paths relative to project root
#     - "root": string   — absolute path to project root
#   Output (stdout): JSON object with keys:
#     - "findings": []Finding
#     - "scanner": string — scanner name + version
#
# Finding schema:
#   {
#     "rule_id":  "CUSTOM-001",
#     "file":     "path/to/file.go",
#     "line":     42,
#     "severity": "warning",  // "error" | "warning" | "info"
#     "message":  "Human-readable description"
#   }
#
# Install into forge:
#   forge plugin install ./tests/fixtures/scan-plugin --as scan-plugin-demo
#
# The plugin will be invoked by `forge scan` with the contract above.
# Findings are merged into the unified forge report.

This is a documentation stub. See scan_plugin_demo.py for a runnable example.
