## Which `projects[]` key does claude read? (ranger-base-elf2v)

Three dispatched seats in a NEW repo (`~/src/HCN`, no prior claude session
there) opened on the folder-trust modal with posse's grant sitting in
`~/.claude.json`, true and timestamped at launch. posse wrote
`projects["<the session worktree>"]`; claude 2.1.288 looks a linked worktree's
trust up under its **main repo**, so all three keys granted nothing. herdr read
the panes blocked, the pass logged "never settled idle (status blocked)" for
hcn-1/2/3, benched erlich, and none of the three took its prompt.

The hop was known. `trust.go` named it and chose not to follow it, on the
grounds that "a fresh worktree of a trusted repo gets an entry it did not
strictly need — belt, not load-bearing". That reasoning had the error running
the safe way only because every repo the fleet had ever launched in was
already trusted by hand. The inverse case — a worktree of an **untrusted**
main repo — gets an entry claude never reads, and it is a dead seat.

### 1. The probe: ask the CLI which key it wants

`claude config list` (or any subcommand that loads project settings) in a
directory whose `.claude/settings.json` carries a `permissions.allow` entry
prints, when that workspace is untrusted:

```
Ignoring 1 permissions.allow entry from .claude/settings.json: this workspace
has not been trusted. Run Claude Code interactively here once and accept the
trust dialog, or set projects["<KEY>"].hasTrustDialogAccepted: true in <config>.
```

— the key in claude's own words, **no API turn, no TTY, no login**. Point
`CLAUDE_CONFIG_DIR` at a temp dir and the answer is the one a config that
trusts nothing gets, which is the only one that names a key.
`scripts/claude-trust-key.sh` is that probe; re-run it on any build.

This replaces the live-pane method the earlier trust measurements used (four
herdr scratch panes, `ranger-base-i0s8`). It is cheaper, it is scriptable, and
it reads the key rather than inferring it from whether a modal appeared —
which is how the hop stayed unmeasured for six weeks while being visible in
one line of output.

### 2. MEASURED 2026-10-03, claude 2.1.288 darwin-arm64

Which key claude names, per shape of cwd:

| cwd | key claude names |
|---|---|
| a dir in no repo | the dir |
| a repo root | the repo root |
| any subdir of a repo | the repo root |
| a linked worktree (`git worktree add`) | **the main repo** |
| a subdir of a linked worktree | **the main repo** |
| a worktree of a BARE repo | the bare repo dir itself (`/x/bare.git`) |
| a submodule (`.git` → `<super>/.git/modules/<n>`) | the submodule dir |
| a dir whose `.git` is a DANGLING symlink | the enclosing repo (claude walks past) |

And which keys silence the gate from inside a linked worktree:

| key present in the config | verdict |
|---|---|
| none | untrusted |
| the worktree's own path | **untrusted** ← what posse wrote |
| the main repo's path | trusted |
| the worktree's parent | untrusted |

Two more arms, from the same run, about the **walk** `ClaudeTrusted` used to
do (start at the session dir, take any `hasTrustDialogAccepted` on the way up,
stop at the enclosing git repo root):

| config | cwd | verdict |
|---|---|---|
| key on `<repo>/pkg` | `<repo>/pkg/sub` | untrusted |
| key on `<parent>` | `<parent>/scratch`, in no repo | **untrusted** |

There is no walk. The lookup is one exact key, and the key is the canonical
repo root. The second arm is a live fleet shape — `posse new` into a fresh
scratch dir under a parent the operator once accepted by hand — and the old
walk read it as trusted, so the launch skipped the seed it needed.

### 3. The rule, from the shipped bundle

Read 2026-10-03 off `~/.local/share/claude/versions/2.1.288` (`strings`, then
the minified source), and it agrees with every row above:

```js
function N0e(){ … let n=Ee(), r=CI(n); return r?cB(r):cB(Nn(Xe(n))) }  // the key
function aV(e){ let n=F_(e); if(!n)return null; return Be(n) }          // canonicalRoot(gitRoot)
function Be(e){ try{ return tn(e,B(x(e,".git"),"utf-8")) }catch{ return e } }
function tn(e,n){ try{
  let r=n.trim(); if(!r.startsWith("gitdir:"))return e;
  let o=r.slice(7).trim(); … let s=N(e,o);                   // the git dir
  let i=B(x(s,"commondir"),"utf-8").trim(); let u=N(s,i);     // the common dir
  if(N(V(s))!==x(u,"worktrees"))return e;                     // the layout
  let f=B(x(s,"gitdir"),"utf-8").trim();
  if(ye(N(s,f))!==x(ye(e),".git"))return e;                   // the back-pointer
  if(he(u)!==".git"){ if(Me(x(u,".git"),u))return e; return Nn(u) }
  return Nn(V(u))                                             // the main repo
}catch{return e} }
```

