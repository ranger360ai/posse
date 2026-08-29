#!/usr/bin/env bash
# macos-install-probe.sh — run the macOS install routes and report what they do
# (ranger-base-hza).
#
# The clean room (scripts/cleanroom.sh) is Linux by construction, so the two
# things that actually bit the operator on ranger-base-253 — zsh's PATH and the
# Homebrew route — were written and never run. This is the instrument that runs
# them. It is to macOS what cleanroom.sh is to the four Linux distros: on
# demand, before a release, not per push.
#
# Usage:
#   scripts/macos-install-probe.sh paths        default PATH + which zsh file
#   scripts/macos-install-probe.sh quarantine   Gatekeeper on a downloaded binary
#   scripts/macos-install-probe.sh tap          the published tap, read-only
#   scripts/macos-install-probe.sh brew         tap/trust/install, scratch prefix
#   scripts/macos-install-probe.sh all          all four
#
# Options:
#   --version vX.Y.Z   which release to probe (default: v0.3.0)
#   --keep             do not delete the scratch root on exit
#   --stub-clt-gate    `brew` mode only, and it makes the result NOT a user's
#                      result — see THE CLT GATE below
#
# Exit: 0 every probe agreed with what INSTALL.md says · 1 a probe disagreed ·
# 2 nothing was measured (not macOS, a tool is missing, the network refused).
# Exit 2 is not a pass. Same convention as verify-credential-paths.sh.
#
# BLAST RADIUS. Everything this script writes lives under one scratch root it
# creates and deletes. It never writes $HOME, never touches /opt/homebrew,
# ~/.homebrew/trust.json, ~/.zprofile, ~/.zshrc, or the operator's brew cache,
# and it installs nothing on the box. `brew` mode gets there by cloning
# Homebrew into the scratch root and pointing HOMEBREW_CACHE, HOMEBREW_LOGS,
# HOMEBREW_TEMP, XDG_CONFIG_HOME and HOME at it — XDG_CONFIG_HOME is the one
# that matters, because it is where `brew trust` writes trust.json, and without
# it the probe would grant a formula on the operator's live brew. Network reads
# only. `brew` mode downloads ~200 MB (Homebrew's history and its portable
# ruby); the other three are small.
#
# HOMEBREW_TEMP IS NOT OPTIONAL. brew refuses outright — "Your HOMEBREW_PREFIX
# is in the Homebrew temporary directory" — when the prefix sits under
# $TMPDIR, which on macOS is where a scratch root naturally goes. Pointing
# HOMEBREW_TEMP inside the scratch root is what makes a scratch prefix legal.
#
# THE CLT GATE. The formula ships no bottle, so brew takes its build-from-
# source path (formula_installer.rb, `unless pour_bottle?`) and runs the fatal
# developer-tools diagnostics BEFORE it unpacks anything. On a Mac whose
# Command Line Tools are behind the running macOS, `brew install` dies with
# "Your Command Line Tools are too outdated" without ever reading our formula —
# on the one route INSTALL.md advertises as "a release binary, no Go needed".
# That refusal is a real user's result and this script reports it as a finding,
# not as an error to route around. `--stub-clt-gate` patches that one check out
# of the SCRATCH brew so the rest of the route can be measured on a box that
# cannot pass it; it says so loudly, because a run with it on no longer answers
# the question a user is asking.
set -uo pipefail

VERSION=v0.3.0
KEEP=0
STUB_CLT=0
REPO=ranger360ai/posse
TAP=ranger360ai/tap
# brew's `user/name` shorthand is the repo `user/homebrew-name`. `brew tap`
# expands it; git does not, and a probe that hands the shorthand to git reports
# an unpublished tap for a tap that is published.
TAP_REPO=ranger360ai/homebrew-tap
FORMULA=$TAP/posse

REPO_ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

fail=0
measured=0

say()  { printf '%s\n' "$*"; }
ok()   { measured=1; printf '  ok      %s\n' "$*"; }
bad()  { measured=1; fail=1; printf '  FAIL    %s\n' "$*"; }
note() { printf '  note    %s\n' "$*"; }
skip() { printf '  skip    %s\n' "$*"; }
head_() { printf '\n== %s\n' "$*"; }
die()  { printf 'macos-install-probe: %s\n' "$*" >&2; exit 2; }

