#!/usr/bin/env python3
"""Patch llmpipe.go to add G-007: tasks.md alongside breakdown.md."""

import sys
import os

PIPE_PATH = os.path.join(os.path.dirname(__file__), '..', 'internal', 'cli', 'cmdship', 'llmpipe.go')

with open(PIPE_PATH, 'rb') as f:
    raw = f.read()

emdash = b'\xe2\x80\x94'
# check if the actual file uses the mojibake version
mojibake = b'\xc3\xa2\xe2\x82\xac\xe2\x80\x9d'

# find generateBreakdown function
# After writing breakdown.md, add tasks.md writing
old_frag = (
b'\t_ = os.WriteFile(filepath.Join(specsDir, "breakdown.md"), []byte(content), 0o600)\r\n'
b'\treturn content, nil\r\n'
b'}'
)

# Check what the actual bytes are
idx = raw.find(b'breakdown.md')
print(f"breakdown.md at: {idx}")
print(repr(raw[idx-30:idx+200]))
