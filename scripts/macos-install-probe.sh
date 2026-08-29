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
#   scripts/macos-install-probe.sh bottle       THIS release's bottles, end to end
#   scripts/macos-install-probe.sh all          all five
#
# Options:
#   --version vX.Y.Z   which release to probe (default: v0.3.0). `bottle` mode
#                      ignores it and uses internal/rhq.Version at HEAD, which
#                      is the only version it can build.
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
# and it installs nothing on the box. The one exception is `bottle` mode, which
# runs scripts/release-artifacts.sh: that adds a detached git worktree of HEAD
# under $TMPDIR and removes it on exit, and it runs `go build` four times. It
# still writes nothing into the repo and nothing into any live Homebrew.
# `bottle` mode also listens on a loopback port for the length of the probe —
# it has to, because a Homebrew `root_url` is a URL and the bottles under test
# are not published yet. `brew` mode gets there by cloning
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
# THE CLT GATE. A formula with no bottle makes brew take its build-from-source
# path (formula_installer.rb, `unless pour_bottle?`) and run the fatal
# developer-tools diagnostics BEFORE it unpacks anything. On a Mac whose
# Command Line Tools are behind the running macOS, `brew install` dies with
# "Your Command Line Tools are too outdated" without ever reading our formula —
# on the one route INSTALL.md advertises as "a release binary, no Go needed".
# That refusal is a real user's result and this script reports it as a finding,
# not as an error to route around. `--stub-clt-gate` patches that one check out
# of the SCRATCH brew so the rest of the route can be measured on a box that
# cannot pass it; it says so loudly, because a run with it on no longer answers
# the question a user is asking.
#
# ranger-base-9vg3 is the fix: the release now ships bottles, so brew pours and
# never enters that path. `bottle` mode is the arm that proves it for the
# CURRENT tree, without waiting for a release to be cut and published — and it
# runs its own control, a second install with the bottle block deleted, so a
# green result cannot come from a box that would have installed either way.
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
	paths|quarantine|tap|brew|bottle|all) MODE=$1; shift ;;
	--version) VERSION=${2:?--version needs a tag}; shift 2 ;;
	--keep) KEEP=1; shift ;;
	--stub-clt-gate) STUB_CLT=1; shift ;;
	-h|--help) sed -n '2,69p' "$0"; exit 0 ;;
	*) die "unknown argument: $1 (try --help)" ;;
	esac
