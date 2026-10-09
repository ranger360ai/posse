# ranger-base-lw0s5 — a merge-back whose only textual conflict was the easy one

Landed 2026-10-09, git 2.50.1 / macOS 26.4.1. Companion to
`docs/notes.d/ranger-base-xea2y.md`, which is the procedure; this is what the
procedure does not catch.

`ranger-base-rjfec` closed with one commit (`d3d9c1e7`) on
`posse/gwart-posse-ranger-base-rjfec` — `posse <verb> -h/--help` read once
from the usage catalog, ahead of main's switch. `main` moved on by six
commits, the launcher's replay conflicted, and this bead was filed. The
replay was `xea2y`'s, unchanged and uneventful: `merge-tree --write-tree`,
no deletions, `cat-file blob` per path, no sequencer, nothing to leak.

## What `merge-tree` reported, and what was actually broken

```
CONFLICT (content): Merge conflict in CHANGELOG.md
Auto-merging cmd/posse/main.go
Auto-merging docs/notes.d/README.md
```

One conflict, and it was the cheap kind — both sides had added a block at the
head of `## Unreleased`. Resolution: keep both, newest first, which the two
commits' own author dates decide (`rjfec` 17:46 over `xko4n` 16:46).
`docs/notes.d/README.md` is generated, so it was regenerated rather than
hand-merged; `scripts/notes-index.py` produced exactly what the auto-merge
had, and `--check` exits 0.

**Two of the three real conflicts were in files git did not report at all,
and the third was inside the file it called clean.** `go vet` found the
first, `go test` the other two. Neither is visible to a textual merge, and
both were between THIS commit and beads that landed on `main` while it was in
flight — `ranger-base-cse63` (`159292c7`) and `ranger-base-1jmtm`
(`eddcd8f9`), both of them work on the same help surface, which is why.

### 1. One Go identifier, two files, no shared line

`verbhelp_qa_test.go` (arriving) and `subverbhelp_qa_test.go` (on `main`,
from `1jmtm`) each declared `mainVerbSwitch`, with different signatures —
`(t) (*ast.SwitchStmt, int)` against `(t, *ast.File) *ast.SwitchStmt`. Both
census main's `switch cmd` from the AST, for the same reason, from two beads
a day apart. Git saw two added files that share no text, so it merged them
cleanly into a package that does not compile:

```
vet: cmd/posse/verbhelp_qa_test.go:65:6: mainVerbSwitch redeclared in this block
```

Kept the arriving one, which parses `main.go` itself and also reports the
switch's index in main's body (one pin reads that); `subVerbs` now calls it
and dropped its own copy and its `go/parser` import. **A merge-back of a new
file in a Go package is a name-space merge, and `git merge-tree` cannot see
a name space.** `go vet` is the whole check and it is cheap.

### 2. Two pins asserting opposite things about one string

`cse63` made `posse backup -h` print the backup block WITH its config keys —
that bead's entire point was that the one place an operator asks the CLI
about `backup_dir:`, `backup_interval:` and the `queue_repo:` they arm over
was the one place that answered with none of them. It realized that inside
the verb's own flag loop.

`rjfec` moved the `-h/--help` reading AHEAD of the switch, so that arm no
longer fires for `posse backup -h`, and `catalogBlock` stops at the first
`config <key>:` line — deliberately, because backup's keys are followed
immediately by two `verify_box_*` keys that are no verb's. So the merged tree
had `TestCatalogBlockStopsAtAConfigKey` demanding the keys be absent and
`TestBackupHelpAnswersEveryFormWithTheKeys` demanding they be present, both
green on their own side of the base.

Resolved with `verbOwnHelp` in `main.go`: a map from catalog path to the
wider text that path answers with, consulted by `catalogHelp` before it
slices the catalog, with `backup` its one member. The reading still sits
ahead of the switch (`rjfec`'s invariant), the keys still print (`cse63`'s
deliverable), and the wider text is still a verbatim slice of the catalog —
`TestVerbOwnHelpWidensARealPath` holds all three for whatever is added there
next, and refuses an entry that widens nothing.

**One divergence, and it is a narrowing of a landed pin.** `cse63` asserted
all six backup help forms print the keys; `posse backup status --help` now
prints the status entry, like every other sub-verb the catalog names. Those
six forms existed because all six exited 1 with the one-line grammar and
printing the whole block was the only answer available; the house rule for a
sub-verb did not exist yet. The pin keeps exit 0 and "not the grammar line"
for all six and asks for the keys on the verb's own two.

### 3. A count that was right when it was written

`TestAParentVerbAnswersWithEveryEntryUnderIt` pinned `posse gates --help` at
4 entries. `1jmtm` gave `posse gates wrap` an entry while this bead was in
flight, so 5. The count is the point — it is what catches an entry that stops
being reachable under its parent — so it is updated by hand and never derived
from the catalog it is checking.

## The lesson, stated for the next merge-back

`merge-tree`'s report is a claim about TEXT. Three things it cannot make a
claim about, all three of which bit here in one commit:

- a declaration added to a package by two files that share no line;
- two tests that assert opposite things about one output;
- a number in one side's test that the other side's change makes wrong.

The cost of reading for them is `go vet ./<the package>` and then the
package's own tests — before the full suite, because all three fail fast. A
merge-back whose branch touches the same SURFACE as anything that landed
while it waited should expect them; textual distance between the two diffs is
no evidence at all, and here the two diffs that collided hardest did not
share a file.

## Verified

`go vet ./cmd/posse`, `go test ./cmd/posse` whole, by hand off the built
binary (`posse backup --help` carries six `config` lines and exits 0,
`posse backup status --help` is the status entry, `posse gates wrap --help`
answers, `posse kill -- --help` still names a session), `scripts/notes-index.py
--check`, `make fmt-check`, `make tree-check`, and `make test` whole.