MODE=
while [ $# -gt 0 ]; do
	case $1 in
	paths|quarantine|tap|brew|all) MODE=$1; shift ;;
	--version) VERSION=${2:?--version needs a tag}; shift 2 ;;
	--keep) KEEP=1; shift ;;
	--stub-clt-gate) STUB_CLT=1; shift ;;
	-h|--help) sed -n '2,57p' "$0"; exit 0 ;;
	*) die "unknown argument: $1 (try --help)" ;;
	esac
done
[ -n "$MODE" ] || die "name a probe: paths | quarantine | tap | brew | all"
case $VERSION in v[0-9]*) ;; *) die "--version must look like vX.Y.Z, got: $VERSION" ;; esac
BARE=${VERSION#v}

[ "$(uname -s)" = "Darwin" ] || die "this probe measures macOS; on $(uname -s) it would measure nothing"

ROOT=$(mktemp -d "${TMPDIR:-/tmp}/posse-macos-probe.XXXXXX") || die "mktemp failed"
cleanup() {
	# A binary that Gatekeeper blocked is still parked on a suspended exec; it
	# has to go before the directory under it does.
	pkill -9 -f "$ROOT/" 2>/dev/null
	if [ "$KEEP" = 1 ]; then printf '\nscratch root kept: %s\n' "$ROOT"; else rm -rf "$ROOT"; fi
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# paths — the default PATH, and which zsh file a PATH line has to go in
# ---------------------------------------------------------------------------
# macOS builds a login shell's PATH in /etc/zprofile, by eval'ing
# /usr/libexec/path_helper. That is the whole of the default PATH: whatever
# path_helper prints. Two consequences the install story depends on, and one
# that is easy to get backwards.
probe_paths() {
	head_ "paths — the default macOS PATH, and where a PATH line survives"

	local def
	def=$(env -i /usr/libexec/path_helper -s 2>/dev/null | sed -n 's/^PATH="\(.*\)"; export PATH;$/\1/p')
	[ -n "$def" ] || { skip "path_helper printed nothing — cannot measure the default PATH"; return; }
	note "default PATH: $def"

	local d
	for d in "\$HOME/go/bin:$HOME/go/bin" "\$HOME/.local/bin:$HOME/.local/bin"; do
		local label=${d%%:*} real=${d#*:}
		case ":$def:" in
		*":$real:"*) bad "$label IS on the default PATH — the ranger-base-253 premise no longer holds, re-check the quickstarts" ;;
		*) ok "$label is not on the default PATH (so 'go install' / 'make install' need the export INSTALL.md prints)" ;;
		esac
	done

	# Homebrew's own prefix. On Apple Silicon /opt/homebrew/bin is NOT on the
	# default PATH — what puts it there is the `eval "$(brew shellenv)"` line
	# brew's installer prints once and never repeats. INSTALL.md §2's Verify
	# ("`which posse` answers /opt/homebrew/bin/posse") cannot pass without it.
	# On Intel the prefix is /usr/local, which IS on the default PATH, so this
	# failure mode is Apple-Silicon-only and invisible to whoever wrote the page
	# on an Intel Mac.
	local prefix
	if prefix=$(/opt/homebrew/bin/brew --prefix 2>/dev/null || /usr/local/bin/brew --prefix 2>/dev/null); then
		case ":$def:" in
		*":$prefix/bin:"*) ok "$prefix/bin is on the default PATH (Intel-style prefix)" ;;
		*) ok "$prefix/bin is NOT on the default PATH — brew's shellenv line is a prerequisite of INSTALL.md §2's Verify" ;;
		esac
	else
		skip "no brew on this box — cannot measure whether its prefix is on the default PATH"
	fi

	# Which startup file. zsh reads .zshenv always; .zprofile and .zlogin only
	# for login shells; .zshrc only for interactive ones. Crossed with macOS
	# running path_helper from /etc/zprofile — i.e. AFTER .zshenv and BEFORE
	# .zprofile — that gives four different answers, and one of them is a trap:
	# a PATH set in .zshenv survives but is DEMOTED behind the system paths at
	# login, which is the same ambiguity that produced ranger-base-253.
	head_ "paths — which zsh startup file carries a PATH line, per shell kind"
	printf '  %-12s %-14s %-14s %-16s %s\n' file login+inter interactive non-inter login-non-inter
	local f kind row
	for f in .zshenv .zprofile .zshrc .zlogin; do
		row=""
		for kind in "-l -i" "-i" "" "-l"; do
			row=$row$(printf '%-15s' "$(zsh_probe "$f" "$kind")")
		done
		printf '  %-12s %s\n' "$f" "$row"
	done
	# Assert the two rows the docs rest on. Everything else in the table is
	# reported, not judged — it is context for whoever reads a failure.
	local rc_login rc_inter env_login
	rc_login=$(zsh_probe .zshrc "-l -i"); rc_inter=$(zsh_probe .zshrc "-i")
	env_login=$(zsh_probe .zshenv "-l -i")
	if [ "$rc_login" = first ] && [ "$rc_inter" = first ]; then
		ok ".zshrc puts the export first for both shell kinds a person types into"
	else
		bad ".zshrc no longer carries the export (login+interactive=$rc_login interactive=$rc_inter) — INSTALL.md §1 names that file"
	fi
	if [ "$env_login" = demoted ]; then
		ok ".zshenv is demoted behind path_helper at login — the trap INSTALL.md warns about is real"
	else
		bad ".zshenv login result is '$env_login', expected 'demoted' — re-derive the PATH-order warning in INSTALL.md §1"
	fi
}

