#!/usr/bin/env python3
"""Append G-041 cache hit rate tests to llmcache_test.go."""

addition = '''

// G-041: TestCache_HitRateReported verifies that HitRate() correctly reflects
// the ratio of cache hits to total lookups. This validates the forge insights
// cache hit metric source.
func TestCache_HitRateReported(t *testing.T) {
\t// Reset global counters so previous test runs don't interfere.
\tResetStats()

\tdir := t.TempDir()
\tc, err := Open(dir)
\tif err != nil {
\t\tt.Fatal(err)
\t}

\t// No lookups yet -- HitRate should be 0.
\tif got := HitRate(); got != 0 {
\t\tt.Errorf("HitRate before any lookup: want 0, got %g", got)
\t}

\tkey := Key("model", "sys", "user-g041")

\t// First lookup: miss.
\tc.GetWithStats(key, nil)
\tif got := HitRate(); got != 0 {
\t\tt.Errorf("HitRate after first miss: want 0, got %g", got)
\t}

\t// Store a value.
\t_ = c.Store(key, "model", "cached-response", nil)

\t// Second lookup: hit.
\tc.GetWithStats(key, nil)
\t// 1 hit / 2 total = 0.5
\twant := 0.5
\tif got := HitRate(); got != want {
\t\tt.Errorf("HitRate after 1 hit 1 miss: want %g, got %g", want, got)
\t}

\t// Third lookup: hit.
\tc.GetWithStats(key, nil)
\t// 2 hits / 3 total ~0.6667
\tif got := HitRate(); got <= 0.6 || got > 0.7 {
\t\tt.Errorf("HitRate after 2 hits 1 miss: want ~0.667, got %g", got)
\t}
}

// TestCache_HitRate_ZeroWhenNoLookups verifies HitRate returns 0 when no
// lookups have been made.
func TestCache_HitRate_ZeroWhenNoLookups(t *testing.T) {
\tResetStats()
\tif got := HitRate(); got != 0 {
\t\tt.Errorf("HitRate with zero lookups: want 0, got %g", got)
\t}
}
'''

path = 'internal/llmcache/llmcache_test.go'
with open(path, 'r', encoding='utf-8') as f:
    content = f.read()

# Remove trailing newline before appending to avoid double blank line
content = content.rstrip('\n') + '\n' + addition

with open(path, 'w', encoding='utf-8', newline='\n') as f:
    f.write(content)

print("Done - G-041 appended to llmcache_test.go")
