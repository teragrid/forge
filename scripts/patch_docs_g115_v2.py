import sys
import re

path = r"i:\AI-Startup\forge\internal\cli\cmddocs\docs.go"
with open(path, 'rb') as f:
    raw = f.read()

content = raw.decode('utf-8', errors='surrogateescape')

# Step 1: Update the Short description and add brokenAnchors logic
# We need to replace the heal RunE body while keeping the surrounding structure
# Find the heal section
heal_start = content.find('func newHealCmd(root *string, dryRun *bool)')
heal_end = content.find('\n// --- helpers ---')
if heal_start < 0 or heal_end < 0:
    print("ERROR: can't find heal function bounds")
    sys.exit(1)

heal_section = content[heal_start:heal_end]
print("Found heal section, length:", len(heal_section))

new_heal = '''func newHealCmd(root *string, dryRun *bool) *cobra.Command {
\treturn &cobra.Command{
\t\tUse:   "heal",
\t\tShort: "Fix broken internal links, stale section headers, and broken anchors (G-115).",
\t\tRunE: func(cmd *cobra.Command, _ []string) error {
\t\t\tr, err := resolveRoot(*root)
\t\t\tif err != nil {
\t\t\t\treturn errcode.New(ErrDocsFailed, "resolve root", err)
\t\t\t}
\t\t\tout := cmd.OutOrStdout()
\t\t\tdocsDir := filepath.Join(r, "docs")
\t\t\tbroken, err := findBrokenLinks(r, docsDir)
\t\t\tif err != nil {
\t\t\t\treturn errcode.New(ErrDocsFailed, "scan links", err)
\t\t\t}
\t\t\t// G-115: also check broken anchors (#heading) within the docs tree.
\t\t\tbrokenAnchors, err := findBrokenAnchors(r, docsDir)
\t\t\tif err != nil {
\t\t\t\treturn errcode.New(ErrDocsFailed, "scan anchors", err)
\t\t\t}
\t\t\tif len(broken) == 0 && len(brokenAnchors) == 0 {
\t\t\t\tfmt.Fprintln(out, "docs heal: no broken links or anchors found")
\t\t\t\treturn nil
\t\t\t}
\t\t\tfor _, b := range broken {
\t\t\t\tfmt.Fprintf(out, "broken link:   %s: %s\\n", relOrAbs(r, b.file), b.link)
\t\t\t}
\t\t\tfor _, a := range brokenAnchors {
\t\t\t\tfmt.Fprintf(out, "broken anchor: %s: %s\\n", relOrAbs(r, a.file), a.link)
\t\t\t}
\t\t\tif *dryRun {
\t\t\t\tfmt.Fprintf(out, "docs heal (dry-run): %d broken links, %d broken anchors -- re-run without --dry-run to fix\\n",
\t\t\t\t\tlen(broken), len(brokenAnchors))
\t\t\t} else {
\t\t\t\tfixed, err := removeStaleLinks(broken)
\t\t\t\tif err != nil {
\t\t\t\t\treturn errcode.New(ErrDocsFailed, "remove stale links", err)
\t\t\t\t}
\t\t\t\tfixedAnchors, err := removeStaleLinks(brokenAnchors)
\t\t\t\tif err != nil {
\t\t\t\t\treturn errcode.New(ErrDocsFailed, "remove stale anchors", err)
\t\t\t\t}
\t\t\t\tfmt.Fprintf(out, "docs heal: removed %d stale links, %d stale anchors\\n", fixed, fixedAnchors)
\t\t\t}
\t\t\treturn nil
\t\t},
\t}
}'''

content = content[:heal_start] + new_heal + content[heal_end:]

# Step 2: Add findBrokenAnchors function before relOrAbs
anchor_fn = '''
// findBrokenAnchors scans markdown files for [text](#anchor) links where the
// target heading does not exist in the same file. G-115.
func findBrokenAnchors(_, dir string) ([]brokenLink, error) {
\tvar result []brokenLink
\terr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
\t\tif err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
\t\t\treturn nil
\t\t}
\t\tdata, err := os.ReadFile(path)
\t\tif err != nil {
\t\t\treturn nil
\t\t}
\t\tfileContent := string(data)
\t\t// Collect headings: "# Some Heading" -> "#some-heading"
\t\theadings := map[string]bool{}
\t\tfor _, line := range strings.Split(fileContent, "\\n") {
\t\t\tif !strings.HasPrefix(line, "#") {
\t\t\t\tcontinue
\t\t\t}
\t\t\ttext := strings.TrimLeft(line, "#")
\t\t\ttext = strings.TrimSpace(text)
\t\t\tslug := strings.ToLower(text)
\t\t\tslug = regexp.MustCompile(`[^a-z0-9\\- ]`).ReplaceAllString(slug, "")
\t\t\tslug = strings.ReplaceAll(slug, " ", "-")
\t\t\theadings["#"+slug] = true
\t\t}
\t\t// Check all #anchor links in the file.
\t\tfor _, m := range mdLinkRe.FindAllStringSubmatch(fileContent, -1) {
\t\t\tlink := m[2]
\t\t\tif !strings.HasPrefix(link, "#") {
\t\t\t\tcontinue
\t\t\t}
\t\t\tif !headings[link] {
\t\t\t\tresult = append(result, brokenLink{file: path, link: link})
\t\t\t}
\t\t}
\t\treturn nil
\t})
\treturn result, err
}

'''

insert_before = 'func relOrAbs('
if insert_before not in content:
    print("ERROR: relOrAbs not found")
    sys.exit(1)
content = content.replace(insert_before, anchor_fn + insert_before, 1)

with open(path, 'wb') as f:
    f.write(content.encode('utf-8', errors='surrogateescape'))

print("Done - G-115 applied")