zsh_probe() { # $1=file $2=shell flags -> first|demoted|not-found
	local h=$ROOT/zp$(printf '%s%s' "$1" "$2" | tr -cd '[:alnum:]')
	rm -rf "$h"; mkdir -p "$h/.local/bin"
	printf '#!/bin/sh\necho HIT\n' > "$h/.local/bin/posse"; chmod +x "$h/.local/bin/posse"
	printf 'export PATH="$HOME/.local/bin:$PATH"\n' > "$h/$1"
	env -i HOME="$h" ZDOTDIR="$h" TERM=dumb /bin/zsh $2 -c \
		'command -v posse >/dev/null || { print not-found; exit }
		 case $PATH in $HOME/.local/bin:*) print first ;; *) print demoted ;; esac' \
		2>/dev/null </dev/null
}

# ---------------------------------------------------------------------------
# quarantine — what Gatekeeper does to a downloaded posse
# ---------------------------------------------------------------------------
# Three arms over three fresh copies of one binary, because the interesting
# result is a HANG and a hang leaves state behind: a copy that has been blocked
# once stays blocked even after the attribute is removed.
#
#   A  no quarantine attribute        must run
#   B  quarantined                    must NOT run (this is the finding)
#   C  quarantined, cleared, unrun    must run (this is the documented fix)
#
# A is the control. Without it a box where the binary simply does not work
# would report B as a Gatekeeper block.
probe_quarantine() {
	head_ "quarantine — Gatekeeper on a downloaded, ad-hoc-signed binary"
	local bin=$ROOT/q/posse
	mkdir -p "$ROOT/q"
	fetch_binary "$bin" || { skip "no binary to probe"; return; }

	note "signature: $(codesign -dv "$bin" 2>&1 | sed -n 's/^Signature=//p' | head -1) (Go ad-hoc signs darwin builds; it is not notarized)"
	note "spctl assessment: $(spctl -a -t execute "$bin" 2>&1 | sed 's/.*: //')"

	local q="0083;$(printf %x "$(date +%s)");Safari;"
	local arm expect got
	for arm in A B C; do
		local p=$ROOT/q/posse-$arm
		cp "$bin" "$p"
		case $arm in
		B) xattr -w com.apple.quarantine "$q" "$p" ;;
		C) xattr -w com.apple.quarantine "$q" "$p"; xattr -d com.apple.quarantine "$p" ;;
		esac
		got=$(run_bounded "$p")
		case $arm in
		A) expect=ran ;; B) expect=blocked ;; C) expect=ran ;;
		esac
		if [ "$got" = "$expect" ]; then
			case $arm in
			A) ok "A no quarantine: $got — control" ;;
			B) ok "B quarantined: $got — a browser download of the tarball does not run, and prints nothing while not running" ;;
			C) ok "C quarantine cleared before first run: $got — \`xattr -d com.apple.quarantine\` is the fix" ;;
			esac
		else
			bad "$arm: expected $expect, got $got"
		fi
	done

	# The tail of the finding: clearing the attribute AFTER a blocked attempt
	# does not give that file back. Whoever hits B and then applies the fix in
	# place is still stuck, and nothing tells them why.
	local p=$ROOT/q/posse-B
	if [ -e "$p" ]; then
		xattr -d com.apple.quarantine "$p" 2>/dev/null
		if [ "$(run_bounded "$p")" = blocked ]; then
			ok "clearing the attribute on an already-blocked file does NOT recover it — re-extract instead"
		else
			note "an already-blocked file recovered after the attribute was cleared; the INSTALL.md 're-extract' sentence can be relaxed"
		fi
	fi
}

