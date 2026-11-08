# A directory this process may not name has no canonical spelling (ranger-base-8j070)

ADR 0082 D5+D6, built 2026-11-08, off ranger-base-g6bs7's one live finding
from verifying ranger-base-ib61s, ranger-base-yx73g, ranger-base-oxk81 and
ranger-base-8kwa5. Not a failed close: ranger-base-yx73g built D1 exactly as
written, and the sentence that was wrong was in the DECISION.

## What was wrong

D1's third bullet: when the path cannot be opened or resolved — "it does not
exist, or the cage refuses it" — `canonicalPath` takes `resolvedPath`'s
string answer, and "that answer is deterministic for a given input". True,
and not what the byte compare needs. The reading is an `open()`, so the
input is not the path string alone: it is the string PLUS the rendering
process's filesystem reach. ADR 0023's ours-vs-stale verdict compares a wall
rendered by ONE process against a wall rendered by ANOTHER. A renderer with
reach gets the kernel's spelling, one without gets a different string for the
same directory, and the fallback is not an error, so nothing says so —
ranger-base-k9x8n's "ours but stale on every launch" reached through the
renderer's privileges instead of through the caller's spelling.

## The measurement

MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive. One directory on disk as `<root>/instance`, `kernelPath`
and `resolvedPath` reproduced verbatim from the shipped code, stdlib only:

| state | `kernelPath` | `resolvedPath`, asked as `<root>/INSTANCE` |
|---|---|---|
| readable | `<root>/instance` (both spellings) | `<root>/INSTANCE` |
| the directory `chmod 0` | EACCES (13) | `<root>/INSTANCE` — it still ANSWERS |
| a PARENT component `chmod 0` | EACCES (13) | keeps `/tmp`, unresolved |
| missing | ENOENT (2) | `<root>/INSTANCE` |

The control holds: with reach, both spellings answer the kernel's one
spelling. That reproduces g6bs7's probe row for row.

**The parent row is new and is the sharper one.** With a component above it
unreadable, `EvalSymlinks` fails as well, so the fallback loses the symlink
resolution too and differs from the canonical spelling even for a path with
no case and no firmlink axis in it at all. g6bs7's finding had the case axis;
this says the degradation is wider than that.

## Which cage loses the open(): none of the three

g6bs7 named this the first thing to measure and said it might make the fix
cheaper than it looks. It does.

- **L1 shims** put up no filesystem boundary at all.
- **L2 seatbelt**: the profile posse emits is `(allow default)` +
  `(deny file-write*)` + write grants + exactly ONE
  `(deny file-read* (literal …))` block, whose members are credential-store
  FILES (`credentialReadDenyLiterals`). `SeatbeltProfile` is the only
  `deny file-read` emission in the tree (grep of non-test `.go`, 2026-11-08).
  So a DIRECTORY opened for read matches `(allow default)` and nothing else.
  **This half is a reading of the emitted profile and is NOT measured**: a
  nested `sandbox_apply` is refused from inside a caged seat
  (`sandboxApplyRefusal`, seatbelt.go) — MEASURED here, `sandbox-exec:
  sandbox_apply: Operation not permitted` — and a caged seat is every seat
  that could have asked.
- **L4 container** runs linux, where `kernelPath` has no reading at all
  (`canonicalpath_other.go`) and the fallback IS D1's non-darwin half.

So the firing configuration is ordinary filesystem permissions: a directory
whose mode, ACL or owner denies the renderer, or an unreadable component
above it. Latent on this box, as g6bs7 said, and reached by a route that is
not a cage.

## The decision, and why it is two decisions

**D5** splits D1's third bullet by what the failure says about the directory.
Nothing there (ENOENT/ENOTDIR, or a non-absolute kernel answer), or no kernel
reading on this platform (`errNoKernelPath`): `resolvedPath`'s answer, as D1
always said. There and not nameable by this process (`fs.ErrPermission` —
EACCES or EPERM): no answer, `("", errPathUnnameable)`.

