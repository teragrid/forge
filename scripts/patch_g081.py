import os

new_test = '''
// TestErrorCodes_NoDuplicates verifies that All() returns no duplicate codes (G-081).
func TestErrorCodes_NoDuplicates(t *testing.T) {
	t.Parallel()
	all := All()
	seen := make(map[Code]bool, len(all))
	for _, c := range all {
		if seen[c] {
			t.Errorf("duplicate error code registered: %d", c)
		}
		seen[c] = true
	}
}

// TestErrorCodes_AllIsSorted verifies that All() returns codes in ascending order (G-081).
func TestErrorCodes_AllIsSorted(t *testing.T) {
	t.Parallel()
	all := All()
	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] {
			t.Errorf("All() not sorted at index %d: %d not greater than %d", i, all[i], all[i-1])
		}
	}
}
'''

path = os.path.join('internal', 'errcode', 'errcode_test.go')
with open(path, 'a', encoding='utf-8', newline='\n') as f:
    f.write(new_test)
print('Done')
