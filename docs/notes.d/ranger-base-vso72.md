## A rendered wall now names its renderer, and promote reads what it records (ranger-base-vso72)

2026-09-28

Findings and measurements behind ranger-base-vso72, filed off
ranger-base-mmhhc ("bob's credential read-deny is not in force on this box").
Read this with `internal/posse/renderstamp.go` (why a header carries a
version) and `internal/posse/wallrenderer.go` (why the count is narrowed).

### 1. The shape of the gap

Every wall a seat launches behind is a **render** from the **installed**
binary: the seatbelt profile, the gate shims, the gate shell, the cage's
mounts and launcher, the egress allowlist, the pane line, the L3 hook bodies.
So a gate fix that lands on `main` does not reach a seat when the launcher
relaunches — it reaches a seat when a **descendant of that fix is installed**
and *then* relaunches. Two surfaces said the first half and neither said the
second:

- the headers (`;; posse seatbelt for bob — rendered from the PID at launch`)
  named the PID and the launch, so a seat that read its own profile and did
  not find the deny it expected could not tell *relaunch pending* from
  *install pending*;
- `posse promote` **recorded** the renderer (`m.Posse = VersionString()`,
  promote.go) beside the source SHA it promoted and never compared the two.
  Its only reader compares promoted **sets** ("the manifest predates this
  binary's promoted set"), never the binary's commit against the source.

### 2. MEASURED: what the new reading says about the incident itself

Environment: worktree `posse/dinesh-posse-ranger-base-vso72` at `1b8ffe2d`,
darwin 25.4.0, 2026-09-28. A posse built from this tree with the Makefile's
own stamp form pointed 40 commits back —
`-ldflags '-X …/internal/posse.Build=02dd122'`, `02dd122 == HEAD~40` — then
run as `posse gates dinesh`:

```
wall renderer (gates) · 0.5.0+02dd122 is 40 commit(s) behind main in ~/src/posse,
2 of them rendering a wall — until a descendant of main is installed, every launch
on this box renders the OLD seatbelt, gate shims, gate shell, cage mounts and
hooks … `make install`:
    577027fa ranger-base-prjck: bob's auth-secrets.json is the fourth credential read-deny literal
    38a348f0 ranger-base-gw9o5: an empty hook slot is not a foreign hook, and a first install is not a re-stamp
    git -C ~/src/posse log --oneline 02dd122..main -- internal/posse/seatbelt.go …
```

`577027fa ranger-base-prjck` is **the commit ranger-base-mmhhc's twelve stale
caged launches did not have**. Before this change, the same binary at the same
commit printed nothing at all — not on `posse gates`, not on `posse promote`.

### 3. MEASURED: the render class

The derived class is "an `internal/posse` source whose own **string literals**
say `do not edit`" — the render's claim about itself. On this tree that is six
files: `seatbelt.go`, `gates.go`, `cagelauncher.go`, `cageinner.go`,
`egress.go`, `paneline.go`. String literals and not file text, because
`renderstamp.go` quotes a header in its doc comment and renders nothing.

`posse.WallRenderSources` names ten: those six plus `cage.go` (the container's
mount set), `hooksredirect.go` (the session hooks dir, ADR 0052 D2),
`skills.go` (the session skills binding, ADR 0007) and `renderstamp.go`
itself. `internal/treepins/wallrendersources_qa_test.go` holds the derived
half in both directions — membership, and that each derived site calls
`RenderedByPosse()`. Both arms were confirmed to red on a control (dropping
`paneline.go` from the list; removing its stamp).

The list is a **floor** and the code says so where it is declared: a render
site carrying no such header is invisible to the pin. That is why nothing keys
off it — the line always reports the WHOLE lag and always names `make
install`, and the wall count only says how loud to read it.

### 4. DIVERGED from the bead's sketch, twice, and why

**a. Hook BODIES carry no version stamp.** The bead asks for "the seatbelt.sb
header (and rendered hook/shim headers)". Hook identity is compared
**byte-for-byte** against this binary's own render (`probeL3Hooks`, ADR 0023),
which already answers "does this wall carry THIS binary's render" for that one
artifact — `hookfresh.go`'s sweep is that reading. A version inside a hook
body would make every hook on the box read as degraded after **any** `make
install`, including installs that changed nothing a hook renders. A stamp that
cries wolf on every release is a stamp nobody reads. The hooks get the docs
sentence (item 3) and the wall count instead.

**b. `hookfresh.go` is not in `WallRenderSources`**, though the bead's example
names it. It READS walls and renders none, so a commit to it changes what
posse reports and nothing a seat launches behind. Counting a reader would
inflate the one number whose whole value is that it is specific.

### 5. Why this is not `LauncherLag`, and why it uses it

`launcherlag.go` (ranger-base-z3hx6) already counts "how many landed commits
is the running binary missing" and prints it on `posse status` and in the
watch pass. That reading is about the **launcher's** own defects — a
merge-back block filed a fourth time. This one asks the narrower question the
operator acts on differently: of those missing commits, how many change a wall
this binary **renders**. Same measurement, so the two counts cannot disagree;
different consequence, so it is said at the operator's `promote` and at `posse
gates`, where the claim being made is precisely *this is the wall*.

A reading, never a control, on `possebinary.go`'s rule: it prints, it warns
and it decides nothing. An unreadable answer is **UNKNOWN**, never OK — a
build that names no commit cannot say it carries anything, and a `go build`
from a linked worktree is exactly that (cagestale.go documents the shape, and
every persona here works in a linked worktree). Rendering that abstention as
silence is how the gap stayed open through three closes that said "reaches a
seat at its next launch".
