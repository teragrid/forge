import sys

path = r'i:\AI-Startup\forge\internal\cli\cmdship\ship.go'
with open(path, 'rb') as f:
    content = f.read()

# Wire appendFailure into checkTest (after timestamp guard returns fail)
old = (b')\r\n'
       b'\t\treturn cp\r\n'
       b'\t}\r\n\r\n'
       b'\tslug := slugify(description)\r\n\r\n'
       b'\t// G-006: write / verify all 4 named test artifacts.')
new = (b')\r\n'
       b'\t\t// G-011: record this failure for future learning context.\r\n'
       b'\t\tappendFailure(root, "test", description, cp.Detail)\r\n'
       b'\t\treturn cp\r\n'
       b'\t}\r\n\r\n'
       b'\tslug := slugify(description)\r\n\r\n'
       b'\t// G-006: write / verify all 4 named test artifacts.')

count = content.count(old)
print(f'Found {count} occurrences')
if count == 1:
    content = content.replace(old, new)
    with open(path, 'wb') as f:
        f.write(content)
    print('Done: wired appendFailure into checkTest')
else:
    # Diagnostic: show what surrounds the target
    idx = content.find(b'tests-precede-code violation')
    if idx >= 0:
        print('Context around violation:')
        print(repr(content[idx:idx+400]))
    sys.exit(1)
