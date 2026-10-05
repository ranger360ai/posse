# ranger-base-hrf47 — the verify-* target census had no door

A `verify-*` target reached the Makefile without its row in
`scripts/verify-box.sh`, and the pin that catches it was reachable only by an
unfiltered 589.965s run of `./internal/treepins`. The missing row is fixed; the
missing door is what this fragment is about.

## The red, and who fixed it

MEASURED 2026-10-04. `verify-nodb-defer` entered the Makefile at d2ec5532
(ranger-base-bwp7h) with no matching row in `scripts/verify-box.sh`, so
`TestQABoxCheckCensusCoversEveryVerifyTarget` failed on `main`.

**That half was already closed when this bead was worked.** 5c2948a2
(ranger-base-d1hax) rostered arm A as `verify-nodb-defer-capability` and put
`verify-nodb-defer` in EXCLUDED with its reason — which is also the ruling the
bead said to go to ranger-base-bwp7h for. Both census pins pass:

    go test ./internal/treepins -count=1 -v \
      -run 'TestQABoxCheckCensusCoversEveryVerify'     # both PASS, 0.03s each

Nothing here re-fixes that row.

## Why it escaped: the derivation rule is right and the pin was still undoored

`make register-check` passed over the red, and the bead asked whether that is a
hole in the register or a correct reading of its rule. It is a correct reading,
and the rule pins it as a deliberate negative case
(`TestQATheTreepinsEnumerationRuleMatchesWhatItClaims`):

    {"reading one named file is not enumeration", `os.ReadFile("Makefile")`, false}

The internal/treepins half of the class is derived from a directory
ENUMERATION rooted at a relative path. The two pins in `boxcheck_qa_test.go`
split exactly on that key:

| pin | how it reads the tree | derived? | doored before this bead? |
|---|---|---|---|
| `…CoversEveryVerifyScript` | `filepath.Glob("scripts/verify-*.sh")` | yes | yes — `QA_SCRIPTS_PINS` |
| `…CoversEveryVerifyTarget` | `os.ReadFile` of the Makefile and of `scripts/verify-box.sh`, both by name | no | **no** |

So `make scripts-check` was green over the undoored sibling in the same file,
and no door variable named it (`grep -c` for its name in the Makefile answered
0). That is ranger-base-rulbl's gap one package over — the bead, which wrote no
fragment; AGENTS.md's bullet "A `-run` filter cannot reach a tree-wide pin" is
the prose it left — with one difference worth keeping:

**A pin whose file set is spelled in the test is not the same as a pin only its
own bead can red.** The rule's premise — a reading whose file set is not spelled
is a reading an unrelated bead can red — is sound as far as it goes, but its
converse does not hold. This pin names its two files, and the Makefile is a file
every bead edits, so an unrelated bead reds it exactly as a tree walk would.
Spelling the file set bounds who *can* red a pin; it says nothing about who
*will*, and the door is what makes the red arrive in seconds instead of 590.

## The fix: the register's own exemption, not a wider rule

Widening the enumeration rule to class "reads the Makefile" as tree-wide would
pull in the register's own arms and any future pin that reads one file, so the
rule stays as pinned. `twdDoorHolders` is the mechanism already built for this —
"tests a door variable may name although no class rule derives them" — and it
already held two members, both of which read the Makefile by name. This pin is
its third, and the first that is not the register inspecting itself.

Three edits, no mechanism invented:

1. `QA_SCRIPTS_PINS` names `TestQABoxCheckCensusCoversEveryVerifyTarget`, so
   `make scripts-check` (~0.9s) and `make tree-check` both reach it.
2. A `twdDoorHolders` row with its reason, which is what keeps
   `TestQAEveryTreeWidePinHasADoor`'s two-way check from rejecting a door name
   no rule derives.
3. The head comment's enumeration row and its live count, which arm 4 holds to
   the Makefile: fifty-one pins became fifty-two.

The `scripts-check` comment in the Makefile now also says to type it when you
add or rename a verify-* TARGET, not only something under `scripts/` — the door
covers both directions of that census now.

## Re-measurement

`make tree-check`, three warm runs, 2026-10-04, this box: **55.34s, 99.78s,
69.69s**, at a one-minute load average of 9.11, 8.36 and 25.06, with a sibling
seat holding one of the two suite slots throughout. The added pin costs 0.03s,
so the spread is the box — the same conclusion ranger-base-g6sb1 reached with a
sibling suite and ranger-base-vofbl reached without one (44-46s at a load of
5.4-7.1).
