#!/bin/sh
# What the commit wall's MESSAGE and IDENTITY arms rest on, asked of the box
# that runs this (ADR 0083's cutover host; the arms are ADR 0024 D2 check 3,
# ADR 0048 D2 and ADR 0050 D2). Written for ranger-base-sydnc, where ten of
# those arms' pins failed on the always-on Linux host while the same pins were
# green in CI's Linux and on the laptop.
#
#   sh docs/adr/0083-commit-wall-host-premises.probe.sh
#
# Needs: sh, git, python3 is NOT needed and no Go toolchain is needed — this
# runs on a host that has neither. Run it as the account the crew will run as,
# from inside a checkout of this repo (section B reads two tracked files).
# Everything it makes is under one mktemp dir and is removed on exit; nothing
# here writes to the repo, a home, or a persona.
#
# Each line prints  PROBE  EXPECT  GOT. A row whose GOT differs from EXPECT is
# the finding, not a failed run. NO VALUE IS EVER PRINTED for an account name,
# an e-mail address or a config value: those are the very class check 3 exists
# to keep out of this repo, so every row that reads one answers YES/NO about it
# instead (ADR 0024 D3). Section A's first rows print versions and paths only.
#
# EXPECT column, MEASURED 2026-11-09 on the laptop (ranger-base-sydnc):
# darwin 25.6.0 arm64, against BOTH git 2.50.1 (Apple Git-155) and a scratch
# git 2.56.0 keg — every C row read the same under the two gits, which is why
# the C rows carry one EXPECT and not two.
set -u
W=$(mktemp -d) || exit 1
trap 'rm -rf "$W"' EXIT
REPO=$(git rev-parse --show-toplevel 2>/dev/null) || REPO=''

row() { printf '%-54s %-22s %s\n' "$1" "$2" "$3"; }

# ─── A. the box ──────────────────────────────────────────────────────────
echo '--- A. the box'
row "A1 git version" "(2.56.0 on the host)" "$(git --version 2>&1)"
row "A2 kernel/arch" "(Linux x86_64)" "$(uname -s -m 2>&1)"
row "A3 /bin/sh is" "(any POSIX sh)" "$(readlink /bin/sh 2>/dev/null || echo 'not a symlink')"
# A4-A6: a git config SCOPE the hand-run binary inherits is the one thing a
# `go test` under CI cannot have. The fixtures scrub $HOME and nothing else
# (newVisWallCfg), so an inherited scope reaches the literal derivation.
# A posse-launched SEAT carries GIT_CONFIG_GLOBAL and GIT_CONFIG_SYSTEM by
# design (ranger-base-obdnk, ranger-base-t301k), so "set" is correct inside a
# seat and the EXPECT below is for the hand-run the lane arm does.
for v in GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_COUNT XDG_CONFIG_HOME; do
	eval "set_or_not=\${$v+set}"
	row "A4 $v" "unset" "${set_or_not:-unset}"
done
row "A5 git's system scope" "(one path)" "$(git var GIT_CONFIG_SYSTEM 2>&1)"
sys=$(git var GIT_CONFIG_SYSTEM 2>/dev/null)
if [ -n "$sys" ] && [ -f "$sys" ]; then
	row "A5b system scope sets these KEYS" "(none)" "$(git config -f "$sys" --name-only --list 2>/dev/null | sort -u | tr '\n' ' ')"
else
	row "A5b system scope sets these KEYS" "(none)" "no such file"
fi
row "A6 EDITOR set" "(either)" "$(if [ -n "${EDITOR+x}" ]; then echo set; else echo unset; fi)"

# ─── B. the literal the wall is built from ───────────────────────────────
# check 3's identity literals are the box's own username, every scope's
# user.email, and the instance repo's path. The username is a BOUNDARY-FREE
# fixed string (identityLiteralERE), so a username that occurs in this
# repository's own vocabulary refuses this repository's own commits, with no
# remedy ADR 0024 D3 can offer. ADR 0080 D2 names the host's account.
echo "--- B. the derived literals against this repo's own vocabulary"
u=$(id -un 2>/dev/null)
row "B1 an account name was read" "YES" "$([ -n "$u" ] && echo YES || echo NO)"
hit() { # hit <needle> <file> -> YES/NO, never the needle
	[ -n "$1" ] && [ -f "$2" ] && grep -qF -- "$1" "$2" && echo YES || echo NO
}
if [ -n "$REPO" ]; then
	row "B2 that name occurs in a tracked PATH" "NO" "$(git -C "$REPO" ls-files | grep -qF -- "$u" && echo YES || echo NO)"
	row "B3 that name occurs in go.mod" "NO" "$(hit "$u" "$REPO/go.mod")"
	row "B4 that name occurs in this probe's own dir listing" "NO" "$(ls "$REPO/docs/adr" | grep -qF -- "$u" && echo YES || echo NO)"
else
	for n in B2 B3 B4; do row "$n (needs a checkout)" "NO" "no checkout here"; done
