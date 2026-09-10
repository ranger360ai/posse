## The YAML subset

Config, recipes, meta files, and agent frontmatter all use the same flat
subset (`internal/posse/yamlflat.go`, ~100 lines, no deps):

```yaml
key: value          # scalars; "null", "~" and empty mean unset
key: [a, b, c]      # inline lists
key:                # block lists
  - a
key:                # one-level maps
  sub: value
```

Top-level keys only (plus one nesting level), `#` comments, a wrapping
pair of double quotes stripped (a lone leading or trailing `"` is data —
`command: … "$(cat {file})"` keeps its quote), no anchors/multiline/deeper
nesting.

