# A scratch root under scripts/ names a namespace the reaper has a class for (ranger-base-w8n6w)

2026-11-12. Fourth bead in one family and the first with a deliverable that is
not a rename: ranger-base-m0j9n (prefixes with no class row), ranger-base-i78k9
(nine bare roots, whose own census named six of them) and ranger-base-tbukc
(five templated roots that matched no class at all) each censused `scripts/` by
hand, fixed what they found, and left nothing in the tree asserting the
property. So root 28 was going to be found by the next leak.

## The rule

A live `mktemp -d` under `scripts/` names `${TMPDIR:-/tmp}/posse-<what>.XXXXXX`
or is rostered with its reason. Three parts, three different reasons:

- `posse-` is the reaper's **posse-scratch** glob (`scripts/reap-test-tmp.sh`'s
  class table, 120m grace). It is the only row in that table that reaches a
  shell root; the rest name Go's `$WORK`, `t.TempDir`'s `Test*[0-9]`, and three
  dead fixture names kept to drain a pre-fix backlog. tbukc's five had a
  template and still matched nothing.
- `${TMPDIR:-/tmp}` is where the reaper **looks** — its default roots are
  `/tmp` plus this shell's `$TMPDIR` when that is elsewhere
  (reap-test-tmp.sh:625-632). A hardcoded `/tmp/posse-x.XXXXXX` is collected on
  this box and ignores the TMPDIR a caged seat is given, which is the one case
  where the two spellings differ.
- `.XXXXXX` is mktemp's own requirement, asserted so the pin's answer is a
  template and not a prefix someone guessed at.

## MEASURED 2026-11-12 (linux, this box)

36 grep hits for `mktemp -d` under `scripts/`: 8 comment lines explaining the
mechanism, 1 prose row inside verify-box.sh's quoted `ROSTER_EOF` heredoc, and
**27 live roots** — 24 of them already the house spelling after tbukc's
renames, 3 on the roster. The bead's own hand census said "28 hits, one of them
prose" over a grep that had already dropped the comments; 27 + 1 is the same
answer from the other side, and it is the last hand census of this directory.

The three roster rows, each a different reason the reaper does not need to
reach it:

| root | why |
|---|---|
| `audit-silent-reverts.sh:250` `$SR_TMPROOT/XXXXXX` | nested inside a conforming root made at :607 — collecting the parent takes it |
| `verify-ghost-composer.sh:225` `/private/tmp/pghost.XXXXXX` | darwin-only; the gate does not run on linux |
| `verify-self-close.sh:62` `/private/tmp/pselfclose.XXXXXX` | darwin-only, and pinned BY NAME in `internal/posse/selfclose_qa_test.go`, so renaming it crosses the arm boundary — which is why tbukc left it out |

## The pin

`internal/treepins/mktemproot_qa_test.go`, in `make scripts-check` (fifth arm
of a door whose subject is already `scripts/` alone):

- `TestQAEveryMktempDirRootUnderScriptsIsReapedOrRostered` — walks `scripts/`,
  so it is in internal/treepins' derived tree-wide class and is named by
  `QA_SCRIPTS_PINS`. Two-way: a roster row the census no longer finds reds as
  stale, and a row that names the house template reds as redundant. Two floors
  (40 files, 20 roots) so a walk that read nothing cannot pass quietly.
- `TestQATheMktempRootReaderCanStillSayNo` (undoored control) — planted
  fixtures under `t.TempDir()`: a comment, a `mktemp` with no `-d`, a heredoc
  body, a trailing comment, a bare `-d`, a hardcoded `/tmp`, a `$VAR` root;
  plus 15 near-miss templates and 8 flag spellings including `-p DIR`, which is
  the one place a wrong answer would be permissive rather than strict.

Mutation-checked by hand against a real script as well: `verify-id-recycle.sh`
re-spelled `mktemp -d` bare, and again as `${TMPDIR:-/tmp}/idrecycle.XXXXXX`
(tbukc's shape), reds the door naming the file, the line and the template.

## A heredoc body is classified, not dropped

A heredoc is data this script hands to something else, so a `mktemp -d` in one
is not a root this script creates — but a heredoc that RENDERS a script with a
bare `mktemp -d` leaks a dir under the rendered script's name, which is this
family's hole exactly. There is one such line in the tree and it is prose in a
roster table, so the pin holds the classification (`mtText`) rather than
guessing: a new heredoc naming `mktemp -d` reds until someone says which of the
two it is.

## Blind spots, stated

A `mktemp` with no `-d` (a file, which no class in that table globs), a root
built as `mkdir -p "$TMPDIR/$$"` or any other spelling of the same idea, a
second `mktemp` on one line, and Go's `os.MkdirTemp` roots (the tests' half of
the posse-scratch class). The subject is the one thing all four beads in this
family found by hand: `mktemp -d` under `scripts/`.
