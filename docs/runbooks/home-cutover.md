# Runbook — the home leaves the symlink: first promote, and what promote must not carry

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

**Ordering constraint that shapes everything below**: the binary
prefers `~/.config/posse` the moment the directory exists
(`app.go` home resolution, **MEASURED**) — a fleet launch between
"promote created the home" and "envs/state carried" comes up with no
env sets and no state. The fleet is quiesced for the queue half of the
window already (queue-cutover step 2); do the whole home sequence
inside that same quiet.

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
in force. (o943 files the §3 amendment; the code says so at
`promoteCleanGate`.)

**2. First promote.** `[o943]`

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
window so the promoted copy carries it.)

It must **not** create `envs/` (§7). It warns if `default_env:` names
an env set that is not at the home — at this point in the sequence
that warning is *expected*; step 3 clears it. The warning names the
**set**, never a value.

**3. Carry the env sets** (§7). Modes must survive the copy — a copy
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

**4. Carry state, link personas.**

```sh
cp -Rp ~/src/ranger-base/rhq/state ~/.config/posse/state
ln -s  ~/src/ranger-base/rhq/personas ~/.config/posse/personas
```

`state/` carries stale pids/locks (`dispatch-watch.pid`, `*.lock`)
with it; they name processes stopped at quiesce and the next dispatch
pass replaces them. The personas symlink is §5's "one symlink that
survives" — promote does not create it, this step does.

**5. Retire the old name.**

```sh
rm ~/.config/rhq        # it is a symlink; the tree it pointed at is untouched
```

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
copy — it fixed it, but check step 3 was run as written.

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
through it. `scripts/draft-pid-deny-promote.sh ~/src/ranger-base/rhq/agents`
drafts `deny: - Bash(posse promote:*)` into every crew PID and stops —
it stages nothing, commits nothing, and promotes nothing. Read the
diff, commit it, `posse promote`. That is ADR 0015 §3's second
spelling of the fence, ratified by the step ADR 0015 adds, which is
the point of doing it in this order.

## Rollback

Cheap before step 7, because nothing is destroyed:

```sh
rm -rf ~/.config/posse
ln -s ~/src/ranger-base/rhq ~/.config/rhq   # if step 5 already ran
```

The binary falls back to the legacy home by existence, so this is
complete. After step 7 the env values live only at
`~/.config/posse/envs` — roll back by `cp -p`ing them to
`~/src/ranger-base/rhq/envs/` first, then the two lines above.