# Run `<bin> version` under a wall-clock bound. Gatekeeper does not refuse, it
# suspends: no output, no exit, no error. A plain `if ! out=$(...)` would sit
# there until the caller's own timeout, so the bound is the measurement.
run_bounded() { # $1=binary -> ran|blocked|broken
	local p=$1 i
	( RHQ_HOME=$ROOT/never-an-instance "$p" version >"$p.out" 2>"$p.err"; echo $? >"$p.rc" ) &
	local child=$!
	for i in $(seq 1 12); do kill -0 "$child" 2>/dev/null || break; sleep 1; done
	if kill -0 "$child" 2>/dev/null; then
		kill -9 "$child" 2>/dev/null
		pkill -9 -f "$p" 2>/dev/null
		printf blocked
		return
	fi
	if [ "$(cat "$p.rc" 2>/dev/null)" = 0 ] && grep -q "$BARE" "$p.out" 2>/dev/null; then
		printf ran
	else
		printf broken
	fi
}

# ---------------------------------------------------------------------------
# tap — the published tap, without tapping it
# ---------------------------------------------------------------------------
# `brew tap` clones into the caller's Homebrew prefix, so this reads the tap the
# way anything else reads a git repo. Three questions, in the order they can
# fail: is it published at all; is the formula still the generator's output or
# has someone hand-edited it; do the sha256s in it match the bytes GitHub
# actually serves.
probe_tap() {
	head_ "tap — the published formula, read-only"
	command -v git >/dev/null || { skip "no git"; return; }

	local tapdir=$ROOT/tap
	if ! git clone --depth 1 -q "https://github.com/$TAP_REPO.git" "$tapdir" 2>/dev/null; then
		bad "https://github.com/$TAP_REPO is not readable — INSTALL.md §2 advertises 'brew tap $TAP', and until a release is cut brew answers 'Repository not found'"
		return
	fi
	ok "https://github.com/$TAP_REPO is published (the repo behind 'brew tap $TAP')"
	local published=$tapdir/Formula/posse.rb
	[ -f "$published" ] || { bad "the tap has no Formula/posse.rb"; return; }

	# Regenerate from the shas the published formula itself carries. If the
	# generator and the tap disagree, one of them was edited by hand — which is
	# the failure the generator exists to make unavailable.
	local sums=$ROOT/checksums.txt plat
	: > "$sums"
	for plat in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
		local sha
		sha=$(awk -v u="posse_${BARE}_${plat}.tar.gz" '
			$1 == "url" && index($0, u) { want = 1; next }
			want && $1 == "sha256" { gsub(/"/, "", $2); print $2; exit }' "$published")
		[ -n "$sha" ] || { bad "the published formula carries no sha256 for $plat"; return; }
		printf '%s  posse_%s_%s.tar.gz\n' "$sha" "$BARE" "$plat" >> "$sums"
	done
	# Render with the generator AS IT WAS AT THE RELEASE TAG, not as it is at
	# HEAD. The published formula was rendered when the release was cut, so
	# comparing it to HEAD's generator reports every later generator change as
	# tap drift — a red instrument that is right about nothing.
	local gen=$ROOT/tap-formula.sh gensrc
	if ( cd "$REPO_ROOT" && git rev-parse -q --verify "$VERSION" >/dev/null 2>&1 ) &&
		( cd "$REPO_ROOT" && git show "$VERSION:scripts/tap-formula.sh" ) > "$gen" 2>/dev/null; then
		gensrc="scripts/tap-formula.sh at $VERSION"
	else
		cp "$REPO_ROOT/scripts/tap-formula.sh" "$gen"
		gensrc="scripts/tap-formula.sh at HEAD (no $VERSION tag here)"
	fi
	local regenerated=$ROOT/regenerated.rb
	if sh "$gen" --version "$VERSION" --checksums "$sums" --repo "$REPO" > "$regenerated" 2>/dev/null; then
		if diff -q "$regenerated" "$published" >/dev/null; then
			ok "the published formula is byte-identical to $gensrc — no hand edit"
		else
			bad "the published formula differs from $gensrc:"
			diff "$regenerated" "$published" | sed 's/^/            /'
		fi
	else
		skip "could not render the formula locally"
	fi

	# The sha256s, against the bytes GitHub serves. A formula with one stale sha
	# fails `brew install` for exactly one architecture — the one the person who
	# cut the release does not use — so all four are checked, not this one.
	command -v curl >/dev/null || { skip "no curl — sha256s unverified"; return; }
	for plat in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
		local want got tgz=$ROOT/$plat.tar.gz
		want=$(awk -v p="posse_${BARE}_${plat}.tar.gz" '$2 == p { print $1 }' "$sums")
		if ! curl -fsSL -o "$tgz" "https://github.com/$REPO/releases/download/$VERSION/posse_${BARE}_${plat}.tar.gz"; then
			bad "$plat: the formula's url is not downloadable"
			continue
		fi
		got=$(shasum -a 256 "$tgz" | awk '{print $1}')
		if [ "$want" = "$got" ]; then ok "$plat: sha256 matches the published asset"
		else bad "$plat: sha256 mismatch — formula $want, asset $got"; fi
	done

	# Both macOS architectures, executed. amd64 needs Rosetta; where it is
	# absent that is a skip, not a failure — it is the box that cannot answer.
	for plat in darwin_arm64 darwin_amd64; do
		local d=$ROOT/x-$plat
		mkdir -p "$d"
		tar xzf "$ROOT/$plat.tar.gz" -C "$d" posse 2>/dev/null || { skip "$plat: no posse in the tarball"; continue; }
		if [ "$plat" = darwin_amd64 ] && [ "$(uname -m)" = arm64 ] && [ ! -d /Library/Apple/usr/libexec/oah ]; then
			skip "darwin_amd64: Rosetta is not installed on this box, so the Intel binary cannot be run here"
			continue
		fi
		case "$(run_bounded "$d/posse")" in
		ran) ok "$plat: the released binary runs here and reports $BARE" ;;
		blocked) bad "$plat: the released binary was blocked (Gatekeeper?)" ;;
		*) bad "$plat: the released binary did not report $BARE" ;;
		esac
	done
}

