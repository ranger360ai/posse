# This repo's own land check: what it costs, and the three things D6 left to the build (ranger-base-pn621)

ADR 0087 D1 says a repo opts in to the replay gate by shipping an executable at
`scripts/posse-land-check`, and D6 says what this repo's copy does. The gate
itself landed under ranger-base-ej47m; this is the script, measured.

Dated 2026-11-11. Every number below was taken on the D1 host (Linux 7.2.15,
16 cores, `go version go1.27.3-X:nodwarf5 linux/amd64`, `GOTOOLCHAIN=local`),
from this bead's worktree.

## The body, and that it is D6's

`scripts/posse-land-check --plan` prints what it would run and runs nothing,
which is both how the plan below is read back and how the arm derivation is
witnessed without paying for five compiles. In this tree today:

```
step 1: go build ./...
step 2: go vet ./...
arms declared here: posse_arm2 posse_arm3
step 3: go vet -tags posse_arm2 ./internal/posse
step 4: go vet -tags posse_arm3 ./internal/posse
step 5: GOOS=darwin GOARCH=amd64 go vet ./...
```

That is D6's body verbatim. Nothing in it executes a program the branch wrote:
no `go test`, no `make`, no `go generate`, no script out of the tree being
judged. The toolchain is the only thing that runs, and `GOTOOLCHAIN=local`
keeps a go.mod from making it fetch a different one.

## What it costs (MEASURED 2026-11-11)

The script was run against a tree exported to a plain directory with
`git archive | tar -x`, with the shared `GOCACHE` warm from the fleet's own
builds — which is the state a replay actually finds, and which also proves the
check needs no git in the tree it judges.

| what | loadavg | wall |
|---|---|---|
| warm, 10d046f9, 20 commits back | 3.4 | 7.1s |
| warm, HEAD~3, 9 go files differ from HEAD | 13.8 | 7.4s |
| warm, HEAD~8, 33 go files differ | 13.5 | 7.4s |
| warm, HEAD~20, 60 go files differ | 13.8 | 8.7s |
| warm, this worktree, twice, nothing to recompile | 3.4 | 1-2s |
| cold, `GOCACHE` emptied | 7.4 → 15.8 | 30.1s |

Cold per step, on a second empty cache at loadavg 16.8 → 19.5: `go build
./...` 7.9s, `go vet ./...` 2.6s, arm2 2.9s, arm3 2.1s, `GOOS=darwin` 8.5s.

**Against ADR 0087's budget of ~6s warm and ~16s cold, warm holds.** It is 7s
rather than 6s because the ADR timed its four steps one after another over a
cache that was warm for all of them, and a replay's tree has files in it that
nothing has compiled. The cold figure is the ADR's four steps plus the darwin
vet the ADR measured separately (17.7s first-ever there, 8.5s here), so it is
the same price in a different arrangement; and a cold cache is the first replay
after a toolchain change or a cache prune, not a per-landing cost.

The bead set a ~30s **warm** ceiling at which the lock-time price in the ADR
stops holding and this goes back to architecture. Nothing warm measured above
8.7s, and that at a loadavg of 13.8 on a 16-core box, so the ADR's cost
paragraph stands and no handback was filed.

## It would have caught the incident

