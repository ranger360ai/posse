## The runtime probe measures one binary and records another (ranger-base-ok1x)

Found verifying the close of rangerhq-66e2 (the `posse runtime probe`
deliverable). The production probe works — I ran ADR 0032's checklist items
2 and 3 live and they pass — but the probe has **two PATHs**, and only one
of them is the one it measures.

**The two PATHs, measured.** The probe's pane is created by the herdr
*daemon*: `RuntimeProbe` calls `h.CreateWorkspace(label, dir, vars)` with an
explicit `[]EnvVar` (`RHQ_HOME`, `RHQ_PERSONA`, `RHQ_GATES_DIR`, …) that
carries **no `PATH`**, and `GatePrefix` is `PATH=<gates bin>:"$PATH" …`
where `$PATH` expands *inside the pane's shell*. So the pane's PATH is the
daemon's, not the calling process's. Measured directly — a scratch
`CreateWorkspace` + `PaneRun` of `sh -c 'echo "$PATH" > f'` writes the
daemon's PATH; a `t.TempDir` the caller had put at the front of its own
`PATH` is absent from it.

Everything else the probe does about the CLI resolves in the **posse
process's** PATH: `canaryExe` → `resolveOutside(rt.Exe(), "")`
(`runtimeprobe.go:645`) fills the record's `cli_path`, and
`ProbeCLIVersion` → `readCLIVersion` (`:295–310`) runs `<that path>
--version` for `version`. `RuntimeCheck`'s drift comparison reads the same
side. The pane never gets a vote on which binary was measured.

### Symptom 1 — the record certifies a binary nobody probed

Planting a decoy `codex` (a two-line `echo` script) at the front of the
*test process's* PATH only, then running the probe against a
`codex`-as-template profile:

    real codex on the daemon's PATH : /opt/homebrew/bin/codex
    decoy planted for posse only    : /var/folders/…/002/codex
    record cli_path                 : /var/folders/…/002/codex
    record version                  : "decoy 9.9.9"
    record passed                   : true
      observable 1 shim-precedence  ok=true
      observable 2 refusal-logged   ok=true
      observable 3 unattended-turn  ok=true
      observable 4 herdr-detection  ok=true

Four green observables measured on `/opt/homebrew/bin/codex`, written into
a record that names a shell script that cannot launch anything. The drift
check then compares that same decoy against itself and reports *current*,
so ADR 0032's currency rule — "keyed on the binary, not on a number nobody
measured" — is keyed on the wrong binary. An operator whose posse process
runs with a PATH the login daemon does not have (`~/.local/bin`, nvm, asdf,
a gated session) gets this without planting anything.

### Symptom 2 — the live wrong arm cannot fail

`runtimeprobe_live_test.go`'s `RHQ_LIVE_PROBE_FAKE` arm — ADR 0032
checklist item 1, and by its own doc comment "the whole reason this file
exists" — installs its login-shell shim with `t.Setenv("PATH", dir+…)`.
By the paragraph above that PATH never reaches the pane, so the pane
launches the real CLI and the probe passes all four observables. The arm
then fails with a message that accuses the production probe:

    a CLI that hardcodes /bin/zsh -l must FAIL the probe — observable 1 is
    the whole reason it exists

A second, independent reason the arm is inert on the runtime its own doc
comment names: the shim works by setting `SHELL=/bin/zsh
GROK_SHELL=/bin/zsh`, and ADR 0009's measured table says codex does not
call `$SHELL` **at all** (`/bin/bash -c` directly, `allow_login_shell`).
`codex` cannot be made to re-exec a login shell that way whatever the PATH
does. grok is the runtime in that table that resolves its shell from
`$SHELL`.

So observable 1's failure path is pinned only at unit level
(`TestProbeShimPrecedenceFailsWhenTheRealBinaryWins`); the *live* half of
the ADR's checklist item 1 has never been able to fail, and the red it
produces points at the wrong component.

Filed for the closer as a bug; the probe's other four deliverables verified.