done
[ -n "$MODE" ] || die "name a probe: paths | quarantine | tap | brew | bottle | all"
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
	# The four BOTTLE digests too (ranger-base-w69s). Since ranger-base-9vg3 the
	# generator refuses a checksums file missing one, so a four-line manifest
	# makes it exit 1 and this whole arm degraded to `skip` — silently, on every
	# bottled release, while the summary still printed "every probe agreed".
	# The anti-hand-edit check is the strongest thing `tap` mode does; it has to
	# be reconstructible or it is not being run.
	local btag
	for btag in arm64_sonoma sonoma arm64_linux x86_64_linux; do
		local bsha
		# `sonoma:` is a suffix of `arm64_sonoma:`, so the tag is anchored on the
		# character before it. Unanchored, the sonoma lookup takes whichever of
		# the two lines comes first and the two digests silently swap.
		bsha=$(awk -v t="$btag" '
			$0 ~ ("(^|[ ,])" t ":") { if (match($0, /[a-f0-9]{64}/)) { print substr($0, RSTART, RLENGTH); exit } }' "$published")
		[ -n "$bsha" ] || { bad "the published formula carries no bottle sha256 for $btag — brew has no prebuilt keg for that platform and will build from source there"; return; }
		printf '%s  posse-%s.%s.bottle.tar.gz\n' "$bsha" "$BARE" "$btag" >> "$sums"
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

	# The four bottles, against the bytes GitHub serves (ranger-base-w69s). The
	# byte-identity arm above cannot see a doctored digest: it reconstructs the
	# manifest FROM the formula under test, so a hand-edited sha256 renders back
	# into itself and the diff stays silent — measured. Only this loop, which
	# fetches the asset the digest claims to describe, can. It is the bottles'
	# half of the check the four tarballs already got, and it is the one that
	# matters for a pour: brew verifies the bottle it downloads against the
	# formula, so one wrong bottle digest breaks `brew install` on exactly one
	# platform — the one whoever cut the release does not run.
	for btag in arm64_sonoma sonoma arm64_linux x86_64_linux; do
		local bwant bgot bfile=posse-$BARE.$btag.bottle.tar.gz
		bwant=$(awk -v p="$bfile" '$2 == p { print $1 }' "$sums")
		if ! curl -fsSL -o "$ROOT/$bfile" "https://github.com/$REPO/releases/download/$VERSION/$bfile"; then
			bad "$btag: the bottle the formula names is not downloadable from the release — brew would build from source on that platform"
			continue
		fi
		bgot=$(shasum -a 256 "$ROOT/$bfile" | awk '{print $1}')
		if [ "$bwant" = "$bgot" ]; then ok "$btag: the bottle's sha256 matches the published asset"
		else bad "$btag: bottle sha256 mismatch — formula $bwant, asset $bgot"; fi
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

	# THE VERSION BREW RESOLVES, asked before the install (ranger-base-w69s).
	# The formula carries no `version` stanza on purpose (scripts/tap-formula.sh),
	# so brew scans one out of the url — and that scan is a property of the brew
	# on the box, not of the formula. Homebrew <= 6.0.13 reads `64` out of
	# `posse_X.Y.Z_darwin_arm64.tar.gz`; 6.0.14 added a releases/download parser
	# and reads X.Y.Z. Because the bottle filename interpolates whatever it
	# scanned, a box on the older brew asks for posse-64.<tag>.bottle.tar.gz and
	# `brew install` dies on a 404 that names our release rather than its brew.
	# Ask first, so such a box says which of the two it is.
	local resolved
	# `.*stable`, not `: stable`: where a keg is already in the prefix brew
	# writes `==> <formula>: <installed> -> stable <version>`, and a pattern
	# anchored on the colon reads nothing there — which lands a real mismatch in
	# the "could not read" arm and reports the wrong defect. Measured on 6.0.13
	# with posse already installed.
	resolved=$("$brew" info --formula "$FORMULA" 2>/dev/null |
		sed -n "s|^==> $FORMULA:.*stable \([^ ]*\).*|\1|p" | head -1)
	if [ "$resolved" = "$BARE" ]; then
		ok "brew resolves this formula's version as $BARE — the bottle url it builds will name a published asset"
	elif [ -z "$resolved" ]; then
		bad "could not read a version out of \`brew info $FORMULA\` — the check below cannot discriminate"
	else
		bad "brew resolves this formula's version as '$resolved', not $BARE. It scans the version out of the url, and this brew ($("$brew" --version 2>/dev/null | head -1)) predates the releases/download parser (Homebrew 6.0.14). \`brew install\` will 404 on posse-$resolved.<tag>.bottle.tar.gz. Fix: a \`version \"$BARE\"\` stanza in the rendered formula, or tell the reader to \`brew update\` first."
	fi

	"$brew" install "$FORMULA" >"$ROOT/install.log" 2>&1
	if grep -q "Command Line Tools are too outdated" "$ROOT/install.log"; then
		bad "brew install $FORMULA refused: this box's Command Line Tools are behind macOS $(sw_vers -productVersion)."
		note "The PUBLISHED formula ships no bottle, so brew runs its fatal"
		note "build-from-source diagnostics before unpacking a prebuilt binary."
		note "ranger-base-9vg3 fixed the generator; a release cut and pushed into the"
		note "tap since then clears this. Until one is, that is what this line means."
		note "\`macos-install-probe.sh bottle\` measures the fix on THIS tree without"
		note "waiting for a release; --stub-clt-gate measures the rest of this route."
		return
	fi
	if ! grep -q "Cellar/posse" "$ROOT/install.log"; then
		bad "brew install $FORMULA did not install: $(tail -3 "$ROOT/install.log" | tr '\n' ' ')"
		return
	fi
	ok "brew install $FORMULA"

	# POURED or BUILT is the whole of ranger-base-9vg3, and on a box with
	# current developer tools both look like a successful install. So it is
	# asked directly: a published formula that stopped shipping bottles would
	# otherwise regress silently and only for people with stale tools — the
	# exact population that cannot report it.
	if grep -q "Pouring posse-" "$ROOT/install.log"; then
		ok "the published formula POURED a bottle: $(grep -o 'Pouring posse-[^ ]*' "$ROOT/install.log" | head -1)"
	else
		bad "the published formula installed WITHOUT pouring a bottle — brew took its build-from-source path, which is fatal on a Mac whose Command Line Tools are behind its macOS (ranger-base-9vg3). Re-render the tap from scripts/tap-formula.sh."
	fi

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

# ---------------------------------------------------------------------------
# bottle — the bottles THIS tree builds, poured into a scratch prefix
# ---------------------------------------------------------------------------
# `brew` mode above measures the PUBLISHED tap, so it cannot answer anything
# about a fix until a release has been cut, drafted, published and pushed into
# the tap — four operator steps. This mode answers it now, for the tree in
# front of you, by running the real generators and pouring what they produce:
#
#   scripts/release-artifacts.sh   builds the four tarballs AND the four bottles
#   scripts/tap-formula.sh         renders the formula, bottle block included
#   a loopback HTTP server          stands in for the GitHub release the bottles
#                                   are not on yet — a Homebrew root_url is a URL
#   a scratch Homebrew + local tap  installs it the way a user would
#
# The ONLY thing edited between the generator and brew is the base URL: the
# rendered `root_url` and the four source `url`s are pointed at the loopback
# server. Every sha256, every bottle tag, every filename and the whole shape of
# the formula are the generator's own output, because those are the things
# under test. The substitution is counted, and a count that is not 5 is a
# failure rather than a silent no-op.
#
# AND IT RUNS ITS OWN CONTROL. After the poured install succeeds, the same
# formula is re-installed with the bottle block deleted. On a box whose Command
# Line Tools are behind its macOS that second install must DIE at the gate —
# that is what makes the first result mean something. On a box with current
# tools it will install anyway, and the probe says so instead of claiming a
# discrimination it did not get.
probe_bottle() {
	head_ "bottle — this tree's bottles, generated, served and poured"
	command -v git >/dev/null || { skip "no git"; return; }
	command -v /usr/bin/python3 >/dev/null || { skip "no /usr/bin/python3 — nothing to serve the bottles with"; return; }
	command -v "${GOBIN:-go}" >/dev/null || { skip "no go — the artifacts cannot be built here"; return; }

	# The version is the SOURCE's, read the way release-artifacts.sh reads it
	# (from the commit, not the working tree) so the two cannot disagree and
	# turn a version mismatch into a confusing build refusal.
	local ver
	ver=$(git -C "$REPO_ROOT" show HEAD:internal/rhq/app.go 2>/dev/null |
		sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
	[ -n "$ver" ] || { skip "could not read internal/rhq.Version at HEAD"; return; }
	note "building v$ver from HEAD ($(git -C "$REPO_ROOT" rev-parse --short HEAD))"

	local D=$ROOT/bottle
	mkdir -p "$D"
	if ! ( cd "$REPO_ROOT" && "$REPO_ROOT/scripts/release-artifacts.sh" \
		--rev HEAD --version "v$ver" --out "$D/dist" ) >"$ROOT/artifacts.log" 2>&1; then
		bad "scripts/release-artifacts.sh failed: $(tail -3 "$ROOT/artifacts.log" | tr '\n' ' ')"
		return
	fi

	# The bottle this box would be served. One tag per arch, and brew falls back
	# to an older macOS tag on its own, so sonoma is what a Tahoe box pours.
	local tag=sonoma
	[ "$(uname -m)" = arm64 ] && tag=arm64_sonoma
	local bottle=$D/dist/posse-$ver.$tag.bottle.tar.gz
	if [ -f "$bottle" ]; then
		ok "release-artifacts.sh wrote $(basename "$bottle") ($(wc -c <"$bottle" | tr -d ' ') bytes)"
	else
		bad "no bottle for this box's tag: expected $(basename "$bottle"), got: $(ls "$D/dist" | tr '\n' ' ')"
		return
	fi

	# The keg layout has to be `<name>/<version>/…` and it has to match what the
	# formula's `def install` would leave behind, or a poured install and a
	# source install differ in their contents and nobody finds out.
	local listing
	listing=$(tar tzf "$bottle" | sed 's:/$::' | sort)
	local want
	for want in "posse/$ver/bin/posse" "posse/$ver/share/doc/posse/README.md" "posse/$ver/share/doc/posse/INSTALL.md"; do
		case $listing in
		*"$want"*) ok "the bottle holds $want" ;;
		*) bad "the bottle is missing $want — a poured install would differ from a source one" ;;
		esac
	done

	# Serve it. A Homebrew root_url is a URL, and these bottles are not on a
	# release yet, so loopback is the only honest stand-in.
	local port
	port=$(/usr/bin/python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()' 2>/dev/null)
	[ -n "$port" ] || { skip "could not find a free loopback port"; return; }
	/usr/bin/python3 -m http.server "$port" --bind 127.0.0.1 --directory "$D/dist" >"$ROOT/http.log" 2>&1 &
	local server=$!
	disown "$server" 2>/dev/null
	local i
	for i in 1 2 3 4 5 6 7 8 9 10; do
		curl -fsS -o /dev/null "http://127.0.0.1:$port/checksums.txt" 2>/dev/null && break
		sleep 1
	done
	if ! curl -fsS -o /dev/null "http://127.0.0.1:$port/checksums.txt" 2>/dev/null; then
		kill "$server" 2>/dev/null
		skip "the loopback server did not come up on $port"
		return
	fi

	# Render, then repoint the base URL and NOTHING else.
	if ! "$REPO_ROOT/scripts/tap-formula.sh" --version "v$ver" \
		--checksums "$D/dist/checksums.txt" --repo "$REPO" \
		--out "$D/dist/posse.rb" >"$ROOT/render.log" 2>&1; then
		kill "$server" 2>/dev/null
		bad "scripts/tap-formula.sh failed: $(tail -2 "$ROOT/render.log" | tr '\n' ' ')"
		return
	fi
	local base=https://github.com/$REPO/releases/download/v$ver
	local rewrites
	rewrites=$(grep -c -- "$base" "$D/dist/posse.rb")
	if [ "$rewrites" != 5 ]; then
		kill "$server" 2>/dev/null
		bad "expected 5 references to the release base URL (root_url + four urls), found $rewrites — the substitution below would be measuring the wrong formula"
		return
	fi
	# ONLY root_url moves. The four source `url`s stay on github, and they must:
	# this formula carries no `version` stanza on purpose (ranger-base-hza), so
	# brew SCANS the version out of the first url — and Version.detect only
	# recognises the GitHub *release* URL shape. Point a source url at a bare
	# host and `posse_0.3.0_darwin_arm64.tar.gz` resolves to version "64"
	# (measured, both with and without a /v0.3.0/ path segment), after which brew
	# asks for `posse-64.<tag>.bottle.tar.gz` and everything 404s. Nothing
	# downloads those urls on this arm anyway — a poured install never fetches
	# the source — so leaving them alone costs nothing and keeps the version, the
	# bottle filename and the pour all being the generator's own answer.
	sed -i '' "s|root_url \"$base\"|root_url \"http://127.0.0.1:$port\"|" "$D/dist/posse.rb"
	if [ "$(grep -c "root_url \"http://127.0.0.1:$port\"" "$D/dist/posse.rb")" != 1 ]; then
		kill "$server" 2>/dev/null
		bad "the root_url substitution did not take — the probe would be fetching bottles from the real release"
		return
	fi
	ok "the rendered formula's root_url points at the loopback server; its version, tags, sha256s and bottle filenames are the generator's"

	# A local tap. brew wants a git repo, so it gets one.
	local tapsrc=$D/tap
	mkdir -p "$tapsrc/Formula"
	cp "$D/dist/posse.rb" "$tapsrc/Formula/posse.rb"
	( cd "$tapsrc" && git init -q . &&
		git -c user.email=probe@example.invalid -c user.name=probe add Formula/posse.rb &&
		git -c user.email=probe@example.invalid -c user.name=probe commit -qm bottle-probe -- Formula/posse.rb
	) >"$ROOT/tapinit.log" 2>&1 || { kill "$server" 2>/dev/null; skip "could not build the local tap fixture"; return; }

	local B=$D/brew
	mkdir -p "$B/home" "$B/cfg" "$B/cache" "$B/logs" "$B/tmp"
	# Reuse `brew` mode's clone when there is one — on APFS `cp -Rc` is a
	# clonefile, so `all` does not pay for Homebrew's history twice.
	if [ -d "$ROOT/brew/prefix" ]; then
		cp -Rc "$ROOT/brew/prefix" "$B/prefix" 2>/dev/null || cp -R "$ROOT/brew/prefix" "$B/prefix"
	else
		say "  cloning Homebrew into the scratch root (~200 MB, once per run)"
		git clone --depth=1 -q https://github.com/Homebrew/brew "$B/prefix" 2>/dev/null ||
			{ kill "$server" 2>/dev/null; skip "could not clone Homebrew"; return; }
		( cd "$B/prefix" && git fetch -q --unshallow --tags 2>/dev/null )
	fi

	export HOME="$B/home" XDG_CONFIG_HOME="$B/cfg"
	export HOMEBREW_CACHE="$B/cache" HOMEBREW_LOGS="$B/logs" HOMEBREW_TEMP="$B/tmp"
	export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1
	local brew=$B/prefix/bin/brew
	note "scratch brew: $("$brew" --version 2>/dev/null | head -1)"
	note "this box: macOS $(sw_vers -productVersion) $(uname -m), CLT $(pkgutil --pkg-info=com.apple.pkg.CLTools_Executables 2>/dev/null | sed -n 's/^version: //p')"

	if ! "$brew" tap "$TAP" "$tapsrc" >"$ROOT/btap.log" 2>&1; then
		kill "$server" 2>/dev/null
		bad "brew tap of the local fixture failed: $(tail -2 "$ROOT/btap.log" | tr '\n' ' ')"
		return
	fi
	"$brew" trust --formula "$FORMULA" >"$ROOT/btrust.log" 2>&1

	# THE MEASUREMENT.
	"$brew" install "$FORMULA" >"$ROOT/binstall.log" 2>&1
	local log=$ROOT/binstall.log
	if grep -q "Command Line Tools are too outdated" "$log"; then
		kill "$server" 2>/dev/null
		bad "brew still hit the Command Line Tools gate WITH a bottle in the formula — the bottle was not poured. That is ranger-base-9vg3 unfixed:"
		grep -iE "bottle|building|pouring|source" "$log" | sed 's/^/            /' | head -6
		return
	fi
	if grep -q "Pouring posse-$ver.$tag.bottle.tar.gz" "$log"; then
		ok "brew POURED posse-$ver.$tag.bottle.tar.gz — the build-from-source path, and its fatal developer-tools checks, were never entered"
	elif grep -q Pouring "$log"; then
		bad "brew poured something else: $(grep Pouring "$log" | head -1)"
	else
		bad "brew did not pour: $(tail -4 "$log" | tr '\n' ' ')"
		kill "$server" 2>/dev/null
		return
	fi

	local installed=$B/prefix/bin/posse
	if [ -x "$installed" ]; then
		BARE=$ver
		case "$(run_bounded "$installed")" in
		ran) ok "the poured posse reports $ver: $(cat "$installed.out")" ;;
		*) bad "the poured posse did not report $ver" ;;
		esac
	else
		bad "no posse in the scratch prefix's bin after the pour"
	fi
	if [ -f "$B/prefix/Cellar/posse/$ver/share/doc/posse/INSTALL.md" ]; then
		ok "the poured keg carries the docs the formula's \`doc.install\` names"
	else
		bad "the poured keg has no share/doc/posse/INSTALL.md — the bottle and \`def install\` disagree"
	fi

	# THE CONTROL. Same box, same brew, same formula minus the bottle block.
	"$brew" uninstall "$FORMULA" >/dev/null 2>&1
	local tapped
	tapped=$("$brew" --repository "$TAP" 2>/dev/null)/Formula/posse.rb
	if [ ! -f "$tapped" ]; then
		kill "$server" 2>/dev/null
		note "control skipped — could not find the tapped formula to strip"
		return
	fi
	/usr/bin/python3 - "$tapped" <<'PYEOF'
