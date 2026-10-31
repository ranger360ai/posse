# The installed wall is two paths wide, and the other fourteen land (ranger-base-7qnag)

Lane: devops. Successor of ranger-base-qr7me, same shape: a landed fix to a
rendered wall guards nobody until `make install`. What is new here is that the
gap was measured on **both** sides by execution rather than by reading, and
that the live side was demonstrated with a commit that actually landed.

## The state, MEASURED 2026-10-31 (this box)

```
installed ~/.local/bin/posse           0.5.3+d4d105d2, mtime 2026-10-31 09:40:03
main                                   0f304f23
git merge-base --is-ancestor ab694844 d4d105d2   -> rc 1   (NOT an ancestor)
git merge-base --is-ancestor ab694844 0f304f23   -> rc 0
```

`ab694844` is ranger-base-t03st, which makes the commit wall's every-repo class
a derived union of the built-in runtimes' `ProjectConfig`
(`ConstitutionEveryRepoPaths`, promote.go:187) instead of two enumerated
claude paths. `internal/posse/gates.go:5688` renders it into the hook.

**The lag has grown since the bead was filed** — it read 6 behind / 1
rendering a wall at 13:5x; `posse gates gilfoyle` on the installed binary now
prints:

```
0.5.3+d4d105d2 is 16 commit(s) behind main in ~/src/posse, 4 of them rendering a wall
```

So `a285b222` (ranger-base-g55ah, the wall's resolution-equivalence matching)
is uninstalled too, not only t03st. The install ask is for the whole lag.

## The two class readings

Installed hook, `~/src/posse/.git/hooks/posse-prepare-commit-msg:1475` —
the pre-t03st literal, **2 members**:

```
posse_cls='
.claude/settings.json
.claude/settings.local.json
'
```

HEAD, `ConstitutionEveryRepoPaths()` executed (not read), **16 members**:

```
.bob/custom_modes.yaml   .claude/plugins              .cursor/hooks.json   .grok/hooks
.bob/hooks               .claude/settings.json        .cursor/mcp.json     .grok/lsp.json
.bob/mcp.json            .claude/settings.local.json  .grok/config.toml    .grok/plugins
.bob/plugins             .codex/config.toml           .mcp.json
.bob/settings
```

Fourteen paths are in the class HEAD renders and absent from the class this
box runs.

## The behaviour, both sides, by execution

**HEAD side** — `go test -tags posse_arm3 -run
TestQAConstitutionWallRefusesAnotherRuntimesProjectConfig ./internal/posse`,
which drives a real `git commit` through the HEAD-rendered hook in a temp
repo. PASS, 7.85s, both arms of both subtests:

```
.grok/hooks/x.json  / a_persona_is_refused                          PASS
.grok/hooks/x.json  / CONTROL the operator's identical commit lands PASS
.mcp.json           / a_persona_is_refused                          PASS
.mcp.json           / CONTROL the operator's identical commit lands PASS
```

**Live side** — the INSTALLED hook copied into a throwaway scratch repo (the
real box untouched; nothing installed, nothing promoted) and driven with real
path-limited commits:

| staged path | `RHQ_PERSONA=gilfoyle` | `env -u RHQ_PERSONA` |
|---|---|---|
| `.grok/hooks/x.json` | **LANDS** (exit 0, `8cf75e2`) | lands |
| `.claude/settings.json` | refused — "a persona commit touching the constitution" | lands (`ce6feae`) |

The second row is the rig's own control: the installed wall **is** live in that
repo and does refuse a member of its 2-path class, so the first row is the
class being two paths wide and not a dead hook. A persona on this box can
commit `.grok/hooks/x.json` at exit 0 right now, and the launcher
fast-forwards such a commit onto `main` when the bead closes.

**One trap worth keeping.** The first attempt at the operator control read
`session gilfoyle` and refused — because a dispatched seat *has*
`RHQ_PERSONA=gilfoyle` in its own environment, so a bare `git commit` in a
probe is the persona arm, not the control. The control needs
`env -u RHQ_PERSONA` explicitly. A probe that forgets it measures the persona
arm twice and reports a wall that holds in both directions.

**A second one.** Invoking the hook script directly
(`sh .../posse-prepare-commit-msg "$msgfile"`) instead of letting git run it
reports the *shared-index* arm, because that arm reads the commit form and a
hand invocation names no paths. The constitution wall sits above the
shared-index arm (`TestQAConstitutionWallSitsAboveTheSharedIndexArm`), so
"shared-index refusal printed" is evidence the constitution arm did **not**
match — but it is not a measurement of the class, and it cannot produce the
landing that the real commit above does. Plant the hook and commit for real.

## Not fixed here, and why

`make install` is **human-run and persona-denied** — ADR 0015 Context table,
MEASURED against the Makefile and settings, and ADR 0015 §3 puts
`posse promote` on the same footing ("Operator-run, like `make install`").
Crew guardrail 3 (deployed systems: per-change permission) says the same of a
binary that is dispatching a live fleet, and ranger-base-qr7me's close
recorded that reasoning already. `posse gates install-hooks` is refused at
this seat as well.

So this bead delivers the measurement and the ask, not the install. The ask
carries the exact command, the blast radius, and the three criteria below.
Precedent for the ask-and-close shape: ranger-base-tvorm (ranger-base-pju9t),
where the install arrived from outside the seat that filed it.

## What "verified" means after the install

```bash
git merge-base --is-ancestor ab694844 "$(posse version | sed 's/.*+//; s/ .*//')"   # rc 0
git merge-base --is-ancestor a285b222 "$(posse version | sed 's/.*+//; s/ .*//')"   # rc 0
grep -A20 "posse_cls='" ~/src/posse/.git/hooks/posse-prepare-commit-msg            # 16 paths
posse gates <persona> | head -1                                                    # 0 wall commits behind
```

and the behaviour, in a scratch repo, with the control shed explicitly:

```bash
RHQ_PERSONA=x  git commit -F - -- .grok/hooks/x.json   # refused
env -u RHQ_PERSONA git commit -F - -- .grok/hooks/x.json   # lands
```
