# Runbook — the home leaves the symlink: the link goes first, then promote, and what promote must not carry

*ADR 0015 §2/§3/§5/§7 · executes inside the rhq retirement window,
ranger-base-3rv9 step 4, the same window as `queue-cutover.md` ·
written on ranger-base-h56a, before o943 is built — steps marked
`[o943]` need that build installed first*

Today `~/.config/rhq` is a symlink to `~/src/ranger-base/rhq` — one
inode, so the home and the constitution repo are the same tree. After
this runbook `~/.config/posse` is a real directory: the promoted set
written by `posse promote`, plus three things promote deliberately
does **not** write and this window must therefore carry by hand:

| what | why promote cannot carry it | ruling |
|---|---|---|
| `envs/` | secret values, gitignored — no commit exists to promote from | ADR 0015 §7 |
| `state/` | machine-local runtime data (ledgers, gates, herdr meta) — never constitution | ADR 0015 §1 |
| `personas/` | persona memory, excluded from promotion, survives as a symlink | ADR 0015 §5 |

**Who runs it.** The operator. `posse promote` refuses under a persona
env marker, and the carry steps handle live tokens.

**Two ordering constraints shape everything below.**

The binary prefers `~/.config/posse` the moment the directory exists
(`app.go` home resolution, **MEASURED**) — a fleet launch between
"promote created the home" and "envs/state carried" comes up with no
env sets and no state. The fleet is quiesced for the queue half of the
window already (queue-cutover step 2); do the whole home sequence
inside that same quiet.

And the promote cannot go first: while `~/.config/rhq` is still a
symlink it *is* the home the binary resolves, and promote refuses onto
a symlinked home. So the link is removed first — step 2 — which is a
change from how this runbook was written, and is what the live window
actually did (ranger-base-j2io).

## The window (home half)

**1. Preconditions.** Fleet quiesced (see queue-cutover step 2). A
posse carrying o943's promote is installed. The constitution's
**promoted paths** are committed — promote refuses otherwise, so check
now:

```sh
git -C ~/src/ranger-base status --porcelain -- rhq/agents rhq/config.yaml rhq/recipes rhq/skills
```

That pathspec, and not the whole tree, is what promote gates on. As
built, promote **refuses on a dirty promoted path** and **reports
anything else dirty without blocking** — because the two things ADR
0015 itself carves out, `.beads` (§4) and `personas/` (§5), are dirty
in this repo essentially always, and neither is prose a promote puts
in force. (§3 was amended to say so — ranger-base-yb9j; the code says
so at `promoteCleanGate`.)

What the gate is **not** is the thing that keeps §3's invariant. Since
ranger-base-znma promote copies the **blobs at the commit** (`git
cat-file`, at `promotedAtCommit`), never the working tree, so "the
promoted bytes equal the bytes at the recorded SHA" holds by
construction — including for a path `git status` has been told to stop
reporting (`update-index --skip-worktree` / `--assume-unchanged`),
which the gate cannot see by design. Promote names any such path in a
note; your local edit there is *not* what goes into force. Two
consequences worth knowing before the window: a promoted file's mode
at the home is git's (`0644`, or `0755` for a committed executable
bit), and a promoted path that is a symlink or a submodule is still a
refusal, because neither is a blob posse can attest to.

**2. Retire the old name — BEFORE the promote, not after.**

```sh
rm ~/.config/rhq        # it is a symlink; the tree it pointed at is untouched
```

This used to be step 5, after the promote, and it is wrong in that order:
until `~/.config/posse` exists the binary resolves the home to
`~/.config/rhq`, and promote **refuses onto a symlinked home** —

> `~/.config/rhq` is a symlink; the promoted home must be a real directory
> (ADR 0015 §2) — remove the link first

