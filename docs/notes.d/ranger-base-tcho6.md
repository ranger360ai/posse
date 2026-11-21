# Check 4's positional guard is pinned against the reader, not against git (ranger-base-tcho6)

The one live finding of the ranger-base-yusee verify (4 closes verified;
readings and method in docs/notes.d/ranger-base-yusee.md). A nit by blast
radius and a debt by shape: the guard held, the arm written for it could not
see it, and the two places saying why drew the wrong inference from a right
premise.

Box, for every reading below: Linux 7.2.15-1-omarchy, go1.27.3, 16 cores, uid
1001, tree at a409b8cd, loadavg 1-4 with no suite slot held.

## What was unpinned

`vulnPrivateUndiff` (internal/posse/vulnprivate.go) blanks two shapes of the
content arm's filtered stream: a line beginning `diff --git `, and a `+++ `
line IMMEDIATELY AFTER one. The second is conditional on `atFile` — the
positional guard ranger-base-pdcf6 landed — and the mutant

    -		case atFile && strings.HasPrefix(line, "+++ "):
    +		case atFile:

SURVIVED internal/posse arms 1, 2 and 3 whole and the treepins commit-gate
pins, laurie MEASURED. The arm added for exactly this half
(`vulnprivate_test.go`, "a file whose header has no `+++` line does not eat
the next file") passes under it too.

## Why no arm could see it

That arm's stream is a no-`+++` header followed by the NEXT file's
`diff --git ` line, and the switch's FIRST case blanks a `diff --git ` line
whatever `atFile` says. So both readings blank it and the arm is identical
under the mutant. The two readings can only disagree on a line after a
`diff --git ` that is neither a `+++ ` header nor another `diff --git ` — and
git emits no such line. MEASURED 2026-11-21 over the content arm's real
filtered stream (laurie's reading, not re-derived here): `root..HEAD` as one
staged diff gives 2,176 file headers, 2,176 of them followed by a `+++ ` line,
0 exceptions; and a fixture carrying both real no-`+++` shapes — a binary
file, an empty new file — has each such header followed by the next file's
`diff --git `.

## The inference, which was written down twice and is now corrected in three places

`vulnprivate.go` and the arm's own comment both said a file with no `+++` line
— a pure rename, a mode change, an empty new file — "is why the second
blanking is conditional rather than positional alone". The premise is true and
the conclusion does not follow, for the reason above: positional alone handles
that file identically. A reader deciding whether the guard could go had two
places telling them it was needed for a case that does not need it, and no
place naming the case that does.

Both are rewritten to say what is true, each naming this bead as the
correction. The third site is `docs/notes.d/ranger-base-x78s5.md`, the verify
record that carried the same clause forward; it keeps its original sentence
with the correction appended rather than being rewritten, because it is a
record of what was read at the time.

## What the guard is actually for

`RunGatesVulnPrivateScan` judges ARBITRARY text. The arm pipes a filtered git
diff into it, but the subcommand reads whatever its stdin holds — so a
`diff --git ` line followed IMMEDIATELY by a content line is a text a caller
can hand it, and positional alone blanks that content line. Where the content
line is a bare open id, the description under it has no id left to continue and
the hit is gone: a fail-open, against the reader's input rather than against
git's output.

The pin is one row of
`TestRunGatesVulnPrivateScanClosesEveryUnitAtAFileBoundary`, written out rather
than through `vpFile` because `vpFile` always writes the `+++ ` line this shape
must not have:

    name: "a content line immediately after a `diff --git` line is content"
    text: vulnPrivateDiffHeader + "a/docs/x.md b/docs/x.md\n" +
            "+- ranger-base-l751c\n+\tthe heap overflow\n"
    want: true

with the control beside it — the same two added lines and no header above them.

MEASURED 2026-11-21, the pin planted and the mutant planted into the restored
tree by `sed`, one package run each:

| reading | row | control | package |
|---|---|---|---|
| real code | HIT (want true) | HIT | ok 0.057s |
| `case atFile:` | **clean — FAIL** | HIT | FAIL 0.228s |

One row red under the mutant and nothing else in the pin moved, which is the
whole claim: the control says the mutant is not broken outright, and the
no-`+++` row above it still passes under the mutant on purpose and now says so
in its comment.

## Not re-derived here

laurie attacked the other three halves of ranger-base-pdcf6's fix and they are
mutation-killed (the pre-pdcf6 prefix test, the grep dropping the `diff --git `
line, and never setting `atFile`); all three of that bead's rows are HITs with
the row-2 control; the real boundary still closes the unit; and content cannot
forge a boundary in five shapes, `+diff --git a/x b/x` included.
