import sys

path = r"i:\AI-Startup\forge\internal\cli\cmddocs\docs.go"
with open(path, 'rb') as f:
    raw = f.read()

content = raw.decode('utf-8', errors='surrogateescape')

old = '''func newHealCmd(root *string, dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "heal",
		Short: "Fix broken internal links and stale section headers.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return errcode.New(ErrDocsFailed, "resolve root", err)
			}
			out := cmd.OutOrStdout()
			docsDir := filepath.Join(r, "docs")
			broken, err := findBrokenLinks(r, docsDir)
			if err != nil {
				return errcode.New(ErrDocsFailed, "scan links", err)
			}
			if len(broken) == 0 {
				fmt.Fprintln(out, "docs heal: no broken links found")
				return nil
			}
			for _, b := range broken {
				fmt.Fprintf(out, "broken link: %s: %s\\n", relOrAbs(r, b.file), b.link)
			}
			if *dryRun {'''

new = '''func newHealCmd(root *string, dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "heal",
		Short: "Fix broken internal links, stale section headers, and broken anchors (G-115).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveRoot(*root)
			if err != nil {
				return errcode.New(ErrDocsFailed, "resolve root", err)
			}
			out := cmd.OutOrStdout()
			docsDir := filepath.Join(r, "docs")
			broken, err := findBrokenLinks(r, docsDir)
			if err != nil {
				return errcode.New(ErrDocsFailed, "scan links", err)
			}
			// G-115: also check broken anchors (#heading) within the docs tree.
			brokenAnchors, err := findBrokenAnchors(r, docsDir)
			if err != nil {
				return errcode.New(ErrDocsFailed, "scan anchors", err)
			}
			if len(broken) == 0 && len(brokenAnchors) == 0 {
				fmt.Fprintln(out, "docs heal: no broken links or anchors found")
				return nil
			}
			for _, b := range broken {
				fmt.Fprintf(out, "broken link:   %s: %s\\n", relOrAbs(r, b.file), b.link)
			}
			for _, a := range brokenAnchors {
				fmt.Fprintf(out, "broken anchor: %s: %s\\n", relOrAbs(r, a.file), a.link)
			}
			if *dryRun {'''

if old not in content:
    print("ERROR: old string not found, looking for differences...")
    # Print first 100 chars of old vs what's in file around that area
    idx = content.find('func newHealCmd')
    print(repr(content[idx:idx+200]))
    sys.exit(1)

content = content.replace(old, new, 1)

# Now also replace the end of the if *dryRun block
old_end = '''			if *dryRun {
				fmt.Fprintf(out, "docs heal (dry-run): %d broken links \u00e2\u0080\u0094 re-run without --dry-run to remove stale links\\n", len(broken))
			} else {
				fixed, err := removeStaleLinks(broken)
				if err != nil {
					return errcode.New(ErrDocsFailed, "remove stale links", err)
				}
				fmt.Fprintf(out, "docs heal: removed %d stale links\\n", fixed)
			}
			return nil
		},
	}
}'''

new_end = '''			if *dryRun {
				fmt.Fprintf(out, "docs heal (dry-run): %d broken links, %d broken anchors -- re-run without --dry-run to fix\\n",
					len(broken), len(brokenAnchors))
			} else {
				fixed, err := removeStaleLinks(broken)
				if err != nil {
					return errcode.New(ErrDocsFailed, "remove stale links", err)
				}
				fixedAnchors, err := removeStaleLinks(brokenAnchors)
				if err != nil {
					return errcode.New(ErrDocsFailed, "remove stale anchors", err)
				}
				fmt.Fprintf(out, "docs heal: removed %d stale links, %d stale anchors\\n", fixed, fixedAnchors)
			}
			return nil
		},
	}
}'''

if old_end not in content:
    print("ERROR: old_end not found")
    idx = content.find('if *dryRun {')
    print(repr(content[idx:idx+400]))
    sys.exit(1)

content = content.replace(old_end, new_end, 1)

# Now add findBrokenAnchors before the relOrAbs function
anchor_fn = '''
// findBrokenAnchors scans markdown files for [text](#anchor) links where the
// target heading does not exist in the same file. G-115.
func findBrokenAnchors(_, dir string) ([]brokenLink, error) {
	var result []brokenLink
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		// Collect headings: "# Some Heading" -> "#some-heading"
		headings := map[string]bool{}
		for _, line := range strings.Split(content, "\\n") {
			if !strings.HasPrefix(line, "#") {
				continue
			}
			text := strings.TrimLeft(line, "#")
			text = strings.TrimSpace(text)
			slug := strings.ToLower(text)
			slug = regexp.MustCompile(`[^a-z0-9\\- ]`).ReplaceAllString(slug, "")
			slug = strings.ReplaceAll(slug, " ", "-")
			headings["#"+slug] = true
		}
		// Check all #anchor links in the file.
		for _, m := range mdLinkRe.FindAllStringSubmatch(content, -1) {
			link := m[2]
			if !strings.HasPrefix(link, "#") {
				continue
			}
			if !headings[link] {
				result = append(result, brokenLink{file: path, link: link})
			}
		}
		return nil
	})
	return result, err
}

'''

if 'findBrokenAnchors' not in content:
    insert_before = 'func relOrAbs('
    if insert_before not in content:
        print("ERROR: insert point not found")
        sys.exit(1)
    content = content.replace(insert_before, anchor_fn + insert_before, 1)

with open(path, 'wb') as f:
    f.write(content.encode('utf-8', errors='surrogateescape'))

print("Done")
