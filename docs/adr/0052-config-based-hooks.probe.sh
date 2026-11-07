#!/bin/sh
# ADR 0052's rule ("posse never causes the managed hook to be skipped") read
# against git's CONFIG-BASED hooks — hook.<friendly-name>.command/.event/
# .enabled, which exist from git 2.54 and are inert below it. Every earlier
# reading of that rule was taken against core.hooksPath on a git where these
# keys did nothing (ADR 0052 "Measured versus assumed", git 2.50.1).
#
# WHY IT EXISTS: ADR 0080 D1 says a git update that breaks posse's git-scope
# pins is held back with the package manager's ignore list, never patched
# around. This is the reading that says whether it did. Re-run it on a host
# whose git has moved, before crews run there.
#
#   sh docs/adr/0052-config-based-hooks.probe.sh
#
# RUN IT OUTSIDE A SEAT. Every row sets GIT_CONFIG_SYSTEM, which is exactly
# what the hook-skip arm refuses, so inside a crew session every row comes
# back `refused by posse gate: ... rule: hookskip:config-env` and the shim
# re-pins the global row under it (MEASURED 2026-11-07, this box). That is
# the arm working, not the probe failing: run this as the host's own user, as
# ADR 0080's cage probe is run.
#
# Needs: git, sh. Everything it makes is under one mktemp dir and is removed
# on exit; it writes nothing to the repo, a home, or a persona, and it reads
# no real system or global config (HOME and XDG_CONFIG_HOME are scratch, and
# GIT_CONFIG_SYSTEM is a fixture on every call).
#
# Each row prints  PROBE  EXPECT  GOT. A row whose GOT differs from EXPECT is
# the finding, not a failed run. The EXPECTs are MEASURED 2026-11-07 on the
# ADR 0080 D1 host (Linux x86_64, git 2.56.0, ranger-base-jep08); the pins
# they stand for are TestQATheGitConfigSystemPinCarriesAConfigBasedManagedHook
# (internal/posse/inletpingitconfig_qa_test.go) and the hook-skip arm's
# .enabled legs (internal/posse/hookskip_qa_test.go, refused by the hook.
# prefix in internal/posse/hookskip.go).
set -u
W=$(mktemp -d) || exit 1
trap 'rm -rf "$W"' EXIT
row() { printf '%-56s %-18s %s\n' "$1" "$2" "$3"; }

echo "== host =="
git version
echo "uname: $(uname -sm)"
echo

CFG="$W/cfg.mark"; DIR="$W/dir.mark"
mkdir -p "$W/mhooks" "$W/alt" "$W/h/.config"
printf '#!/bin/sh\necho dir >> %s\n' "$DIR" > "$W/mhooks/pre-commit"
printf '#!/bin/sh\necho cfg >> %s\n' "$CFG" > "$W/scan"
chmod +x "$W/mhooks/pre-commit" "$W/scan"

# Two fixture system files: a config hook alone, and both shapes at once.
cat > "$W/sys-cfg" <<EOF
[hook "scan"]
	command = $W/scan
	event = pre-commit
EOF
cat > "$W/sys-both" <<EOF
[core]
	hooksPath = $W/mhooks
[hook "scan"]
	command = $W/scan
	event = pre-commit
EOF

R="$W/repo"
git init -q "$R" || exit 1
git -C "$R" config user.email qa@example.invalid
git -C "$R" config user.name qa

