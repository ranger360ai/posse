# The launcher knew the path and not its kind (ranger-base-ppnds)

Three findings from holden's verify of ranger-base-vnp01 — two in ADR 0080
D17's render (`BwrapArgs`, ranger-base-qoamy) and one in D19's credential arm
(ranger-base-tc41o). Built 2026-11-08.

Nothing calls `BwrapArgs` or `BwrapStateDirs` yet, so none of this was a
dispatch defect; finding 1 becomes one on the day D18 (ranger-base-djl3c)
wires the render to a launch line, which is why it is fixed before that lands
rather than after.

## 1. A `mkdir -p` at a path the carve-out means as a file

Shape 2's missing-deny arm emitted `PrepMkdirAll` for any absent `Deny` entry
inside a write grant. Several `Deny` entries are FILE paths that are absent on
every healthy repo: `<gitdir>/config.lock` (`sessionGitConfigFiles`),
`<gitdir>/config.worktree` and `config.worktree.lock`
(`sessionWorktreeIdentityFiles`), and the beads store's own `.git/config.lock`
when a redirect puts the store in another repo (granted whole by
`beadsGitDirs`). Every one of them sits inside a grant, so the arm fired on
each.

The steps run on the HOST, outside the namespace, in the SHARED repo, in the
seat's name, and nothing cleans them up. A session cannot clear one either:
the path is behind the wall that denies it. What each one costs, MEASURED by
holden by hand against a real repo at git 2.50.1, each with a green control
first:

| planted as a directory | what breaks |
|---|---|
| `.git/config.lock` | `git config` dies: `error: could not lock config file .git/config: File exists`. commit and status unaffected, so it is found late and far from here. |
| `config.worktree` | inert while `extensions.worktreeConfig` is unset; with it set — git sets it itself for `sparse-checkout` — `git status`, `git config --list` and `git commit` all exit 128 at `fatal: unknown error occurred while reading the configuration files`. |
| `config.worktree.lock` | `git config --worktree` refused. |
| `<ref>.lock` (shape 1) | `git commit` exit 128, `fatal: cannot lock ref 'HEAD'` — the failure `sessionRefDirs`' own comment describes, manufactured by the launcher. |
| `packed-refs.lock` (shape 1) | `git pack-refs` and `git gc` exit 128; `git commit` prints the error and exits 0. AGENTS.md makes a stray one the operator's to remove and no persona's, so posse would create a thing no persona may clear. |

**The ADR is not wrong about this, and that is the point.** D17 item 3's letter
says `mkdir -p` with no kind distinction, because SBPL never needed one:
`(deny file-write* (subpath "<p>"))` is the same statement whether `<p>` is a
file, a directory or absent, and the profile says nothing at all about paths
that are not there. A MOUNT has to be made of something — bwrap can only bind
a source that exists — so the kind becomes a thing the launcher writes. What
was wrong is D17's *As built* residual, which took on exactly this class ("a
missing grant is not free here") and then enumerated it as four darwin-only
literals under `/private/tmp` and the home: it missed every git lock path,
none of which is darwin-only and two of which damage the shared repo rather
than the session.

### The fix is a channel, because the render cannot work the kind out

An absent path's shape is exactly what cannot be stat'd, and the render must
not guess. The producers know — `<gitdir>/config.lock` is a lockfile by
construction — so `SeatbeltCarveOut` carries `DenyFile`, the subset of `Deny`
its producer means as a file, with a `FileShaped(p)` predicate and "not
marked" meaning a directory. Three producers fill it: the two git ones above,
and `HomeConstitutionFiles` (`config.yaml` and the promote manifest — never
reachable through the missing-deny arm today, because ADR 0015 §2 keeps that
area out of every grant, but "unreachable" is a property of another function
and the kind is a property of the path).

An absent file-shaped deny then renders **no mount and no prep**.

**`touch` is not the other half of the fix.** An empty FILE at `config.lock`
is the same `File exists` refusal the directory buys, so there is no kind that
is safe to create there.

