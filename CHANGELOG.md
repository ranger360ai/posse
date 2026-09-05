# CHANGELOG

Notable changes, newest first, one section per release. Write these for
someone deciding whether to upgrade, not for the person who wrote the fix.

A release's section is what `.github/workflows/release.yml` puts at the TOP of
that release's notes on GitHub, above the generated commit list —
`scripts/release-notes.sh` is the reader, `make release-notes VERSION=vX.Y.Z`
prints exactly what will be prepended. Renaming `## Unreleased` to the version
being cut is a precondition of the tag; see `docs/runbooks/release.md`.

## Unreleased

### Upgrading

**`runtimes/` is now part of the promoted set, so this release needs `make
install` *and* `posse promote` — in that order.**

The per-key runtime overlay (`runtimes/<name>.yaml`, the file that sets a
tier's model) is read at every launch and decides which model a session
actually runs on, but nothing attested to it: it was in no manifest entry
and in no promoted path, so a hand edit at the config home changed every
future launch and no check anywhere noticed. It is promoted prose now, on
the same terms as `config.yaml` — written only by `posse promote`, hashed
into `promoted.json`, in no session's writable set, and walled from persona
commits in the constitution repo as `rhq/runtimes`.

What that means the first time you run the new binary: a config home whose
`promoted.json` predates this release names no `runtimes/` entry, so the
launch verify reads `unpromoted runtimes/<name>.yaml` and **every dispatched
launch refuses** until you promote. That is the fence working. The ritual is
`make install` then `posse promote`; `posse promote` does not itself run the
launch verify, so the two cannot deadlock.

One thing to do before promoting: any `runtimes/*.yaml` you placed at the
config home by hand must be committed to the constitution repo first (as
`rhq/runtimes/<name>.yaml`). Promote copies out what the commit carries and
removes what it does not, printing each removal — so an uncommitted overlay
leaves, loudly, rather than quietly staying in force. `posse promote
--dry-run` shows the whole ratification diff, including the arriving and
departing overlay files, and writes nothing.

### Added

**Your own CLI can now declare which reader parses its screen for a
permission mode: `pane_mode: claude-footer | grok-border | none`.**

The pane readers above shipped keyed on the runtime's NAME, so a fourth
runtime listed every session as `mode:?` — "nobody has measured what its pane
says" — forever, and the only way out was editing Go. `pane_mode:` is a
registry key on the runtime profile, the same shape `turn_outcome:` wears: a
CLI that paints a claude-shaped footer declares `claude-footer` and is read
today, with no code change.

`none` is a DECLARATION, not an omission, and that is the point of having it:
a CLI you have MEASURED to render no mode on any screen gets codex's permanent
`mode:—` and costs no pane read per session, while a CLI nobody has measured
still gets the loud `mode:?`. A `pane_mode:` no reader implements refuses at
load and names the three that are registered, rather than promising a reading
nothing performs.

A new screen vocabulary still needs a reader plus the captures it was measured
against — the same price `turn_outcome:` charges. And because which reader
parses a CLI's screen is code measured against that CLI's captures rather than
a fact about your box, `pane_mode:` is a MECHANISM key: declaring it in a
built-in's `runtimes/<name>.yaml` overlay refuses the load (ADR 0021 D2). A
claude release that rewords its footer is fixed at the corpus.

**`posse runtime check <name>` grows a `pane_mode` row, so the dimension is on
the onboarding checklist instead of in a listing after the first launch.**

The row is rendered from the reader registry rather than spelled per runtime:
a CLI that declares a reader gets that reader's own contract and what an
absent mode means under it (claude's footer is COVERED by a dialog, grok's
border leaves four modes UNNAMEABLE), `none` reads as a DECLARED DIFFERENCE
and a permanent `mode:—` rather than a failure, and a runtime that declares
nothing reads as UNDECLARED with the remedy — which registered reader to
declare if you measured one, and what a screen vocabulary none of them parses
costs. The `missing →` half names the price of the blindness: without a
reader, `posse list` and `posse gates <persona>` cannot tell a session that
lost its `--permission-mode` at launch from one that kept it. A fourth reader
registered in `permissionmode.go` reaches this screen with no change here.

**`posse list` and `posse gates <persona>` now say which permission mode each
session's PANE is in — and say plainly when they cannot tell.**

A launch line is a claim about the launch, not a fact about the process. A
hand relaunch, a drifted template, or an argv path that dropped a token all
leave a session in a mode nobody typed, and nothing surfaced that: sessions
have sat blocked on approval dialogs for hours because the flag posse types
was the only thing anyone could read. Every persona row of `posse list` now
carries a `mode:` token read off the live screen tail, and `posse gates
<persona>` reports the same per live session with the reason attached.

The three runtimes do not offer the same contract, so the field is
three-valued per runtime rather than "a mode or a blank" (measured on claude
2.1.251, codex-cli 0.150.1 and grok 1.0.5):

- **claude** names all six modes in its footer — three of them without the
  word "mode" (`accept edits on`, `bypass permissions on`, `don't ask on`),
  which is why the reader matches names and not a pattern. A modal dialog
  REPLACES the footer, so a pane sitting on one reads `mode:?covered`: it
  proves nothing about its own mode, and that is the state worth seeing.
- **grok** names two of six on the composer border (`auto`,
  `always-approve`). The other four, and a pane still on the startup splash,
  render nothing at all and read `mode:?unnamed` — four modes and an unknown,
  never "default". `always-approve` is also what `~/.grok/config.toml`
  produces with no flag, so the report says the mode and not which layer set
  it.
- **codex** renders no approval policy on any screen. Its column is a
  permanent `mode:—`, costs no pane read, and is deliberately not the same
  token as an unknown somebody could close with more work.

Nothing types into a pane to obtain this, and nothing reads the launch
command sitting in the pane's own scrollback. A mode is a default
disposition, not a promise not to block (ADR 0035 §4): the `mode:` token and
the `working`/`blocked` state stay separate facts. This is the compensating
control ADR 0035 §3 names for grok, which gets no second mode layer.

**A dispatch pass now files a bead when CI is red on `main`, and says on
that bead when `main` is green again (`ci_workflow:`, on by default where
there is a GitHub gate to read).**

Merge-back fast-forwards a bead branch onto `main` and opens no pull request,
so the repo's CI workflow is the only gate a commit on `main` ever passes and
nothing runs before it lands. Nothing said so anywhere. Measured on this
repo's own `ci.yml` over its whole 300-run history: green at 01:23Z on
2026-08-30, red at 01:53Z, and then **191 consecutive failed runs over five
days and ~120 commits** with nobody noticing. What that costs is not the reds
— it is attribution: for five days a red run said nothing about the commit it
was attached to, so a genuine break (the package not building at all, twice,
for over an hour each) was indistinguishable from the standing noise.
`gh run list` is a command a human has to remember, which is the same reason
the workflow exists at all.

So a pass reads the gate and files ONE bead — `-l ci-red,devops`, P1, `-t
bug` — carrying the streak, both ends of the episode and the two commands
that reproduce the reading. One bead per episode, never one per push: the
dedupe is an open bead carrying that gate's marker, read out of the store
rather than kept in the launcher, and a `cancelled` run is treated as no
verdict at all rather than as red or green. Over the same 300-run history
that files **7 beads in 6.6 days** against 196 red runs; counting cancelled
runs as red would file 16, as green 13. While it stays red the bead's number
is re-said on a doubling cadence — 1, 2, 4, 8, … — so a five-day red earns
eight comments and not 191. When the next verdict-bearing run on the branch
is a success, the bead is told which run cleared the gate — including when
the fix landed under some other bead entirely.

**It closes that bead only if nobody ever claimed it** — status still `open`,
no assignee. That is the one exception ADR 0013 §4 admits, ruled by the
operator rather than assumed here: a bead nothing was ever dispatched onto
grades nobody's record, which is the harm the section names. Six of the seven
episodes measured above self-healed, and each of those would otherwise have
been a dispatched session that reads one comment and closes — about six a
week. The moment a seat claims the bead the close is the seat's again: the
green half is then the comment it always was, naming the run that cleared the
gate and saying to close it, and the harness leaves it alone. A cleared bead
somebody holds does not suppress the next red either — the dedupe reads that
comment back and steps over it.

It abstains wherever it cannot read, and never renders as green. A repo that
has **no gate** — not a git checkout, no such workflow file, no `github.com`
origin — is silent about it, so a shop whose repos have no CI never hears
from this at all. A repo that **has** a gate this pass could not read — no
`gh`, an unauthenticated `gh`, a network that does not answer, no
verdict-bearing run — says so once per process, because that one reads as an
all-clear if it reads as silence. The
branch is the one `origin/HEAD` names (`ci_branch:` overrides), and
`ci_workflow:` present-but-empty turns the whole thing off, the way an empty
`verify_labels:` turns verify-after off. The reading happens OUTSIDE the
launcher lock and only the writes inside it, so a green pass never parks the
fire loop behind a network call.

One interaction worth knowing: verify-after does not file a QA bead for a
`ci-red` bead closed with no commits naming it — closing a bead because the
gate cleared itself built nothing for a session to verify. A `ci-red` bead
that commits DO name is verified like any other close, so a persona who
actually fixed CI still gets the gate.

**A watch pass now says when the launcher binary running it is behind its
own repo, and `posse status` answers the same question on demand.**

A stale launcher does not fail. It dispatches, merges back and files beads
exactly as designed — it just does so with the defects its own repo fixed
hours ago, and the beads it files are indistinguishable from real ones.
`git log` shows nothing, because `main` is perfect; it is the binary that is
behind. On 2026-09-04 a build sat at eight hours and 34 commits behind, two
of which each independently stop one merge-back block from being re-filed,
and the block was filed a fourth time 77 minutes after the second of them
landed — costing a dispatched seat a whole session re-deriving a verdict
that two commits on `main` already held.

The reading is the running binary's own build stamp counted against the
branch of the checkout it came out of, which the stamp itself picks out of
the repos this instance knows about; nothing a session worktree holds can
move it. `posse status` prints it every time, including "not counted" and
why. The watch loop prints it only when it is behind, and then on a doubling
cadence — at 1, 2, 4, 8, 16, 32 commits — so a fleet falling further behind
gets louder while a fleet standing still goes quiet.

It is deliberately in the PASS and not in the loop preamble beside the
binary's identity: a launcher is built from the tip and the loop is started
right after, so a start-of-loop reading speaks at the one moment the number
is always zero (measured at 0 on both of that day's installs). The gap is
created entirely afterwards, under a loop that cannot change its own binary.

A reading, never a control: it prints, it warns, it decides nothing, and it
does not move `posse status`'s exit code. Installing over a binary that is
dispatching a live fleet stays the operator's move.

**The commit hook now refuses a sha in `docs/adr/` that is not on your main
checkout's branch, and prints the token.**

A record that says "landed `c067486`" can only ever name the writer's own
session tree, and the launcher rebases a third of those trees before it
lands them — 48 of 134 landings measured — which mints a new sha. Twelve of
the thirty-two resolving shas in this repo's own `docs/adr` were ancestors
of nothing by the time anyone looked. The `prepare-commit-msg` hook gains a
fourth wall: every 7–40 hex token on a line a commit ADDS under `docs/adr/`
that resolves here and is not an ancestor of the branch your main checkout
has checked out is a refusal naming that token. Cite the bead id instead —
`git log --grep <id>` survives the rebase. Tokens that resolve to nothing
here are prose and are passed; with a detached main checkout the arm has no
base, judges nothing, and says so rather than guessing a branch.

No override env. The one exemption is a record whose subject IS the stale
sha — a census, an incident writeup — which carries the landed twin in the
same file; the arm admits a pair with the same patch-id when one half is on
the base branch (ADR 0051 D5), and a sha minted in a session tree has no such
twin until the launcher lands it. `posse gates adr-census [files...]` is
the same predicate — the hook's own text, rendered from one place so the
two cannot drift — over whole files, for checking a record before committing
it; it replaces `scripts/adr-sha-census.sh`, which was a second copy of the
rule. Needs `posse gates install-hooks` (or the next session launch) to reach
a repo already hooked.

**A blind plan meter now says how OLD the reading it is still ruling on
is — loudly, in `posse status`, in every `--watch` pass, and in the
cockpit.**

The plan guard has had a clock ("guard blind 40m") and a failure class
("credential stale (401)") for a while. Neither said what the last reading
was or when it was taken, and the headroom rule that decides whether a
blind pass parks or degrades rules on exactly that number. On 2026-09-02
it was ten hours old, every hourly re-ask had come back 429, and the only
trace anywhere was one log line per hour.

Past `plan_usage_stale_after:` (new, default `2h`, `0` disables) all three
surfaces print the same line:

```
plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%)
— ruling on it under the headroom rule; 10 consecutive 429
```

The streak and its class come from `state/plan-usage.log`, which now marks
each failed read with its class token, so "10 consecutive 429" and "3
consecutive 401" are different sentences. Read only where a
`plan_guard_<window>:` is set — with no meter guard, no headroom rule is
ruling on anything.

The pulse's governance key gains the same two facts:
`guard-blind` is now `guard-blind:10h:429`. The hour bucket is the point —
a blind stretch that keeps growing re-reaches the coordinator once an hour
instead of being fingerprint-deduped after the first delivery, and an hour
is coarse enough that it is an escalation and not a storm. A credential
condition keeps its own `guard-credential:401` key, unchanged.

Nothing about what the guard DOES changed: same thresholds, same blind
budget, same park-vs-degrade rule (ADR 0018).

**`posse runtimes` now says whether the account can actually run each
tier's model, and `--probe` re-asks.**

Under every launch profile whose `egress:` names the model catalog host,
the listing prints one availability verdict per mapped tier — the same
sentence a launch would print, without launching anything. Profiles posse
knows no catalog for (codex, grok, a template pointed elsewhere) print no
such line. `posse runtimes --probe` re-reads the catalog instead of ruling
off the shared snapshot; a live rate-limit cooldown is still honoured
first, so a forced read cannot extend a 429.

**A catalog reading now demotes a tier only while it is fresh, and says
so when it is not.**

The availability check rules off a shared snapshot of the account's model
list. When the probe cannot refresh that snapshot — an expired credential
answers 401, and that can last days — the snapshot used to go on ruling as
fact for as long as the outage lasted: bump a tier to a model id the old
list predates, and every launch in the shop was demoted until somebody
hand-edited `state/model-catalog.json`.

`model_probe_ttl:` (default 1h) is that reading's lease now. Inside it,
nothing changes: a list that does not name the wanted model still demotes,
loudly, per `tier_fallback:`. Past it, with the refresh failing, the
reading is quoted but obeyed by nothing — the launch takes the tier it was
asked for, and when the wanted id is absent from that stale list it says
so, once per launch:

```
architect: tier strong wants claude-fable-5-1 — not in the catalog read
48h00m ago and the probe is failing (401 Unauthorized); availability
UNKNOWN, launching as asked
```

`posse runtimes` and `posse gates` print the same verdict without
launching anything. A rate-limit cooldown still governs whether posse
re-asks the endpoint, and never whether it trusts what it last heard — a
429 renewed all day cannot renew trust in a day-old list. If the default
lease is too short for your account, lengthen `model_probe_ttl:`; there is
no second dial. The sentence that used to end at "unavailable" was true
and taught the wrong lesson — it read as "the account lost the model" when
what had happened was that the probe had been failing for two days. An
operator reading the new one knows to refresh a credential rather than to
edit a state file.

What it gives up, stated plainly: an account that really does lose a model
while its probe is down will launch on an id the CLI cannot serve, bounded
by the line above on every such launch and by `posse cost`, which groups by
the model that did the work rather than by the tier that was asked for.

**`posse backup` — one command that archives the work graph and the
constitution, on this box, and refuses to write anywhere else.**

The queue repo is the store of record for the whole work graph and it never
grows a git remote, which makes one disk the whole graph and its journal.
Until now the answer to that was whatever each deployment arranged for
itself. It is a verb now.

`posse backup` writes one archive holding the queue repo's history as a
`git bundle --all`, its beads database staged through SQLite's online backup
API against a source posse never writes to, the JSONL projections beside it,
and the config home's promoted set, `runtimes/` and `promoted.json`. `envs/`
and `secrets/` are never archived, and the archive's manifest says so by
name, so a restore reads their absence as policy rather than as loss. The
archive is a plain `tar.gz`: `tar -xzf` is the recovery path and it needs no
posse binary.

**The destination is on-box, and that is not a setting.** A URL, an
scp-style `host:path`, a UNC path, or any directory on a volume the kernel
does not report as local is refused — and so is a volume whose locality
cannot be read at all, because a refusal that fails open is not a refusal.
There is no flag that lifts it. The same rule runs on the source: a queue
repo that has grown a git remote is refused rather than copied.

**The source rule is yours to set, per instance: `queue_remote:`.** Every
`git clone` mints an `origin`, so a verb that refuses any queue with a
remote is a verb most deployments cannot run — and an instance whose queue
lives on a sanctioned internal remote is refused on its own box. Set
`queue_remote:` in config.yaml to the URL exactly as `git remote get-url`
prints it, and a queue whose only remote's fetch and push URLs both equal
that string is backed up; a second remote, a different URL, or a push URL
that points elsewhere still refuses, printing what you declared beside what
it found. A queue with no remote passes either way: the key sanctions, it
does not require. Leave it unset and today's rule stands, with the refusal
now naming the key as the way out. It is not a backup key — declaring your
remote arms no schedule and starts no clock — and the archive is unchanged:
the queue half carries no remote on any instance, and the declaration rides
only in `home/config.yaml`, where a restore brings the sanction back with the
line that made it. `posse backup status` prints which posture you are in
(ADR 0049).

Every archive is read back and checked against the manifest it carries
BEFORE it is given its name, so an archive that is there is one that
verified — which means the directory itself is the record, with no second
status file that can disagree with it. `posse backup status` reads it;
`posse backup verify` re-opens an archive on demand and extracts nothing.
Retention (`backup_keep:`, default 3) only ever prunes after a newer archive
has verified, so a run that produced garbage cannot destroy the last good
copy on its way down, and a free-space floor (`backup_min_free_mb:`, default
384) refuses rather than filling the disk.

`posse status` grows a line with the age of the newest archive, and past
`backup_max_age:` (default 48h, or twice the interval below) the shop check
carries it as a condition — in the terminal and in the cockpit's governance
block. An install that has never asked for backups says nothing at all; one
that asked and has no archive says so loudly, which is the failure this is
for.

**And it runs on a clock, if you set one.** `backup_interval:` in
config.yaml arms a ticker inside the `posse dispatch --watch` loop — the one
standing process the harness already owns, so there is no plist to install
and no second daemon to leave running. It is level-triggered against the
archive directory rather than against its own tick: it asks how old the
newest archive is, so a watch loop that restarts five times an hour under a
daily interval still makes one archive a day, and a loop that comes up next
to an overdue archive writes one at once instead of waiting out a fresh
interval. `posse pause` does not stop it — pause stops dispatching, and the
queue still mutates in a paused shop. With a schedule armed, `backup_max_age:`
defaults to twice the interval instead of 48h, so changing the cadence moves
the alarm with it. No `backup_interval:` starts no ticker at all, and
`posse backup status` says which of the two you have.

### Security

**A persona session could commit your constitution — the PIDs every future
session runs under — and nothing refused it.**

*Affected: every build before this one. Two walls added, both on by default.*

The gates already fenced the *command* (`posse promote` is denied in every
crew PID) and the *form* (an unqualified commit is refused in a shared
checkout), and under `cage: seatbelt` the constitution area was never
writable. Under `cage: shims`, which is what most personas run on, nothing
checked the *path class*: a session could rewrite `rhq/agents/*.md` and commit
it like any other file, and the next promote would put its own edits in force.

The `prepare-commit-msg` hook now carries a third arm. A commit from a session
carrying `RHQ_PERSONA` is refused when it touches `.claude/settings.json` or
`.claude/settings.local.json` in any hooked repo — that file holds the deny
list fencing the session's own destructive commands — and, in the repo whose
top level has `rhq/agents`, when it touches `rhq/agents`, `rhq/config.yaml`,
`rhq/recipes`, `rhq/skills` or `rhq/envs`. Your own shell carries no marker
and is untouched. The refusal names the paths and tells the session to stage
what it means somewhere outside the class for you to apply.

Behind it, the launcher will not fast-forward a session branch whose diff
touches those paths: it reports and leaves the branch alone for you to read
and land. That half runs in your process, so a session cannot scrub its way
past it — which the hook's arm, keyed on an environment variable, can be.
`core.hooksPath` still defeats every hook-tier gate.

Reinstall the hooks in repos you have already hooked (`posse gates
install-hooks <repo>`); a dispatch into a repo refreshes it automatically.

**...and the walls above go stale in every repo no session enters — including
the one that holds the constitution.**

*Affected: every build before this one.*

The hook bodies are compiled into the binary, so every installed hook is a
*copy* of the render that was current when someone wrote it. Only `posse gates
install-hooks` and a dispatch re-render one, and a dispatch refreshes the repo
it was cut from and no other — so a repo that never holds a session keeps
whatever it was given, indefinitely. That is how a constitution repo can run a
`prepare-commit-msg` without the arm above for hours after the arm shipped:
the wall existed exactly where sessions launch, and nowhere else.

`posse promote` and the `posse dispatch --watch` preamble now sweep every repo
`beads_visibility:` names and print the ones whose hooks are not this binary's
render — stale, foreign, or never installed — naming the repo and the command
that fixes it. Both report and rewrite nothing: a hook rewrite in a shared
checkout is a change you should type. A configured repo that is absent or is
not a git repository is skipped, not reported, and an instance with no
`beads_visibility:` block hears nothing at all. `make verify-hook-freshness`
in a posse checkout is the same question on demand.

**The bd argv gate you installed by hand went stale the moment the source
changed, and nothing told you.**

*Affected: every build before this one, on any box where the gate is
installed. Reports only — it installs nothing.*

`scripts/bd-argv-gate.{sh,py}` is source; what fences your box is the copy a
PreToolUse hook names in your Claude settings, installed by hand because posse
renders no box-wide hook (ADR 0015 §3). So a fix landing in this repo moves
nothing, and the copy falls behind in silence — which is how a wrapper that
failed OPEN on any bd call not on a command's first line stayed live after the
fix for it had landed.

`make install` now ends with that comparison, and `make verify-gate-freshness`
asks it on demand. It resolves the wrapper out of your settings rather than
assuming a path, compares both files against the main checkout's HEAD — never
a worktree's, so no unfinished branch is ever prescribed for a box-wide hook —
and then runs the installed wrapper three times, because a byte-perfect gate
with no working `python3` under it passes everything: an allowed verb must get
through, a denied one must be refused by the parser rather than by the
wrapper's own fallback, and an unrelated command must be untouched. A finding
prints the one line to type — a line written to survive the gate it repairs.
A promote is never failed by it, and a box that never installed the gate hears
nothing.

### Fixed

**`posse runtime check` and `posse runtime probe` refused a CLI they could
not see, on a PATH that does not decide whether a launch can see it.**

*Affected: the `exe` preflight gap, on any box where posse's own PATH and the
herdr daemon's differ — herdr started from a login shell and posse run from a
gated session, a stripped PATH, `~/.local/bin`, nvm, asdf.* The check ran
`exec.LookPath` in the posse process, but the pane a launch opens is a child
of the long-running herdr daemon and resolves in the daemon's environment. So
a CLI the daemon has and posse does not was refused with "is not on PATH — a
launch opens a pane that prints command not found", which is exactly what
would not have happened: the pane launches it fine. The refusal also blocked
`posse runtime probe`, the one command that opens a real pane and measures
the CLI the session actually launches — the only thing that could have said
which of the two was true — so a runtime in that shape could not be
onboarded at all.

The gap now reports and does not refuse, and its line says which PATH was
looked on and names `posse runtime probe <name>` as the thing that measures
the session's own answer. An empty `command:` still blocks — no PATH anywhere
makes an absent argv0 launchable. The reverse divergence (posse resolves the
CLI, the daemon does not) is a real dead pane that nothing cheap can see from
outside a pane; the probe row is what answers it, and `posse runtime check`'s
clean line no longer says a runtime "is installed" when what it measured was
posse's own PATH.

**`posse cost` reported $0 of grok (or codex) spend on a box where
`$GROK_HOME` / `$CODEX_HOME` moves the CLI's home — no error, no uncounted
line.**

*Affected: `posse cost`, the cockpit's spend readings and every report built
on them, on any box that sets either variable. Nothing in posse sets them, so
this bit an operator who moved a CLI home by hand.* The two cost adapters
rooted their walk at `~/.grok` / `~/.codex` while posse's other readers of
the same stores — the interstitial version probe and grok's turn-outcome
reader — already honoured the overrides, so the same binary disagreed with
itself about where a CLI keeps its records. A walk on an absent root is "this
machine never ran the CLI" by design (ADR 0018 §3), which is exactly what a
walk on the WRONG root looks like: the spend read as zero rather than as
unreadable. Both adapters now resolve the home the way every other reader
does, and each carries a pin that a store under the override is listed and
counted.

**`posse worktrees` called a caged session's committed work "nothing
unlanded", and the sweep called an already-landed one unreferenced — the two
commands disagreed about the same tree, each in the direction that costs.**

*Affected: `posse worktrees`, `posse worktrees --land` and the launcher's
landing sweep, for any session worktree whose HEAD is not on its own branch —
which is every container-tier session, by design.* The listing asked git a
pure ancestry question about the BRANCH (`<base>..<branch>`), and a caged
session is launched on a detached HEAD on purpose, because a commit that
writes no ref is what buys the read-only git mount. So the branch never
moved, the count was zero, and the listing printed "nothing unlanded" — its
one phrase for a tree that is safe to retire — over the whole of that
session's work, while the merge on the same tree said the work was on neither
the base nor the branch. The listing now asks the tree's own tip, the same
one every other surface here already asks, and when the work is off the
branch it says so and prints the `git branch -f` that puts it back.

The same disagreement ran the other way at the merge. Its report answered by
ancestry alone on the path taken when the branch itself never moved, so a
detached tree whose commits were cherry-picked onto the base was called
"unreferenced and a retire would lose it" over work the base was holding. It
now asks patch equivalence there too, and reports it with its evidence named
— a measurement, or a human's `-x` trailer, which are not the same claim and
are not printed as one.

Nothing changes for a tree whose HEAD is on its branch: the two tips are the
same commit, so the sentence is the one it always was.

**`posse runtime probe` read which binary it had measured off the wrong
PATH, so a passing record could certify a CLI nobody probed.**

*Affected: `state/runtimes/<name>/probe.json` and everything that reads it
— `posse runtime check`, and the `Bash(…)` parity claim on a template-only
runtime. Records written by an older posse go back to `ASSUMED` until you
re-probe once.*

The probe has two PATHs. Its pane is created by the herdr daemon and
inherits that daemon's environment, so the session resolves the CLI there;
`cli_path` and `version` were resolved in the posse process's own PATH
instead, and nothing reconciled the two. Measured, with a decoy named for
the CLI planted in front of posse's PATH alone: the record came back
`passed: true`, `version: "decoy 9.9.9"`, and a `cli_path` naming a
two-line shell script that cannot launch anything — over four observables
taken on the real CLI in the pane. The drift check then compared that decoy
against itself and reported *current*. Nothing needs planting in the field:
any box whose posse runs with a PATH the herdr daemon does not have
(`~/.local/bin`, nvm, asdf, a gated session) diverges the same way.

`cli_path` is now what the **session** resolved, read by typing `command -v`
into the probe's own pane under the launch line's own PATH prefix — before
the launch line, and before a model turn is spent. A pane that will not
answer gets no record at all: the probe refuses rather than name a binary it
did not measure. posse's own answer is kept beside it as
`launcher_cli_path`, because that is the only side `posse runtime check` can
cheaply re-read, so the drift comparison is launcher against launcher. Where
the two named different files at probe time, the check says version drift on
the measured binary *cannot be checked from outside a pane* and asks for a
re-probe after any upgrade, rather than reporting a currency it did not
check.

The probe's own wrong arm was inert for the same reason, and that is the
half that matters for trusting the claim: the opt-in live test that must
FAIL on a CLI whose children resolve in a login shell's demoted PATH
installed its shim by mutating the test process's PATH, which the pane never
sees, so it passed all four observables every time — and then blamed the
production probe. It now reaches the pane by absolute path, re-execs the
login shell itself, and checks a witness that its shim actually ran before
it judges anything.

### Fixed

**The shop check stopped sending the coordinator to clear a prompt that had
already been sent, and `--resume` stopped parking a bead behind one.**

*Affected: `posse status` / the cockpit's G2 row, and `posse dispatch
--resume`, for any claude session whose composer previews text.* Both read
claude's prompt box off `herdr agent explain` and called any text there "a
prompt sitting UNSENT in its box". That is a matcher over a screen region
that can hold a line nobody is about to send, and a matcher like that cannot
go false on its own: the same reading took the pulse arm off for about ten
hours on 2026-09-04 (~586 skipped ticks on lines the operator had already
sent), and the G2 row carried a `settled-unsent:` condition into the
coordinator's prompt with nothing anybody could clear.

The reading is now checked against the store that owns the fact — claude's
own submitted-prompt log — scoped to that pane's claude session. A box
previewing the line the pane LAST SUBMITTED is echoing it, not holding it:
`--resume` re-prompts, and the G2 row is the plain `settled:` one and says in
its detail which reading changed its shape. A box previewing anything else is
still a hold, still skipped, still `settled-unsent:` — including both boxes
on the live fleet when this was measured. Every failure on the way answers
"not an echo": no log, an unreadable one, a pane herdr will not name a claude
session for, rows in a shape posse does not know. Nothing changes for codex,
grok or any other runtime, which never had the reading.

One thing this cannot see, said out loud: an operator who retypes the line
this pane most recently submitted, and leaves it unsent, is read as an echo.
Only the LAST row is compared, so what gets typed over in that case is a
duplicate of what was just sent.

**A session could not relaunch itself: it waited the whole landing timeout to
be told to try again later, and the flag that skipped the wait would have
destroyed it.**

*Affected: `posse relaunch <name>` typed inside the session it names — the
long-lived coordinating session's own refresh, which is where the accumulated
context, and the cost, is.* The landing turn waits for the target's agent to
go idle before the workspace closes, and a session running that command is
working precisely because it is running it: the wait could only end at the
bound, and the message then offered a longer `--timeout`, which buys the same
words later. `--no-land` skipped the wait and reached the kill, which is
worse. A process does not outlive closing the workspace its own pane is in —
it ends inside the close call, and `nohup` does not save it, because what goes
down is the pane's process group — so the session would have been destroyed
and its name freed with nothing left running to make the replacement.

`posse relaunch` now recognises the case — the caller pane's own workspace id
against the session record's, on the same herdr server — and refuses **both**
arms in zero seconds, before it plans or waits, saying what each half would
do and naming the way through: run it from another session, or from a shell
outside it. Nothing else changes. A relaunch typed anywhere else is untouched,
`--no-land` is unaffected everywhere but this one case, and a session record
too old to name its herdr server keeps the old behaviour rather than being
refused on a comparison that cannot be made.

The measurement is `scripts/verify-self-close.sh` (`make verify-self-close`),
which also records that a child calling `setsid(2)` first *does* survive
closing its own workspace — so a self-refresh is buildable, and is not built
here: a new session leader cannot inherit the launcher lock that makes the
kill and the recreate one step or neither.

### Fixed

**The pulse arm switched itself off for ten hours a day, skipping on an
"unsent prompt" in a box that was empty.**

*Affected: every install with `pulse:` armed. Measured 2026-09-04: three
episodes in one day, ~586 consecutive skipped shop checks, ~10 of the day's
hours with no pulse at all while 108 commits stacked behind it.*

The pulse would not type a shop check after somebody's unsent prompt, since
the two texts go in as one garbled message. It read the box off herdr's
composer region — and for the coordinator's own pane that region kept
previewing the operator's last **sent** line, answered hours earlier, over a
box `posse peek` showed empty. The skip line quoted that line back every
tick. Nothing could make the reading go false: the persona cannot clear a
box that is already clear, and only a new operator message replaced the
phantom, so the arm stayed off until the operator happened to type again.

The pulse no longer gates on that reading. It still makes it, and prints one
line naming what the box previewed before delivering, so a genuinely garbled
turn stays explicable from the watch log — and a herdr that starts clearing
the composer on submit shows up as that line going quiet. What this costs is
one re-typed line on the rare tick that lands while the operator is actually
mid-keystroke. Everything else is unchanged: a screen herdr can see is
working still refuses the pulse, and `posse prompt`'s warning, its submit
read-back, dispatch's `--resume` skip and the shop check's G2 row all still
read the composer and still act on it — those are about a dispatched
holder's pane, where the reading has not been measured wrong and failing
towards not acting is the safe direction.

**`posse gates <persona>` printed a wall of green for a persona that could
not launch at all, and exited 0.**

*Affected: any persona whose `runtime:` names neither a built-in nor an
existing `runtimes/<name>.yaml`.* The parity table is built by walking the
runtime catalog, so an unresolvable runtime was not a row reading
"unresolvable" — it was no row at all, and every row that did print belonged
to a runtime that persona does not launch on. `posse agent check` had refused
the same PID with `unknown runtime` all along; `posse gates`, which INSTALL.md
§7 sends you to read *before* your first dispatch precisely to avoid a
confusing refusal later, said nothing and exited 0.

It now names the runtime above the table and exits 1, matching
`posse agent check`. The rest of the report still prints — the gates dir, the
shims and the refusals log are true whatever the runtime is. The header also
says whose directory it computed the matrix for (`launching in this shell's
cwd …`): driving a second instance, that is usually the *other* instance's
repo.

**`dispatch --watch` ran for hours without completing a pass on a busy shop,
and half its duties silently did not run.**

*Affected: every `--watch` loop busy enough to keep refilling seats.* A pass
gathered until every prompt it was waiting on had settled, and each settle
refilled the seat it freed — so each refill's own prompt joined the set the
pass was draining, and on a busy shop the set was fed faster than it emptied.
MEASURED 2026-09-04: one loop held `4 prompt(s) in flight, gathering` for
2h20m. The loop looked healthy from the outside — sessions launching, work
landing, the pulse ticking — because refills are not passes: the merge-back
sweep, the hook-wall check, the backup ticker, the plan-guard read, the
launch-cap epoch accounting and any offer of ready work to a seat that became
free with **no settle** behind it all live in the pass, and none of them ran.
A P1 the operator had asked for by name sat ready and unhired for an hour with
its persona's seat empty.

The gather is now bounded per pass. Prompts that settle inside the window are
judged and refill their seats exactly as before; prompts still outstanding are
*carried* into the next pass — the same wait, the same claim, judged by
whichever pass sees it land. A pass that returns with work outstanding says so
in one line, naming the beads, their sessions and how long each has been in
flight:

    … 2 prompt(s) still in flight, carried into the next pass: ranger-base-hl0sp in gwart-posse-… 10m15s, …

And the silence watchdog gained a second reading for whatever holds a pass open
next time: no completed pass inside its budget is a finding, said once, naming
the in-flight set that is holding it. The existing reading watches for the log
stopping, which this incident never did — the refills scrolling past kept it
quiet all night.

**An older `posse` earlier on PATH refused every dispatched launch for 90
minutes, pointing at a file that was present and hash-matched.**

*Affected: any box where two posse binaries are installed — the classic one
is `brew install ranger360ai/tap/posse` on a box whose promoted binary lives
in `~/.local/bin`, since `/opt/homebrew/bin` precedes it.* The release binary
was three days older than the promoted one, from before `runtimes/` joined
the promoted set, so it never walked `runtimes/` while the manifest — written
by the newer binary — named `runtimes/claude.yaml`. The launch verify said
`missing runtimes/claude.yaml`, which was false in every ordinary reading of
the word: the file was there, readable, and byte-identical to what the
manifest recorded. Dispatch refused every launch behind that line for ninety
minutes and burned an entire `-n 30` epoch on the refusals.

Four things changed:

- `promoted.json` now records **which posse wrote it** and **what promoted
  set that posse walked** (`posse`, `set`). Both are additive; an older posse
  reads the file exactly as before.
- The launch verify leads with the disagreement instead of a filename:
  `manifest written for promoted set [agents config.yaml recipes runtimes
  skills] by posse 0.4.0+92da1bc; this binary (0.4.0+feaf301) walks [agents
  config.yaml recipes skills] — a different, OLDER posse is answering here`.
  The path classes still follow it. This also works on the manifests already
  on disk, which record no `set`: the roots are read out of the file keys.
  The other direction — this binary walks *more* than the manifest was
  written for — is the ordinary upgrade order (a release that widens the set,
  installed but not yet promoted), and says so instead: `the manifest
  predates this binary's promoted set — re-promote`. The drift alone never
  refuses a launch.
- `posse status` and every `--watch` preamble now print **which posse binary
  is running**, and warn when PATH would answer `posse` with a different one:
  `warning: PATH resolves posse to /opt/homebrew/bin/posse, not the running
  ~/.local/bin/posse`. A persona session's own gate shim is followed to what
  it execs rather than counted as a shadow.
- A launch refused by the constitution verify **no longer spends the `-n`
  ration**. It creates no session, claims no bead and sends no prompt, so it
  attempted nothing; and since the fact is one reading of one home, the fire
  pass stops instead of reprinting the same refusal once per seat.

**`posse init` now refuses a home that `posse promote` manages.**

*Affected: anyone who runs the install steps on a box that already has a
promoted instance.* Init's operator fence (ADR 0031 §2) keys on the persona
marker, so it fences sessions and not the operator's own hands — and a
by-hand `posse init` on the fleet box re-seeded `examples/` and `secrets/`
under a promoted constitution. A promoted manifest is a claim about a commit
that only `posse promote` may restate, so an init that fills a gap there
leaves the launch verify refusing every dispatched launch with no re-stamp
available to fix it. It now refuses before its first write, naming the
manifest, the commit behind it, and `posse promote`. A **seeded** home is
unchanged: re-running init on one is still the generics upgrade INSTALL.md §7
advertises.

**A 429 storm on the plan-usage endpoint could keep itself alive: posse
honoured `Retry-After` exactly and asked again at the boundary, forever.**

*Affected: any instance with the plan guard armed, during a rate limit that
outlasts one window.* On 2026-09-02 this shop drew fourteen consecutive 429s
between 03:30Z and 16:35Z, each naming `Retry-After: 3600`, and three of the
asks that drew one were made *after* the window the previous 429 had stated
had ended. Every ask plausibly re-arms the hour, so a poller that waits
exactly one window and then asks is a loop that need not terminate — and the
plan guard is blind, and an unattended `--watch` fails closed, for the whole
of it. It was blind for thirteen hours that day.

The honoured cooldown now **doubles per consecutive 429 and resets on the
first success**: 1h, 2h, 4h, then a ceiling of 8h — six requests in a day
where the old cadence made twenty-four. An isolated 429 still costs exactly
what it asked for, and no single `Retry-After` is believed past an hour; what
changed is only the repeat. Nothing is muted silently: the wait is on the
blind line (`… 10 consecutive 429, next ask in 3h12m`), and
`state/plan-usage.log` now carries the endpoint's raw `Retry-After` and the
streak beside the honoured wait — so a reader can finally tell "the endpoint
asked for an hour" from "the endpoint asked for a day and posse truncated
it".

**The load guard stopped being read at all while a `dispatch --watch` pass
waited on its sessions, so nothing ended the leaked processes that were
saturating the box.**

*Affected: any `--watch` loop with a bead in flight — which, since seats
became rolling, is most of the day.* The guard's reading (and, under it, the
orphan report and its kill arm) was taken at the top of a pass. A pass no
longer returns while a bead is in flight, so on one measured loop the reading
happened once and then not again for 1h40m, while the box climbed to load 85
and eight orphaned gate-shell children burned about half a core each for
thirty-seven minutes. Nothing evaluated, nothing was reported, nothing was
ended; the operator killed them by hand.

The reading has its own clock now — one tick per `--interval`, on its own
goroutine beside the pulse and the backup clock, launching nothing and joined
before the loop says it has stopped. It prints only when it has something
true to say, so a healthy box adds no lines to the watch log.

**The orphan census now runs whether or not the box is over `load_guard:`.**

The kill arm rode inside the guard's refusal, so it only ever looked on a
pass the guard was already skipping. On the same incident, a restarted loop
sampled load 44 against `load_guard: 60`, did not skip, and therefore never
looked at the same eight orphans — which had matched the predicate the whole
time. Load was never the predicate: an orphaned, CPU-burning gate-shell child
with no `POSSE_KEEP=` marker is a leak at any load. `load_guard_kill:` is now
the only key gating the kill; `load_guard: 0` turns off the launch gate and no
longer turns off the census with it.

**An agent idle behind its own suite run was read as a persona that
stopped — re-prompted, reported to the coordinator, and on the second pass
escalated to a human.**

*Affected: any session that finishes its edits and then waits on background
work it started — a full-suite run behind a Monitor, a background shell, a
subagent — which is what the shipped developer prompts ask for. Measured
three times on one shop in one morning.*

Nothing posse reads could tell that agent from one that gave up. herdr's
`agent list` carries an agent's status and no word about what it is waiting
on; `agent prompt --wait` returns the instant the turn ends; and claude's
own detection manifest has no rule for a live shell or monitor, so the pane
matches `live_prompt_box` and reports `idle`. Dispatch therefore judged a
settle-without-close, `posse status` and the pulse carried a `settled:`
condition for a session that was working as designed, and a second such
pass would have filed a question bead for the operator about it.

posse now reads the two screen regions herdr previews while evaluating its
rules — claude's footer hint line and its prompt box — and treats an idle
agent that is holding either as WAITING. A settle behind live background
work is not judged and not counted: the claim is kept and the pass says
what it is waiting on (`settled "idle" ... with 1 shell, 1 monitor still
running — waiting, not judged this pass`). `--resume` does not re-prompt
such a holder. The shop check drops the row rather than reporting a
condition nobody earned. A herdr that cannot show a screen changes nothing:
every one of those readings fails back to the behaviour above, because
ignorance that an agent is waiting must never hold a genuinely stuck bead
claimed forever.

**A prompt could be typed into a session and never submitted, and every
surface reported it as delivered.**

*Affected: `posse prompt`, the pulse's shop check, and dispatch's
`--resume`. Measured three times on 2026-09-02; on the last, the text sat
in the composer for four hours.*

herdr answers `agent_prompted` for a prompt it typed, whether or not the
submit landed, so the return value was never evidence that a turn started —
and an agent that was never spoken to settles exactly like one that
finished. `posse prompt` now reads the composer back afterwards and says
what is still in it, and warns before typing when the box already holds
somebody's unsent prompt, since the two texts would otherwise go in as one
garbled message. The pulse skips a pane in that state instead of typing
after it, dispatch does not re-prompt one, and the shop check keeps the row
and names it — nobody has actually spoken to that session, which is the one
thing here a human has to fix. None of these is a refusal: the measured
recovery from this state is a hand `posse prompt`, and a gate in front of
it would block the fix along with the mistake.

**One backup archive stamped in the future stopped the schedule, and the
freshness surface called it fresh.**

*Affected: any install with `backup_interval:` armed whose archive directory
holds a file stamped ahead of the box's clock. Not exotic and needs nobody to
do anything wrong: a box whose clock was ahead when an archive was published
and then corrected, a restore or a hand copy of an archive from such a box, or
a laptop whose clock jumped.*

The watch loop's level trigger asked whether the newest archive was younger
than the interval, and a stamp from the future makes that age NEGATIVE —
under every interval there is. So the loop declined, every tick, for as long
as the stamp led the clock, and being a level trigger that declines it said
nothing while it did it. The freshness reading derived from the same stamp,
where negative durations render as `0s`: `posse status` and `posse backup
status` both answered `0s ago`, the freshest reading there is, and no shop
check fired. Measured: a 72h-ahead stamp under a 6h interval is three days
with no archive and no alarm — the exact failure the verb exists to prevent,
with both of its surfaces agreeing that everything was fine.

An archive that cannot be older than now is no longer treated as evidence the
duty was done. Both readers now skip it as a clock and fall through to the
newest archive they *can* date — or to none, which writes one, because an
extra archive costs disk that `backup_keep:` already bounds and a missing one
costs the store. The file itself is never deleted or renamed: it may well be a
good archive wearing a bad time, and `posse backup verify --archive` still
opens it. What changed is that nothing is silent any more. The watch loop
names it on every tick; `posse status` and `posse backup status` name it on
the freshness line beside the real age; an instance whose archives are *all*
undatable reads `NO USABLE ARCHIVE`, is stale, and raises the same shop-check
carry-over as an instance with no archive at all.

**`instance:` freed a session name only from an instance that also set it.**

*Affected: any machine running two posse homes on one herdr server where only
the newer one sets `instance:` — which is the ordering you get by adding a
second home to a machine that has been running one for a while.*

The tag prefixes this home's herdr labels, so the second home's `posse new
smoke` should go out as `work/smoke` and coexist with the first home's
`smoke`. It did — but only if the *other* home was tagged too. Against an
untagged home's bare `smoke` the create still died on `session 'smoke'
already exists`, the exact sentence the key exists to remove, and the
asymmetry ran the wrong way: the untagged home met `work/smoke` and created
happily, so the only home that lost was the one that had been configured.
Worse, a tagged home could not `posse relaunch` its *own* session while an
untagged home held a bare row of that name — and the refusal told the
operator to rename or close that other home's workspace, which every
destructive posse path otherwise refuses to touch.

Both checks asked predicates that read a bare label as this home's own. They
now compare against the label this home would actually write, so a
differently-labelled row — anyone else's — is no longer in the way of either
a create or a relaunch. One thing got stricter with it: a workspace already
wearing this home's *own* rendered label (two homes sharing one tag, or a row
of yours whose session record is gone) used to slip past the create
altogether and leave herdr holding two workspaces under one label. That
refuses now, naming the row by the name `posse list` prints.

**The bd argv gate read your prose as commands, and refused it.**

*Affected: every build before this one, on any box that installed the gate as
a PreToolUse hook. It now refuses less, and says more when it does; nothing it
refused as an invocation is refused any less.*

Appending a lesson to a notes file with `cat >> file <<'EOF'` was refused
whenever a line of the heredoc *body* happened to open with `bd` and a word.
The gate split the whole typed line on newlines — a newline is a list
separator to a shell — so every line of every heredoc body became a segment
with a command word of its own. A sentence of English resolved as an
invocation of a verb that does not exist, on a line whose `argv[0]` was `cat`,
with no bd binary anywhere on it. The same sentence with one word in front of
it passed all along. Two other spellings of the same thing: a body quoting a
command in backticks was refused as "a construct this gate cannot read", and a
body with an apostrophe in it as "unterminated quote" — refusals aimed at a
shell construct, delivered about a sentence. That is the shape that teaches
people to spell around a fence, and it did, twice in one session.

A heredoc body is data now. The parser recognises the redirection in all its
spellings, keeps the body with the command that opened it, and never tokenizes
it — and asks instead the questions a body can honestly answer: does anything
on the line *execute* what it is handed on stdin (`sh <<'EOF'` and
`cat <<'EOF' | sh` are both still refused, and the shell family is the line —
a `python3 - <<'PY'` whose body quotes a command is prose, the same way
`python3 -c` with that text in it always was), is the delimiter unquoted so
the shell expands a substitution in the body before anyone reads it, and is a
real invocation still standing outside the body. Every refusal now names what
was matched — resolved verb, command word, heredoc body — and where, as
segment N of M with its text on one line, so the next false positive is a bug
report rather than a puzzle.

Measured against every Bash command line in this box's transcripts, 51408 of
them: 1085 stop being refused, 7 start — each of those 7 a heredoc whose line
really does carry a shell.

**A red cage pin could mean the image was two days old, and nothing said so.**

*Affected: every build before this one, on `cage: container`. `posse cage`
prints one more line and `posse cage build` stamps what it builds.*

The L1/L3 wall inside a container is rendered by the posse in the IMAGE, so
every change to that render is invisible in the cage until `posse cage build`
runs again. Nothing ever said the image was behind — the only thing that
noticed was a live test, and it noticed by failing. A FAIL is read as a
regression before it is read at all: one instance cost half an hour proving a
red pin was a two-day-old image and not a broken wall, against an image
carrying a posse thirty commits behind the render it was being asked about.

`posse cage` now prints the image's age above everything else it says about
it: which posse the image carries, which one this source (or, outside a
checkout, this binary) is, and whether those are the same build. `posse cage
build` stamps the posse it puts in the image with the checkout it came from,
so the image can answer that at all — a build from a linked git worktree,
which is where every persona works, carries no commit identity of its own and
called itself "dev". And the live pin that used to fail on a stale image now
skips, naming both versions and the rebuild: "the artifact is too old to be
asked" is a third state, not a broken claim. An image that cannot be asked at
all is still read exactly as before — unclear is not stale, because a pin that
skips on a probe failure is a pin that goes green and stays there.

**Persona memory was never committed by anything, so it accumulated on disk
until a human noticed.**

*Affected: every build before this one, in any home that keeps `personas/`
in git. On by default; homes that keep it outside git are unchanged.*

Every persona appends what it learned to its standing orders
(`$RHQ_PERSONA_DIR/ORDERS.md`) and no path in posse ever committed the
result — measured on one instance at 203 uncommitted lines one day, 1419 the
next, 1538 by the time someone landed them by hand. That file is the one
artifact with no other copy: a bead has the queue behind it and code has git,
but a lesson lives in exactly one place until something commits it, and
sessions get reaped by the dozen.

`posse kill` now commits it — by hand, from the cockpit, and from the
end-of-pass auto-reaper — after the workspace closes and before the session's
worktree is landed. The commit is path-limited to the killed session's own
persona directory, so it can take neither another persona's memory nor
`rhq/agents` beside it, and what it is about to add is scanned for credential
shapes first: a hit holds the commit, names the file and line, and never
prints what it matched. It never pushes.

Killing a session by hand also lands the plane first, the way `posse relaunch`
does: one bounded turn asking the agent to write down what it learned, spent
only when that persona actually has memory no commit holds. A turn that does
not settle refuses the kill rather than closing a workspace mid-commit.
`posse kill --no-land` skips the turn (`--timeout` re-bounds it) and still
commits — the flag declines to spend a turn, not to keep the memory. The
cockpit and the auto-reaper take no turn at all, so neither the TUI nor a
dispatch pass can block behind one.

**`posse cockpit` could take a whole CPU core while sitting there displaying,
and cost a fifth of one even when it did not.**

*Affected: every build with the cockpit's periodic scans. Measured at 101.9%
of a core on a loaded box, 26.3% on an idle one.*

The footer's cost reading re-decoded the entire fourteen-day transcript window
every thirty seconds — on the shop this was measured on, 1211 files and 786 MB,
of which 784.6 MB could not have been written since the previous tick — and the
governance block's budget row scanned a second time on its own tick. That is
the standing cost. The runaway on top of it was the timers: each scan was
started unconditionally, so a scan that outran its own period (the governance
check already used 78% of its thirty seconds on an idle box) had the next tick
start a second one over the top of it, then a third.

The cockpit now keeps its scans' results and re-reads only the transcripts
whose bytes have moved, and it will not start a scan while that scan is still
running — a tick that arrives during one is dropped, because these are level
readings and the next tick takes a fresher one than a queued one would have.
Same numbers on screen, same cadence; 0.69% of a core instead of 26.3%.

`posse cost` and the dispatcher's per-launch budget guard are unchanged: they
scan once and keep nothing, exactly as before.

Not fixed here, and still open: the cockpit's two-second redraw actually takes
around ten seconds, because it reads the bead lists on its own event loop. Key
presses can wait behind one.

**A deny rule naming a subcommand's flag — `Bash(bd sync --full:*)`,
`Bash(git push --force:*)` — only refused the flag in one position.**

*Affected: every PID carrying a rule of that shape, on every runtime.*

The PATH shim rendered a two-word rule as a test of the subcommand followed by
a test of the very next word, so any other flag in front of the denied one
moved it out of position and the command ran: `bd sync --push --full` and `git
push --tags --force` both walked past rules written to stop them. A flag has no
position — the parsers these rules describe accept it anywhere after the
subcommand — so the shim now looks for it anywhere in that subcommand's own
arguments, `--flag=value` included.

Two things it still does not treat as the flag: an option's value (`bd sync -m
--full` is a commit message) and an operand after `--`. Where the flag could be
a value the shim cannot rule out, it refuses — a rule may now stop a spelling
that is technically something else, which a respelling gets past.

Rules naming a *short* flag (`-f`) are unchanged and still matched by position;
`posse gates` reports them best-effort rather than claiming them, because a
short flag can hide inside a cluster.

Read this as a fix to *where the flag may sit*, not as a wall around what the
flag does. A flag rule still walls one spelling, and the spellings below are a
floor, not a count: `git push -f`, `--force-with-lease`, `git push origin
+main` and `git push --mirror origin` all force-push and none of them carries
the token `Bash(git push --force:*)` names. `+main` carries no option to spell
at all; under `remote.<remote>.mirror`, `--mirror`'s force-update is what a
*bare* `git push origin` does, with no option and no refspec in the argv
either. Widening the matcher cannot reach any of these. Deny the verb
(`Bash(git push:*)`) wherever the effect is what must not happen; every PID in
`examples/agents` does, and posse's own tests now require it. ADR 0001 says so
in one paragraph.

**`brew install ranger360ai/tap/posse` 404s on any Homebrew older than 6.0.14.**

*Affected: the published v0.4.0 formula. Fixed in the generator; it reaches
deployers when the tap is re-rendered.*

The v0.4.0 formula states no version of its own, so brew scans one out of the
download URL — and that scan is a property of the brew on the box. Homebrew
6.0.14 (2026-07-28) added the parser that reads `0.4.0` out of
`.../releases/download/v0.4.0/…`; every brew before it reads `64` out of
`posse_0.4.0_darwin_arm64.tar.gz` instead. Since v0.4.0 ships bottles, and brew
names a bottle after the formula's version, such a box asks the release for
`posse-64.<tag>.bottle.tar.gz` and the install exits 1 on a 404.

`scripts/tap-formula.sh` now renders an explicit `version` stanza, so nothing
is scanned and no version of brew can get it wrong. On the published v0.4.0
tap the fix is `brew update` — `INSTALL.md` §2 now says so, and says how to
tell before installing.

**`brew install` still needed a developer toolchain on macOS 13 and older.**

*Affected: v0.4.0 on macOS 13 Ventura, 12 Monterey and 11 Big Sur. Fixed in the
generators; it reaches deployers with the next release's bottles and formula.*

v0.4.0 shipped one bottle per architecture tagged `sonoma`, chosen by reading
`HOMEBREW_MACOS_OLDEST_SUPPORTED` (14) as "the oldest macOS Homebrew supports".
It is not: that constant is the oldest macOS Homebrew *builds bottles for*, and
the oldest it *runs on* is `HOMEBREW_MACOS_OLDEST_ALLOWED` (10.15). brew falls
back only downwards — an older bottle pours on a newer macOS, never the reverse
— so every Mac on macOS 13 or older matched no bottle, took the
build-from-source path, and met the same fatal `Your Command Line Tools are too
outdated` gate the entry below says v0.4.0 closed. Measured against the
published tap on Homebrew 6.0.20: `brew fetch --bottle-tag=ventura` — and
`arm64_ventura`, `monterey`, `arm64_monterey`, `big_sur` — each answered
`Bottle for tag … is unavailable`.

The floor is now **11 Big Sur** on both architectures, the oldest macOS arm64
has at all, so every Mac from Big Sur up pours a bottle and none of them needs
Xcode. The asset count is unchanged: it is the same four bottles, tagged lower.
macOS 10.15 Catalina, Intel only, is what is left on the source path — Homebrew
stops supporting it from September 2026 — and `INSTALL.md` §2 now names it
instead of telling a Ventura reader their tap is out of date.

## v0.4.0

### Security

**Two endpoint environment variables handed the account's OAuth token to any
host they named.**

*Affected: v0.3.0 and every earlier build. Fixed in this release (`8a01e01`,
`0ba56cb`).*

The plan guard and the tier preflight each took their HTTP endpoint from an
environment variable — `RHQ_PLAN_USAGE_URL` and `RHQ_MODEL_LIST_URL` — with no
validation of any kind. Setting either to a URL of the caller's choosing made
posse read the account's Claude Code OAuth token and send it as
`Authorization: Bearer …` to that host. The response was then written into
`state/`, so the same override was a cache-poisoning primitive as well as a way
out for the credential: an override's plan numbers became the fact every posse
process on the instance read for the cache TTL, with no credential needed to
put them there.

**Impact is local.** Reaching this requires setting an environment variable in
the environment posse itself runs in; there is no remote or network-only
vector. What it added over reading the credential directly is that the read
happened *inside* the harness, so it left no trace in the refusal log — and it
would have outlived a re-locked credential store, after which posse is the
process permitted to read the item and arbitrary sessions are not.

The fix is `internal/rhq/credpin.go`, one rule for both readers:

- `RHQ_MODEL_LIST_URL` is **deleted**. Nothing read it but the vulnerability —
  tests inject the reader through a struct field.
- `RHQ_PLAN_USAGE_URL` is honoured only when its host is loopback **by name**,
  so a hostname that resolves to `127.0.0.1` is still refused and DNS rebinding
  cannot buy the override back. A refused override is said out loud, never
  silently swapped back for the real endpoint.
- The credential goes only to the compiled-in endpoint URL, **byte for byte**
  (an identity test, not a host test, so a near-miss spelling is an override
  too). A loopback override is asked with **no `Authorization` header** and the
  credential store is not read for it at all: the test seam keeps working,
  uncredentialed.
- An override's answer is never written to the shared plan snapshot, on the
  cooldown path as well as the success path.

**Upgrade guidance.** Upgrade to this release. On an affected build there is no
configuration workaround: not setting these variables yourself does not help,
because anything that can set them in posse's environment can obtain the token.
If either variable has ever been pointed at a host you do not control on an
affected build, treat the account's OAuth token as exposed and reissue it by
signing out of Claude Code and back in.

### Fixed

**`brew install ranger360ai/tap/posse` no longer needs a working developer
toolchain.**

*Affected: v0.3.0 and every earlier build, on macOS only. Fixed in this
release.*

The Homebrew route is advertised as "a release binary, no Go needed", and for
one class of Mac it was the opposite. The formula shipped per-architecture
tarballs and **no bottle**, so brew took its build-from-source path and ran its
*fatal* developer-tools diagnostics before it unpacked anything. On a Mac whose
Command Line Tools are behind the running macOS, the install died with `Your
Command Line Tools are too outdated` — having never read our formula, and
naming Xcode rather than us.

Releases now ship four Homebrew bottles beside the four tarballs, and the
formula carries a `bottle do` block pointing at them. brew pours the prebuilt
keg and never enters that path. A successful install now prints `Pouring
posse-<version>.<tag>.bottle.tar.gz`.

**Upgrade guidance.** Nothing to do if `brew install` already worked for you —
the binary and its contents are unchanged. If it did **not**, `brew update` and
re-run: with a tap carrying this release's formula, stale Command Line Tools
stop mattering on **macOS 14 Sonoma and newer**. brew falls back only
*downwards*, so this release's one tag per architecture covers every macOS
above Sonoma without a new release each time Apple ships one — and covers none
below it. macOS 13 Ventura and older still built from source here, and still
met the gate; that is the `ranger-base-olwk` entry above.

## v0.3.0

First tagged release. See the release notes on GitHub.
