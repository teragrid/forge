#!/usr/bin/env python3
"""Append G-043 tests to tokenledger_test.go."""

path = 'internal/tokenledger/tokenledger_test.go'

lines = [
    '\n',
    '// G-043: TestTokenLedger_DailyBudgetAlert\n',
    'func TestTokenLedger_DailyBudgetAlert(t *testing.T) {\n',
    '\tt.Parallel()\n',
    '\tl := ledgerAt(t)\n',
    '\tnow := time.Now().UTC()\n',
    '\n',
    '\t// Under limit: no alert.\n',
    '\t_ = l.Append(tokenledger.Entry{Time: now, Model: "gpt-4o", CostUSD: 0.05})\n',
    '\tif err := l.DailyBudgetAlert(now, 0.10); err != nil {\n',
    '\t\tt.Errorf("DailyBudgetAlert under limit: unexpected error: %v", err)\n',
    '\t}\n',
    '\n',
    '\t// Add more to breach the limit.\n',
    '\t_ = l.Append(tokenledger.Entry{Time: now, Model: "gpt-4o", CostUSD: 0.06})\n',
    '\tif err := l.DailyBudgetAlert(now, 0.10); err == nil {\n',
    '\t\tt.Error("DailyBudgetAlert over limit: want error, got nil")\n',
    '\t}\n',
    '}\n',
    '\n',
    'func TestTokenLedger_DailyBudgetAlert_ZeroLimitIsUnlimited(t *testing.T) {\n',
    '\tt.Parallel()\n',
    '\tl := ledgerAt(t)\n',
    '\tnow := time.Now().UTC()\n',
    '\t_ = l.Append(tokenledger.Entry{Time: now, Model: "gpt-4o", CostUSD: 999.0})\n',
    '\tif err := l.DailyBudgetAlert(now, 0); err != nil {\n',
    '\t\tt.Errorf("zero limit should be unlimited, got error: %v", err)\n',
    '\t}\n',
    '}\n',
    '\n',
    'func TestTokenLedger_DailySpend_ExcludesOtherDays(t *testing.T) {\n',
    '\tt.Parallel()\n',
    '\tl := ledgerAt(t)\n',
    '\ttoday := time.Now().UTC()\n',
    '\tyesterday := today.AddDate(0, 0, -1)\n',
    '\t_ = l.Append(tokenledger.Entry{Time: today, Model: "m", CostUSD: 0.10})\n',
    '\t_ = l.Append(tokenledger.Entry{Time: yesterday, Model: "m", CostUSD: 0.90})\n',
    '\tgot, err := l.DailySpend(today)\n',
    '\tif err != nil {\n',
    '\t\tt.Fatalf("DailySpend: %v", err)\n',
    '\t}\n',
    '\tif got != 0.10 {\n',
    '\t\tt.Errorf("DailySpend: want 0.10, got %g", got)\n',
    '\t}\n',
    '}\n',
]

with open(path, 'a', encoding='utf-8') as f:
    f.writelines(lines)
print('Done - G-043 appended to tokenledger_test.go')