**Residual, stated rather than closed**, because a mount cannot say "this name
may not come into being": the session CAN create those files inside the grant
that contains them, and no wall is over them when it does. What it still
cannot do is make one take effect. The file each lock would be renamed onto —
`config`, and `config.worktree` where it exists — is itself bound read-only,
and B8 measured that a rename cannot cross a mount point, so the refusal lands
one step LATER than at the SBPL mechanism (`could not write config file` where
seatbelt says `could not lock`, both arms measured in
`seatbeltgitidentity_qa_test.go`) and the attacker-chosen config reaches the
lock and never the config. What remains is a stray lockfile in the shared repo
that an operator's next `git config` trips over, and an inert
`config.worktree` that goes live only if something first writes the extension
into a config this wall denies. Both are smaller than a directory the launcher
makes unconditionally at every launch.

### Shape 1 still guesses, and that is a difference in what producers can say

A file-shaped GRANT comes from a `state_dir:` declaration, and ADR 0012 D4's
key says "directories (and single files)" without saying which: nothing but a
stat can tell, and the path is not there. The live members are the two beads'
— claude's `~/.claude.json` (D19) and git's ref leaf, its `.lock` and
`packed-refs.lock` (D20) — and the exists-as-a-file arm already refuses them
loudly on any box that HAS run the CLI. The next file-shaped grant after those
reopens it; what closes it is that grant's producer saying so, the way the
deny half now does.

## 2. A Linux claude seat's own credential, behind a spelling

`credentialReadDenyLiterals`' `own()` decided whether a credential store
belongs to THIS seat's runtime by string equality against the declared
`state_dir:` literal. `state_dir:` REPLACES the built-in list rather than
merging (`runtime.go`, ADR 0021 D1), so what arrives is whatever this box's
operator typed — and D19's new conjunct subjects claude to that test off
darwin, where the file IS the store of record and not a keychain fallback (ADR
0019 D2). MEASURED, `credentialReadDenyLiterals("linux", <state_dir>)`:

