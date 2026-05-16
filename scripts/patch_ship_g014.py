"""G-014: Add skip-checkpoint audit log entry in ship.go."""
import sys

with open('internal/cli/cmdship/ship.go', 'rb') as f:
    raw = f.read()

content = raw.decode('utf-8', errors='surrogateescape')

# 1) Add audit import after verbmeta import
old_imp = '"github.com/teragrid/forge/internal/verbmeta"'
new_imp = '"github.com/teragrid/forge/internal/audit"\r\n\t"github.com/teragrid/forge/internal/verbmeta"'
if old_imp in content:
    content = content.replace(old_imp, new_imp, 1)
    print("Added audit import")
else:
    print("ERROR: verbmeta import not found", file=sys.stderr)
    sys.exit(1)

# 2) After the skipCheckpoint filter block, add audit log.
# The closing brace of the if-block is: \t\t\t\tnames = filtered\r\n\t\t\t}\r\n
old_skip = '\t\t\tnames = filtered\r\n\t\t\t}'
new_skip = (
    '\t\t\tnames = filtered\r\n'
    '\t\t\t// G-014: Audit skip-checkpoint usage.\r\n'
    '\t\t\tif al, aErr := audit.Open(audit.DefaultPath); aErr == nil {\r\n'
    '\t\t\t\t_ = al.Append(audit.Entry{\r\n'
    '\t\t\t\t\tVerb:   "ship",\r\n'
    '\t\t\t\t\tAction: "skip_checkpoint",\r\n'
    '\t\t\t\t\tDetail: map[string]string{"checkpoint": skipCheckpoint},\r\n'
    '\t\t\t\t})\r\n'
    '\t\t\t}\r\n'
    '\t\t\t}'
)
if old_skip in content:
    content = content.replace(old_skip, new_skip, 1)
    print("Added audit call after skip-checkpoint filter")
else:
    print("ERROR: skipCheckpoint filter block not found", file=sys.stderr)
    sys.exit(1)

with open('internal/cli/cmdship/ship.go', 'wb') as f:
    f.write(content.encode('utf-8', errors='surrogateescape'))
print("Done")