fetch_binary() { # $1=destination
	local plat=darwin_arm64
	[ "$(uname -m)" = arm64 ] || plat=darwin_amd64
	command -v curl >/dev/null || return 1
	curl -fsSL -o "$ROOT/fetch.tar.gz" \
		"https://github.com/$REPO/releases/download/$VERSION/posse_${BARE}_${plat}.tar.gz" || return 1
	tar xzf "$ROOT/fetch.tar.gz" -C "$(dirname "$1")" posse || return 1
	[ -x "$1" ]
}

# ---------------------------------------------------------------------------
# brew — the three commands of INSTALL.md §2, against a scratch prefix
# ---------------------------------------------------------------------------
probe_brew() {
	head_ "brew — tap, trust and install against a scratch Homebrew prefix"
	command -v git >/dev/null || { skip "no git"; return; }

	local B=$ROOT/brew
	mkdir -p "$B/home" "$B/cfg" "$B/cache" "$B/logs" "$B/tmp"
	say "  cloning Homebrew into the scratch root (~200 MB, once per run)"
	git clone --depth=1 -q https://github.com/Homebrew/brew "$B/prefix" 2>/dev/null || { skip "could not clone Homebrew"; return; }
	# Shallow, brew reports ">=4.3.0 (shallow or no git repository)" and the
	# probe would be measuring a version nobody runs. The history is what makes
	# it answer with the same version string as the box's own brew.
	( cd "$B/prefix" && git fetch -q --unshallow --tags 2>/dev/null )

	export HOME="$B/home" XDG_CONFIG_HOME="$B/cfg"
	export HOMEBREW_CACHE="$B/cache" HOMEBREW_LOGS="$B/logs" HOMEBREW_TEMP="$B/tmp"
	export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1
	local brew=$B/prefix/bin/brew
	note "scratch brew: $("$brew" --version 2>/dev/null | head -1)"

	if [ "$STUB_CLT" = 1 ]; then
		local diag=$B/prefix/Library/Homebrew/extend/os/mac/diagnostic.rb
		if /usr/bin/python3 - "$diag" "$B/skip-clt" <<'PY'
import sys
path, marker = sys.argv[1], sys.argv[2]
src = open(path).read()
anchor = "        def check_clt_minimum_version\n"
assert anchor in src, "check_clt_minimum_version moved; the stub has to be re-aimed"
open(path, "w").write(src.replace(anchor, anchor + '          return if File.exist?("' + marker + '")\n', 1))
PY
		then
			: > "$B/skip-clt"
			say "  WARNING : --stub-clt-gate is on. brew's Command Line Tools check is patched out"
			say "            of the scratch clone, so what follows is NOT what a user with this"
			say "            box's developer tools would get. The CLT finding below is suppressed."
		else
			skip "could not stub the CLT gate — brew's diagnostic moved"
		fi
	fi

	"$brew" tap "$TAP" >"$ROOT/tap.log" 2>&1
	if grep -qi "Tapped\|already tapped" "$ROOT/tap.log"; then
		ok "brew tap $TAP"
	else
		bad "brew tap $TAP failed: $(tail -3 "$ROOT/tap.log" | tr '\n' ' ')"
		return
	fi

	# INSTALL.md §2 says tap-info reads Untrusted before the grant. It also
	# reads Untrusted AFTER it: `brew trust --formula` is a FORMULA grant and
	# tap-info reports TAP trust. The narrow grant we recommend never flips it,
	# so a reader who checks tap-info to confirm the trust line worked will
	# conclude it did not.
	local before after
	before=$("$brew" tap-info "$TAP" 2>/dev/null | sed -n '2p')
	"$brew" trust --formula "$FORMULA" >"$ROOT/trust.log" 2>&1
	local trustline; trustline=$(tail -1 "$ROOT/trust.log")
	case $trustline in
	"Trusted formula: $FORMULA"|"Already trusted formula: $FORMULA")
		ok "brew trust --formula $FORMULA -> $trustline" ;;
	*) bad "brew trust --formula printed: $trustline" ;;
	esac
	if [ -f "$B/cfg/homebrew/trust.json" ]; then
		ok "the grant is one string in \$XDG_CONFIG_HOME/homebrew/trust.json: $(tr -d ' \n' < "$B/cfg/homebrew/trust.json")"
	else
		bad "no trust.json under \$XDG_CONFIG_HOME — find out where the grant went before trusting anything on a live box"
	fi
	after=$("$brew" tap-info "$TAP" 2>/dev/null | sed -n '2p')
	if [ "$before" = Untrusted ] && [ "$after" = Untrusted ]; then
		ok "tap-info reads Untrusted before AND after the formula grant — it reports tap trust, not formula trust"
	else
		note "tap-info: before=$before after=$after (INSTALL.md §2 describes 'Untrusted' both times)"
	fi

	"$brew" install "$FORMULA" >"$ROOT/install.log" 2>&1
	if grep -q "Command Line Tools are too outdated" "$ROOT/install.log"; then
		bad "brew install $FORMULA refused: this box's Command Line Tools are behind macOS $(sw_vers -productVersion)."
		note "the formula ships no bottle, so brew runs its fatal build-from-source"
		note "diagnostics before unpacking a prebuilt binary. The fix is the operator's"
		note "(Software Update, or sudo xcode-select --install). Re-run with"
		note "--stub-clt-gate to measure the rest of the route anyway."
		return
	fi
	if ! grep -q "Cellar/posse" "$ROOT/install.log"; then
		bad "brew install $FORMULA did not install: $(tail -3 "$ROOT/install.log" | tr '\n' ' ')"
		return
	fi
	ok "brew install $FORMULA"

	local installed=$B/prefix/bin/posse
	if [ ! -x "$installed" ]; then bad "no posse in the scratch prefix's bin"; return; fi
	case "$(run_bounded "$installed")" in
	ran) ok "the brew-installed posse reports $BARE: $(cat "$installed.out")" ;;
	*) bad "the brew-installed posse did not report $BARE" ;;
	esac
	if [ "$(xattr "$installed" | grep -c com.apple.quarantine)" = 0 ]; then
		ok "brew's download carries no com.apple.quarantine — the brew route never meets Gatekeeper"
	else
		bad "the brew-installed binary is quarantined"
	fi
	if grep -q "brew install beads" "$ROOT/install.log"; then
		ok "the caveats print, and they still name 'brew install beads' as the wrong bd"
	else
		bad "the formula's caveats did not print — the bd 1.2.x warning is how a new installer learns the pin"
	fi
}

case $MODE in
paths) probe_paths ;;
quarantine) probe_quarantine ;;
tap) probe_tap ;;
brew) probe_brew ;;
all) probe_paths; probe_quarantine; probe_tap; probe_brew ;;
esac

printf '\n'
if [ "$measured" = 0 ]; then
	say "nothing was measured — that is exit 2, not a pass"
	exit 2
fi
if [ "$fail" = 0 ]; then
	say "macos-install-probe: every probe agreed with INSTALL.md"
	exit 0
fi
say "macos-install-probe: a probe disagreed with INSTALL.md — read the FAIL lines above"
exit 1