(`internal/rhq/promote.go`, the `os.Lstat` gate; it refuses both directions,
because promoting a tree onto itself would then "remove what the source no
longer has" from the source). **MEASURED at the live window 2026-08-28**,
ranger-base-j2io: the order that ran was rm-then-promote, and steps 2–5 are
renumbered here to match. Steps 1 and 6–8 keep their old numbers.

Between this line and step 3 there is **no home at all**. That is safe only
inside the quiet this runbook's preamble asks for — do not take this step
until the fleet is quiesced.

**3. First promote.** `[o943]`

```sh
posse promote ~/src/ranger-base/rhq          # or --dry-run first
```

Creates `~/.config/posse` as a real directory, writes `agents/`,
`config.yaml`, `recipes/`, `skills/` and `promoted.json` beside them
— `{source, repo, sha, sha256 per file}`, the anchor every later
launch re-hashes against. It prints the diff it is putting in force
first; `--dry-run` prints that and writes nothing.

The path argument is needed only this once: the manifest records it,
so later promotes take no argument. (`constitution:` in config.yaml is
the other way to name it, and survives a home rebuilt from scratch —
worth adding to the constitution's own `config.yaml` before the
window so the promoted copy carries it — the same reasoning
`queue-cutover.md` step 5 now applies to its own two keys.)

It must **not** create `envs/` (§7). It warns if `default_env:` names
an env set that is not at the home — at this point in the sequence
that warning is *expected*; step 4 clears it. The warning names the
**set**, never a value.

**4. Carry the env sets** (§7). Modes must survive the copy — a copy
that widens 0600 publishes tokens to every process of this user.
`cp -p` preserves them; the umask covers the directory creation:

```sh
( umask 077
  mkdir -p ~/.config/posse/envs
  cp -p ~/src/ranger-base/rhq/envs/*.env ~/.config/posse/envs/ )
ls -la ~/.config/posse/envs/     # dir drwx------, files -rw-------
```

Four files as of 2026-08-26: `container.env` (the cage's OAuth token),
`default.env`, `glcc-box.env` (empty — carry it anyway, it is a name a
recipe may reference), `projA.env`. Do **not** delete the originals
yet — until step 6 verifies, they are the only proven copy.

**5. Carry state, link personas.**

```sh
cp -Rp ~/src/ranger-base/rhq/state ~/.config/posse/state
ln -s  ~/src/ranger-base/rhq/personas ~/.config/posse/personas
```

`state/` carries stale pids/locks (`dispatch-watch.pid`, `*.lock`)
with it; they name processes stopped at quiesce and the next dispatch
pass replaces them. The personas symlink is §5's "one symlink that
survives" — promote does not create it, this step does.

**6. Verify** — ADR 0015 verification items 1 and 7:

```sh
readlink ~/.config/posse && echo "FAIL: home is a symlink" || true
ls ~/.config/posse                     # promoted set + manifest + envs, state, personas
posse envs                             # the four set names + key names, never values
stat -f '%Sp %N' ~/.config/posse/envs ~/.config/posse/envs/*.env
```

Then launch one interactive session and one dispatched one: no
DEGRADED, no "env set not found", no legacy-home notice. The
`TightenEnvPerms` line printing at launch means a mode drifted in the
copy — it fixed it, but check step 4 was run as written.

A DEGRADED line here, or a dispatch refused with "constitution does
not match its manifest", means the promoted set at the home is not
what promote wrote — re-read the paths it names, then `posse promote`
again. It can never be an env file: `envs/` is in no manifest entry
(§7), which item 7 above checks directly.

**7. Only after step 6 passes: delete the env values from the
constitution tree.** They are gitignored, so this is a disk change,
not a git change — the point is that live tokens stop sitting in a
repo sessions get dispatched into:

```sh
rm ~/src/ranger-base/rhq/envs/*.env
```

The `.gitignore` rule stays: it is the fence against a future env file
dropped there ever reaching a commit.

**8. The first ratified change.** With promote in force, put the fence
through it.

```sh
scripts/draft-pid-deny-promote.sh ~/src/ranger-base/rhq/agents
# READ THE LAST LINE: "drafted into <n> PID(s), <m> left alone".
# n must be the number of PIDs that did not already carry the rule.
```

It adds `Bash(posse promote:*)` to every crew PID's `deny:` list and
stops — it stages nothing, commits nothing, and promotes nothing. Read
the diff, commit it, `posse promote`. That is ADR 0015 §3's second
spelling of the fence, ratified by the step ADR 0015 adds, which is the
point of doing it in this order.

**Check the count, do not assume it.** At the live window this script
drafted **0 of 11** and the edit went in by hand with `sed`
(ranger-base-j2io): it understood only the block `deny:` shape, and every
live crew PID is written inline, `deny: [a, b]`. It reported every skip —
it was not silent — but a drafting step that drafts nothing is a step that
did not run. Both shapes are handled as of that fix, and
`pidfence_qa_test.go` reads each drafted PID back through posse's own PID
reader; anything the script still cannot parse it names and leaves alone,
because a mangled PID is prose in force.

## Rollback

Cheap before step 7, because nothing is destroyed:

```sh
rm -rf ~/.config/posse
ln -s ~/src/ranger-base/rhq ~/.config/rhq   # step 2 removed it; put it back
```

The binary falls back to the legacy home by existence, so this is
complete. After step 7 the env values live only at
`~/.config/posse/envs` — roll back by `cp -p`ing them to
`~/src/ranger-base/rhq/envs/` first, then the two lines above.
