# ranger-base-zpk8r — the launch DEGRADED call site, and two inverted policy-tier lines

ranger-base-u4uwg's verify of four closes left three LIVE findings, each one a
thing a process PRINTS. None was a money / constitution / dispatch-correctness
defect, so all three landed under one bead (ADR 0006 §6). This is what shipped.

## Finding 1 — the three launch DEGRADED warnings were unpinned at their call site

`planLaunch` prints three launch-time readings of this box behind one guard
(internal/posse/herdrback.go):

```go
if d := GitSystemScopeDegraded(); d != "" { warn("posse: %s\n", d) }  // ranger-base-t301k
if d := PolicyPinDegraded();      d != "" { warn("posse: %s\n", d) }  // ranger-base-s69ep
if d := GitGlobalScopeDegraded(); d != "" { warn("posse: %s\n", d) }  // ranger-base-obdnk
```

Every one of the three FUNCTIONS was pinned — inletpingitconfig_qa_test.go,
inletpingitglobal_qa_test.go, policypin_test.go — and **no pin asserted that
planLaunch still printed any of them.**

MEASURED 2026-11-04, this box (darwin 25.6.0, go test over internal/posse):

```
sed -i '' '2879s|.*|\t\t\t\t_ = d|' internal/posse/herdrback.go
go test -run 'Policy|Parity|Gate|Inlet|Wall|LaunchCheck|HookConfig|HookParity' ./internal/posse
-> ok
```

Same result for the other two `warn()` lines. A launch that says nothing is
exactly the silence all three beads were filed about, and the suite could not
tell that state from a clean box.

**What shipped:**
`TestQAEveryZeroArgDegradedLaunchReadingIsPrintedAtItsCallSite`
(internal/posse/verify_zpk8r_qa_test.go), an ast census over this package's
own non-test sources.

**THE CLASS IS DERIVED, not a list of three.** That was the finding's own
request: a fix for one line that leaves the other two is the shape to avoid.
The rule is **an exported, zero-argument, string-returning `*Degraded`
function in this package** — the signature of "a reading of THIS BOX, taken at
launch, with nothing to pass it". The package's other `*Degraded` functions all
take arguments (`DetectionDegraded`, `RexRuntimeDegraded`,
`RexNoReportDegraded`, `PIDChannelDegraded`), which is the honest tell: they
are readings ABOUT something a caller already holds, reached from wherever that
thing is, and they carry their own pins. So a fourth zero-argument reading
added tomorrow is in the class without anyone editing a list, and reds its own
bead if it is declared and never printed.

The census requires, per member, a call site of the shape `if d := R(); d != ""
{ …(…, d…) }` — the guard, because without it the reading's clean answer `""`
would print a line on every launch, and a call inside the body passing the
bound value to a printer. `zpkPrinters` is the set of callee names that reach a
seat's streams (`warn`, `Printf`, `Fprintf`, …), matched on the last component
so `warn`, `b.warn` and `fmt.Fprintf` all read the same. **Nothing derives that
set**: a new way to print adds a name there, and that edit is the point — it is
what makes someone say out loud that a launch reading now goes somewhere else.

**NOT A TREE-WIDE PIN, deliberately.** The scan is `filepath.Glob("*.go")` over
this package with no climb to a repo root, which is the derivation
internal/treepins/treewidedoor_qa_test.go keys on for internal/posse
(`qibRepoRoot`). Its door is `$(QA_PKGCENSUS_PINS)` / `make pkgcensus-check`,
the one-package-census door, and it needs no new Makefile target.

What that scope does not cover, said rather than left silent: a launch path
assembled in ANOTHER package that prints these readings, and a reading of this
class declared outside internal/posse. Neither exists today — all three call
sites are in planLaunch. The stronger shape, a launch-level pin over a fixture
policy dir, **is not reachable from a seat at all**: the policy dir is
root-owned and `claudePolicyDir` takes no injection point.

## Finding 2 — the policy-tier DEGRADED wording inverted on the /dev/null fallback

`policyPinDegraded` compares each policy file's `GIT_CONFIG_SYSTEM` against
`gitSystemScopePinValue()`, which is git's resolved system-scope path **or**,
when resolution fails, `gitSystemScopeFallback` (`/dev/null`). In the fallback
case a policy file pinning a REAL path is `Stale`, and the s69ep sentence then
read:

> DEGRADED: git system scope is NOT carried into this session — the policy tier
> outranks the launch pin's GIT_CONFIG_SYSTEM=/dev/null, and <file> pins
> "/etc/gitconfig". A commit scanner or hook configured only in system scope
> will not run here. …

Both sentences are false there. The policy tier is **carrying** system scope;
the launch pin is what would have dropped it. A detective row that inverts its
verdict is the failure mode ranger-base-s69ep exists to end, one configuration
over.

**What shipped:** `policyPinDegraded` has two wordings now, chosen on `want ==
gitSystemScopeFallback`. The fallback arm says the policy tier is what carries
system scope here, that posse neither resolved nor vetted that value, and that
a hook in the system scope of the git this seat actually runs may still be
skipped. Its remedy differs too, and that is the half worth keeping: **replacing
the installed file with the shipped copy would not help** — the shipped drop-in
has pinned nothing since ranger-base-t301k, so that would leave the seat on the
launch pin's `/dev/null` and no system scope at all. The arm points at the
resolution failure instead, which `GitSystemScopeDegraded` is printing beside
it (gitSystemScopePinValue returns the fallback exactly when `gitSystemScope()`
errs, which is exactly when that line fires — so the pair is never silent).

