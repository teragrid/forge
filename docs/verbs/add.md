# forge add

Add a verified dependency with security scan.

## Synopsis

```
forge add <package[@version]> [--root <path>] [--dry-run]
```

## Description

`forge add` adds a dependency (Go module, npm package, etc.), runs
`forge scan security` against it, and only applies the change if no
High/Critical issues are found.

## Examples

```bash
forge add github.com/some/lib@v1.2.3
forge add express@4.18.2
forge add --dry-run lodash@latest
```
