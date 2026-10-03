#!/usr/bin/env bash
# Which projects[] key does claude want for a directory? (ranger-base-elf2v)
#
# Ask the CLI instead of guessing. `claude config list` in a directory whose
# .claude/settings.json carries a permissions.allow entry prints, when that
# workspace is not trusted:
#
#   Ignoring 1 permissions.allow entry from .claude/settings.json: this
#   workspace has not been trusted. … or set
#   projects["<KEY>"].hasTrustDialogAccepted: true in <config>.
#
# — the key in claude's own words, with no API turn, no TTY and no login. That
# is the probe this script wraps, and it is how trust.go's table was measured.
# It never touches the operator's config: CLAUDE_CONFIG_DIR points at a temp
# dir for the duration, so the answer is the one a config that trusts nothing
# gets, which is the only one that names a key.
#
# Usage:
#   scripts/claude-trust-key.sh                 # this directory
#   scripts/claude-trust-key.sh DIR [DIR …]     # each of these
#
# Prints one `<dir> -> <key>` per directory, or `<dir> -> (trusted)` when the
# scratch config already covers it (which it cannot, so that line means the
# probe stopped working and the answer is unknown — never read it as a key).
#
# A directory of your own is modified: the probe needs a settings file with a
# project-scoped permission in it, so one is planted under <dir>/.claude and
# removed again. A pre-existing .claude/settings.json is left alone and used
# as-is — if it has no permissions.allow entry the line does not print, and
# the script says so rather than inventing a key.
set -euo pipefail

cfg=$(mktemp -d "${TMPDIR:-/tmp}/claude-trust-key.XXXXXX")
trap 'rm -rf "$cfg"' EXIT
printf '{}\n' >"$cfg/.claude.json"

probe() {
	local dir=$1 planted="" out key
	dir=${dir%/}
	if [ ! -d "$dir" ]; then
		printf '%s -> (no such directory)\n' "$dir"
		return
	fi
	if [ ! -e "$dir/.claude/settings.json" ]; then
		mkdir -p "$dir/.claude"
		printf '{"permissions":{"allow":["Bash(true)"]}}\n' >"$dir/.claude/settings.json"
		planted=$dir/.claude/settings.json
	fi
	out=$(cd "$dir" && CLAUDE_CONFIG_DIR="$cfg" claude config list 2>&1 || true)
	[ -n "$planted" ] && rm -f "$planted" && rmdir "$dir/.claude" 2>/dev/null
	key=$(printf '%s' "$out" | sed -n 's/.*set projects\["\([^"]*\)"\].*/\1/p' | head -1)
	if [ -n "$key" ]; then
		printf '%s -> %s\n' "$dir" "$key"
	elif printf '%s' "$out" | grep -q 'not been trusted'; then
		printf '%s -> (untrusted, but the message named no key — read it yourself)\n' "$dir"
	else
		printf '%s -> (no drop line: this dir has a settings.json with no permissions.allow entry, or the probe no longer prints)\n' "$dir"
	fi
}

if [ $# -eq 0 ]; then
	probe "$PWD"
else
	for d in "$@"; do probe "$d"; done
fi
