# The sweep's one unopt-outable directory, and the spelling that is still outside the fence (ranger-base-wq349)

Four closes verified in one bundle on 2026-10-31 — ranger-base-57045 (hoover),
ranger-base-h1q5o (gwart), ranger-base-56eln (dinesh), ranger-base-l5v9k
(gwart). All four verify on their own acceptance. Two of them carry a live
finding beside the verdict, and both findings are the same shape as the bead
they sit next to: **the fix closed the spelling the bead named, and the class
has one more spelling in it.**

Environment for everything below: darwin 25.6.0, bash 3.2, go 1.26.10,
worktree `holden-posse-ranger-base-wq349`. HEAD `5b8e5e01` for the three closes
that landed; the 56eln work read at its branch tip `e27b7618`, which has not
reached main (`ranger-base-rq986` is the merge-back block and was in flight at
a sibling seat while this ran). Scripts are in the session scratchpad; nothing
in this repo was run as a privileged user and no credential file was read.

## 1. verify-credential-paths: HOME is the directory nobody can opt out of

ranger-base-57045 filed, and ranger-base-l5v9k fixed, a relative
`CLAUDE_CONFIG_DIR` / `CLAUDE_SECURESTORAGE_CONFIG_DIR` being taken verbatim —
so the same value read `FINDING` from one cwd and `clean` from another. Both
halves re-measured here.

MEASURED, the defect, against the PRE-FIX bytes (`git show
e98f5a84:scripts/verify-credential-paths.sh`, md5 `f6e5adcb…`), fake HOME, a
zero-byte credential file planted in `A/rel` only:

| variable | cwd=A | cwd=B |
|---|---|---|
| `CLAUDE_CONFIG_DIR=rel` | `FINDING rel/<the file>`, exit 1 | `absent rel` → `clean`, exit 0 |
| `CLAUDE_SECURESTORAGE_CONFIG_DIR=rel` | `FINDING rel/<the file>`, exit 1 | `absent rel` → `clean`, exit 0 |

So hoover's widening of the bead (the second variable has the same gap) is
right, and posse really does absolutize both: `credentialDirNamed`
(credential.go, `filepath.IsAbs` → `filepath.Abs`) for the secure-storage one
and `runtimeHomeOverride` (runtimehome.go:107) for the config one, which
`ClaudeConfigDirIn` is the only other reader of. The gap is latent on this box
— re-measured in this seat: `CLAUDE_CONFIG_DIR` absolute (a path under the operator's home),
`CLAUDE_SECURESTORAGE_CONFIG_DIR` set-and-EMPTY, `HOME` absolute.

MEASURED, the fix, at `5b8e5e01`: 24 runs with a relative value all exit 2,
all print the variable name and `nothing measured, not a pass`, and none
prints `clean` or `FINDING`. Sixteen of them are the grid — two variables ×
`rel`, `./rel`, `rel/`, `../A/rel` × a cwd holding the planted file and one
that does not. The other eight are two mixed runs (one variable absolute, the
other relative, in both orders), a literal `~/.claude`, a single space, a
value with an embedded newline, and two `--quiet` runs, which refuse on
stdout like every other arm of this script. `//abs` is accepted as
absolute; an absolute value still scans (exit 1 naming the file,
exit 0 over an empty dir); set-and-empty is unchanged (`clean — 0 findings in
1 scanned`); a relative value beside an absolute one still refuses, in either
order. Six aimed mutants of the refusal all die, each for its own reason — no
refusal at all, `exit 1`, a message without the variable name, resolving
instead of refusing, refusing an absolute value too, refusing only one of the
two variables — and a seventh mutant that drops the dedupe survives, as it
should: another arm of the same file owns that.

**And the directory the sweep always scans is still read against the sweep's
own cwd.** `dirs=("$HOME/.claude")` is seeded before `add_dir` is ever called,
and line 54 asks only whether `HOME` is NON-EMPTY. MEASURED 2026-10-31 with
`HOME=relhome` and a credential file planted in `A/relhome/.claude`:

    cwd=A  →  FINDING  relhome/.claude/<the file>   exit 1
    cwd=B  →  clean    relhome/.claude                            exit 0
    HOME=. →  absent   ./.claude  → "nothing measured, not a pass"  exit 2

Same box, same environment, same store of record, opposite verdicts — and the
`FINDING` line names a relative path that means nothing to whoever reads the
report, which was ranger-base-57045's second reading verbatim. This is the one
directory an operator cannot opt out of, and it is the arm the script's own
head comment leans on for a present-but-empty
`CLAUDE_SECURESTORAGE_CONFIG_DIR`.

`TestQACredentialPathsRelativeHomeIsStillCwdDependent`
(internal/treepins/credentialpaths_qa_test.go) is the live-defect pin: it
ships GREEN asserting the hole, with the inversion in its failure message, and
the third assertion — `hitCode == missCode` — is the one no fix can satisfy
while the hole is open. MEASURED able to fail two ways: refusing a relative
HOME with exit 2 (the recommended fix, the sibling of what `add_dir` now does)
reds it on all three assertions; resolving HOME against the sweep's cwd (the
WRONG fix, for the reason l5v9k's own spec gives — the launcher cwd cannot be
recovered) reds it too. Neither mutant reds any other arm of that file, so the
fix costs nothing already pinned.

