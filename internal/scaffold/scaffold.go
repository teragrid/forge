// Package scaffold renders an embedded project template into a target
// directory (DEV-M0-13). MVP scope:
//
//   - Templates are embedded via go:embed; one built-in template ("go-service").
//   - File names ending in `.tmpl` get text/template substitution and the
//     suffix stripped.
//   - File names containing `__name__` get replaced with the target project name.
//   - Refuses to write into a non-empty directory unless Force is set.
//   - Determinism: file walk is sorted; for the same Vars and Force the output
//     is byte-identical (DEV-M0-13 TC-13-03 idempotency).
package scaffold

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/teragrid/forge/internal/errcode"
)

//go:embed all:templates
var templatesFS embed.FS

// Reserved error codes (range 2200..2299 -> scaffold).
var (
	ErrUnknownTemplate = errcode.Register(errcode.Code(2200), "unknown scaffold template")
	ErrTargetNotEmpty  = errcode.Register(errcode.Code(2201), "target directory not empty")
	ErrTemplateRender  = errcode.Register(errcode.Code(2202), "template render failed")
	ErrWrite           = errcode.Register(errcode.Code(2203), "write failed during scaffold")
)

// Vars are passed to every `.tmpl` file and used in path placeholder
// substitution.
type Vars struct {
	Name        string // project name (also replaces `__name__` in paths)
	Module      string // Go module path (defaults to "example.com/<name>")
	ForgeVer    string // forge CLI version stamping the managed blocks
	Description string
}

// Options controls the render.
type Options struct {
	Template string    // template directory under templates/
	Target   string    // destination directory (created if missing)
	Vars     Vars      // template variables
	Force    bool      // overwrite into a non-empty target
	Out      io.Writer // for "wrote N files" line; defaults to stdout
}

// Result summarises a successful scaffold.
type Result struct {
	Template string
	Target   string
	Files    []string // relative to Target, sorted
}

// AvailableTemplates lists the embedded template names.
func AvailableTemplates() []string {
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Render writes the template into opt.Target.
func Render(opt Options) (*Result, error) {
	if opt.Vars.Name == "" {
		opt.Vars.Name = filepath.Base(opt.Target)
	}
	if opt.Vars.Module == "" {
		opt.Vars.Module = "example.com/" + opt.Vars.Name
	}
	if opt.Vars.ForgeVer == "" {
		opt.Vars.ForgeVer = "0.0.0-dev"
	}

	root := "templates/" + opt.Template
	if _, err := fs.Stat(templatesFS, root); err != nil {
		return nil, errcode.Newf(ErrUnknownTemplate, err,
			"template %q not found; available: %v", opt.Template, AvailableTemplates())
	}

	if err := ensureTarget(opt.Target, opt.Force); err != nil {
		return nil, err
	}

	files := []string{}
	walkErr := fs.WalkDir(templatesFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel := strings.TrimPrefix(p, root+"/")
		rel = strings.ReplaceAll(rel, "__name__", opt.Vars.Name)

		// `.tmpl` files get rendered AND have the suffix stripped from the
		// destination name.
		render := strings.HasSuffix(rel, ".tmpl")
		if render {
			rel = strings.TrimSuffix(rel, ".tmpl")
		}

		dst := filepath.Join(opt.Target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755) //nolint:gosec // intentional dir perms
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec
			return errcode.New(ErrWrite, "mkdir parent", err)
		}
		body, err := templatesFS.ReadFile(p)
		if err != nil {
			return errcode.New(ErrWrite, "read embedded "+p, err)
		}
		if render {
			body, err = renderTemplate(p, body, opt.Vars)
			if err != nil {
				return err
			}
		}
		if err := os.WriteFile(dst, body, 0o600); err != nil {
			return errcode.New(ErrWrite, "write "+dst, err)
		}
		files = append(files, rel)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return &Result{Template: opt.Template, Target: opt.Target, Files: files}, nil
}

func ensureTarget(target string, force bool) error {
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(target, 0o755) //nolint:gosec
		}
		return errcode.New(ErrWrite, "stat target", err)
	}
	if !info.IsDir() {
		return errcode.Newf(ErrWrite, nil, "target %q is not a directory", target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return errcode.New(ErrWrite, "read target", err)
	}
	if len(entries) > 0 && !force {
		return errcode.Newf(ErrTargetNotEmpty, nil,
			"target %q is not empty; pass --force to scaffold over it", target)
	}
	return nil
}

func renderTemplate(name string, body []byte, vars Vars) ([]byte, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(body))
	if err != nil {
		return nil, errcode.New(ErrTemplateRender, "parse "+name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, errcode.New(ErrTemplateRender, "execute "+name, err)
	}
	return []byte(buf.String()), nil
}

// FilesEqual compares two scaffold renders for byte-identity. Test helper.
func FilesEqual(a, b string) (bool, error) {
	ab, err := os.ReadFile(a) //nolint:gosec
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b) //nolint:gosec
	if err != nil {
		return false, err
	}
	if len(ab) != len(bb) {
		return false, nil
	}
	for i := range ab {
		if ab[i] != bb[i] {
			return false, nil
		}
	}
	return true, nil
}
