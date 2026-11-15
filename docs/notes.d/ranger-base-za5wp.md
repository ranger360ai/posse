# ranger-base-za5wp — /tmp becomes a git repo for 10ms, and a trust key moves under a test

`TestEveryLaunchPathSeedsDirectoryTrust` red once under full-arm load
(ranger-base-wsgus's worktree, 2026-11-14) with the trust entry sitting in the
config under the session dir's own path and `ClaudeTrusted(cfg, dir)` still
reading false:

    --- FAIL: TestEveryLaunchPathSeedsDirectoryTrust (0.27s)
        trust_test.go:754: posse new launched into an unanswered trust dialog:
        map[hasSeenAutoModeOutsideReadPrompt:true projects:map[
          /tmp/TestEveryLaunchPathSeedsDirectoryTrust1823738735/005:map[
            hasCompletedProjectOnboarding:true hasTrustDialogAccepted:true]]]

Green alone, green on a re-run of the same arm. The printed state is the
state a PASSING run writes — `/005` is this test's own `dir` (MEASURED: the
fifth `t.TempDir()` of this test's setup, after the backend's home, its fake
dir, `hermetic`'s plan-usage dir and the config's dir) — so the seed did its
job and the READ asked for a different key.

## The mechanism, measured

`ClaudeTrustKey` is `gitRootOf` plus claude's canonical-root hop: the key is
the NEAREST ancestor holding a `.git`, else the dir. The test's dir had no
`.git`, so its key was itself — and the stops above it are the test's own
temp root, then `/tmp`, then `/`. Two of those three are shared with every
other process on the box.

MEASURED 2026-11-15, this box (Linux 7.2.15-1-omarchy, 16 cores, go1.27.3),
worktree `posse/jian-yang-posse-ranger-base-za5wp`, two experiments run
alongside `make test-arm2`:

1. **The key moves.** A probe holding one fixed `t.TempDir()` and recomputing
   its key every millisecond for 90s:

       key moved after 57514 reads: got "/tmp"
         want "/tmp/TestZZProbeKeyStability1938892353/001" (gitroot="/tmp")

2. **What moves it.** A 10ms poller over `/tmp/.git`, `/.git` and every
   `/tmp/Test*/.git` caught it twice, in two separate arm2 runs — at
   03:18:04.660 and at 03:24:27.206:

       03:24:27.206 FOUND /tmp/.git
         entries: []
         /tmp touched in the last 5s: [(0.01, '.agents'), (0.01, '.aws'),
           (0.01, '.codex'), (0.01, '.git'),
           (0.01, 'codex-bwrap-synthetic-mount-targets-1001'), ...]
         processes with cwd=/tmp: []
       03:24:27.217 GONE /tmp/.git

   An EMPTY `/tmp/.git`, directory, 0755, created and removed inside ~11ms,
   beside `/tmp/.aws`, `/tmp/.codex` and `/tmp/.agents` and a touch of
   `/tmp/codex-bwrap-synthetic-mount-targets-1001` (whose `lock` file dates
   from 2026-11-09). That is a codex sandbox's synthetic mount-target prep —
   the string appears nowhere in this tree, so it is the codex CLI's own,
   fired by whatever codex seat or live test starts a sandbox, and nothing
   this suite can call off.

While that directory exists, `gitEntryPresent("/tmp/.git")` is true, so
`gitRootOf` answers `/tmp` for EVERY path under it and `claudeMainRepoOf`
returns it unchanged (a `.git` directory fails the `gitdir:` read with
EISDIR). Every trust key in the test binary is `/tmp` for those 11ms. A seed
that writes the dir's key and an assertion that reads it a moment later then
disagree, and the red reads exactly like a launch that landed in the trust
dialog — a dead seat — which is why this was worth a bead rather than a
retry.

This is not a posse defect: a real claude launched in `/tmp/x` during that
window would key on `/tmp` too, and no fleet session dir is under `/tmp`.

## The fix here

The session dir gets its own empty `.git` before the launch. The nearest
`.git` is what decides, so one the test plants cannot be outvoted by one the
box plants above it, and the key is pinned to the dir at the write and at the
read alike.

MEASURED, same box, the three fixtures side by side: a bare dir, a dir with a
planted empty `.git`, and a dir with a real `git init`. All three launch and
all three seed. After the launch the planted-`.git` dir holds that `.git` and
nothing else — posse's wall machinery asks git, and an empty `.git` is not a
repo to git — while the real repo collects a `prepare-commit-msg` and a
`pre-push`. So the empty plant buys key stability and changes nothing else
about what the test measures; a real repo would have added the whole
commit-wall path to a test whose subject is the seed.

Both assertions now print the key, the nearest `.git` and the config, so the
next red of this shape names itself instead of looking like a dead seat.

## The fix, measured against the hijack

Waiting for a codex seat to start a sandbox is not a test, so the window was
reproduced in an ISOLATED temp root — `GOTMPDIR`, which is what cmd/go hands
the test binary as its `TMPDIR` (`cmd/coldexec/room_linux.go` says so, and it
is why setting `TMPDIR` alone changes nothing) — with a flicker holding
`<root>/.git` for ~11ms every ~60ms. Nothing outside that root sees it, which
matters: planting a real `/tmp/.git` on this box would red every other seat's
trust assertions, which is the whole finding.

MEASURED 2026-11-15, same box, one probe running the bead's test body twice
per iteration, shielded and unshielded, `-count=30` under that flicker:

    unshielded reds: 15 / 30
    shielded   reds:  0 / 30

and the unshielded red is the bead's red, word for word — the seed's key is
the flicker root, the assertion's key is the dir:

    unshielded: NOT trusted — dir=<root>/tmp/Test…unshielded2325159389/005
      key=<root>/tmp/Test…unshielded2325159389/005 nearest .git=""
      config=map[hasSeenAutoModeOutsideReadPrompt:true projects:map[
        <root>:map[hasCompletedProjectOnboarding:true
                   hasTrustDialogAccepted:true]]]

The gap is what makes a launch test the canary for this class: the seed and
the assertion are ~70ms apart with a ~11ms window between them, so a straddle
is likely rather than rare. A `SeedClaudeTrust` call and an assertion
microseconds apart are nearly always inside or outside one window together.

## Not fixed here

The window is open for every OTHER assertion that reads a claude trust key
over a `/tmp` temp dir, and there are around twenty, across
`trust_test.go`, `trust_qa_test.go`, `trustlock_qa_test.go`,
`outsideread_test.go`, `runtimehome_qa_test.go`, `atomicreplace_qa_test.go`
and the two `configdirfence_test.go`. The exposure per site is one instant
against an ~11ms window, so each is about as flaky as this one was — and the
arms whose SUBJECT is "a dir in no repo keys on itself" cannot take this
fix, because the plant is the thing they are measuring the absence of. That
sweep, and whatever precondition a no-repo arm needs instead, is
ranger-base-tw0mn.
