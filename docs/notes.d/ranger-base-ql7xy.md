# A test fixture that spelled a live open vuln-private id (ranger-base-ql7xy)

*2026-11-15 · dinesh · follows ranger-base-cq3vi*

## What changed

`internal/posse/vulnprivate_test.go` used a real, still-open `vuln-private`
bead id as a fixture — in two tables and, once, on a line that put a
description beside it. That was harmless while check 4's content arm diffed
the staged markdown alone. Since ranger-base-cq3vi widened that arm to every
staged path but the beads jsonl, `.go` is in scope, and the fixture became
the exact shape the gate refuses: anyone editing one of those lines has
their commit refused over a hole that is not theirs, and the own-bead
exemption cannot help them, because it drops only the id their branch is
recorded for.

The id is now `ranger-base-fixt3`, which no bead has — the synthetic id
ranger-base-cq3vi's own test already used, so the file is consistent with
itself. A header comment says why, and says the rule the next writer needs:
an id in this file's tables is either CLOSED or spells no bead at all.
`ranger-base-l751c` stays because it is closed, which is what
`Bd.OpenIssues` filters on — but a closed bead is only closed once, so new
rows should take the synthetic id.

## How it was verified

Check 4's own judge over the edited file, which is the only check that
matters here and is cheaper than reasoning about the render:

```
sed 's/^/+/' internal/posse/vulnprivate_test.go | posse gates vuln-private-scan
```

Exit 0 and no output after; before the change the same command named the
fixture id. `go test -run VulnPrivate ./internal/posse` is green (39 tests,
arm 1), the three touched tests among them, and `make fmt-check` passes.

Running that same per-file scan over **every** tracked file is worth
knowing about as a technique, and it is how the residual below was found.
Do it one file at a time: concatenating the tree lets a markdown unit
straddle a file boundary, which invents hits that are not there.

## Not closed here

- ranger-base-shft5
