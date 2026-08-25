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
# Run after `make install-detection`, and again after any `herdr update`:
# herdr refreshes remote manifests in the background AND ships bundled ones
# inside the binary, and either moving past our fork point is the signal to
# re-check whether the override can be retired.
set -uo pipefail

cd "$(dirname "$0")/.."
root=etc/herdr/agent-detection
fail=0 n=0

command -v herdr >/dev/null || { echo "verify-detection: herdr not on PATH"; exit 2; }

agents=()
for toml in "$root"/*.toml; do
  [ -e "$toml" ] || continue
  agents+=("$(basename "$toml" .toml)")
done
[ ${#agents[@]} -gt 0 ] || { echo "verify-detection: no overrides in $root"; exit 2; }

printf '%-8s %-38s %-8s %-8s %s\n' AGENT FIXTURE EXPECT ACTUAL RULE
for agent in "${agents[@]}"; do
  for f in "$root/testdata/$agent"/*.txt; do
    [ -e "$f" ] || continue
    base=$(basename "$f" .txt)
    want=${base%%-*}
    out=$(herdr agent explain --file "$f" --agent "$agent" 2>&1)
    got=$(printf '%s\n' "$out" | awk -F': ' '/^state/{print $2; exit}')
    rule=$(printf '%s\n' "$out" | awk -F': ' '/^rule/{print $2; exit}')
    rule_id=${rule%% *}
    n=$((n + 1))
    why=
    if [ "$got" != "$want" ]; then
      why=state
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

# Report which manifest actually answered, so a passing run cannot be a
# stale-override illusion, and surface upstream movement past our fork point.
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
  herdr server agent-manifests --json 2>/dev/null | \
    AGENT="$agent" FORK="$fork" BUNDLED_FROM="$bundled_from" HERDR_NOW="$herdr_now" python3 -c '
import json, os, sys
agent = os.environ["AGENT"]
try: m = [x for x in json.load(sys.stdin)["result"]["manifests"] if x["agent"] == agent][0]
except Exception: sys.exit(0)
print("\n%s manifest: %s (%s)" % (agent, m["active_version"], m["source_kind"]))
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
echo "verify-detection: $n/$n fixtures OK"
