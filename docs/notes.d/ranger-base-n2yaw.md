# Two gates that judged what nobody had measured (ranger-base-n2yaw)

Three findings from the ranger-base-3fm26 verify, two subjects: the
record-reach gate's directory filter (`internal/posse/reachability.go`) and
`scripts/posse-land-check`'s arm derivation. All four closes that verify read
VERIFIED; these were the live findings beside them.

Dated 2026-11-11. Every number below was taken on the D1 host (Linux 7.2.15,
`go version go1.27.3-X:nodwarf5 linux/amd64`, `GOTOOLCHAIN=local`), from this
bead's worktree.

## 1. The `isDirPath` filter became load-bearing and nothing said so

`recordTargets` filters its git half through `isDirPath`. The filter has been
there since 9c00e192, for a `.git` that does not exist, and it had nothing to
do: the git half was always `beadsGitDirs`, which names directories only.
ranger-base-6hjm5 changed the producer to `sessionGitGrants` for a store
inside the session's own repo, and three of that function's six entries are
FILE paths — `<common>/packed-refs.lock`, `<common>/refs/heads/<branch>` and
that path plus `.lock` (ADR 0059 D1).

The four shipped arms cannot see it by construction: each asserts the targets
are a SUBSET of the grants, and all three lock paths ARE granted. So the
filter could go and every arm would stay green, while
`seatbeltReachProbe`'s `: > <target>/posse-reach-probe.<pid>` answers ENOTDIR
on the ref file and ENOENT on the two locks, the record row reads unrealized,
and the launch refuses — for exactly the open-PID / linked-worktree /
store-inside-its-own-repo shape ranger-base-6hjm5 exists to fix.

MEASURED, with the filter replaced by
`out = append(out, recordGitTargets(ag, dir, home)...)`:
`TestQARecordTargetsAreAllDirectoriesTheProbeCanWriteIn` reds and names all
three paths and their kinds (`-` absent, `f` regular file), while the four
arms beside it stay green. The ref file is real, not hypothetical: the
fixture's `<common>/refs/heads/posse/developer-2-probe` is a regular file,
as `<common>/refs/heads/posse/<session>` is in every dispatched worktree.

The arm reads the KIND and not the containment — the way
seatbeltdenygitgrant's arms do, and for the same reason: "inside a git dir"
and "is it a file" are different questions, and only the second decides
whether the probe can answer at all. Its non-vacuity check is a `t.Fatalf`
on the producer underneath: the day `recordGitTargets` hands this shape
nothing but directories, the arm is no longer over the defect and says so
rather than passing.

## 2. The arm derivation read one of the toolchain's two spellings

`scripts/posse-land-check`'s `arm_declarations` awk matched `//go:build`
only, so an arm declared the legacy way was invisible to it and went unvetted
at the one gate that judges a replayed tip. The script's own header claimed
the opposite as a property ("a fourth arm is covered by the commit that
declares it, with nobody remembering this file").

MEASURED 2026-11-11, go1.27.3 linux/amd64, over a planted module whose
`internal/posse/arm5.go` carries `// +build posse_arm5` and no `//go:build`
line:

```
$ go list -f '{{.GoFiles}}' ./internal/posse
[x.go]
$ go list -tags posse_arm5 -f '{{.GoFiles}}' ./internal/posse
[arm5.go x.go]
$ gofmt -l internal/posse/arm5.go
internal/posse/arm5.go
```

Honoured by the toolchain, and flagged by gofmt. The compensating control is
real but not this gate's: `make fmt-check` reds such a file, fmt-check is not
a prerequisite of the land check, and AGENTS.md's own ranger-base-rulbl
paragraph records four commits that reached main not gofmt-clean straight
through that gap.

The fix is one more pattern in the same awk pass, in the toolchain's own
shape — `//`, optional space, `+build`, then whitespace before the tags, so a
`+build` naming no tag declares nothing and the `pre` guard still refuses
both spellings below the package clause.

## 3. Nothing in the suite ran the script

`make test` never invoked it; `scripts/shell-syntax.sh` parsed it and
`make scripts-check`'s censuses read scripts/ as text. The part designed to
outlive its author — the derivation that covers a fourth arm with nobody
editing the file — was witnessed only by four by-hand `--plan` readings on
ranger-base-pn621's close. A derivation returning the WRONG dirs (as against
none, which the `[ -n "$tags" ]` guard catches) would be found by a missed arm
at a landing and never by a red.

`internal/posse/landcheckplan_qa_test.go` is eight arms over the `--plan`
output, arm 2 of the suite, **0.14s for all eight** (MEASURED, `-count=1`):
the full baseline step list including the one cross-GOOS vet read as a
pattern; a fourth arm planted in an existing package; the same tag in a NEW
package, vetted there and not in the one the script was written beside; the
legacy spelling (arm 4, which RED before the fix above); a build line at
column 0 inside a raw string below the package clause, in both spellings, read
as a mention; the zero-arms red with its real exit code and sentence; a real
run that reds where the plan read green; and the D1 cwd contract.

### Why a planted fixture and not an exported tree

The finding's repro said "export a tree". A pin over this repo's own arms
would assert a package list every unrelated landing can move, and it would be
a tree-wide pin living in a package nobody runs whole (ranger-base-rulbl's
class) — reachable only behind a Makefile door, with a register entry, a door
variable and an AGENTS.md row to keep in step. The subject is the DERIVATION,
not this tree's arm count, so the fixture spells its own file set: four files
and a go.mod, plus whatever the arm plants.

That is also what keeps the pin out of the tree-wide class by the class's own
rules. It calls no repo-root helper and enumerates nothing: it reads ONE NAMED
FILE through a climb (`filepath.Abs("../../scripts/posse-land-check")`), which
is `TestQAOneRepoRootHelperInTheTestPackage`'s own negative case, and the
fixture it walks is a `t.TempDir()`. `make register-check` agrees — no door is
owed.

### `--plan` is free, and the fixture proves it

Plan mode runs no toolchain step, so the whole pin costs one `go env` per
fixture. Every fixture carries a `.go` file the toolchain refuses, and arm 7
is the witness: the same tree without `--plan` exits 1 in **15ms** on
`step 1: go build ./...`, naming the syntax error. So the plan's green is the
absence of execution rather than a tree that happens to compile, and ADR 0087
D6's "it executes nothing the branch wrote" has a positive witness instead of
resting on a reading of the script.

## Not filed, and said so nobody re-finds them as gaps

Carried over from ranger-base-3fm26's reading, all MEASURED there:

- A directory whose NAME holds a space, declaring an arm, reds the run with
  `package dir is not in std` — the `$dirs` word-splitting the script
  documents as deliberate. Fail-closed over a tree Go itself will not build
  normally.
- A branch that ADDS a cgo file does not break D6's "executes nothing the
  branch wrote": Go's own `#cgo` flag allow-list refuses the
  compiler-plugin shape outright (`invalid flag in #cgo CFLAGS:
  -fplugin=...`), so the branch does not get to choose the C compiler's argv.
- A `.go` file under a `testdata/` directory declaring an arm IS picked up and
  vetted as its own package pattern. No `.go` file exists under any testdata/
  in this tree today, so there is no exposure — but testdata is where a repo
  keeps Go that is not meant to compile, and that would be a spurious red at a
  gate where a spurious red blocks a landing.
