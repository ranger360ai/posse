## Four closes verified, and the mutant nobody runs (ranger-base-0eysw)

A verify bead over ranger-base-7fjmv, ranger-base-6sbng, ranger-base-miyv2 and
ranger-base-nka1h. All four verify. What is worth keeping is the METHOD that
found the one live gap, because it is cheap and it generalises: a closer's
mutation battery combines a dead arm with the defect that arm reads, which
shows the arm is NECESSARY. Run the same mutants UN-COMBINED — the arm dead,
the subject untouched — and they answer a different question: is the arm
WITNESSED? An arm whose death nothing notices has no control in the pin,
only a mutant nobody runs.

Everything below was MEASURED 2026-11-14 on this Linux box (16 cores), at
main db0e46e2, one-minute loadavg 10-15 unless a row says otherwise. Each
mutant was applied to the committed tree and restored from a golden copy
taken before any mutation, with an md5 check after every restore; the
batteries are not committed, for the reason ranger-base-51ttd's and
ranger-base-6sbng's rigs are not — they replace a correct swap with a broken
one.

### 1. ranger-base-6sbng — the blend arm had no control

The pin's probe classifies three failure states: a failed open (ABSENT), a
size other than the binary's (SHORT), and two end bytes that disagree (a
BLEND — the target being rewritten in place at full length). The pin ships
one positive control, `installShapedSwap`, which unlinks the target and
writes a new one.

| un-combined mutant, real `upgradeSwap` | verdict | reading |
|---|---|---|
| the ABSENT arm never fires | PASS x3 | and the install(1) shape still reds through the size arm (FAIL x3) — the detection survives, its attribution moves |
| the SIZE arm never fires | PASS, SKIP, PASS | the control notices: only the ENOENT window is left to it, and a run that misses it skips |
| the BLEND arm never fires | PASS x3 | nothing notices |

The blend arm is invisible because the control cannot produce a blend: having
unlinked the target it only ever leaves it absent, then short. So the pin's
own claim — that reading both ends is what keeps the in-place-rewrite shape
seen — rested on the closer's M7 mutant and on nothing that runs.

FIXED HERE, in `internal/posse/upgrade_test.go`: a second control
(`blendShapedSwap` — open the live target, overwrite it in install(1)'s
chunks, never unlink and never truncate), a `blend` sub-count of `partial`
so the control can require that ONE arm rather than the union of three, and
one more skip clause. The control loop became `swapControl`, which takes the
shape's own predicate, so one shape's detections cannot vouch for another
shape's arm.