import re, sys
p = sys.argv[1]
src = open(p).read()
out = re.sub(r"\n  bottle do\n.*?\n  end\n", "\n", src, count=1, flags=re.S)
if out == src:
    sys.exit("the bottle block was not found, so the control would not be a control")
open(p, "w").write(out)
PYEOF
	if [ $? != 0 ]; then
		kill "$server" 2>/dev/null
		bad "could not strip the bottle block for the control arm"
		return
	fi
	# Unlike the arm above, the control DOES fetch a source tarball, and the
	# sha256s in this formula are the ones we just built — not the published
	# release's. So its urls move to the loopback server too. That costs the
	# version scan (brew will call it "64"; see above) and costs nothing else:
	# the developer-tools gate fires in `install`, after the download and before
	# anything is unpacked, so what the version string says never enters it.
	sed -i '' "s|$base|http://127.0.0.1:$port|g" "$tapped"
	"$brew" install "$FORMULA" >"$ROOT/bcontrol.log" 2>&1
	kill "$server" 2>/dev/null
	if grep -q "Command Line Tools are too outdated" "$ROOT/bcontrol.log"; then
		ok "CONTROL: with the bottle block deleted, the same install on this box dies at the Command Line Tools gate — so the pour above is the difference, not the box"
	elif grep -q "Cellar/posse" "$ROOT/bcontrol.log"; then
		note "CONTROL: this box installs the bottle-LESS formula too, so its developer tools are current."
		note "         The pour above is measured, but this run does not discriminate the CLT"
		note "         gate. Re-run on a box whose Command Line Tools are behind its macOS"
		note "         (or after ranger-base-3m40) to get the contrast."
	else
		bad "CONTROL: the bottle-less install neither poured, installed nor hit the gate: $(tail -3 "$ROOT/bcontrol.log" | tr '\n' ' ')"
	fi
}

case $MODE in
paths) probe_paths ;;
quarantine) probe_quarantine ;;
tap) probe_tap ;;
brew) probe_brew ;;
bottle) probe_bottle ;;
all) probe_paths; probe_quarantine; probe_tap; probe_brew; probe_bottle ;;
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