fi
# The e-mail the fixtures' own HOME shape would derive: scrub $HOME the way
# newVisWallCfg does and ask every scope, as DeriveIdentityLiterals does.
# B5 is "(either)" on purpose and is not the finding: a posse-launched seat
# carries GIT_CONFIG_GLOBAL, so the laptop reads YES there and a CI runner
# reads NO, and both are green. B6 is the row that matters — an address this
# repository's own paths carry is a literal with no remedy.
mkdir -p "$W/h"
em=$(HOME="$W/h" git -C "$W" config --get-all user.email 2>/dev/null | head -1)
row "B5 a user.email survives a scrubbed HOME" "(either)" "$([ -n "$em" ] && echo YES || echo NO)"
if [ -n "$em" ] && [ -n "$REPO" ]; then
	row "B6 that address occurs in a tracked PATH" "NO" "$(git -C "$REPO" ls-files | grep -qF -- "$em" && echo YES || echo NO)"
else
	row "B6 that address occurs in a tracked PATH" "NO" "no address to test"
fi

# ─── C. the premises the message arm's pins assert ───────────────────────
echo '--- C. what git does with a commit message here'
export HOME="$W/h"
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
ED="$W/ed.sh"
printf '#!/bin/sh\n{ printf "%%s\\n" "wire it"; cat "$1"; } > "$1.new" && mv "$1.new" "$1"\n' > "$ED"
chmod +x "$ED"
export GIT_EDITOR="$ED"
printf '# a heading\n' > "$W/tmpl"

newrepo() { # newrepo <name> -> prints the repo dir
	d="$W/$1"
	mkdir -p "$d"
	( cd "$d" && git init -q -b main . ) >/dev/null 2>&1
	printf 'package posse\n' > "$d/staged.txt"
	( cd "$d" && git add -- staged.txt ) >/dev/null 2>&1
	printf 'untracked\n' > "$d/stray-notes.txt"
	printf '%s' "$d"
}

d=$(newrepo verbatim)
( cd "$d" && git config commit.cleanup verbatim && git commit -q -- staged.txt ) >/dev/null 2>&1
body=$( cd "$d" && git log -1 --format=%B 2>/dev/null )
row "C1 verbatim KEEPS git's own template in the object" "YES" "$(printf '%s' "$body" | grep -qF '# On branch ' && echo YES || echo NO)"
row "C2 ... including the UNTRACKED file's name" "YES" "$(printf '%s' "$body" | grep -qF 'stray-notes.txt' && echo YES || echo NO)"

d=$(newrepo autotmpl)
( cd "$d" && git config core.commentChar auto && git config commit.template "$W/tmpl" && git commit -q -- staged.txt ) >/dev/null 2>&1
row "C3 commentChar=auto wrote a ';' template" "YES" "$(grep -q '^; ' "$d/.git/COMMIT_EDITMSG" 2>/dev/null && echo YES || echo NO)"
row "C4 ... with at least 4 lines starting ';'" "YES" "$([ "$(cut -c1 "$d/.git/COMMIT_EDITMSG" 2>/dev/null | grep -c '^;$')" -ge 4 ] && echo YES || echo NO)"
row "C5 that editor commit LANDED" "YES" "$( ( cd "$d" && git rev-parse -q --verify HEAD ) >/dev/null 2>&1 && echo YES || echo NO)"

d=$(newrepo scissors)
( cd "$d" && git config commit.cleanup scissors && git commit -q -- staged.txt ) >/dev/null 2>&1
row "C6 scissors TRUNCATED the untracked name" "YES" "$( ( cd "$d" && git log -1 --format=%B ) 2>/dev/null | grep -qF 'stray-notes.txt' && echo NO || echo YES)"
row "C7 a '>8' cut line was written" "YES" "$(grep -q '>8' "$d/.git/COMMIT_EDITMSG" 2>/dev/null && echo YES || echo NO)"

d=$(newrepo verbose)
( cd "$d" && git config core.commentChar auto && git config commit.template "$W/tmpl" && git config commit.verbose true && git commit -q -- staged.txt ) >/dev/null 2>&1
row "C8 commit.verbose appended the diff after the block" "YES" "$(grep -qF 'diff --git ' "$d/.git/COMMIT_EDITMSG" 2>/dev/null && echo YES || echo NO)"

row "C9 stripspace under auto KEEPS a '#' line" "NO" "$(printf '#x\nkeep\n' | git -c core.commentChar=auto stripspace --strip-comments 2>/dev/null | grep -q '^#x$' && echo YES || echo NO)"

d=$(newrepo dashm)
( cd "$d" && git config commit.cleanup verbatim && git commit -q -m typed -- staged.txt ) >/dev/null 2>&1
row "C10 -m under verbatim appended a template" "NO" "$(grep -qF '# On branch ' "$d/.git/COMMIT_EDITMSG" 2>/dev/null && echo YES || echo NO)"