| mutant, hardened pin | before | after |
|---|---|---|
| the blend arm never fires, real swap | PASS x3 | **SKIP x3** |
| the blend arm never fires AND the in-place-rewrite swap (the closer's M7) | PASS x3 | **SKIP x3** |
| the in-place-rewrite swap, arms live (M3) | FAIL x3 | FAIL x3 |
| `blendShapedSwap` made atomic — the new control's own wrong arm | — | **SKIP x3** |
| `installShapedSwap` made atomic — the sibling control through the new helper | SKIP x3 | SKIP x3 |

Cost: 0.10s to 0.15s focused, one more probe run. `-count=8` green, gofmt
and `go vet` clean.

### 2. ranger-base-6sbng — the close's own readings, re-measured

| claim | closer | re-measured here |
|---|---|---|
| mean observation, dense probe | 5.5-6.2 µs | 5.9-6.7 µs (loadavg 10-15) |
| observations per run | — | 3,321-8,324, against ~24 for the 8 MiB read it replaced |
| M1/M2/M3 | FAIL x3 each | FAIL x3 each |
| M4 probe coarsened | SKIP x3 on the density witness | SKIP x3, 1.9-3.7 ms mean against the 260 µs budget |
| M5 control made atomic | SKIP x3 in 3 attempts | SKIP x3 in 3 attempts |
| M6 size arm dead AND M2 | FAIL x3, 121-134 detections | FAIL x3, 126 detections |
| M8 density witness dead AND coarsened probe | PASS SKIP PASS | not re-run |
| GOMAXPROCS=1 `-count=30` | 60 PASS, 0 SKIP over two runs | 87 PASS, 3 SKIP, **0 FAIL** over three runs at loadavg 10-15 |
| `-race -count=5` | clean | clean |

The GOMAXPROCS=1 skip rate is the one reading that differs, and it differs in
the direction the box moved: the closer read 0 of 60 on a quieter box, this
read 3 of 90 at a loadavg of 10-15. Neither run ever reds, which is the
property ranger-base-51ttd's ruling asks for — a starved reader skips.

### 3. ranger-base-7fjmv — the fourth lag row, re-derived

The priced reading reproduces exactly. `git log --oneline 7d0f8c31..35e8cf41`
over each list's pathspecs, the window being 75 commits:

| list | commits in the window |
|---|---|
| WallRenderSources | 12 |
| LaunchCheckSources | 9 |
| CommitGateSources | 4 |
| BdWriteSources | 0 |

Three commits in main's history have ever touched the write-time paths —
21508a1a, 2a9f4e06, 6b4cd64d — and 21508a1a does touch `internal/posse/gates.go`,
which is what makes the close's "counted as a WALL, one row over" attribution
a fact rather than an inference.

Eight mutants, all killed, each naming the arm the close says it names:

| mutant | arm that red |
|---|---|
| `bdline.go` dropped from BdWriteSources | arm 1, derived-but-unnamed |
| `bdwriteshim.go` dropped | arm 1 |
| `skills.go` hand-added | arm 3, named-but-underived |
| a nonexistent pathspec added | arm 2 (and arm 3) |
| `bdlineshim.go` hand-added — the floor residual | arm 3 |
| `vulnprivate.go` added — the commit-time row's own path | arm 4, boundary 2 |
| **`bdWriteSplitArgv` moved to a new file, list untouched** | arm 1, naming the new file |
| the fourth `lagCategory` row deleted from `categories()` | four renderer pins; treepins stays green, which is the right division |

The seventh is the realistic escape and the one the closer's list did not
carry: a refactor splits the judge across one more file and nobody edits the
list. It is caught, by the derivation and not by a literal.

### 4. ranger-base-miyv2 — the merge adaptation, and a conjunct that is not a distinction

The landed commit is patch-id identical to the branch commit the close names
(`938e4567…` both sides), so the rebase stranded nothing. Nine files,
+905/-75, as claimed.

WORDING, run rather than read: `posse gates vuln-private-scan --message`
built from this tree PASSES the landed message, and REFUSES a control message
that describes the same open residual (exit 1, "a line here … carries more
than the bare id — the text is withheld"). The five residual sites are bare
ids and the message's "Not closed here" section is a bare id. This run is
stricter than the closer's: the branch-bead exemption keys on the branch's
own recorded bead, which here is ranger-base-0eysw and not the one under test.

MERGE ADAPTATION 3 is load-bearing. Reverting the `hideMounted` skip to the
parked commit's `reopens(d)` shape reds exactly
`TestQALandedCheckCageRendersTheSecretMasks` — 467 of 468 pass over a
`-run 'LandedCheckCage|Bwrap|CredStore|…'` filter, and that one fails.

THE NEIGHBOUR QUESTION, asked and answered NO. Steps 5a (SSHDir) and 5b
(DenyReadDir) still skip on `reopens`, which keys on `hidden` — the list that
holds an ABSENT Hide entry too, which is the exact shape of the bug 5c fixed.
It is not reachable today, in either render: a seat's `Hide` is
`bwrapRuntimeDirs` (the user and system run dirs), which contains neither
`~/.ssh` nor `rexNodeStateDirs`; and the landed-check judge, whose `Hide` does
hold the runtime state roots since ranger-base-8ukiy, sets no SSHDir and no
DenyReadDir at all. So the fix was made at the one loop where the two lists
meet. Nothing pins that coupling, and a bead that puts a runtime state root
into a SEAT's `Hide` would inherit the hole one loop over.

RECORDED, changing nothing: the 5c skip reads
`(writeGranted(hideMounted, d) && !writeGranted(grants, d)) || writeGranted(grants, d) || grantInside(grants, d)`.
The conjunct is dead — `(H && !G) || G || I` and `H || G || I` are the same
function, since the clause that follows skips on `G` outright. It is inherited
from inlining `reopens`, where it IS load-bearing because there the expression
stands alone. Stripping it leaves all 468 green, which agrees with the algebra
rather than standing in for it. A reader of a security-relevant skip will
infer a distinction the code does not make.
