import sys

path = r'i:\AI-Startup\forge\internal\cli\cmdship\ship.go'
with open(path, 'rb') as f:
    content = f.read()

# 1. Wire appendFailure into checkBreakdown LLM error block
# Target: the return cp after "no breakdown.md" error
old = (b'cp.Detail = fmt.Sprintf("no breakdown.md [LLM:%s '
       + b'\xc3\xa2\xe2\x82\xac\xe2\x80\x9d %s] \xc3\xa2\xe2\x82\xac\xe2\x80\x9d run forge ship breakdown to generate",\r\n'
       b'\t\t\t\t\tpipe.ProviderName(), llmErrNote(err))\r\n'
       b'\t\t\t\treturn cp\r\n')
new = (b'cp.Detail = fmt.Sprintf("no breakdown.md [LLM:%s '
       + b'\xc3\xa2\xe2\x82\xac\xe2\x80\x9d %s] \xc3\xa2\xe2\x82\xac\xe2\x80\x9d run forge ship breakdown to generate",\r\n'
       b'\t\t\t\t\tpipe.ProviderName(), llmErrNote(err))\r\n'
       b'\t\t\t\t// G-011: record breakdown failure for future learning context.\r\n'
       b'\t\t\t\tappendFailure(root, "breakdown", description, cp.Detail)\r\n'
       b'\t\t\t\treturn cp\r\n')

count = content.count(old)
print(f'Breakdown patch: Found {count} occurrences')
if count == 1:
    content = content.replace(old, new)
    print('Breakdown patch: applied')
else:
    # diagnostic
    idx = content.find(b'no breakdown.md')
    print(repr(content[idx:idx+300]))
    sys.exit(1)

with open(path, 'wb') as f:
    f.write(content)
print('Done')
