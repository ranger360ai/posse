# bob self-updated under the probe, 60 seconds in — and the probe still measured 2.0.5 (ranger-base-lf6it)

monica's pulse of 2026-10-31 found `bob` on this box at **2.0.7** with
`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js` carrying an mtime of
`00:26` local, against a tree that records 2.0.5 everywhere and an operator
upgrade nobody had typed. Two questions came off that: which version did the
ranger-base-lfk7t skill-discovery probe actually measure, and does bob need a
fleet pin like grok, codex and bd.

This fragment is the measurement half. Every reading below is from this box on
**2026-10-31**, from bob's own logs under `~/.bob/logs/shell` and from the
2.0.7 bundle's own code. Nothing here changed the box: every command was a
read, and `bob` itself was run exactly once, as `bob --version` (see "A gate
can read this safely" below).

## 1. The probe measured 2.0.5. The timeline settles it.

MEASURED, from `~/.bob/logs/shell/bob-shell-20261031T002538.log` (bob logs in
UTC; this box is UTC-4, so subtract four hours for the local times monica's
pulse used):

```
04:25:38.829Z  session starts, cwd <operator scratchpad>/bobprobe/ws   <- the probe
04:25:39.070Z  SkillDirectoryWatcher registers its ten discovery roots
04:25:39.070Z  Auth: "Starting SSO login flow"
04:26:35.896Z  Auth: "Login successful"; TaskStore opened
04:26:38       dist/bob.js, package.json and the package dir are REPLACED  <- 2.0.5 -> 2.0.7
05:05:36.098Z  last line of this session
```

The probe session is the only bob that ran in that window — the three other
bob sessions on 2026-10-31 (`01:04:06`, `01:05:13`, `01:07:33` local) are
richard's ranger-base-ch2rb hook spike, in a `ch2rb.*/bob` workspace, and no
bob had run since 2026-10-04.

So the process that drew the `/skills` listing **started 60 seconds before the
on-disk swap**. node reads `dist/bob.js` once at startup and runs it from
memory; replacing the file underneath a live process does not reload it. The
operator's own `bob --version` reading in the ranger-base-lfk7t comment —
`2.0.5`, `commit: 2dc180906` — is consistent with that and with nothing else:
the binary on disk now answers `2.0.7`, `commit: e1a2a9f87`.

The strongest single line is the watcher registration at `04:25:39.070Z`, 59
seconds before the swap. The thing the probe was measuring IS that root set,
and it was resolved, logged and watched by 2.0.5's code before 2.0.7 existed
on this box.

**ranger-base-6r1rf's `SkillsVerified: "2.0.5"` is correct and needs no
correction.** The fragment ranger-base-6r1rf.md left this "unknown and
deliberately unasserted"; it is now measured, and the conclusion it already
drew is the right one.

## 2. One arm of the probe is re-verified on 2.0.7 for free. Two are not.

The ranger-base-ch2rb sessions ran the 2.0.7 bundle 38 minutes later, and they
logged their own discovery roots. Diffed against the probe's, normalising the
workspace path, the two sets are **identical** — same ten roots, same order:

```
<ws>/.bob/skills            <ws>/.bob/plugins/*/skills
<ws>/.agents/skills         <ws>/.claude/skills
~/.bob/skills               ~/.bob/plugins/*/skills
~/.agents/skills            ~/.agents/plugins/*/skills
~/.claude/skills            ~/.claude/plugins/*/skills
```

So `<cwd>/.bob/plugins/posse/skills/*` — ADR 0007 §6a's whole subject — is
still a discovery root on 2.0.7, and arm c's control root `<cwd>/.bob/skills`
still is too. What this does NOT re-verify is **arm b, the symlink**: whether
2.0.7 still follows a `plugins/posse/skills/<name>` entry that is a symlink to
a dir outside the workspace is a readdir/stat behaviour under the root, not
the root itself, and nothing in these logs speaks to it. Nor does it verify
that a discovered skill is actually LISTED by `/skills` rather than merely
watched.

ranger-base-bs49e is already filed for the operator to re-run all three arms
on 2.0.7; this narrows what it has to establish without replacing it.

## 3. bob self-updates, by default, on every `bob chat` start

MEASURED off the 2.0.7 bundle (`/opt/homebrew/lib/node_modules/bobshell/dist`):

- **The default is ON.** The user-config schema's `bobShell` block is
  `{autoUpdate: !0, lastRunVersion: "", preferredEditor: "", vimMode: !1}`.
  This box's `~/.bob/settings/settings.json` carries **no `autoUpdate` key at
  all**, so the default is what was in force. (It does carry
  `"bobShell": {"lastRunVersion": "2.0.7"}`, written by the 01:07 session.)
- **The trigger is one startup chain,** run at the END of init, after profile
  and auth resolve and `isInitializing` goes false: a "Bob Shell was updated
  to X" notice keyed on `lastRunVersion`, then the legacy-package migration of
  §5, then the update check, then the spawn. That is why the install landed
  2.1 seconds after `Login successful` and not at `04:25:39` — a once-per-run
  check that sits behind sign-in, not a background poll.
- **The check is an unauthenticated public-registry read:**
  `GET https://registry.npmjs.org/@ibm/bob/latest`, 15s timeout, `cache:
  "no-store"`, and it installs whenever that version sorts above the running
  one. There is no ceiling key: nothing in the config says "never above X".
- **The install is a detached child,** `dist/bobshell-release-update-worker.js`
  under `process.execPath`, `detached: true`, serialised on a lock directory at
  `$TMPDIR/bobshell-update.lock` whose `owner.json` carries the pid. Its npm
  arm is, verbatim:

  ```
  npm install --registry=https://registry.npmjs.org/ \
      --allow-scripts=@officecli/officecli --progress=false \
      --loglevel=error -g --force @ibm/bob@<version>
  ```

  It detects npm / pnpm / yarn ownership of the global prefix first — the
  `owningGlobalManager: npm checked 1 locations, owns=true` line in the 2.0.7
  logs — and declines with "install context does not support auto-update" for
  npx, pnpx and local installs.
- **The channel is npm-global, NOT Homebrew.** `brew list bobshell` answers
  `No available formula with the name "bobshell"`; the only bob-adjacent
  formula on this box is `node`, and `npm prefix -g` is `/opt/homebrew`, which
  is the entire reason an npm-global package appears under a Homebrew path.
  **So codex's `brew pin --cask` arm — the one control that refuses to MOVE
  the binary — does not exist for bob.** Nothing was pinned here, and nothing
  could have been.

## 4. Turning it off: one route that works, one that does not, on this box

1. **The user setting.** `"bobShell": {"autoUpdate": false}` in
   `~/.bob/settings/settings.json`. This is the key the check itself reads
   (`Skipping update check. Disabled by user setting or policy`), it is a
   declared schema key so bob's own writes to that file preserve it, and it is
   the whole of the control. **It is the operator's file, so the edit is the
   operator's** — ranger-base-zneyv carries it.
2. **The enterprise policy.** A policy named `UpdateMode`, config path
   `bobShell.autoUpdate`, values `default` / `start` / `manual` / `none`, whose
   POLICY-tier default is `none`. Its POSIX source is `/etc/bob/policy.json`,
   keyed by policy name — but **on darwin that file is not read**: MEASURED
   across all nine bob session logs on this box, bob logs `Starting native
   policy watcher for product "com.ibm.bob"` nine times and
   `Reading policy file: /etc/bob/policy.json` zero times, `/etc/bob` does not
   exist, and `defaults read com.ibm.bob` answers `Domain com.ibm.bob does not
   exist`. The darwin route is a managed-preferences domain, i.e. an MDM
   payload — this box has `/Library/Managed Preferences/<user>/` with an
   MDM `complete.plist` in it, so the route is real but it is MDM-delivered and
   not a file anyone edits by hand.
3. **There is no env var.** The bundle's `BOB_*`/`BOBSHELL_*` names are
   `BOB_SESSION`, `BOB_SUPERVISED`, `BOB_USE_MODEL_ENV` and three telemetry
   keys. None of them touches updates.

## 5. A package rename is armed, and it fires on the next quiet `bob chat`

The 2.0.7 bundle carries `dist/bobshell-legacy-migration-worker.js` and a
migration from the `bobshell` global package to **`@ibm/bob`** — which is
already the name the update worker installs. All three 2.0.7 sessions tried it
and all three failed the same way:

```
LegacyMigrationBlockedError: Bob Shell migration is blocked by running
instances: 23267
```

pid 23267 was the probe session, still alive until 01:05. So the migration is
pending, not declined: `~/.bob/settings/bobshell-migration/2.0.7/` exists and
is empty, and the next `bob chat` that starts with no other bob running will
move the global install off `bobshell`.

**Consequence for anything written today:** a check keyed on the npm package
name `bobshell`, or on the path
`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js` that monica's pulse read
the mtime of, is already half-stale. `bob --version` and `command -v bob` are
the readings that survive the rename.

**And it is a reason not to start `bob chat` casually from a seat.** Blast
radius of one `bob chat` on this box today: it replaces the global bob package
(`bobshell` → `@ibm/bob`) and, with `autoUpdate` still at its default, installs
whatever `@ibm/bob@latest` is at that moment.

## 6. A gate can read this safely

MEASURED: `bob --version` wrote no session log, left
`~/.bob/settings/settings.json` at its 01:07 mtime, left
`~/.bob/settings/bobshell-migration/` at its 01:04 mtime, and left the package
dir at 00:26:38. It prints two lines — `2.0.7` and `commit: e1a2a9f87` — and
exits. The update chain hangs off the TUI's init, not off the version flag, so
the one reading a pin would need is a read.

(Also noted, since the lane's rule is names and locations only: the live IBM
tokens sit in `~/.bob/settings/auth-secrets.json`, which is the known fourth
credential path of ranger-base-prjck, already pinned in
`internal/posse/seatbelt.go` and `seatbeltcredentialread_qa_test.go`. Nothing
from it is reproduced here or on the bead.)

## 7. What the drift actually costs today

The tree names bob 2.0.5 **99 times across 31 tracked files** — ADR 0007, ADR
0060, ADR 0062, nine notes fragments, the herdr-bob detection captures and
their four upstream write-ups, and eleven Go files including `runtime.go`,
`personamode.go`, `interstitial.go` and `skills.go`. Every one of those is a
measurement, and as of 00:26:38 none of them is about the binary this box runs.

Against that, two things limit the damage:

- **bob is not on the dispatch path.** `posse runtime check bob` prints the
  grid but a launch refuses — there is no herdr 0.8.2 `bob` kind
  (ranger-base-lfk7t §1), so no lane dispatches onto bob. An un-re-audited bob
  cannot break a running seat; it can only make documentation wrong.
- **The detector already exists and already says the right thing.**
  `skillsVerifiedDrift` in `internal/posse/parity.go` compares
  `skills_verified:` against `ProbeCLIVersion`'s live read, and
  `posse runtime check bob` prints, today, unprompted:

  ```
  drift: skills_verified: "2.0.5" but bob reports "2.0.7" — re-measure live
  discovery and update skills_verified: (ADR 0007 §6c)
  ```

  It is exactly the finding monica's pulse made by reading an mtime. **It is
  just not on a clock.** `posse runtime check bob` exits 0 — §6c reports and
  never refuses, by design — and nothing in `scripts/verify-box.sh`'s roster
  runs it, so the line sat there correct and unread.

## 8. The proposal

The standing order is the smallest thing that fails loud. What "a fleet pin
like grok/codex/bd" costs here, MEASURED on this tree: those three pins are
**5,241 lines over nine files** — 1,420 of script, 458 of `version-pin.toml`
declaration, 3,363 of `internal/treepins/*pin_qa_test.go` — and a fourth would
also have to be named in the Makefile target list and `.PHONY`, in
`verify-box.sh`'s ROSTER or EXCLUDED table (pinned by
`internal/treepins/boxcheck_qa_test.go`), and in
`internal/posse/verifybox_qa_test.go`. And its best arm would be missing
anyway: there is no cask to `brew pin` (§3) and no ceiling key to assert (§3),
so a `verify-bob-pin.sh` would be the codex script with its strongest row cut
out — down to "is `bob --version` still what the declaration says, and is
`autoUpdate` still false".