The verification the bead asked for, run the way the gate runs it: the BASE's
copy of the script (this worktree's) with the tree to judge as its cwd. Both
trees were exported with `git archive`, and neither ships the check — which is
the point, since a branch that edits the script does not change what judges it.

- **a2327230** (holden's repro, the replayed tip): **exit 1 in 297ms**, with
  `go build ./...` naming both errors the ADR cites —
  `internal/posse/panework.go:375:26: cannot use target (variable of struct
  type Seat) as string value in argument to b.capturedHold` and
  `internal/posse/panework.go:483:30: undefined: paneSeat`. Those two lines
  plus the `# github.com/ranger360ai/posse/internal/posse` header above them
  are exactly the last three non-empty lines, which is what `landCheckSaid`
  keeps and what a merge-back bead would carry.
- **10d046f9** (its parent, ranger-base-r5131's tip): **exit 0**, green in
  7.1s.

So the night of a2327230 becomes one merge-back bead, which is what the ADR's
first Consequence claims.

One note on D6's reasoning while here: at a2327230 `go vet ./...` reports the
same two non-test errors and stops, because the package does not type-check at
all. The seven `go test -c` errors D6 credits vet with catching are behind
those two. Vet's claim is still right in general — it type-checks test files
without running them — but this particular tree does not demonstrate it.

## The three things D6 left to the build

**1. The arm list is derived, and finding none is a red.** D6 says "every
`posse_arm*` tag the tree declares", so the list comes out of the tree's
`//go:build` lines rather than a literal `posse_arm2 posse_arm3`. The scan
reads what is on disk (which is what the toolchain compiles, not what git
tracks) and only the lines above each file's `package` clause. Witnessed in
both directions, with `--plan`, over an exported copy of HEAD:

- a planted `//go:build posse_arm4` in `internal/posse` → step 5 becomes
  `go vet -tags posse_arm4 ./internal/posse`;
- the same tag planted in a NEW package → `go vet -tags posse_arm4
  ./internal/newpkg`, because the packages are derived too. Hardcoding
  `./internal/posse` would have vetted a fourth arm into a hole;
- a `//go:build posse_arm9` line at column 0 inside a raw string literal
  *below* the package clause → not picked up, so a mention is not a
  declaration;
- every `posse_arm` occurrence renamed away → **exit 1**, not a silent skip:
  `no posse_arm* build tags found in this tree — this repo has three test arms
  (ranger-base-qp1hm), so this is the derivation broken and not a tree without
  arms`. A derivation that silently returns nothing restores exactly the hole
  it exists to close: the arm vets vanish and the run still prints green. The
  day the arms are genuinely retired this reds once and the fix is to delete
  the guard in the same commit.

**2. The cross-GOOS step is "whichever of the two is not native", not
hardcoded darwin.** D6's body names `GOOS=darwin go vet ./...` because the D1
host is linux, where the plain `go vet ./...` has already covered linux. This
repo has linux-only files as well as darwin-only ones
(`backupvolume_*`, `loadavg_*`, `proctable_darwin.go`, `procstart_linux.go`,
`canonicalpath_darwin.go`, `cmd/coldexec/main_linux.go`), and ci.yml has a leg
for each. So the script cross-vets the non-native one of {darwin, linux}: on
the D1 host that is `GOOS=darwin GOARCH=amd64 go vet ./...`, D6's body
verbatim and D6's measured price, and on a darwin launcher it is the mirror
image instead of a redundant native vet beside an unvetted linux arm. The
GOARCH is the host's, except where the host's arch is one darwin has no port
for — not a pair go accepts, and a spurious red rather than a finding.

**STATED RESIDUAL**: the `!darwin && !linux` files (`backupvolume_other.go`,
`loadavg_other.go`, `procstart_other.go`, `proctable_other.go`,
`canonicalpath_other.go`, `cmd/coldexec/main_other.go`) compile under neither
GOOS — here or in ci.yml, which is where that gap already lived. Closing it
means a third GOOS and a price nobody has measured. Not filed as a defect:
nothing measured it wrong today.

**3. Nothing may resolve a path out of `$0`.** The copy that runs is the base
checkout's, with the session tree as its cwd (D1), so `$BASH_SOURCE` names a
*different tree*. A check that derived a repo root from its own location — the
shape every other script in `scripts/` uses, `REPO_ROOT=$(cd -- "$(dirname --
"${BASH_SOURCE[0]}")/.." && pwd)` — would pass every replay by judging the
base twice while looking exactly like a check. This one uses relative paths
and `$PWD` only, and says so where the next person will copy from. It refuses
with `no go.mod here` (exit 1) if the cwd is not a tree root at all.
