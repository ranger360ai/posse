#!/usr/bin/env bash
# Detective control for ADR 0019 "path 3": a credential file sitting in the
# Claude Code config directory (ranger-base-zzc / ranger-base-m6cm).
#
# WHY THIS EXISTS AS A SCRIPT AND NOT AS A DELETE
# ranger-base-zzc verified the file dead and the operator deleted it
# (ranger-base-66y) on 2026-08-26 03:40. A NEW one was created at 11:47:07 the
# same day — 8h06m later — and nothing on this box noticed for two days. The
# file's defining property is that it REGENERATES on file-auth login flows, so
# deleting it once is not a control against it. This is the control: it runs,
# it prints, and it exits non-zero while a file is there.
#
# WHAT A FINDING MEANS. ~/.claude is in every runtime's writable set
# (internal/posse/seatbelt.go) and the seatbelt denies no reads at all — the
# rendered profile's only deny is file-write* — so any same-user persona
# session below the container tier can read whatever is in that directory.
# The keychain OAuth item is the credential the fleet actually uses; a file
# here is a second copy of a live grant, outside the store ADR 0019 counts.
#
# WHAT THIS SCRIPT WILL NOT DO. It never reads a byte of any file it finds, it
# never deletes or renames one, and it never runs `security`. Removing a live
# credential is the operator's call every time (that is how 66y was handled).
# Deleting the file also does NOT revoke the grant at Anthropic — that is a
# /logout + /login, which re-touches the keychain item the guard depends on and
# makes any CLAUDE_CODE_OAUTH_TOKEN in envs/ stale (ADR 0019 D7). Two steps.
#
# THE MATCHER IS A GLOB, DELIBERATELY. On 2026-08-23 the file was RENAMED, not
# removed: `.credentials.json.stale-20260823`, same bytes, same directory, same
# mode 600. A rename changes the name, not the exposure, so anything matching
# `.credentials.json*` is a finding. ADR 0019 D5 line 201 still words the check
# as the exact path and PASSES on that box; reconciling the ADR is rangerhq-m10j.
#
# The ~/.codex/auth.json and ~/.grok/auth.json siblings are deliberately out of
# scope here: for those runtimes the file IS the store, not a leftover, so the
# same matcher would print a finding that is not one. They belong to m10j when
# their lanes reach the cage tier.
#
# Runbook: docs/runbooks/credential-rotation.md.
set -uo pipefail

quiet=0
case "${1:-}" in
  --quiet) quiet=1 ;;
  "") ;;
  *) echo "usage: $(basename "$0") [--quiet]" >&2; exit 2 ;;
esac

[ -n "${HOME:-}" ] || { echo "verify-credential-paths: HOME is unset — nothing to scan"; exit 2; }

# The lookup dir claude actually uses: $CLAUDE_CONFIG_DIR when set, else
# ~/.claude (internal/posse/trust.go:92). Scan both when they differ, so setting
# the variable cannot turn a finding into a silent pass.
dirs=("$HOME/.claude")
if [ -n "${CLAUDE_CONFIG_DIR:-}" ] && [ "$CLAUDE_CONFIG_DIR" != "$HOME/.claude" ]; then
  dirs+=("$CLAUDE_CONFIG_DIR")
fi

present=0
findings=0

# Metadata only — mode, size, birth and content mtime. No content, ever.
# atime is NOT reported: measured 2026-08-28 on this box's APFS data volume, a
# read does not advance it, so it is not a witness to anything (ranger-base-m6cm,
# correcting the "opened once, never since" reading in that bead's description).
meta() {
  stat -f '    mode %Sp  size %z  btime %SB  mtime %Sm' -t '%Y-%m-%d %H:%M:%S' "$1" 2>/dev/null ||
    stat -c '    mode %A  size %s  mtime %y' "$1" 2>/dev/null ||
    echo "    (stat unavailable)"
}

for d in "${dirs[@]}"; do
  if [ ! -d "$d" ]; then
    [ "$quiet" -eq 1 ] || echo "  absent   $d"
    continue
  fi
  present=$((present + 1))
  # -H follows a symlinked "$d" itself (the operand) without following
  # symlinks under it — [ -d "$d" ] above already follows the operand, so
  # find must match or a symlinked config dir scans as empty (ranger-base-dpuf).
  hits=$(find -H "$d" -maxdepth 1 -name '.credentials.json*' -print 2>/dev/null)
  rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "verify-credential-paths: could not scan $d (find exit $rc) — nothing measured, not a pass"
    exit 2
  fi
  if [ -z "$hits" ]; then
    [ "$quiet" -eq 1 ] || echo "  clean    $d"
    continue
  fi
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    findings=$((findings + 1))
    echo "  FINDING  $f"
    [ "$quiet" -eq 1 ] || meta "$f"
  done <<<"$hits"
done

# A clean report is only evidence if something was actually looked at: with no
# directory present this script has measured nothing, and must not say "clean".
if [ "$present" -eq 0 ]; then
  echo "verify-credential-paths: no config directory present of ${#dirs[@]} scanned — nothing measured, not a pass"
  exit 2
fi

if [ "$findings" -ne 0 ]; then
  cat <<EOF
verify-credential-paths: $findings credential file(s) in $present scanned director(ies)

Anything printed above is a finding. It is a second copy of a live grant in a
directory every persona session can read. Removing it is the operator's — file
the ask, do not delete it yourself; and revoking the grant is a separate step
(ADR 0019 D7). Runbook: docs/runbooks/credential-rotation.md.
EOF
  exit 1
fi

echo "verify-credential-paths: clean — 0 findings in $present scanned director(ies)"
