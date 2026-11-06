#!/bin/sh
# ADR 0076 Amendment 3 probe (ranger-base-1vgrw) — on Rex: can grok's OWN hook
# events reach an OSC 7501 record, which carrier gets a posse hook into grok,
# and what does `program_status` do to grok's `7501;?` query (the `n,auth` leak)?
# Usage: sh docs/adr/0076-grok-hook.probe.sh [arm…]   (repo root; no args = all)
#   native arms:  auto on off offenv onidle onlife
#   carrier arms: hhome hwt hagent hcfgpath hglobal
#   combined:     final        control: ctl
#   REX=<path to rex> overrides the CLI (it is not on PATH by default).
#
# No model turn is possible — the ranger-base-1g8gf rig, three locks:
#   - every arm runs under its OWN fixture HOME (so its own ~/.grok), never the
#     operator's; nothing outside the scratch dir is written;
#   - XAI_API_KEY is junk, and every *_PROXY points at 127.0.0.1:9 (nothing
#     listens), so an inference request dies locally;
#   - in the hook arms the UserPromptSubmit hook exits 2 (grok blocks the
#     prompt before any request is built).
# Telemetry is off by env and by the fixture config; auto_update is off.
# Each arm creates ONE Rex session labelled g1vgrw-<arm>-$$ and kills only that.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
GROK=$(command -v grok) || { echo "grok not on PATH"; exit 2; }
W=$(mktemp -d "${TMPDIR:-/tmp}/g1vgrw.XXXXXX")
echo "== rex $("$REX" version | head -1)   $("$GROK" --version)   $(uname -sr)"
echo "== scratch $W"

# The hook: grok's event -> a 7501 report on /dev/tty, plus a witness line
# (the hook's own stdout is a pipe). Same mapping as ADR 0076 D6's for codex.
cat > "$W/hook.sh" <<'EOF'
#!/bin/sh
tag=$1; W=$2
ev=$(python3 -I -c 'import sys,json; print(json.load(sys.stdin).get("hook_event_name",""))')
case $ev in
  SessionStart|Stop) st=idle ;;
  PermissionRequest|Notification) st=blocked:kind=permission ;;
  SessionEnd) st=clear ;;
  *) st=working ;;
esac
if printf '\033]7501;state=%s:app=grok-hook\033\\' "$st" > /dev/tty 2> "$W/err.$$"; then r=ok; else r="FAILED: $(cat "$W/err.$$")"; fi
ids=$(python3 -I -c 'import os; print("ppid=%d sid=%d pgid=%d" % (os.getppid(), os.getsid(0), os.getpgrp()))')
echo "$tag $ev -> $st  tty-write=$r  $ids" >> "$W/witness"
[ "$ev" = UserPromptSubmit ] && { echo "g1vgrw probe: prompt blocked by hook" >&2; exit 2; }
exit 0
EOF
chmod +x "$W/hook.sh"

EVENTS="SessionStart UserPromptSubmit PreToolUse PostToolUse Notification Stop SessionEnd"
hooks_json() { # hooks_json <tag> [script] — grok's hook-file JSON (Claude-compatible shape)
  python3 -I - "$W" "$1" "${2:-hook.sh}" $EVENTS <<'EOF'
import json, sys
w, tag, script, evs = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
cmd = f"sh '{w}/{script}' {tag} '{w}'"
print(json.dumps({"hooks": {e: [{"hooks": [{"type": "command", "command": cmd, "timeout": 5}]}] for e in evs}}, indent=1))
EOF
}
hooks_yaml() { # hooks_yaml <tag> — the same, as agent-definition frontmatter
  printf 'hooks:\n'
  for e in $EVENTS; do
    printf '  %s:\n    - hooks:\n        - type: command\n          command: "sh %s/hook.sh %s %s"\n          timeout: 5\n' "$e" "$W" "$1" "$W"
  done
}

ARMS=${*:-auto on off offenv onidle onlife onend hhome hwt hagent hcfgpath hglobal final ctl}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }

