#!/usr/bin/env bash
# Verify herdr's active agent-detection manifests against the fixtures in
# etc/herdr/agent-detection/testdata/<agent>/. Each fixture is a real pane
# snapshot (`herdr pane read <pane> --source detection`) whose expected state
# is encoded in its filename: <state>-<what>.txt.
#
# This is the regression test for our detection overrides:
#   codex (rangerhq-7ia) — the "Hooks need review" dialog, and every codex
#     modal sharing its "esc to go back" footer, read as idle.
#   grok  (rangerhq-37c, rangerhq-1xsj) — the startup splash (New worktree /
#     Resume session menu + changelog line + "Help improve Grok" consent
#     banner) matches no upstream rule. Ours names it and reports `idle`:
#     measured, that screen is decoration over a live composer that takes a
#     prompt (rangerhq-1xsj). It reported `blocked` between 37c and 1xsj.
#
# codex's was the dangerous hole: no rule matched, and detection fell through
# to default_known_agent_idle_fallback, which for a KNOWN agent resolves to
# `idle` — a prompt typed into a modal. grok's rule ends up at the same state
# as that fallback on purpose, so the *state* encoded in idle-startup-splash*.txt
# is the same answer herdr would give with the rule deleted. After rangerhq-1xsj
# that is not a pin: a missing rule is still idle, via the fallback 37c closed.
# testdata/grok/*startup-splash* must therefore resolve to rule id
# startup_splash, not `none` / default_known_agent_idle_fallback (rangerhq-uglc).
# The state check still fails loudly if the screen ever becomes a real modal.
#
# Fixtures are replayed against the manifests in THIS CHECKOUT, staged into a
# throwaway XDG_CONFIG_HOME — never against ~/.config/herdr/agent-detection.
# Until ranger-base-53w1 this script explained against the installed copy, so
# deleting a rule from etc/ and running it reported every fixture OK: the run
# measured whatever the operator had last installed. `make install-detection`
# copies then verifies, which hid it further. The tree is what a PR changes
# and what review needs failed, so the tree is what is replayed here; the
# install is checked separately at the bottom.
#
# --check-install additionally *fails* when the installed copy is missing or
# differs from the tree. Only meaningful right after `make install-detection`,
# which is where it is wired: at that one moment the two must be byte-identical,
# and anything else means the install did not land where herdr reads (the
# ranger-base-neyn class — an override that drifts back below the fixtures).
# Plain runs report the same mismatch as a note: a checkout you have not
# installed is not a broken tree, and CI has no install at all.
#
# Run it any time, no install needed. Run it again after any `herdr update`:
# herdr refreshes remote manifests in the background AND ships bundled ones
# inside the binary, and either moving past our fork point is the signal to
# re-check whether the override can be retired.
set -uo pipefail

cd "$(dirname "$0")/.."
root=etc/herdr/agent-detection
fail=0 n=0 install_fail=0 check_install=0
for arg in "$@"; do
  case "$arg" in
    --check-install) check_install=1 ;;
    *) echo "verify-detection: unknown argument $arg"; exit 2 ;;
  esac
done

command -v herdr >/dev/null || { echo "verify-detection: herdr not on PATH"; exit 2; }