## 2. ranger-base-h1q5o: what a replay has to be checked for is what it DROPPED

gwart replayed `d59bc965` (na4rm) rather than rebasing it, per the
ranger-base-xea2y recipe, and landed it as `251e6442`. Author identity and
AUTHOR date are byte-preserved (`2026-10-31T21:22:28-04:00`, committer date
`22:54:02`) and so is the subject, so `git log --grep ranger-base-na4rm`
still pairs them. The merge-blocked pin for that branch is gone from
`refs/posse/merge-blocked/**`; only 56eln's remains.

The claim worth checking mechanically is gwart's "every line of main's the
merge drops is one na4rm deliberately rewrote". Checked the other way round,
which a replay cannot talk its way out of: for every file both sides touched,
the lines **main added** between the branch point (`72381699`) and the
replay's parent (`3fe66e84`) that are NOT present at HEAD.

| file | lines main added in the window | absent at HEAD |
|---|---|---|
| internal/treepins/treewidedoor_qa_test.go | 350 | 13 |
| Makefile | 28 | 0 |
| internal/posse/runtimehome_qa_test.go | 57 | 0 |
| docs/notes.d/README.md | 3 | 0 |
| internal/posse/shelfpidlint_qa_test.go | 1 | 0 |
| AGENTS.md, internal/treepins/{README.md,armtags_qa_test.go} | 0 | 0 |

All 13 are head-comment prose, and all 13 are REFLOW, not loss: the
ranger-base-8yv7z reading they belong to is at HEAD in full (`81s, 68s and
106s in that order, with a one-minute load average of 11.15, 9.45 and 10.76`,
`both suite slots held by sibling seats throughout`, the 38s-spread argument
and the `0.464-0.514s measured alone` cost of that bead's own pins), rewrapped
around the new count sentence. Nothing of main's was dropped.

`make tree-check` at `5b8e5e01`: all seventeen doors `ok`, **108.97s wall at a
one-minute load that went 26.10 → 33.07 across the run**, both suite slots
held by sibling seats throughout. Worth adding to the series AGENTS.md carries
because it is the first reading of this class taken while the load was ABOVE
30: 55-56s at a load of 3-5, 81-89s at a steady 25-28, 109s at a moving 26-33.
It does not contradict the "what the spread tracks is how much the load MOVES"
reading — it is one run, not three, so it says nothing about spread.

## 3. ranger-base-56eln: the fence is complete for the spelling the bead named

dinesh's close verifies, and the parts worth recording are the ones proved
with an unmutated control on the parent tree (`975d0d82`) beside the fix tree
(`e27b7618`), both as `git archive` scratch trees. In that rig
`TestQATheTreeWideDoorsReportRealDrift` reds on **both** trees — three
`seedpub` pins want real git history and the archive has none (`git rev-list
--max-parents=0 HEAD: exit status 128`) — so that red is the rig, and the
register's other nine pins are green at the 56eln tree, arm 4 (the
`ninety-seven pins and fifteen doors` sentence) included.

