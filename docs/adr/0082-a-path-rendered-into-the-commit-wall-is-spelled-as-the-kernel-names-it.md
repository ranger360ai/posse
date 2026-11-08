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
  keeps the caller's case and does not fold firmlinks, so `~/src/hcn` vs
  `~/src/HCN` still renders two walls. "Every site" also means three places
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