agents=()
for toml in "$root"/*.toml; do
  [ -e "$toml" ] || continue
  agents+=("$(basename "$toml" .toml)")
done
[ ${#agents[@]} -gt 0 ] || { echo "verify-detection: no overrides in $root"; exit 2; }

# Stage the checkout's overrides where herdr resolves a local override from.
# XDG_STATE_HOME goes with it: herdr picks the HIGHEST version among local
# override / cached remote / bundled, and an isolated state dir keeps a newer
# cached remote out of the answer.
stage=$(mktemp -d "${TMPDIR:-/tmp}/verify-detection.XXXXXX") || exit 2
trap 'rm -rf "$stage"' EXIT
staged=$stage/config/herdr/agent-detection
mkdir -p "$staged" "$stage/state"
cp "$root"/*.toml "$staged/"
explain() {
  env XDG_CONFIG_HOME="$stage/config" XDG_STATE_HOME="$stage/state" \
    herdr agent explain "$@"
}

printf '%-8s %-38s %-8s %-8s %s\n' AGENT FIXTURE EXPECT ACTUAL RULE
for agent in "${agents[@]}"; do
  for f in "$root/testdata/$agent"/*.txt; do
    [ -e "$f" ] || continue
    base=$(basename "$f" .txt)
    want=${base%%-*}
    out=$(explain --file "$f" --agent "$agent" 2>&1)
    got=$(printf '%s\n' "$out" | awk -F': ' '/^state/{print $2; exit}')
    rule=$(printf '%s\n' "$out" | awk -F': ' '/^rule/{print $2; exit}')
    rule_id=${rule%% *}
    src=$(printf '%s\n' "$out" | awk '/^manifest: /{print $2; exit}')
    n=$((n + 1))
    why=
    if [ "$got" != "$want" ]; then
      why=state
    fi
    # Which file answered. Staging can only prove the tree if the tree is what
    # herdr read: a bundled or cached manifest winning on version would put us
    # straight back in the illusion this script was fixed to end.
    if [ "$src" != "$staged/$agent.toml" ]; then
      why="${why:+$why,}manifest=${src:-none}"
    fi
    # Splash fixtures are idle on purpose (rangerhq-1xsj). Without a named-rule
    # check, deleting startup_splash still goes green: herdr falls through to
    # default_known_agent_idle_fallback, which is the original 37c hole.
    case "$agent/$base" in
      grok/*startup-splash*)
        if [ "$rule_id" != "startup_splash" ]; then
          why="${why:+$why,}rule"
        fi
        ;;
    esac
    if [ -z "$why" ]; then
      printf '%-8s %-38s %-8s %-8s %s\n' "$agent" "$base" "$want" "$got" "${rule:-—}"
    else
      printf '%-8s %-38s %-8s %-8s %s   <-- FAIL (%s)\n' "$agent" "$base" "$want" "${got:-?}" "${rule:-—}" "$why"
      fail=$((fail + 1))
    fi
  done
done

# The fixtures above measured the checkout. This block measures the INSTALL —
# whether the operator's fleet is running what the tree says, and whether
# upstream has moved past our fork point. Notes only: a stale install is not a
# broken tree, and this script's exit code is the tree's.
#
# Two fork points matter, because herdr resolves a manifest from three places
# (local override > cached remote > bundled-in-binary) and picks the HIGHEST
# version, not the nearest source. grok's override is forked from the BUNDLED
# manifest, which is invisible once our override shadows it — so the binary's
# own version is what we watch for drift there.
herdr_now=$(herdr --version 2>/dev/null | awk '{print $NF}')
for agent in "${agents[@]}"; do
  toml="$root/$agent.toml"
  fork=$(sed -n 's/^# posse_forked_from = "\(.*\)".*/\1/p' "$toml")
  bundled_from=$(sed -n 's/^# posse_bundled_from_herdr = "\(.*\)".*/\1/p' "$toml")
  installed=${XDG_CONFIG_HOME:-$HOME/.config}/herdr/agent-detection/$agent.toml
  if [ ! -e "$installed" ]; then
    printf '\n%s install: not installed — run `make install-detection`\n' "$agent"
    install_fail=$((install_fail + 1))
  elif ! cmp -s "$toml" "$installed"; then
    printf '\n%s install: %s differs from the checkout — run `make install-detection`\n' \
      "$agent" "$installed"
    install_fail=$((install_fail + 1))
  else
    printf '\n%s install: matches the checkout\n' "$agent"
  fi
  herdr server agent-manifests --json 2>/dev/null | \
    AGENT="$agent" FORK="$fork" BUNDLED_FROM="$bundled_from" HERDR_NOW="$herdr_now" python3 -c '
import json, os, sys
agent = os.environ["AGENT"]
try: m = [x for x in json.load(sys.stdin)["result"]["manifests"] if x["agent"] == agent][0]
except Exception: sys.exit(0)
print("  active manifest: %s (%s)" % (m["active_version"], m["source_kind"]))
if m["source_kind"] != "local override":
    print("  note: the posse override is not active — run `make install-detection`")
    for line in (m.get("warning") or "").splitlines():
        print("        " + line)
def ver(v):
    return tuple(int(x) for x in v.split(".") if x.isdigit())
fork, remote = os.environ.get("FORK", ""), m.get("cached_remote_version") or ""
if fork and remote and ver(remote) > ver(fork):
    print("  note: upstream %s manifest is now %s; we forked %s." % (agent, remote, fork))
    print("        Re-check whether herdr fixed this upstream. If it did, delete")
    print("        %s/%s.toml and ~/.config/herdr/agent-detection/%s.toml." % ("etc/herdr/agent-detection", agent, agent))
was, now = os.environ.get("BUNDLED_FROM", ""), os.environ.get("HERDR_NOW", "")
if was and now and was != now:
    print("  note: this override forked the manifest BUNDLED in herdr %s; herdr is now %s." % (was, now))
    print("        A bundled manifest can move without the remote one moving, and it is")
    print("        invisible while our override shadows it. Re-extract and re-diff —")
    print("        see \"Re-checking a bundled fork\" in the README.")
'
done

echo
if [ "$fail" -ne 0 ]; then
  echo "verify-detection: $fail/$n fixtures FAILED"
  exit 1
fi
echo "verify-detection: $n/$n fixtures OK (against $root in this checkout)"
if [ "$check_install" -eq 1 ] && [ "$install_fail" -ne 0 ]; then
  echo "verify-detection: $install_fail installed manifest(s) do not match the checkout"
  exit 1
fi