| arm | parent `975d0d82` (pre-fix) | tip `e27b7618` (post-fix) |
|---|---|---|
| gatedkeychain, walked `cmd` renamed | **PASS in 0.01s** — the escape | RED, naming the walk |
| visibility, walked `cmd` renamed | **PASS in 0.01s** — the escape | RED, naming the walk |
| exactmodel, read `cmd/posse` renamed | RED | RED (it already had `t.Fatal`) |
| either floor raised to 100000 | — | RED at **159** non-test `.go` files |
| any one of the three pins' root reverted to the climb | register GREEN | register RED, twice, naming that pin by file:line |

So FINDING 2 of the bead ("one of the two passes reading nothing") was two of
the three, as dinesh's close says, and `159` is the real count: 153 non-test
`.go` under internal/posse plus 6 under cmd/ — and `internal` really is
internal/posse for non-test `.go`, because internal/ holds only `posse` and
`treepins` and treepins is 0 non-test / 79 test files. No pin's population
changed.

### The finding: the same reading, two spellings, opposite class membership

dinesh's residual paragraph says the remaining shape — "a named path JOINED
onto a bare-root identifier … which is the hidden arm's shape and is scored by
its dataflow". MEASURED: it is scored by nothing. Five probes planted in
internal/posse at `e27b7618`, each a genuine tree reading, differing only in
how the root arrives; the register run after each:

| probe | root | register |
|---|---|---|
| P3 **positive control** | `dir := "../../internal"` | **RED**, names the probe by file:line |
| P4 | `dir := filepath.Join("..", "..")`, the ROOT walked | **RED** (the hidden arm) |
| P5 | `os.Getwd()` + `"..", ".."`, the ROOT walked | **RED** (the hidden arm) |
| P1 | `os.Getwd()` + `"..", "..", "internal"` | GREEN — invisible |
| P2 | `repo := filepath.Join("..", ".."); walk(repo/"internal")` | GREEN — invisible |

P2 is the fragment's own example. The hidden arm fires when the walk's root IS
the bare root and not when a SUBDIRECTORY is joined onto it, which the fixture
table does say out loud ("which scores a walk of a SUBDIRECTORY of it as not
tree-wide") — but the residual paragraph a later reader trusts says that shape
is covered, and it is not. That is the exact mechanism ranger-base-56eln was
filed over: nt0pi's register paragraph was short by one spelling, and "the
paragraph a reader trusts instead of re-censusing" is this bead's own phrase.

And the jurisdiction line does not survive being asked about this bead's own
work. P6: ranger-base-56eln's gatedkeychain census — the same two directories
(`internal/posse`, `cmd`), the same non-test `.go` file set, the same
forbidden forms, the same floor — with `repo := filepath.Join("..", "..")` in
place of `qibRepoRoot(t)`:

    the probe itself: PASS   |   the register: GREEN (INVISIBLE: no door asked for)

The pin this bead doored would not be in the class if its author had spelled
the root the other way. A LIVE pin is in that shape today:
`internal/posse/bobtripwire_qa_test.go:58`
`TestQABobFixturesAreStillUnknownToHerdr` reads `os.Getwd()`, climbs two, and
`os.ReadDir`s `etc/herdr/agent-detection/upstream/bob` — in no `QA_*_PINS`
variable (`grep` over the Makefile: 0), in no census this family has written,
and its subject is what an unrelated bead puts in that directory. It has a
floor, so it is not green over nothing; what it has no door is.

## Residual of THIS bead, said out loud

- The findings above are filed, not fixed: a verifier does not edit production
  code, and `treewidedoor_qa_test.go` was being merged at a sibling seat while
  this ran, so no pin was added there.
- `make test` was not run: the box was at a load of 26-41 with both suite
  slots held by sibling seats for the whole session. Everything above is a
  `-run` filter or a door, and `make fmt-check`, `make notes-check` and `make
  tree-check` were typed by hand for exactly that reason.
- The 56eln readings are the BRANCH's, not main's. When `ranger-base-rq986`
  lands them the count sentence and the door lists will have moved under them
  (dinesh's own MERGE NOTE says so), and arm 4 is what will say by how much.