`Nn` is `t.normalize("NFC")` and `ye` strips trailing slashes. **Nothing in
that chain realpaths anything** — the answer is git's own spelling, out of the
pointer files. That is why the fix must not "correct" it: the bead's case note
is that `bd` lists `~/src/hcn` while the filesystem spells `HCN`, APFS is
case-insensitive, and claude's exact-string lookup only matches git's
spelling. `internal/posse/trust.go`'s `claudeMainRepoOf` mirrors `tn` bail for
bail, and `TestTrustKeyFollowsTheSamePointerChainClaudeDoes` pins the three
bails plus the pointer-spelling rule.

One row above is a step earlier than `tn` and cost `gitRootOf` its `Lstat`:
claude's find_git_root probe resolves the `.git` entry and takes
`isFile() || isDirectory()`, so a dangling `.git` symlink is **not** a root
and claude walks past it to the enclosing repo (measured). `gitRootOf` used
`os.Lstat`, which would have stopped there and keyed a path claude never
looks up — the same failure as the hop itself, one hop earlier.

One piece of the bundle is **inert** and should not be mistaken for a second
rule: `yor`/`hor`/`Mr` look like a root resolver with a `rootOnly` flag, but
`eB` (its only source of a root) is `function eB(e,{uncached:n=!1}={}){return
null}` and `Mr` is `{return!1}`, so `yor` returns null on every call and
`mw(e)` is always `aV(e)`. It is remote/host-files machinery with no host
path through it on this build.

### 4. Why `ClaudeTrusted` lost its walk rather than gaining the hop

The bead's fix shape said to seed the main repo "too" and have `ClaudeTrusted`
walk the same hop. The measurement says less is needed and less is correct:

- The worktree's own key grants nothing, so writing it as well is a line of
  the operator's config that means nothing — and one more per session worktree
  the fleet ever makes. One key per REPO, idempotent across all of its
  worktrees, is strictly smaller than what posse writes today.
- The walk is not claude's rule and it erred in the one direction that costs a
  seat. `ClaudeTrusted` returning true makes the launch write **nothing**, so a
  false trusted is the modal coming back in a pane nobody is watching, while a
  false untrusted is one idempotent key in a file posse already merges. The
  asymmetry is the whole argument; the walk sat on the wrong side of it twice.

What the walk got right it still gets right, because the KEY moved instead of
the lookup: a subdir of a repo is covered by its repo root's entry (the key for
both IS the repo root), and a trusted parent still does not cover a repo
underneath it (the repo keys on its own root, never on the parent).

The dialog's own predicate (`q0` → `jE` → `zE`) does still contain an upward
walk, bounded at the git root, in addition to the canonical-root check. The
settings-drop predicate (`Fm` → `Cbt`, measured above) does not. posse
satisfies the stricter of the two, which is the only choice that is safe
whichever one a future build keeps — and the drop predicate is load-bearing in
its own right, since what it drops is the project's `permissions.allow`.

### 5. The cage tier had the same bug

`SeedCageHome` keyed on the container workdir unmodified. The hop resolves
inside a container too: `gitCommonDirOutside` (cageinner.go) mounts a linked
worktree's git common dir **same-path in and out**, precisely so the `.git`
pointer file resolves to what it points at, so a caged claude walks the same
chain to the same main repo path. It now calls `claudeProjectKey`, which is the
hop without symlink resolution — the cage must keep the literal mount path,
since resolving macOS's `/var` → `/private/var` on the host names a directory
the Linux container does not have (`cagehomelock_qa_test.go`'s note).

### 6. Operator remedy for a repo already in this state

Nothing needs undoing: a dead key is inert. Either relaunch the seats on a
posse with this change, or answer the dialog once by hand —
`cd <the main repo> && claude`, "Yes, I trust this folder", `/exit` — which
writes the same key this now writes, and covers every worktree of that repo.