Reach is low: resolution must fail (no git on the gates-free PATH, or `git var
GIT_CONFIG_SYSTEM` failing) AND a policy file must pin a non-/dev/null value,
which wants a hand-edited or third-party file. Hence one bead, not its own.

Checked and NOT a finding: with resolution failed and the policy file also
pinning `/dev/null`, `v == want` reads CLEAN and `GitSystemScopeDegraded` fires
separately, so that pair is not silent either.

## Finding 3 — a null JSON value was reported as pinning the empty string

`json.Unmarshal` of JSON `null` into a `string` is a **no-op that returns no
error**, so `"GIT_CONFIG_SYSTEM": null` took the `v == ""` path and not the
`Value: string(raw)` branch one line above. The line then read `pins ""` over a
file that pins null. The raw branch was never dead — a number, an object and an
array each reach it (MEASURED: `Value` `0`, `{"a":1}`) — null alone slipped
past it.

**What shipped:** an explicit `strings.TrimSpace(string(raw)) == "null"` arm
before the string unmarshal, and a `Raw bool` on `policyPinFinding` with one
formatter (`policyPinFinding.pins`) reading it. A raw value is named as the
token it is — `pins the JSON value null`, `pins the JSON value 0` — because
`%q` over a JSON token reads as a string the file does not carry either, which
the old raw branch already did for numbers. A string stays quoted: the quotes
are what distinguish `pins ""` from an absent row.

Staleness does not change on any arm — null is not a path, and it was `Stale`
through the `v == ""` route too — so this is output wording and nothing else.

## The register bill for finding 1's door

Adding a member to `$(QA_PKGCENSUS_PINS)` reds
`internal/treepins/treewidedoor_qa_test.go`, which is the design and not a
surprise: that file's head comment is the ONE place this shop says how many
tree-wide pins and doors there are, and `make register-check` holds the
sentence and the enumeration under it to the Makefile. Both went short at once
— the count sentence (123 pins -> 124) and the one-package-census group, whose
own `FIFTEEN more` and `(all fifteen ranger-base-na4rm)` could no longer be
said as written. All of it is updated, with the sixteenth member attributed to
this bead rather than folded into na4rm's fifteen.

The classification is mechanical and was not asserted by hand:
`TestQAEveryTreeWidePinHasADoor` derives the one-package-census class from a
package-directory enumeration (`os.ReadDir(".")`, `filepath.Glob("*.go")`), and
this pin's `filepath.Glob("*.go")` puts it there on its own. `Seven of the
sixteen are compiled out of arm 1` still reads seven: this pin is arm 1.

### MEASURED: `make tree-check` at 124 pins, 2026-11-04, this box

90.7s, 71.7s then 126.1s in that order, at a one-minute load average of 30.01,
21.42 and 12.34 (five-minute 37.85, 32.99, 27.77), with one suite slot held by
a sibling seat for the first two runs and **both** held for the third; one
warm-up run discarded at 97.5s. The pin this bead adds is 0.09s measured
alone.

The entry went into the head comment's chain, and what it contributes there is
a counterexample rather than a price. **The one-minute load average is not the
variable.** It fell by a factor of two and a half across these three runs while
the wall rose 35.4s, and the slowest run is the one at the *lowest* load —
126.1s at a one-minute 12.34, where every earlier reading under a load of 16
sat at 55-62s. What that run had and the other two did not is both suite slots
held. A loadavg is an exponentially weighted average over the preceding minute,
so it reports a box that has already emptied while two full suites are in fact
spending it. Price the command off `scripts/suite-lock.sh --status`, which is a
fact about now, and read a loadavg as the lagging indicator it is.

## Verification

`make test` green (three arms). Seven mutation arms, each red on its own pin
and the tree green when restored:

| mutation | reds |
|---|---|
| `2879s/.*/_ = d/` (the measured repro, PolicyPinDegraded) | call-site census, "value goes to no printer" |
| delete the `GitGlobalScopeDegraded` guard | call-site census, "NO guarded call site" |
| `2874s/.*/_ = d/` (GitSystemScopeDegraded) | call-site census |
| `GitGlobalScopeDegraded` grows a parameter | class floor, "no longer an exported zero-argument string reading" |
| drop the fallback wording arm | finding-2 pin, both inverted sentences named |
| drop the `null` arm | finding-3 pin, `Value:"" Raw:false` |
| `%q` over a raw token again | finding-3 pin, `pins "0"` |

Also typed by hand: `make fmt-check`, `make tree-check`, `make pkgcensus-check`.

## Files

- `internal/posse/policypin.go` — findings 2 and 3.
- `internal/posse/verify_zpk8r_qa_test.go` — all three pins (arm 1).
- `internal/posse/verify_u4uwg_qa_test.go` — head comment, which said this
  class was filed and not held; it is held now and points at the pin.
- `Makefile` — `$(QA_PKGCENSUS_PINS)` gains the census.
- `internal/treepins/treewidedoor_qa_test.go` — head comment: the enumeration,
  the one-package-census group count, and the single pin/door count sentence,
  plus the measured reading at the new class size.
