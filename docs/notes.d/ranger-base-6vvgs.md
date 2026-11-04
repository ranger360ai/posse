## A moved HOME moves the module cache, and a package built against an empty one (ranger-base-6vvgs)

The ci-red gate filed two consecutive reds on `main`, 2026-11-04. They were
two independent failures on two platforms, and only the second one was ours
to find.

### (a) ubuntu: the pin that had no positional parameters

Both red runs failed `TestQAHookSkipRefusesTheSystemScopeHideDirection` on
ubuntu-latest, arm 1 — four hide legs reading "NOT refused" with no output at
all, and no `refusals.log`. Diagnosed and fixed under ranger-base-zvg7q
before this bead was claimed (`e30e6b73`): the pin dot-sourced the arm with
positional arguments, which bash honours and dash ignores, so the arm saw
`$# = 0` and returned at its own guard. Re-verified here by execution rather
than by reading, against `/bin/dash` on this darwin box, MEASURED 2026-11-04:

    pre-fix form, /bin/dash   refused=false, output ""      (= what ubuntu printed)
    pre-fix form, /bin/sh     refused=true, rule named      (= what darwin printed)
    post-fix form, dash shebang  3 controls allowed, 4 legs refused, 4 log lines

The contrast in the middle row is the whole of why it landed green: the fix
had to be checked on the shell that ignored the arguments, not on the one
that honoured them.

### (b) macos: the module cache that moves with HOME

Run 41811904569 (macos-latest, arm 1) failed
`TestPosseSupportsBdWriteOnARealBinary` 49.44s in, with `go build
./cmd/posse` dying on three modules:

    backupvolume_darwin.go:8:2: golang.org/x/sys@v0.47.0: … lookup proxy.golang.org: no such host
    refresh.go:50:2:            golang.org/x/term@v0.45.0: … lookup proxy.golang.org: i/o timeout
    credential.go:59:2:         golang.org/x/text@v0.43.0: … lookup proxy.golang.org: no such host

It reads as a resolver flake, and the resolver did fail. What it is NOT is a
run that happened to need the network: **that build needed the network on
every run, on both platforms, and had done since the temp home landed.**

`internal/posse`'s `TestMain` moves `HOME` to a temp dir for the whole test
binary (ADR 0047 D1, ranger-base-aupee — one home for 2718 parallel tests
instead of a `t.Setenv` that forbids `t.Parallel`). `GOMODCACHE` defaults to
`$GOPATH/pkg/mod` and so to `$HOME/go/pkg/mod`, and `GOCACHE` to the user
cache dir, also under `$HOME`. So both move with it, and the two `go build`
shellouts in this package — `buildBdWritePosse` and the `cagePosseBuildArgv`
call in `cagestale_test.go` — build `cmd/posse` against an **empty module
cache and an empty build cache**, with the runner's own populated caches
sitting right there under its real home. `cmd/posse` is the only tree in this
repo that imports `golang.org/x/{sys,term,text}`, which is why the three
modules named above are the three it fetched, and why `go vet ./...` in the
same job — run before `HOME` moved anywhere — downloaded nothing.

MEASURED 2026-11-04, darwin/arm64, `GOPROXY=off` so any reach for the network
is a failure rather than a delay:

    HOME=<temp>                                   go build ./cmd/posse  FAILS
    HOME=<temp> GOMODCACHE=<real> GOCACHE=<real>  go build ./cmd/posse  exit 0

and at the test level, the same two states, also offline:

    pre-fix  TestPosseSupportsBdWriteOnARealBinary  FAIL — the same three
             modules, the same three source lines as run 41811904569
    post-fix PASS, 2.60s   (against 49.44s on the CI runner's cold caches)

`TestQAGoCachesDoNotFollowTheTempHome` is the cheap guard the expensive
symptom did not have: it reads env, never the network, and it reds in
milliseconds where the regression it catches reds after a 49s build, on one
platform, in one arm, and only when DNS blinks. MUTATION-CHECKED — with the
two `TestMain` lines removed it reds naming both variables, and it is green
with them restored.

The fix is three lines in `TestMain`: read `go env GOMODCACHE GOCACHE` while
the real home is still visible, and export both before `HOME` is replaced. It
is set once in `TestMain` rather than on each `exec.Command`, so the next `go`
shellout a test adds is covered by construction; a `go` that cannot answer
leaves both alone, which is today's behaviour and fails at the build, loudly,
rather than quietly here.

### What this cost, and what it did not

Nothing was unverified: the suite's subject was never the module cache, and
every arm that was green was green on its merits. What a red gate costs here
is **attribution** — `ci.yml` is the only gate a commit on `main` ever passes,
because merge-back fast-forwards a bead branch and opens no PR. For two runs,
a red on `main` said nothing about the commit it was attached to, and the
macos half of it was a build that would have reddened on any commit the moment
the runner's DNS blinked.

### The two reds that were not this, and the merge-back

The first full `make test` here was red, 11 tests, and none of them either of
the above: `TestDeriveIdentityLiteralsSkipsAbsentSourcesSilently` plus ten
`TestQAHookFreshness*`. Both sets reproduce with this bead's changes removed,
and both go green the moment the seat's pinned git-config inlet is shed —
`GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM`, pinned into every persona seat
by ranger-base-obdnk. A fixture's `t.Setenv("HOME", t.TempDir())` does not
hide the operator's real git config, because git reads `GIT_CONFIG_GLOBAL`
whatever `HOME` says. So those 11 were green on CI, which carries no launch
pin, and red on every seat on this box — the mirror image of (b) above, which
was red on CI and green on a seat.

Nothing was filed for them: the lane that owned it landed it while this bead
ran, and `main` moved from `e30e6b73` to `fe39111a` (ranger-base-loi28, "name
git's config scopes in the fixtures that scrub HOME"), touching exactly the
two files those reds live in. This worktree was fast-forwarded onto it and the
suite re-run from there: `make test` exit 0, all three arms, no FAIL line, in
the seat's own environment with the inlet pinned as a seat has it.

### The push

The clearing push is not a session's to make. At the close of this bead
`origin/main` was still `bdd3954b`, the red tip; `e30e6b73`, `fe39111a` and
this commit were local. The operator pushes.
