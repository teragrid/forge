#!/usr/bin/env python3
"""Append DailySpend/DailyBudgetAlert methods to tokenledger.go."""

addition = (
    '\n'
    '// DailySpend returns the total CostUSD recorded in the ledger for the\n'
    '// calendar day of t (UTC).\n'
    'func (l *Ledger) DailySpend(t time.Time) (float64, error) {\n'
    '\tentries, err := l.ReadAll()\n'
    '\tif err != nil {\n'
    '\t\treturn 0, err\n'
    '\t}\n'
    '\ty, mo, d := t.UTC().Date()\n'
    '\tvar total float64\n'
    '\tfor _, e := range entries {\n'
    '\t\tey, emo, ed := e.Time.UTC().Date()\n'
    '\t\tif ey == y && emo == mo && ed == d {\n'
    '\t\t\ttotal += e.CostUSD\n'
    '\t\t}\n'
    '\t}\n'
    '\treturn total, nil\n'
    '}\n'
    '\n'
    '// DailyBudgetAlert returns a non-nil error when the ledger spend on the\n'
    '// calendar day of t (UTC) meets or exceeds limitUSD. A zero limitUSD is\n'
    '// treated as unlimited (always returns nil).\n'
    'func (l *Ledger) DailyBudgetAlert(t time.Time, limitUSD float64) error {\n'
    '\tif limitUSD <= 0 {\n'
    '\t\treturn nil\n'
    '\t}\n'
    '\tspent, err := l.DailySpend(t)\n'
    '\tif err != nil {\n'
    '\t\treturn err\n'
    '\t}\n'
    '\tif spent >= limitUSD {\n'
    '\t\treturn fmt.Errorf("tokenledger: daily spend USD%.4f meets/exceeds limit USD%.4f", spent, limitUSD)\n'
    '\t}\n'
    '\treturn nil\n'
    '}\n'
)

path = 'internal/tokenledger/tokenledger.go'
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()
content = content.rstrip('\n') + '\n' + addition
with open(path, 'w', encoding='utf-8', newline='\n') as f:
    f.write(content)
print('Done - DailyBudgetAlert added to tokenledger.go')
