# The census went blind one root at a time, and its floor was seven below the live count (ranger-base-sxez7)

*2026-11-13 · from ranger-base-89axe, the verify of four closes · the census is
`internal/treepins/temproot_qa_test.go`, written under ranger-base-qbg7c, and
its door is `make corpus-check`*

ranger-base-qbg7c built `TestQAEveryTempRootInTheTreeIsInPossesTempNamespace`
to be "the pin that makes the eleventh site impossible". It had two holes that
compose into a third, and the third is the one that matters: a live temp root
could leave the census by an ordinary rename, and the door would stay green.

## The reader's ambiguity rule, and the floor that was supposed to catch it

The Go half resolves a call's PARENT argument — `os.MkdirTemp(parent, prefix)`
— through package-wide identifier resolution, which is how qbg7c's reader found
site eleven that qbg7c's own hand census had missed:
`internal/posse/bwraptmp_qa_test.go`'s parent is the const `bwrapTmpMount`,
declared in a different file of that package. A grep for `MkdirTemp("/tmp"`
cannot see that line.

That resolution DROPPED any name the package binds to two different literals,
rather than guessing which one a call site meant, and a dropped name took its
call site out of the census with it. The head comment said so and called it
"the one direction in which this reader is loose". What was not held anywhere
was the consequence: the drop is silent.

Silent, because the roots floor read `gos < 8` against a live count of 15.
Seven roots could have left the census one at a time, each by a one-token
rename, with `make corpus-check` green every time — while the floor's own
comment said the job it was there to do was to notice "a reader that silently
stopped recognising either call".

**And it had already happened once, on this census's first run, in this same
package.** `internal/posse/wall_helpers_test.go`'s `/var/tmp` root held its
parent in a local `const base`; `internal/posse` also binds `base` to a
seven-line yaml literal in `systemdunit_test.go`; the name was dropped and the
census read 14 Go roots where a hand count said 15. Nothing in the tree
reported it. qbg7c's own hand count did, and the fix went to the call site
(`base` -> `varTmpRoot`) with the reader's rule left standing. Nobody would be
hand-counting the next one.

## The fix: ambiguity is scored IN, and the floor is the live count

**An ambiguous parent is in scope** (`trParent`, both halves). A name the
package binds to more than one literal is still not RESOLVED to either — the
reader does not guess which binding a call site meant — but if ANY of those
bindings is a shared temp parent, the root is in scope and its prefix is
demanded. That can only add in-scope roots and never remove one. It costs a
false positive only where a parent name really is bound to a shared temp
directory somewhere in its package, and the remedy there is the one-token
rename qbg7c already made once. Prefix resolution is unchanged and still
strict: an ambiguous name in a `prefix=` resolves to nothing, which reds.

The Python half had the identical drop one language over — a module-level name
bound twice took its `dir=NAME` call site out of scope the same way — so it
carries the same rule, built from the same env/amb split.

**The roots floor is now the live count itself** (`trGoRoots = 15`,
`trPyRootsN = 7`), not a margin under it. A margin is right for the FILES floor
beside it, because a landing moves the file count every day and nothing is
learned from it; a root count moves only when somebody adds or removes a temp
root, and noticing one leave is that floor's whole job. The cost is that a
landing which legitimately removes a temp root reds this arm and edits two
lines — the intended cost, and the cheap half of the fix. They are the numbers
the arm's own `t.Logf` prints, so `-v` re-measures them.

That floor is also what now covers the looseness that REMAINS, and the head
comment says so rather than half-handling it: a parent this reader cannot read
at all — a function parameter, a name bound to no string literal, a `+` chain
over either — is still scored out of scope. The floor does not care why a root
left.

## MEASURED 2026-11-13 (linux, this box, tree f885b8d8 + this change)

The census over the live tree: **15 Go roots, 7 Python, over 1029 .go files in
8 package directories and 15 .py files**, 0.36s, every one of the 22 inside the
namespace. Unchanged by the fix, because no live site has an ambiguous temp
parent today — `base` was renamed under qbg7c.

Three probes, each planted, run and restored:

| probe | before | after |
|---|---|---|
| qbg7c's incident replayed: `perl -pi -e 's/\bvarTmpRoot\b/base/g'` on `internal/posse/wall_helpers_test.go` | `14 Go`, **PASS**, exit 0 | `15 Go`, PASS — the root never leaves the census |
| a planted leaky site whose parent name the package binds to `"/tmp"` in one function and `"/var/tmp"` in another, with prefix `leaky-` | `15 Go`, **PASS** | `16 Go`, **FAIL**, naming the planted line and both bindings in the diagnostic |
| a live root taken out of scope by a spelling the reader genuinely cannot read: `os.MkdirTemp(os.Getenv("X")+varTmpRoot, …)` | `14 Go`, **PASS** | `14 Go`, **FAIL** at the roots floor, "want at least 15 and 7" |

The first row is the fix in the reader; the third is the floor catching what no
reader change can, which is why both halves are here. The second is the shape
ranger-base-89axe planted to find this.

The control arm `TestQATheTempRootCensusCanStillSayNo` grew the three readings
the new rule has to tell apart, in both languages: an ambiguous name whose
bindings are all shared temp parents (in), one with a single temp binding among
non-temp ones (in, on the strength of that one), and one with no temp binding at
all (out) — plus an ambiguous name used as a PREFIX, which must stay unreadable
and red. Go reds 8 of 13 planted roots now and Python 8 of 15, against 5 of 10
and 5 of 12 before.

## Not fixed here

Three further findings from the same verify are recorded on
ranger-base-89axe's close comment and were deliberately not filed: a stale
Makefile comment, a wrong number inside a pin's own failure message, and a
stale row in another bead's dated notes fragment. The second of those three was
the `(11 and 7, MEASURED 2026-11-12)` inside this pin's roots-floor `Fatalf` —
the live count was 15 — and it is gone, because rewriting that message is the
floor fix and leaving a wrong number in a line this bead rewrote would be a
choice. The other two are untouched.
