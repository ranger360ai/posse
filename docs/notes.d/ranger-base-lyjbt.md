# ranger-base-lyjbt — verifying four closes at the v0.5.1 gate

A batched verify bead over `ranger-base-y7t4n` (the v0.5.1 release prep),
`ranger-base-eawjq` and `ranger-base-aza46` (two merge-backs) and
`ranger-base-d1hax` (the box-check roster red that held main unpushed). All
four verify. What is worth keeping is the method on two of them and one gap
the verification found and closed.

MEASURED 2026-10-04, this box (darwin 25.4.0, git 2.50.1, bd 0.50.3
`bd25acbc`), at `main` = `093cfb1f`, with the four closes' work all on main.

## 1. Where the work actually landed

A merge-back close names shas on a seat branch, and none of the three
branches' own shas are what reached main. Reading the close against the
branch sha alone says "not on main" for work that is entirely there.

| close | branch tip the close named | on main as |
|---|---|---|
| eawjq | `f1468868` `ed44ff95` `faa3969b` | `d2ec5532` `30e434f6` `ca92fb64` |
| aza46 | `778bb729` `64c094c1` `08c3b90c` `50c84e02` | the same (it ff'd) |
| d1hax | `6e505759` | `5c2948a2` |

eawjq's three were replayed a SECOND time by the launcher onto `32a90cde`
(the release commit landed while gilfoyle's session was open), and d1hax's
one was landed by hand by monica under `ranger-base-840ft`. Both preserved
`(%ae, %aI, %s)`, so both still pair. `5c2948a2`'s patch-id differs from
`6e505759`'s and the one path that differs is `docs/notes.d/README.md` — the
generated index, regenerated at landing time, which is the right way for that
file to differ.

**Read a merge-back close by (author email, AUTHOR date, subject), never by
sha and never by patch-id.** Patch-id disagrees whenever a generated file was
regenerated on the way in, which on this repo is most landings.

## 2. The .PHONY union, checked as a set

eawjq resolved a `Makefile` `.PHONY` conflict as a computed set union and said
so. Checked as sets rather than by reading the diff:

```
main at the time (32a90cde) : 74 targets
bwp7h branch tip (c14e2092) : 68
the landing (30e434f6)      : 75   == the union exactly
lost from either side       : NONE
```

The branch added exactly `verify-nodb-defer`; main now carries 76, the one
addition since being d1hax's `verify-nodb-defer-capability`. A set comparison
settles a union claim in one reading where `grep -c '^-'` over the diff — which
is what the close offered — only bounds it.

## 3. The dangling-citation census reproduces; its count does not

`ranger-base-nnnf1` (filed by aza46's close, still open) reports 2 dangling
`docs/notes.d/` citations on `ca92fb64` and 1 on `50c84e02`. Re-measured with
an independently written 20-line scanner over `git ls-tree -r --name-only`:

| ref | tracked .md | citations | dangling |
|---|---|---|---|
| `32a90cde` (the v0.5.1 commit) | 310 | 141 | 2 |
| `ca92fb64` | 312 | 143 | 2 |
| `50c84e02` | 314 | 149 | 1 |
| `main` (`093cfb1f`) | 317 | 154 | 1 |

The **dangling set is identical** to nnnf1's, both members and both counts,
and the escape it reports is confirmed at the release commit itself:
`docs/adr/0062-…:407` cited `docs/notes.d/ranger-base-4mrmc.md` at
`32a90cde`, where no such file was tracked. The surviving one is
`docs/notes.d/ranger-base-l1rjl.md:130` → `ranger-base-fm23s.md`, never
written in any branch.

The CITATION TOTALS do not reproduce: nnnf1 says 127 and 131 where a plain
`docs/notes\.d/[A-Za-z0-9._-]+\.md` scan says 143 and 149. Two scanners, one
narrower than the other; the verdict is unaffected, but **nnnf1's totals are
not re-derivable from its own description** and whoever builds that pin should
re-measure rather than target those two numbers. Noted on nnnf1.

## 4. The conflict set depends on which base you ask — reproduced

aza46's close called an earlier comment's two-file conflict report an artifact
of the base. Reproduced here, both arms, against `ca92fb64`:

```
--merge-base=7bd14afa (the commit's PARENT, which is what a cherry-pick uses)
  CONFLICT (content):       docs/adr/0062-…md
  CONFLICT (modify/delete): docs/notes.d/ranger-base-4mrmc.md
                            "deleted in ca92fb64 and modified in 4dc3a47e"
--merge-base=4dff757a (git merge-base main <tip>)
  CONFLICT (content):       docs/adr/0062-…md        — and nothing else
```

The fragment is a one-sided ADD over the real merge-base. The close's rule
holds: **read a conflict set with `--merge-base=$(git merge-base main <tip>)`,
and treat a cherry-pick's extra modify/delete on a file main does not have as
noise** — believing it costs a hand-merge of a file with one author.

Also checked, because landing round 3's ADR verbatim could have reverted
main's own rewrites of it: `git diff ca92fb64 4dc3a47e` over the ADR is 105/11
(the close's figure), all eleven removed lines are re-wraps or struck ASSUMED
items, `ranger-base-er6mt`'s amendment header and `ranger-base-mkcsy`'s
measurement are both present on main today, and `make verify-silent-reverts`
exits 0 with none of the nine landing shas flagged.

## 5. The gap: all-or-nothing pairing was unpinned

`equivalentOnBase` (`internal/posse/worktree.go`) `return nil`s the moment one
commit in `base..tip` is unaccounted for. **Both merge-back closes made a
landing decision on that rule** — eawjq wrote two twins rather than one
collapsed commit, aza46 wrote three — and every fixture in
`mergebackreplay_test.go` put a SINGLE commit on the branch, so `return nil`
and "return what paired so far" were indistinguishable to the whole suite.

MEASURED, by planting `continue` in place of that `return nil` against a
two-commit fixture whose first commit alone has a twin on main:

```
real code: equivalentOnBase -> 0 entries, MergeSessionWork blocked=true
mutant   : equivalentOnBase -> 1 entry,   MergeSessionWork blocked=false
```

So the risk is not only the strand gilfoyle's close named — it is the other
side of that branch: a collapsed landing would have read as **LANDED**, with
the branch's second commit dropped from main and a green word over it.

Pinned as `TestMergeBackPairsNothingWhenOnlySomeOfABranchLanded`
(`internal/posse/mergebackreplay_test.go`, arm 2), mutation-checked by the
mutant above.

## 6. d1hax: four mutants, and the row is reached

The census pin was green on main. Green for the right reason, 4/4 killed:

| mutant | pin that caught it |
|---|---|
| the ROSTER row removed | census: `verify-nodb-defer-capability` on neither table |
| the EXCLUDED row removed | census: `verify-nodb-defer` on neither table |
| roster command ≠ the target's recipe | `…RosterCommandsAreTheirTargetsRecipe` |
| the rostered target deleted | census: "rosters X, which is not a target" |

And applied is not reached, so the aggregate was run: `RHQ_HOME=<scratch>
scripts/verify-box.sh` → exit 0, 9 checks, `verify-nodb-defer-capability ok`,
the verdict column aligned at the same offset as the 15-character names
(the `%-26s` → `%-29s` change, five sites, all five moved). Scratch
`RHQ_HOME` because the aggregate writes `state/verify-box.yaml`, the file G10
reads — a verification run has no business dating the operator's surface.

Both of the close's flag claims hold by running them: `--arm-a-only` exits 0
reporting `status='deferred' defer_until=None` (the defect present, which is
this door's clean verdict), and exits **2** with a refusal when a store is also
named. The first twin's script (`d2ec5532`) has no `--arm-a-only` flag at all
and exits 2 bare — which is why the two twins are not interchangeable.
