## verify-detection replays the checkout, not the install (ranger-base-53w1)

`scripts/verify-detection.sh` called `herdr agent explain --file`, which
resolves the agent-detection manifest from `~/.config/herdr/agent-detection`
— the **installed** copy. The fixtures therefore measured whatever the
operator had last installed, never the tree a PR changes.

**Measured before the fix** (this checkout, herdr 0.8.0): cut the
`update_menu` rule out of `etc/herdr/agent-detection/codex.toml`, leave the
installed copy alone, run the script → `9/9 fixtures OK`, and the table even
printed `update_menu` as the matching rule. `make install-detection` copies
and *then* verifies, so the thing under test had just been written over the
thing it was verified against.

**Fix.** The script stages `etc/herdr/agent-detection/*.toml` into a
throwaway `XDG_CONFIG_HOME` (`XDG_STATE_HOME` with it, so a newer cached
remote cannot win on version) and explains there. Every fixture also checks
the `manifest:` path herdr reports: if the answer did not come from the
staged file, that fixture FAILs. Drop the staging and all nine fixtures fail
on `manifest=/Users/…/.config/…` rather than quietly measuring the install
again.

**Measured after** — same three arms, from the same checkout:

| arm | before | after |
|---|---|---|
| tree intact | 9/9 OK | 9/9 OK (control) |
| `update_menu` cut from `etc/` | 9/9 OK | `blocked-update-menu … FAIL (state)`, exit 1 |
| `startup_splash` cut from `etc/` | 9/9 OK | 4 splash fixtures `FAIL (rule)` |

**The install side kept its teeth.** Explaining against the tree would have
dropped the arm ranger-base-neyn wanted — an installed override that drifts
back below the fixtures. It is now a byte comparison against the checkout,
printed on every run as `<agent> install: matches the checkout` / `differs …`,
and `--check-install` turns a mismatch into exit 1. That flag is only honest
immediately after installing, which is where the Makefile wires it
(`install-detection` runs `scripts/verify-detection.sh --check-install`); a
plain `make verify-detection` on a checkout nobody installed stays green,
because the exit code is the tree's.

Pins: `internal/rhq/verifydetection_qa_test.go` — a rig copies the script and
`etc/herdr/agent-detection/` into a `t.TempDir()`, plants a **complete**
manifest in a scratch `XDG_CONFIG_HOME` (the operator who has installed the
good copy), cuts the rule from the tree, and requires a non-zero exit. Both
tests go red against a script mutated back to the pre-fix behaviour, and the
`--check-install` test goes red if the flag is accepted but inert.