**D6** is the half the finding's fix shape (a) stopped one rung short of.
`DeriveIdentityLiterals` propagates the refusal, which is what it already
does for a literal it cannot render, and the three renderers already refuse
on that error. The L3 probe did not — it dropped the derivation error on
purpose, because "an install that hit it already failed loudly". Sound for
the single-quote class, which is a property of the VALUE and identical in
every process; unsound here, because the install may have HAD the reach the
probe lacks. So the probe tells them apart with `errors.Is` and abstains:
`l3Unnameable`, neither held nor stale, with a NOT MEASURED line and
`commitGuardUnmeasured` as its consequence clause.

### Both halves measured as mutation arms

`TestQAAWallWhoseInstancePathCannotBeNamedIsRefusedNotRespelled`:

- **`canonicalPath` reverted to the fallback** (D1's bullet as written): the
  probe asked through the ON-DISK spelling reads `l3Held` — reach cannot move
  a literal the caller already spelled canonically, which is the whole of
  "latent on this box". Asked through the case-changed spelling of the same
  directory it reads `l3Stale`, with the line `ours but stale — run posse
  gates install-hooks; no named guard is missing from the installed body …
  so the drift is elsewhere`, over a wall that is perfectly good. **So the
  pin asks both spellings, and only the second can catch anything.** A pin
  that asked only the on-disk spelling would have been green over the defect.
- **`canonicalPath` refusing but the probe still degrading to no literals**
  (fix shape (a) exactly as worded — "abstain there (skip both instance-path
  literals) … so two renderers with different reach disagree loudly instead
  of silently"): the same `l3Stale` line on BOTH spellings. Skipping the
  literals does not make them disagree loudly. It writes a wall short two
  lines, which the other renderer reads as stale and re-renders, which this
  one reads as stale and re-renders. The silence moves; it does not close.

## Residuals, stated

- **The `# posse-home:` stamp keeps its existing answer.**
  `commitGuardHomeStamp` has no error return, so an unnameable home gets what
  an unnameable home already got — no stamp, ADR 0081 D3 case 4's migration
  body — and a renderer without reach would differ from one with it in that
  line. Unreachable on the install path twice over:
  `DeriveIdentityLiterals` asks `canonicalAbbrevHome` of that same HOME and
  refuses the whole derivation first whenever the repo has a redirect at all,
  and a process that read its config out of that home can open that home.
  What is left is a repo with NO redirect whose renderer cannot open its own
  `RHQ_HOME`, and nothing here is that.
- **Off darwin there is no reach axis to lose**, because nothing opens
  anything. A darwin HOST renderer and a linux CONTAINER renderer of one repo
  therefore still disagree wherever the two spellings differ. That is D1's
  pre-existing non-darwin residual; this amendment walked past it and names
  it rather than closing it.
- **The classifier's default is the FALLBACK, not the refusal.** ENOENT is the
  common real case and must keep working, so an errno nobody classified falls
  back rather than refusing a launch. The rule for the next one is in D5: does
  the error say something about the PATH, or about THIS PROCESS?

## Paths

- `internal/posse/visibility.go` — `canonicalPath` (now `(string, error)`),
  `errPathUnnameable`, `canonicalAbbrevHome`, `DeriveIdentityLiterals`.
- `internal/posse/gates.go` — `l3Unnameable`, `probeL3HooksAsk`'s
  classification, `l3DegradeLine`, `commitGuardUnmeasured`,
  `commitGuardConsequence`, `commitGuardHomeStamp`.
- `internal/posse/hooksredirect.go` — `l3DegradeLineIn` passes `l3Unnameable`
  through unworded.
- `internal/posse/canonicalwall_qa_test.go` — `qcwNoReach`,
  `TestQACanonicalPathRefusesADirectoryItMayNotName`,
  `TestQAAWallWhoseInstancePathCannotBeNamedIsRefusedNotRespelled`.
- `internal/posse/gates_helpers_test.go` — `qaCanonicalPath`.