| declared | before |
|---|---|
| `["~/.claude" "~/.claude.json"]` | spared (the built-in) |
| `[<config dir> "~/.claude"]` | spared (`BwrapStateDirs`' output) |
| `[<home>/.claude]` | **DENIED** — the same directory, spelled absolutely |
| `["~/.claude/"]` | **DENIED** — one trailing slash |
| `[/srv/claude-state]` | **DENIED** — an ADR 0021 D1 overlay |

A seat denied its own credential cannot authenticate itself, which is the one
thing that function's own comment says a runtime must never be. Half of the
shape is inherited: the three sibling runtimes have had the same literal
coupling since they existed. What D19 newly did is subject claude to it off
darwin, so the one runtime whose credential file is the store of record on that
OS became the one that gets stranded.

`own()` asks containment now — does any file of this store lie under a
directory the launch declared — through `underDir`, which resolves both sides
and is what every other containment question in that file already uses, with
`ExpandTilde` first because the declared value is a `~/…` literal in every
built-in. One line, all four runtimes.

Two things deliberately unchanged:

- **Whole-store, not per-file.** Ownership is a fact about the RUNTIME, so a
  seat that owns the store is spared every spelling of it, including the home
  copy of whatever the CLI wrote before a variable moved the write (ADR 0019
  D2 store 3). That is also what keeps ranger-base-b52r3's residual where it
  was: a codex seat declaring `~/.codex` on a box exporting a moved
  `CODEX_HOME` is spared BOTH spellings, because the declared one is owned.
- **The overlay row stays denied**, and that is the answer rather than a
  residual. `SeatbeltWritable` grants the declared state dirs and nothing
  else, so on `[/srv/claude-state]` nothing grants `~/.claude` either — the
  wall and the grant agree about which directory is this seat's state. An
  operator who moves claude's state moves `CLAUDE_CONFIG_DIR` with it, and
  that row (the config dir declared, the tree dropped) is spared both
  spellings.

A declared state dir wide enough to contain ANOTHER runtime's store now spares
that store too — `state_dir: ["~"]` is the shape. Not a hole this wall can
close: the same declaration grants that directory WRITE, so a read deny inside
it is theatre and the breach is the grant. `posse runtime check` is where an
overlay that wide gets caught.

It also takes the sharp edge off the coupling ranger-base-tc41o pinned. D19
keeps `~/.claude` in claude's list and should go on keeping it — the home copy
is still there — but a list that had lost the tree no longer reads as "not a
claude seat" when the config dir is declared.

## 3. The producer-driven arm's cwd was not a git repo

`TestBwrapArgsRendersTheTiersOwnLists` is the one arm in
`bwrapargs_qa_test.go` driven from `SeatbeltWritable`/`SeatbeltCarveOut`
rather than a hand-built list, and its cwd was
`bwTree(t, []string{"proj", …})` — a fixture DIRECTORY. So `gitPath` failed, `LinkedGitDirs` returned nothing,
and `sessionGitConfigFiles`, `sessionWorktreeIdentityFiles` and
`sessionGitGrants` all returned nil: **not one file-shaped deny and not one
git grant reached the render in any of the file's 18 arms.** That is why
finding 1 escaped. The arm's prep assertion was
`writeGranted(writable, s.Path)`, which asks whether a step is inside a grant
and never whether a `mkdir -p` belongs at that path, so it stayed green over
all five of the directories above.

It is a `git init` plus a `git worktree add` now — the shape every dispatched
session has, slash in the branch name and all — with three assertions the old
arm could not make:

- the kind of every prep step against what its producer means the path to be
  (no `mkdir -p` at a `FileShaped` path; no `touch` at a path nothing
  declared, the Keep files aside);
- a POSITIVE WITNESS that a file-shaped deny **under this session's git dirs**
  is both absent and inside a grant, so the assertions above are not satisfied
  by a render that was handed no file-shaped deny at all — which is the state
  the arm was in for two beads. It is scoped to the git dirs on purpose: the
  fixture home sits under TMPDIR, which the writable set grants, so
  `config.yaml` and the promote manifest are witnesses too and would go on
  satisfying a bare count the day the git half stopped arriving;
- what the real lists do TODAY. They REFUSE, naming the branch ref leaf, which
  is shape 1's B8 refusal and is what makes D20 a prerequisite of a launch
  rather than an improvement to one. The deny half is then read with D20's
  narrowing applied as a transform of the real list (drop the leaf, its
  `.lock` and `<common>/packed-refs.lock`; bind the ref's parent directory) —
  holden's REPRO (b) as a pin. The day D20 lands, the first half goes red and
  the transform becomes a no-op.

Plus two focused arms with no git in them:
`TestBwrapArgsRendersNothingForAMissingFileShapedDeny` (both kinds at one
path, in one table, because each row alone is satisfied by a renderer that is
wrong in the other direction) and
`TestBwrapArgsFileShapedOnlySpeaksForTheDenyList` (marking a grant, a Keep
file or a read mask moves the render not at all — `DenyFile` is a statement
about `Deny` and about no other list).

## Verified

- MUTATION-CHECKED, each independently: removing the `FileShaped` skip in
  `BwrapArgs` reds the focused file row and the producer-driven arm; dropping
  the `DenyFile` marking from the two git producers reds the producer-driven
  arm's witness; restoring `own()` to string equality against `"~/.claude"`
  and the sibling literals reds four rows of the new containment table and
  leaves its three controls green.
- `make fmt-check`, `make adr-check`, `make doc-check`, `make notes-check`,
  `make tree-check`, and `make test`'s three arms — numbers on the bead.
- Environment: darwin 25.6.0 arm64, git 2.50.1 for the repro table (holden's,
  reproduced here only as the fixture the pins drive).
