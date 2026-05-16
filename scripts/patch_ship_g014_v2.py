"""G-014: Patch ship.go to add skip-checkpoint audit log entry."""
import sys

with open('internal/cli/cmdship/ship.go', 'rb') as f:
    raw = f.read()

content = raw.decode('utf-8', errors='surrogateescape')

AUDIT_IMPORT = 'github.com/teragrid/forge/internal/audit'
VERBMETA_IMPORT = 'github.com/teragrid/forge/internal/verbmeta'

# 1) Add audit import if not present
if AUDIT_IMPORT not in content:
    content = content.replace(
        VERBMETA_IMPORT + '"',
        AUDIT_IMPORT + '"\n\t"' + VERBMETA_IMPORT + '"',
        1
    )
    print('Added audit import')
else:
    print('audit import already present')

# 2) Inject audit call after skipCheckpoint filter block.
# Exact key: '\t\t\tnames = filtered\r\n\t\t}\r\n\r\n\t\t// Build approval gate'
NEEDLE = '\t\t\tnames = filtered\r\n\t\t}\r\n\r\n\t\t// Build approval gate'
REPLACEMENT = (
    '\t\t\tnames = filtered\r\n'
    '\t\t\t// G-014: Audit skip-checkpoint usage.\r\n'
    '\t\t\tif al, aErr := audit.Open(audit.DefaultPath); aErr == nil {\r\n'
    '\t\t\t\t_ = al.Append(audit.Entry{\r\n'
    '\t\t\t\t\tVerb:   "ship",\r\n'
    '\t\t\t\t\tAction: "skip_checkpoint",\r\n'
    '\t\t\t\t\tDetail: map[string]string{"checkpoint": skipCheckpoint},\r\n'
    '\t\t\t\t})\r\n'
    '\t\t\t}\r\n'
    '\t\t}\r\n'
    '\r\n'
    '\t\t// Build approval gate'
)

if NEEDLE in content:
    content = content.replace(NEEDLE, REPLACEMENT, 1)
    print('Added audit call')
else:
    print('ERROR: needle not found', file=sys.stderr)
    sys.exit(1)

with open('internal/cli/cmdship/ship.go', 'wb') as f:
    f.write(content.encode('utf-8', errors='surrogateescape'))
print('Done')
