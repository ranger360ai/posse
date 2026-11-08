# ADR 0082 — a path rendered into the commit wall is spelled as the kernel names it

*Status: accepted 2026-11-08 (ranger-base-5wq6g, from ranger-base-k9x8n) ·
owner: architect · amends ADR 0024 D2 check 3 (the instance-path literals are
rendered in their canonical spelling) and ADR 0081 D2 (the `# posse-home:`
stamp is too) · keeps ADR 0023's identity half byte-for-byte · builds in the
code beads named on ranger-base-5wq6g (dinesh)*

## Context

Three renderers write or re-render the shared commit wall: `install-hooks`
(its `dir` is the operator's argument, or cwd), `RenderSessionHooks` and the
L3 probe (both get the session dir). The probe decides "ours" by rendering
again and comparing BYTE FOR BYTE (ADR 0023). All three already reach the
wall's inputs through ONE function, `App.commitGuardLiterals(dir)`
(internal/posse/gates.go). The function is shared, but its output still
depends on how the caller spelled `dir`. ADR 0024 D2 check 3's instance-path
literal is `filepath.Dir` of the `.beads/redirect` target, and a relative
redirect is joined onto `hookRepo(dir)` as typed. So two spellings of one repo
render two walls that behave the same and differ in bytes, and the second
reads as "ours but stale" on every launch. That is ranger-base-up22's class,
reached through the path spelling instead of the visibility mark.

MEASURED 2026-11-08 (ranger-base-k9x8n's reproduction, darwin kernel 25.6.0):
`install-hooks <real>/repo` and `install-hooks <link>/repo` wrote walls that
differ in exactly the three instance-path lines.

MEASURED 2026-11-08 (this bead, macOS 26.7.1, go1.26.5 darwin/arm64, APFS
case-insensitive, scratch dirs `real/Repo` and `link -> real`):

| spelling handed in | `filepath.EvalSymlinks` | `fcntl(F_GETPATH)` on an open fd |
|---|---|---|
| `link/Repo` | `real/Repo` | `real/Repo` |
| `real/repo` | `real/repo` (case **kept**) | `real/Repo` |
| `LINK/REPO` | `real/REPO` (case **kept**) | `real/Repo` |
| `/tmp`, `/var/folders` | `/private/...` | `/private/...` |
| `/System/Volumes/Data/Users/<u>/src/posse` | same string (firmlink) | `/Users/<u>/src/posse` |
| a missing path | error | error (open fails) |

`git rev-parse --show-toplevel` also folds case and symlinks, but it only
answers for a git work tree. The instance dir is not required to be one.

So the bead's option (a), EvalSymlinks, closes one spelling axis and leaves
two open: letter case (ranger-base-99gww's axis, the one `samePath` exists
for) and firmlinks.

## Decision

**D1 — The canonical spelling.** A path rendered into the commit wall is
spelled the way the kernel names the directory:
- **darwin**: open the path (`O_RDONLY`, the directory itself) and read
  `fcntl(fd, F_GETPATH)`. This gives one name per vnode: symlinks resolved,
  on-disk case, firmlinks folded.
- **elsewhere**: `filepath.EvalSymlinks` of the absolute, cleaned path.
  ASSUMED for Linux: case-sensitive volumes, which is the default and what
  ADR 0080's Rex host runs. A casefolded ext4 dir keeps the caller's case.
  That is a residual, not a promise.
- **when the path cannot be opened or resolved** (it does not exist, or the
  cage refuses it): `resolvedPath`'s string answer. That answer is
  deterministic for a given input, and a missing instance dir is already a
  broken redirect.

One function, `canonicalPath(p string) string`, lives beside `resolvedPath`
and `samePath` (internal/posse/visibility.go). The darwin half goes in a
build-tagged file. `samePath` stays the COMPARISON: it answers "same
directory?" by identity. `canonicalPath` is the RENDER: it answers "which
string do we write?". They are different questions and must not merge.

**D2 — Applied once, at the literal, not at the caller.**
`DeriveIdentityLiterals` takes `repo := canonicalPath(filepath.Dir(target))`.
It renders `instance-path-abs` from that value, and renders `instance-path`
as `~/` + the path relative to `canonicalPath($HOME)` whenever `repo` sits
under it. That gives one `~` form for every spelling of HOME, rather than
none when HOME was spelled differently from the repo. `hookRepo`, `dir` and
the three callers do not change. Every renderer already funnels through
`commitGuardLiterals`, so canonicalising inside it covers all three by
construction. This also canonicalises an ABSOLUTE redirect that was written
through a symlink, which canonicalising `dir` at the entry would not do.

**D3 — The home stamp takes the same spelling.** ADR 0081 D2's
`# posse-home: <absolute a.Home>` becomes `canonicalPath(a.Home)`. This is
the one other path the wall renders, and it is the same class: one home
reached through two spellings (an `RHQ_HOME` typed through a symlink, say)
renders two stamps, and the byte compare reads stale. Readers keep comparing
the stamp with `samePath`, so a body stamped before this lands is still
"ours". It re-renders once, through the existing stale→reinstall path, and
no migration case is added. ADR 0081 said "the operator acts on the spelling
they configured", and that sentence is withdrawn. A refusal that names the
kernel's spelling names the same directory, and `samePath` is what
D3 case 3 asks.

## Consequences

- The three renderers produce identical bytes for one repo, however the
  operator, cwd or session dir spelled it. The L3 probe's "ours but stale"
  verdict once again means only that an input changed.
- Literal values change once, on darwin, for every repo whose spelling was
  not already canonical (`/var/...` fixtures and `/tmp` scratch repos). The
  first launch after landing re-renders through the stale path. Test
  fixtures that build an expected literal from `t.TempDir()` must pass that
  through `canonicalPath`. Updating them is the build's work, not a
  divergence.
- Coverage of check 3 is the canonical spelling and its `~` form, no more.
  Content that leaks the instance path through a symlink spelling
  (`/tmp/x` for `/private/tmp/x`) is not matched by the literal. Before this
  ADR such content was matched only when the rendering caller happened to
  hold that same spelling, so no coverage is lost that was ever promised.
  This joins ADR 0024's Residuals.
- If a symlink in the path is repointed, the next render changes the
  literal. That is correct, because the instance path did change. The bead
  priced this as a cost of (a). It is the wall following the box.
- Exit hatch: one function with two platform halves and no state held.
  Swapping F_GETPATH for anything else is one file.

## Alternatives rejected

- **(a) EvalSymlinks at every render and probe site.** MEASURED above: it
  keeps the caller's case and does not fold firmlinks, so `~/src/proj` vs
  `~/src/PROJ` still renders two walls. "Every site" also means three places
  to keep in step, which is the drift `commitGuardLiterals` exists to
  prevent.
- **(b) Make the probe compare check 3's lines semantically.** This breaks
  ADR 0023's byte-for-byte identity, the property the probe exists for. A
  probe that judges some lines by meaning is a second matcher, with its own
  bugs, sitting on the privacy wall. If this had been chosen it would have
  been hoover's, as the bead says.
- **Canonicalise `dir` at `commitGuardLiterals`' entry (the bead's (c) as
  literally worded).** This fixes a relative redirect but not an absolute
  one spelled through a symlink. Unless kept local, it also changes the
  `dir` that `hookRepo`, `BeadsVisibility` and `installHook` see, which is a
  wider blast radius for no extra closure. D2 is (c) moved down one call, to
  the value that is actually rendered.
- **Use git's `--show-toplevel`.** It folds both axes (MEASURED), but it only
  answers inside a work tree, costs a spawn, and the instance dir need not
  be a repo.
- **Render every spelling seen (the clever one).** Each render would emit the
  union of spellings that any caller has used. That makes the set a function
  of history: the bytes would depend on which renderer ran first, which is
  the defect again, now remembered. It was also unbounded.

## Amendment: a path a refusal tells the operator to paste is a render too (ranger-base-wvoqe, 2026-11-08)

**Context.** D1 scoped `canonicalPath` to what is rendered INTO the wall.
Two refusals also print a path and then tell the operator to paste it as a
`beads_visibility:` key: ADR 0081 D7's queue-commit refusal
(internal/posse/queuejsonl.go, fed by `QueueRepo()`) and D4's install-hooks
refusal (cmd/posse/main.go, `not installed: prepare-commit-msg`). Both print
the repo as `AbbrevHome(hookRepo(dir))`, which is the caller's spelling. Both
print the owning home from the wall's `# posse-home:` stamp, and D3 made that
canonical. So one refusal line shows two paths in two spellings.
ranger-base-8kwa5 made `QueueRepo()` absolute (`filepath.Abs`). This bead asks
whether it should canonicalise as well.

Matching is not the issue. The prescribed key is read by `samePath`, which
compares by identity, so any absolute spelling of the repo matches.
Absolute was the property the remedy needed in order to work, and 8kwa5
supplied it.

MEASURED 2026-11-08 (go1.26.5 darwin/arm64, APFS case-insensitive, scratch
`real/Repo/.beads`): `underDir`'s two sides, after `absResolve`, are
`…/real/REPO` and `…/real/Repo/.beads`. `filepath.Rel` gives
`../Repo/.beads`, so the answer is "not inside", although `os.SameFile`
says the two are the same directory. `underDir` compares strings, and
`EvalSymlinks` keeps the caller's case.

**D4 — Render at the render point, never at the value.** A path that a
refusal or remedy prints for the operator to paste into config is a render.
It is spelled `canonicalPath`, with its `~` form taken through
`canonicalAbbrevHome`. That applies to both of the paths such a line prints,
and to both refusals above. It is done in ONE exported helper, so that
cmd/posse and internal/posse cannot drift apart, and it is called where the
line is formatted.

`QueueRepo()` stays `filepath.Abs` and does not canonicalise. It is a VALUE
that feeds `underDir`, `OwnsCommitWall` and `InstallCommitGuardHook`, and
those are comparisons. The rule this ADR states twice applies here: the
comparison and the render must not merge.

So the bead's last question, "which spelling does `underDir` compare
against?", is answered: the one it compares against today. `canonicalPath`
never feeds a comparison. The case defect measured above belongs to the
comparison, and its fix is the comparison's own. Containment should be
answered by identity: walk up from the store and check `os.SameFile`
against the queue repo, the way `samePath` falls back. It should not be
answered by canonicalising one side to hide the mismatch.

**Consequences.**
- Every spelling of `queue_repo:`, of the cwd, and of the `install-hooks`
  argument prints the same refusal, byte for byte. That refusal's key is the
  string the identity wall already prints for that repo, so an operator who
  greps for it finds one spelling.
- Matching does not change: a key the operator already wrote in their own
  spelling still matches through `samePath`. The prescribed key is a
  suggestion that is guaranteed to match, not the only key that will.
- The pin 8kwa5 added (TestQAARelativeQueueRepoIsRefusedByItsAbsolutePath)
  asserts a property and not a string, so it stays green.
- The separate fix this section hands off — containment by identity — landed
  as `underSameDir` (internal/posse/visibility.go, ranger-base-oczuq). It
  runs `underDir`'s string answer first and unchanged, then climbs the
  store's ancestors asking `samePath`. `CommitQueueJSONL` is its one caller;
  `underDir` itself stays a string comparison and now says in its own doc
  comment why, so no sandbox grant, cage mount or `writable:` reach decided
  through it is moved by this fix. The prediction above held: no site
  needed `QueueRepo()` to canonicalise, because `absResolve` inside the
  predicate already absolutises whatever spelling the config carries.
- Radius: these two are the only sites that MEASURED as prescribing a path to
  paste (a grep of non-test `.go` for `beads_visibility:` inside format
  strings, 2026-11-08). The next site to prescribe a path uses the same
  helper. No census pin is added, because none would name the process it
  protects (ADR 0006 §7).
- **The helper landed as `CanonicalAbbrevPath`** (internal/posse/visibility.go,
  ranger-base-w79sc) — the composition D2 already renders the identity wall's
  own instance path with. It landed as the bare
  `canonicalAbbrevHome(canonicalPath(p))`, which was that composition when
  both callees answered one value; what it does with D5's refusal is D7 below.
  Exported, because the two sites this section names live one in cmd/posse and
  one in internal/posse and the whole point is that they cannot drift. Both
  paths in both lines of both refusals go through it; `QueueRepo()`, `hookRepo`
  and `install-hooks`' target are untouched, as D4 requires. Pinned by
  internal/posse/queuerefusalspelling_qa_test.go (a case-changed and a
  symlinked `queue_repo:`) and cmd/posse/installhooksrefusalspelling_qa_test.go
  (a symlinked target, and the byte equality of the two spellings' refusals),
  each of which also asserts that the VALUE is still the config's own spelling
  — so the rejected alternative reds a premise rather than passing quietly.
- **This section's Radius is one site short, re-measured by the same grep
  while landing it** (2026-11-08, ranger-base-w79sc). `QueueReadiness.GovRows`
  (internal/posse/queuejsonl.go) prints the queue repo twice in G13's
  `queue-unmarked` row and prescribes the second as `` `%s: private` `` — a
  remedy prescribing a key to paste, in the caller's spelling, in non-test
  `.go`, inside a format string carrying `beads_visibility:`. It was not taken
  in w79sc, whose DO and DONE WHEN enumerate the two sites above, and because
  a governance row's Detail has a different audience (`posse status`, the
  cockpit) than a refusal; filed as ranger-base-qm7eg with the cost measured
  (no pin asserts that Detail, so nothing in the suite constrains its
  wording). The two sentences above stand as written and are left as written:
  what is corrected is the count, here, where a reader of the Radius claim
  reaches it.
  **Taken 2026-11-08 (ranger-base-qm7eg):** both of that row's arguments go
  through `CanonicalAbbrevPath`, so the three sites D4 governs are one
  helper's callers and the Radius above reads as three. The different
  audience turned out not to be a different rule — `posse status` and the
  cockpit print the Detail verbatim, so the paste happens from there, and the
  D7 refusal cites this row by name for the same cutover step, which is two
  reachable lines for one remedy. The SIBLING `queue-not-a-repo` row keeps
  `AbbrevHome` and that is now a decision rather than an omission: it
  prescribes nothing and quotes the `queue_repo:` line the operator must go
  and edit, so the canonical spelling would quote their file back to them in
  words it does not contain. Pinned in both directions by
  internal/posse/queuegovspelling_qa_test.go (a case-changed and a symlinked
  `queue_repo:` for the row that prescribes, the config's own spelling for
  the row that quotes), each arm also asserting `QueueReady().Repo` is still
  the config's spelling. The "no census pin" sentence above still holds and
  is still the reason this escaped: what now constrains the radius is three
  named pins at three sites, not a grep anybody has to remember to re-run.

**Alternatives rejected.**
- **Absolute is enough; leave it alone.** It is functionally true, because
  the key matches. But it leaves one line showing two spellings, and a remedy
  whose text depends on how the reader got there. That is why the bead
  was P3 and not P2. The fix costs two format arguments.
- **Canonicalise inside `QueueRepo()`** (the bead's question as worded). This
  changes the comparison that three consumers make, and only on one side. It
  would fix the measured case mismatch for a store spelled the way it is on
  disk. It would also create a new mismatch for a config and store that were
  both spelled `PROJ` while the disk says `proj`, and that config works today.
  Merging the render into the value turns a wording change into a
  behaviour change.
- **Canonicalise both sides of `underDir` (the clever one).** This closes
  the case axis by string, which is what `samePath`'s note says cannot be
  done in general. It also puts F_GETPATH's open and fcntl into a predicate
  that the seatbelt code reuses. Identity is the honest answer, and it is a
  separate fix.

## Amendment: a directory this process may not name has no canonical spelling (ranger-base-8j070, 2026-11-08)

**Context.** D1's third bullet said that whenever the path cannot be opened
or resolved — "it does not exist, or the cage refuses it" — `canonicalPath`
takes `resolvedPath`'s string answer, and that "that answer is deterministic
for a given input". True, and not what the byte compare needs.
ranger-base-g6bs7's finding, off verifying ranger-base-yx73g (which
implemented the clause exactly as written): the reading is an `open()`, so
this function's input is not the path string alone — it is the path string
PLUS the rendering process's filesystem reach. ADR 0023's ours-vs-stale
verdict compares a wall rendered by ONE process against a wall rendered by
ANOTHER. A renderer that can open the instance directory gets the kernel's
spelling; a renderer that cannot gets a different string for the same
directory; and the fallback is not an error, so nothing says so. That is
ranger-base-k9x8n's "ours but stale on every launch" reached through the
renderer's privileges instead of through the caller's spelling.

MEASURED 2026-11-08 (darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive), one directory on disk as `<root>/instance`:

| state | `kernelPath` | `resolvedPath`, asked as `<root>/INSTANCE` |
|---|---|---|
| readable | `<root>/instance` (both spellings) | `<root>/INSTANCE` |
| the directory `chmod 0` | EACCES (13) | `<root>/INSTANCE` — it still ANSWERS |
| a PARENT component `chmod 0` | EACCES (13) | keeps `/tmp`, unresolved |
| missing | ENOENT (2) | `<root>/INSTANCE` |

The parent row is the sharper one and is new here: with a component above it
unreadable, `EvalSymlinks` fails too, so the fallback loses the symlink
resolution as well and differs from the canonical spelling even for a path
with no case and no firmlink axis in it at all.

**WHICH CAGE LOSES THIS `open()`: none of the three.** The finding named this
the first thing to measure and said it might make the fix cheaper than it
looks. It does. L1 shims put up no filesystem boundary. The L2 seatbelt
profile posse emits is `(allow default)` + `(deny file-write*)` + write
grants + exactly ONE `(deny file-read* (literal …))` block whose members are
credential-store FILES (`credentialReadDenyLiterals`; `SeatbeltProfile` is
the only `deny file-read` emission in the tree), so a directory opened for
read matches `(allow default)` and nothing else. An L4 container runs linux,
where `kernelPath` has no reading at all and the fallback IS D1's non-darwin
half. So the firing configuration is ordinary filesystem permissions — a
directory whose mode, ACL or owner denies the renderer, or an unreadable
component above it — which is why the finding is latent on this box rather
than live. The seatbelt half is read off the emitted profile and NOT
measured: a nested `sandbox_apply` is refused from inside a caged seat
(`sandboxApplyRefusal`), which is every seat that could have asked. That is
a reading, labelled as one.

**D5 — The two failures are told apart by what they say about the
directory.** D1's third bullet is replaced by:

- **Nothing is there** (ENOENT, ENOTDIR, or a kernel answer that is not
  absolute), or **this platform has no kernel reading** (`errNoKernelPath`,
  which IS D1's non-darwin half): `resolvedPath`'s string answer, as D1
  always said. Every process that can see the parent derives the same string
  from the same input, and a missing instance directory is already a broken
  redirect.
- **The directory is there and this process may not name it**
  (`fs.ErrPermission` — EACCES or EPERM): **no answer.** `canonicalPath`
  returns `("", errPathUnnameable)`. A caller that drops the error gets a
  loudly empty string and never a plausible second spelling.

The classifier is `errors.Is(err, fs.ErrPermission)` and not an errno list,
because that is exactly EACCES-or-EPERM on every platform Go builds here and
it needs no `syscall` import in `visibility.go`. The default is the
FALLBACK, deliberately: ENOENT is the common real case and must keep
working, so an errno nobody classified falls back rather than refusing a
launch. The residual is an unclassified error that is nonetheless
process-dependent; the rule to apply to the next one is the bullet above —
does this error say something about the PATH, or about THIS PROCESS?

**D6 — A render that cannot be spelled is refused, and the probe abstains.**
Refusing in `canonicalPath` is half of it. `DeriveIdentityLiterals`
propagates, which is the answer it already gives a literal it cannot render
(one carrying a single quote), and the three renderers already refuse on
that error: both installers and `RenderSessionHooks`. The L3 probe did NOT
— it dropped the derivation error on purpose, because "an install that hit
it already failed loudly". That reasoning is sound for the single-quote
class, which is a property of the VALUE and identical in every process, and
unsound for this one: the install that wrote the wall may have had the reach
the probe lacks. So the probe tells the two apart by `errors.Is` and, for
this class, abstains — a new verdict `l3Unnameable`, which is neither
`l3Held` (a privacy wall is not vouched for by a comparison nobody made) nor
`l3Stale` (nothing says the body is old; the remedy is reach, not an
install). Its degraded line says NOT MEASURED and names the directory, its
consequence clause is `commitGuardUnmeasured` rather than the worst case
(which would be a claim about a body this probe did not read), and
`l3DegradeLineIn` passes it through unworded in redirect mode, because
"re-launch to re-render" would prescribe a re-launch from the same seat.

MEASURED as two mutation arms, which is why D6 is here and not just D5
(ranger-base-8j070,
`TestQAAWallWhoseInstancePathCannotBeNamedIsRefusedNotRespelled`):

- With `canonicalPath` reverted to the fallback — D1's third bullet as
  originally written — the probe asked through the ON-DISK spelling reads
  `l3Held`: reach cannot move a literal the caller already spelled
  canonically, which is the whole of "latent on this box". Asked through the
  case-changed spelling of the same directory it reads `l3Stale`, with the
  degraded line `ours but stale — run posse gates install-hooks; no named
  guard is missing from the installed body … so the drift is elsewhere`,
  over a wall that is perfectly good. So the pin asks both spellings and
  only the second can catch anything.
- With `canonicalPath` refusing but the probe still degrading to no literals
  — the finding's fix shape (a) as it was literally worded, "abstain there
  (skip both instance-path literals)" — the same `l3Stale` line appears on
  BOTH spellings. Abstaining at the literals alone does not make two
  renderers with different reach disagree loudly; it writes a wall short two
  lines, which the other renderer reads as stale and re-renders, which this
  one reads as stale and re-renders. The silence moves, it does not close.

**Consequences.**
- On a box where every renderer can read the instance directory — every box
  today — nothing changes: no new refusal is reachable and the walls are the
  bytes they already were.
- Where it is NOT readable, an `install-hooks` refuses with the reason
  instead of writing a second spelling, and the launch reports L3
  prepare-commit-msg as not measured, which fails parity and so refuses the
  launch unless the operator passes `--allow-degraded`. That is the intended
  trade: a session that cannot verify its privacy wall should say so.
- `canonicalPath` now returns `(string, error)`. Fixtures that build an
  expected literal from a temp dir go through `qaCanonicalPath(t, …)`, which
  Fatals on the refusal arm — that arm has its own pin
  (`TestQACanonicalPathRefusesADirectoryItMayNotName`).
- **The `# posse-home:` stamp keeps its existing answer and that is a
  residual, stated rather than hidden.** `commitGuardHomeStamp` has no error
  return, so an unnameable home gets the answer an unnameable home already
  got — no stamp, D3 case 4's migration body — and a renderer without reach
  would differ from one with it in that line. It is unreachable on the
  install path twice over: `DeriveIdentityLiterals` asks
  `canonicalAbbrevHome` of that same HOME and refuses the whole derivation
  first whenever the repo has a redirect at all, and a process that read its
  config out of that home can open that home. What is left is a repo with no
  redirect whose renderer cannot open its own `RHQ_HOME`, and nothing on this
  box is that.
- **Off darwin there is no reach axis to lose**, because nothing opens
  anything: `kernelPath` answers `errNoKernelPath` for every path and the
  fallback is D1's non-darwin half. Both new pins skip there and say so.
  A darwin HOST renderer and a linux CONTAINER renderer of one repo
  therefore still disagree wherever the two spellings differ — that is D1's
  pre-existing non-darwin residual, named here because this amendment walked
  past it, and not closed by it.
- **The hook-wall sweep's SUMMARY line does not learn this verdict, and the
  per-repo line does.** `hookWallLine` passes `l3Unnameable` straight
  through, so the repo's own line reads NOT MEASURED, which is right. The
  header above it counts that repo in `Findings` and says "do NOT carry this
  binary's render", and the prescription under it says to re-render — both of
  which are claims the probe did not make. Separating an unmeasured repo from
  a failing one costs a second counter in `hookWallSummary`, its header
  wording and its pins, which is a deliverable and not a line; it is left
  undone deliberately rather than half-done, and it is reachable only in the
  configuration above, which no cage on this box produces. The honest line is
  the one the reader sees next to the repo.

**Alternatives rejected.**
- **Keep the fallback; it is deterministic for a given input.** D1's own
  sentence, and it is withdrawn: the input is not the string alone. MEASURED
  above as a mutation arm, not argued.
- **Abstain at the literals only (the finding's (a), as worded).** MEASURED
  above: the flip-flop survives on both spellings. It is the right
  instinct — do not render a second spelling — stopped one rung short of
  the verdict that reads it.
- **Take the reading once where the wall is INSTALLED and have the probe
  compare against the stored spelling (the finding's (b)).** This makes the
  rendered bytes a function of history, which is this ADR's own rejected
  "render every spelling seen", and it has the probe trust the wall's
  content instead of re-deriving it, which is the byte-for-byte identity
  ADR 0023 exists for. Two decided properties for one latent defect.
- **Classify by errno list rather than `fs.ErrPermission`.** Same answer,
  a `syscall` import in `visibility.go`, and a list to keep in step with a
  platform nobody here runs.

## Amendment: the refusal a human reads degrades where the wall refuses (ranger-base-vkmv3, 2026-11-08)

**Context.** D4's one helper, `CanonicalAbbrevPath`, and D5's refusal were
decided and landed in parallel, off one base, by two beads that did not
conflict textually: D5/D6 gave `canonicalPath` and `canonicalAbbrevHome` an
error return (ranger-base-8j070), and the helper that composes them was
written against the one-value signatures (ranger-base-w79sc). The merge
succeeded quietly and `go build ./internal/posse` was what broke — MEASURED
2026-11-08 on main at 9ac5f656, go1.26.10 darwin/arm64:
`visibility.go:576:9: too many return values`, plus a second break in
`internal/posse/queuerefusalspelling_qa_test.go` (three one-value
`canonicalPath` calls in a fixture premise) that only the test build reaches.

The signature is not the decision. Both of the helper's call sites are
REFUSAL lines — ADR 0081 D7's queue-commit refusal (internal/posse/queuejsonl.go)
and D4's `not installed: prepare-commit-msg` with its remedy
(cmd/posse/main.go) — and a refusal has to print something. So what is being
decided is what such a line prescribes as a `beads_visibility:` key for a repo
whose directory this process may not name, and whatever this one helper does,
every later prescribing site inherits — including ranger-base-qm7eg's G13
`queue-unmarked` row.

**D7 — A path a refusal PRINTS degrades; a path the wall WRITES refuses.**
`CanonicalAbbrevPath` keeps its single return value and absorbs both refusals,
per half:

| the half that refused | the answer |
|---|---|
| the path itself | `AbbrevHome(p)` — the caller's spelling, which is then the only spelling of that directory in existence |
| `$HOME` | the canonical absolute path, unabbreviated — the `~` form is given up, the kernel reading of the path is not |

The reason is the CONSUMER, and it is the one thing that differs from the
wall. D5 refuses because ADR 0023's ours-vs-stale verdict compares a wall
rendered by one process against a wall rendered by ANOTHER, byte for byte; a
second spelling there is a flip-flop nothing reports. This helper's answer is
printed once into a line an operator reads, and is never stored, re-derived or
compared — there is no second render to disagree with, so the defect D5 closes
has no analogue at this site. And "" is here the one answer that cannot be
given: a remedy reading `add : <private|public>` prescribes nothing, and both
call sites are the last thing that happens on their path (one returns the
error, one `os.Exit(1)`).

The degraded spelling is still ACTIONABLE, which is the property that matters.
The prescribed key is read by `samePath`, which answers by identity, so the
caller's own spelling matches exactly as well as the kernel's — D4's own
consequence, that the canonical one is the spelling guaranteed to match rather
than the only one that will. Canonicalising buys agreement with the identity
wall's spelling of that repo and nothing else, and in exactly the case the
degrade is reached there is no such spelling to agree with: D6 refuses the
whole derivation over an instance path it cannot name, so that repo's wall
has no instance-path literal. **The reason to canonicalise is void in the one
case the degrade is taken.**

MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
case-insensitive (internal/posse/canonicalabbrevdegrade_qa_test.go, two arms,
both reached and neither skipped on this box):

| fixture | reading |
|---|---|
| repo readable | `CanonicalAbbrevPath` -> the kernel's spelling, through both the on-disk and the case-changed spelling |
| repo `chmod 0` | `canonicalPath` -> `errPathUnnameable`; the degrade is `AbbrevHome`'s of the caller's spelling, and through the case-changed spelling that is a DIFFERENT string from the one reach rendered |
| `$HOME` at mode **0111**, repo under it at 0755 | `canonicalAbbrevHome` -> `errPathUnnameable` while `canonicalPath(repo)` still answers; the degrade is the canonical absolute path and the `~` form is lost |

**Mode 0111 is why the second arm can be written at all, and it is also the
firing configuration this amendment makes real.** F_GETPATH's reading is an
`open(dir, O_RDONLY)`, which needs READ on the directory; reaching a path
BELOW it needs only SEARCH. So a directory at 0111 is one this process may not
name and may still reach through — which is both the only state where the home
half refuses while the path half answers, and an ordinary permission setting a
box can be in. D5's "latent rather than live" stands for the three cages; it
is `chmod 0` that is exotic, not the class.

**Consequences.**
- The build is whole again on all three arms, and `go vet` is clean under
  each (`""`, `posse_arm2`, `posse_arm3`).
- A degraded answer may be non-canonical, so it may never be stored, compared
  or rendered into the wall — on top of D4's own rule that a render never
  feeds a comparison. The wall's own path keeps going through `canonicalPath`
  directly and propagating (`commitGuardLiterals`, `DeriveIdentityLiterals`),
  so D6 is untouched: nothing this amendment does can put a second spelling
  into a wall.
- On every box today nothing changes, for D5's own reason: no renderer here
  lacks the reach, so neither degrade is reached outside its pin.
- The first arm MEASURES the cost rather than asserting it — with reach
  withdrawn the case-changed spelling renders a second string — so the thing
  D5 refuses at the wall is on the record as accepted here, in a string that
  is printed and dropped.
- ranger-base-qm7eg's G13 `queue-unmarked` row inherits this answer, which is
  what it said it was blocked on, and it LANDED on the same day through the
  same helper (f04ecbcf, the Radius note above): its Detail has a different
  audience — `posse status`, the cockpit — and that turned out not to be a
  different rule, so nothing there needed a second decision. Three sites are
  now one helper's callers, which is also what makes this decision's radius
  three rather than two. That is why D7 keeps the single return value rather
  than giving the helper an error: a signature change here would have been a
  third site to teach, landing after the first two.
- The two pins are in arm 1, beside `canonicalwall_qa_test.go`, because they
  share `qcwNoReach` and because the sibling they complete
  (`TestQACanonicalPathRefusesADirectoryItMayNotName`) is there. No census pin
  is added, for D4's reason (ADR 0006 §7).

**Alternatives rejected.**
- **Give `CanonicalAbbrevPath` an error return and let each call site decide.**
  Two sites each inventing an answer is precisely the drift D4's ONE helper
  exists to prevent, and neither site has anywhere to put the error: one is
  already returning a refusal about something else, the other exits 1.
- **Return `("", err)` and print the empty string.** D5's loud answer, and the
  right one for a literal that must not be written. Printed, it is a remedy
  with no path in it.
- **Degrade in one line — `if either half refused, return AbbrevHome(p)`.**
  It passes the first arm and reds the second, which is why the second arm
  exists: it throws away a kernel reading that was successfully taken, and
  abbreviates against a home this process cannot name — the second spelling
  one rung in, which is `canonicalAbbrevHome`'s own stated reason for
  propagating rather than degrading.
- **Say in the line that the spelling is degraded.** The most honest option
  and it is not available at this helper: the wording would have to thread
  through both call sites, which is the drift again, and a key with a
  parenthetical in it cannot be pasted. The honest thing a one-string helper
  can do is print a spelling that names the directory.