n=0
# commit <label> <expect> <system-file> [VAR=VAL ...] — prints one row whose
# GOT is "cfg=<yes|no> dir=<yes|no>", plus anything git said (nothing is the
# point: a skipped hook is silent at rc 0).
commit() {
	lbl=$1; want=$2; sys=$3; shift 3
	: > "$CFG"; : > "$DIR"; n=$((n + 1)); f="f$n"
	echo "$f" > "$R/$f"
	out=$(cd "$R" && env HOME="$W/h" XDG_CONFIG_HOME="$W/h/.config" \
		GIT_CONFIG_SYSTEM="$sys" "$@" git add -- "$f" 2>&1 &&
		env HOME="$W/h" XDG_CONFIG_HOME="$W/h/.config" \
			GIT_CONFIG_SYSTEM="$sys" "$@" git commit -q -m "$f" -- "$f" 2>&1)
	rc=$?
	cfg=no; dir=no
	[ -s "$CFG" ] && cfg=yes
	[ -s "$DIR" ] && dir=yes
	got="cfg=$cfg dir=$dir"
	[ "$rc" = 0 ] || got="$got rc=$rc"
	[ -n "$out" ] && got="$got said=$(printf '%s' "$out" | tr '\n' '|' | cut -c1-60)"
	row "$lbl" "$want" "$got"
}

echo "== the pin's two values, over a hook.* managed scanner alone =="
commit "system scope carried (the landed GIT_CONFIG_SYSTEM row)" "cfg=yes dir=no" "$W/sys-cfg"
commit "system scope emptied (the retired /dev/null row)" "cfg=no dir=no" /dev/null
commit "carried + command-scope core.hooksPath (ADR 0052 D2)" "cfg=yes dir=no" "$W/sys-cfg" \
	env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0="$W/alt"

echo
echo "== both shapes carried: a hooks DIRECTORY and a config hook =="
commit "both carried, nothing typed over them" "cfg=yes dir=yes" "$W/sys-both"
commit "both + D2 redirect (-c core.hooksPath=<empty dir>)" "cfg=yes dir=no" "$W/sys-both" \
	env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0="$W/alt"

echo
echo "== the .enabled hide directions, live from 2.54, inert below it =="
commit "both + command-scope hook.pre-commit.enabled=false" "cfg=no dir=yes" "$W/sys-both" \
	env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=hook.pre-commit.enabled GIT_CONFIG_VALUE_0=false
commit "both + command-scope hook.scan.enabled=false" "cfg=no dir=yes" "$W/sys-both" \
	env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=hook.scan.enabled GIT_CONFIG_VALUE_0=false
git -C "$R" config --local hook.pre-commit.enabled false
commit "both + repo-local hook.pre-commit.enabled=false" "cfg=no dir=yes" "$W/sys-both"
git -C "$R" config --local --unset hook.pre-commit.enabled
commit "both, that local key removed again (control)" "cfg=yes dir=yes" "$W/sys-both"

echo
echo "== what posse's readers ask git (the parity row and the two scope rows) =="
echo "-- git config -z --show-scope --show-origin --get-regexp '^(hook\\.|core\\.hookspath)'"
echo "   EXPECT: scope NUL origin NUL key LF value NUL, three entries, all system"
(cd "$R" && env HOME="$W/h" XDG_CONFIG_HOME="$W/h/.config" GIT_CONFIG_SYSTEM="$W/sys-both" \
	git config -z --show-scope --show-origin --get-regexp '^(hook\.|core\.hookspath)' |
	tr '\0' '\n' | sed 's/^/   GOT: /')
echo "-- git var GIT_CONFIG_SYSTEM   EXPECT: the fixture path"
echo "   GOT: $(env HOME="$W/h" GIT_CONFIG_SYSTEM="$W/sys-both" git var GIT_CONFIG_SYSTEM)"
echo "-- git var GIT_CONFIG_GLOBAL   EXPECT: two lines, XDG first"
env HOME="$W/h" XDG_CONFIG_HOME="$W/h/.config" git var GIT_CONFIG_GLOBAL | sed 's/^/   GOT: /'
echo "-- git hook list pre-commit    EXPECT: scan, then the hookdir hook"
(cd "$R" && env HOME="$W/h" GIT_CONFIG_SYSTEM="$W/sys-both" git hook list pre-commit 2>&1 |
	sed 's/^/   GOT: /')