# fixture <arm> [program_status] — a private HOME with a quiet grok config.
fixture() {
  H=$W/$1/home WS=$W/$1/ws; mkdir -p "$H/.grok" "$WS"
  { printf '[cli]\nauto_update = false\n[features]\ntelemetry = false\n'
    [ -n "${2:-}" ] && printf '[ui.notifications]\nprogram_status = "%s"\n' "$2"
  } > "$H/.grok/config.toml"
}
# arm <name> <extra env> <grok args> — the line goes in a launcher script
# (a typed line past the tty's 1024-byte canonical buffer is cut, ch2rb).
arm() {
  S=g1vgrw-$1-$$
  cat > "$W/$1.launch" <<EOF
#!/bin/sh
export PATH='$PATH' HOME='$H' XAI_API_KEY=g1vgrw-junk-key
export HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9
export https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy=
export DISABLE_TELEMETRY=1 GROK_TELEMETRY_ENABLED=0 $2
cd '$WS' && exec '$GROK' $3
EOF
  chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
status() { printf '  %-30s ' "$1"; "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^\s*$' | tail -${1:-6} | sed 's/^/  | /'; }
leak() { if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'n,auth'; then echo "  leak: n,auth IS on screen"; else echo "  leak: no n,auth on screen"; fi; }
done_arm() { "$REX" send-key -s "$S" -b "$B" ctrl+c >/dev/null 2>&1; pause 1; "$REX" send-key -s "$S" -b "$B" ctrl+c >/dev/null 2>&1; pause 2
  status "after Ctrl-C x2 (exit)"; "$REX" kill "$S" >/dev/null 2>&1; }
witness() { echo "  witness:"; grep "^$1 " "$W/witness" 2>/dev/null | sed 's/^/    /' || echo "    (no hook ran)"; }
inspect_hooks() { # what grok itself says it loaded, read-only
  (cd "$WS" && env HOME="$H" XAI_API_KEY=x HTTPS_PROXY=http://127.0.0.1:9 DISABLE_TELEMETRY=1 GROK_TELEMETRY_ENABLED=0 $1 "$GROK" inspect $2 2>&1) \
    | grep -i -A3 'hooks\|trusted\|agent' | grep -v '^\s*$' | head -12 | sed 's/^/  inspect| /'; }

# --- Q2 and the native question: an argv prompt starts a (dead) turn ---------
native() { # native <arm> <program_status or ''> <extra env>
  fixture "$1" "$2"; arm "$1" "$3" "'g1vgrw probe: reply ok'"
  pause 4; status "t+4s"; pause 8; status "t+12s"; leak; screen 5; done_arm
}
want auto   && { echo "== auto: no program_status key, argv prompt, dead proxy"; native auto "" ""; }
want on     && { echo "== on: program_status=on in the HOME config.toml (a layer that reads ui.*)"; native on on ""; }
want off    && { echo "== off: program_status=off in the HOME config.toml"; native off off ""; }
want offenv && { echo "== offenv: program_status=off only in the GROK_CONFIG overlay (allowlist: models/features/toolset/sep)"
  native offenv "" "GROK_CONFIG='{\"ui\":{\"notifications\":{\"program_status\":\"off\"}}}'"; }

# --- the native lifecycle under `on`: launch, a turn that fails, a cancel, exit
if want onidle; then echo "== onidle: program_status=on, launched with NO prompt"
  fixture onidle on; arm onidle "" ""
  pause 3; status "t+3s, no prompt"; pause 5; status "t+8s, no prompt"; leak; screen 4; done_arm; fi
if want onlife; then echo "== onlife: program_status=on, models.max_retries=0, argv prompt; then fail, cancel, /quit"
  fixture onlife on; printf '[models]\nmax_retries = 0\n' >> "$H/.grok/config.toml"
  arm onlife "" "'g1vgrw probe: reply ok'"
  pause 3; status "t+3s (turn 1, dead proxy)"
  i=0; while [ $i -lt 15 ] && "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}" | grep -q '"state":"working"'; do pause 2; i=$((i+1)); done
  status "turn 1 settled (+$((i*2))s)"; screen 4
  "$REX" send -s "$S" -b "$B" "g1vgrw probe two"; "$REX" send-key -s "$S" -b "$B" enter
  pause 2; status "turn 2 started"; "$REX" send-key -s "$S" -b "$B" escape; pause 3; status "after Esc (cancel)"; screen 3
  "$REX" send -s "$S" -b "$B" "/quit"; "$REX" send-key -s "$S" -b "$B" enter; pause 5
  status "after /quit"; screen 3; "$REX" kill "$S" >/dev/null 2>&1; fi
if want onend; then echo "== onend: program_status=on, argv prompt; Ctrl-C a lone turn, then let a second dead turn exhaust its retries"
  fixture onend on; arm onend "" "'g1vgrw probe: reply ok'"; pause 6; status "turn 1 running"
  "$REX" send-key -s "$S" -b "$B" ctrl+c; pause 3; status "after one Ctrl-C, nothing queued"; screen 3
  "$REX" send -s "$S" -b "$B" "g1vgrw probe two"; "$REX" send-key -s "$S" -b "$B" enter; pause 4; status "turn 2 started"
  i=0; while [ $i -lt 100 ] && "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}" | grep -q '"state":"working"'; do pause 5; i=$((i+1)); done
  status "turn 2 after +$((i*5))s"; screen 6
  "$REX" send -s "$S" -b "$B" "/quit"; "$REX" send-key -s "$S" -b "$B" enter; pause 5; status "after /quit"
  "$REX" kill "$S" >/dev/null 2>&1; fi

# --- Q1 and Q3: which carrier gets a posse hook into grok ---------------------
hookarm() { # hookarm <arm> <extra env> <grok args>
  arm "$1" "$2" "$3"
  pause 6; status "launched (t+6s)"; witness "$1"; pause 6; status "t+12s"; witness "$1"; leak; screen 4
  done_arm; witness "$1"
}
if want hhome; then echo "== hhome: hook in \$GROK_HOME/hooks/ (a per-seat GROK_HOME's place), argv prompt"
  fixture hhome; mkdir -p "$H/.grok/hooks"; hooks_json hhome > "$H/.grok/hooks/posse.json"
  inspect_hooks "" ""; hookarm hhome "" "'g1vgrw probe'"; fi
if want hwt; then echo "== hwt: hook in the session dir's .grok/hooks/, dir trusted in the fixture's trusted_folders.toml"
  fixture hwt; mkdir -p "$WS/.grok/hooks"; hooks_json hwt > "$WS/.grok/hooks/posse.json"
  printf '[folders."%s"]\ntrusted = true\ndecided_at = 1\n' "$(cd "$WS" && pwd -P)" > "$H/.grok/trusted_folders.toml"
  inspect_hooks "" ""; hookarm hwt "" "'g1vgrw probe'"; fi
if want hagent; then echo "== hagent: hooks in an agent-definition file OUTSIDE the session dir, named by --agent"
  fixture hagent; { printf -- '---\nname: g1vgrw-probe\ndescription: probe agent\n'; hooks_yaml hagent; printf -- '---\nYou are a probe.\n'; } > "$W/hagent/agent.md"
  inspect_hooks "" "--agent $W/hagent/agent.md"; hookarm hagent "" "--agent '$W/hagent/agent.md' 'g1vgrw probe'"; fi
if want hcfgpath; then echo "== hcfgpath: hooks in a GROK_CONFIG_PATH overlay file outside the session dir"
  fixture hcfgpath; hooks_json hcfgpath > "$W/hcfgpath/overlay.json"
  inspect_hooks "env GROK_CONFIG_PATH=$W/hcfgpath/overlay.json" ""; hookarm hcfgpath "GROK_CONFIG_PATH='$W/hcfgpath/overlay.json'" "'g1vgrw probe'"; fi
if want hglobal; then echo "== hglobal: an env-gated hook in ~/.grok/hooks (herdr's own pattern), gate variable SET"
  fixture hglobal; mkdir -p "$H/.grok/hooks"
  printf '#!/bin/sh\n[ -n "$POSSE_REX_REPORT" ] || exit 0\nexec sh "$(dirname "$0")/hook.sh" "$@"\n' > "$W/gate.sh"
  hooks_json hglobal gate.sh > "$H/.grok/hooks/posse.json"
  hookarm hglobal "POSSE_REX_REPORT=1" "'g1vgrw probe'"; fi

# --- the combination a slice would ship: carrier + program_status=off ---------
if want final; then echo "== final: hook in \$GROK_HOME/hooks AND program_status=off, argv prompt"
  fixture final off; mkdir -p "$H/.grok/hooks"; hooks_json final > "$H/.grok/hooks/posse.json"
  hookarm final "" "'g1vgrw probe'"; fi

if want ctl; then echo "== control: the same hook run from the bare shell, no grok in the foreground"
  fixture ctl; S=g1vgrw-ctl-$$
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  "$REX" send -s "$S" -b "$B" "echo '{\"hook_event_name\":\"PreToolUse\"}' | sh '$W/hook.sh' ctl '$W'"; "$REX" send-key -s "$S" -b "$B" enter
  pause 2; status "control hook from the shell"; witness ctl; "$REX" kill "$S" >/dev/null 2>&1; fi
echo "== scratch left at $W (witness, launchers, fixture homes)"