So, in increasing cost:

- **A. Record it as floating, and stop there.** This fragment plus §4's route
  is the record. Costs nothing, catches nothing: the next self-update is found
  by the next pulse, if someone reads an mtime again.
- **B. Put the drift reading that already exists on the clock.** §7's
  `skillsVerifiedDrift` is one function with one seam; a third reader of it
  that exits nonzero, rostered in `verify-box.sh`, reddens the day bob moves —
  and does the same for codex and grok on the same mechanism. It needs no live
  change to the box, no operator ruling and no new declaration file. It does
  need an exit code rather than prose: `posse runtime check` has no `--json`,
  and grepping its output for `drift:` is precisely the forked-matcher shape
  these scripts were rewritten to get rid of (ranger-base-s8b4g — a reader
  that answers nothing must not be indistinguishable from "nothing to
  report"). That makes the exit-code surface a `-l code` item, not a devops
  one. **This is the recommendation.**
- **C. B, plus `autoUpdate: false`, plus a declaration to verify it against.**
  The only version of this that actually refuses to move the binary. It is the
  operator's config file and therefore the operator's call
  (ranger-base-zneyv), and it buys the 99 measurements of §7 a version that
  holds still. Its cost is a fourth pin with its best arm missing, and its own
  accepted risk to state every run: a bob above the declaration still starts,
  because nothing on this runtime refuses to run one.

B and C are not exclusive and C contains B. The ordering that keeps the shop
honest is B first — it is the half that needs nobody's permission — and C if
the operator wants the coupling in §7 to stop drifting rather than merely
report.

## Filed, not fixed

- **ranger-base-zneyv** (operator, `-l question`) — the `autoUpdate` decision:
  the exact edit, its blast radius, and what "verified" means.
- **ranger-base-f5jbs** (`-l code`) — option B's exit-code surface for
  `skillsVerifiedDrift`, so a version drift can red a clocked check.
- **ranger-base-bs49e** (operator, `-l question`, filed by ranger-base-6r1rf)
  — re-run the three discovery arms on 2.0.7. §2 narrows it to arm b and to
  the listing itself.
