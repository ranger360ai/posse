#!/usr/bin/env bash
# gotest.sh — run a package's tests from a REUSED test binary, so macOS
# Gatekeeper assesses it once instead of once per invocation
# (ranger-base-nw9zg).
#
# Usage: scripts/gotest.sh [<pkg>...] [-- ] [go-test flags...]
#          scripts/gotest.sh ./internal/posse -run TestFoo -count=1
#          scripts/gotest.sh ./... -run Gate
#          scripts/gotest.sh --self-test          prove the reuse works
#          scripts/gotest.sh --prune              drop all but the newest
#                                                 binary per package
#
# Environment:
#   POSSE_TESTBIN_CACHE   where the binaries live (default ~/.cache/posse-testbin)
#   POSSE_TESTBIN_KEEP    binaries kept per package by --prune (default 3)
#
# ─── WHY ─────────────────────────────────────────────────────────────────────
#
# `go test <pkg>` COPIES the linked test binary out of the build cache into a
# fresh $TMPDIR/go-buildNNN work dir on every invocation, and throws it away at
# the end. Measured on this box, 2026-09-02, two cached runs of the identical
# command over ./internal/posse:
#
#   go-build1660568306/b001/posse.test   inode 243177760  links=1
#   go-build3490873990/b001/posse.test   inode 243178437  links=1
#
# Two invocations, two inodes, one content. To macOS that is a brand-new
# unsigned executable each time, and the first exec of one is a full Gatekeeper
# assessment: syspolicyd hands the file to XprotectService for a yara scan and
# makes a notarization round trip to Apple before the process starts.
#
# THE COST, measured by execution rather than by reading (each figure a fresh
# copy of the 23 MB internal/posse test binary, cwd = the package dir,
# -test.run set to a pattern that matches nothing so the number is startup and
# nothing else):
#
#   first exec of a fresh copy   0.806s  1.066s  1.059s
#   second exec of that copy     0.030s  0.035s  0.039s
#
# ~1 second of wall clock per `go test` invocation, and one XProtect scan, for
# a binary the box already scanned. On a 16 KB probe binary the same pair is
# 0.241s / 0.023s, so the scan is roughly proportional to the file, not a flat
# per-exec tax.
#
# ─── WHAT IS AND IS NOT KEYED ON THE PATH ────────────────────────────────────
#
# This is the finding that decides the design, and it is the opposite of the
# obvious guess. Four arms, 200 execs each, on this box, with idle controls of
# the same wall length on either side to subtract the fleet's background rate
# (which ran 0.8-5.3 assessments/sec while these were taken):
#
#   A  200 execs of ONE path                          1s    2 scans
#   B  200 execs of 200 COPIES (200 inodes)          39s  217 scans
#   C  200 execs of 200 HARD LINKS (1 inode)          1s    1 scan
#   E  200 execs of the arm-B copies, second time     2s    7 scans
#
# Gatekeeper caches its verdict against the FILE, not the path. 200 distinct
# paths pointing at one inode cost one assessment (C); 200 distinct inodes with
# byte-identical content cost 200 (B); and the verdict survives — arm E is the
# same 200 paths half a minute later, for free, and copies assessed 25 minutes
# earlier still exec in 0.023-0.050s against 0.241s for a fresh one.
#
# So: a stable GOTMPDIR buys NOTHING here. Pointing every link at one directory
# still writes a new file each time, and a new file is a new assessment
# whatever it is called. Reusing the FILE is the whole saving, which is what
# this script does. (GOTMPDIR is also not free to set: `go test` puts
# t.TempDir() under it while a directly-run binary puts t.TempDir() under
# $TMPDIR — measured, and the reverse of that swap is the trap
# docs/notes.d/ranger-base-krra.md records greening a full-disk repro.)
#
# ─── WHY NOT A HAND-ROLLED TREE HASH ─────────────────────────────────────────
#
# The obvious cache key is "git HEAD plus a hash of the uncommitted diff". It
# is rejected here, measured rather than argued: `go test -c` against a warm
# build cache costs 0.59s and 0.69s on the 23 MB package, against the 0.8-1.07s
# assessment it avoids. Paying that 0.6s buys correctness that no hand-rolled
# key can promise — go's own build cache already accounts for every test file,
# every dependency, the toolchain and the build flags, and a tree hash that is
# wrong ONCE runs a stale suite and reports it green. So the key here is the
# built binary's own sha256: build first, then reuse the FILE if a binary with
# those exact bytes is already on disk and already assessed.
#
# Proven, not assumed, on ./cmd/posse:
#
#   build 1  fresh                inode 243734228  sha dbdeb069…  exec 0.389s
#   build 2  no source change     inode 243734228  sha dbdeb069…  exec 0.153s
#   build 3  one test file ADDED  inode 243734666  sha 77f90e89…  exec 0.421s
#            -test.list finds the added test: the binary is not stale
#   build 4  that file removed    inode 243735601  sha dbdeb069…  exec 0.378s
#            -test.list no longer finds it
#
# ─── WHAT THIS DOES NOT FIX ──────────────────────────────────────────────────
#
# It does not move this box's Gatekeeper load, and nobody should close a bead
# claiming it did. Over the same 10 minutes that syspolicyd assessed 1644
# DISTINCT executables, this repo has three packages with tests, so a full
# `go test ./...` contributes three. Attribution from the log's own signature
# field over 5 minutes: 590 assessments with `(id: (null))` — no code signature
# at all — against 21 with `(id: a.out)`, which is what a Go-linked binary
# reports. The Go toolchain is ~3% of the rate. The rest is somebody else's
# churn and is measured on ranger-base-nw9zg, not fixed here.
#
# What it does fix is ours: ~1 second and one XProtect yara scan per `go test`
# invocation, every invocation, for as long as the binary is unchanged.

set -euo pipefail

# $0 is re-invoked by --self-test from inside a temp package dir, so resolve
# it to an absolute path once, here, rather than in each arm.
SELF=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")

# The box-wide suite queue (ranger-base-uvzjk). `make test-reuse` runs this
# wrapper over `./...`, which is a full suite by any measure that matters to
# the box, so it queues on the same slots scripts/test-times.sh does.
#
# Guarded, because this file gets COPIED away from its siblings — arm 3 of
# gotestreuse_qa_test.go writes a mutated copy into a temp dir and runs it —
# and because a wrapper that refuses to run at all when its QUEUE is missing
# is worse than one that runs unqueued and says so. The loud failure for a
# genuinely missing file is `make verify-suite-lock`, which is a prerequisite
# of `make test`.
if [ -r "$(dirname "$SELF")/suite-lock.sh" ]; then
	. "$(dirname "$SELF")/suite-lock.sh"
else
	printf 'gotest.sh: scripts/suite-lock.sh is missing — running unqueued (ranger-base-uvzjk)\n' >&2
	suite_lock_acquire() { :; }
	suite_lock_release() { :; }
fi

# THE DEFAULT CACHE IS OUTSIDE EVERY WALL GRANT (ranger-base-g66m3). This
# directory holds files this wrapper EXECS, so whoever can write it chooses
# what the next run executes — the operator's uncaged run included. It used to
# be `~/.cache/posse/testbin`, and `~/.cache/posse` is the one home cache dir
# every walled seat is granted write on (homeCacheGrants in
# internal/posse/seatbelt.go: the box's lock dir, shared on purpose). A seat
# could overwrite the cached binary for an unchanged package, and the next
# run anywhere built the same bytes, hashed them to the same name and exec'd
# the seat's file. `~/.cache` itself left the grant (ranger-base-olr0g), so a
# sibling of `posse` under it is read-only to every seat.
# internal/treepins/gotestreuse_qa_test.go holds this default against that
# grant list. A seat that cannot write here is told so and stops; pointing
# POSSE_TESTBIN_CACHE at a dir only that seat writes is its own business,
# never at one another principal also execs from.
CACHE=${POSSE_TESTBIN_CACHE:-$HOME/.cache/posse-testbin}
KEEP=${POSSE_TESTBIN_KEEP:-3}

die() { printf 'gotest.sh: %s\n' "$*" >&2; exit 2; }

# `shasum` IS NOT ON EVERY BOX'S PATH, and the cache key is a sha, so this
# wrapper ran a STALE BINARY and reported its result on a box where it is not
# (ranger-base-sydnc). Arch — what the always-on Linux host runs — ships it
# with perl at /usr/bin/core_perl/shasum, off the default PATH, and carries
# coreutils' `sha256sum` instead; ubuntu and macOS carry both, which is why CI
# and the laptop were green. MEASURED 2026-11-09 on the host, and reproduced
# here with shasum hidden from PATH: every build hashed to the EMPTY STRING,
# so every build named the same cache file (`<slug>-.test`), and the reuse
# branch below handed back the FIRST binary ever cached for that package
# forever after. `--self-test` reds exactly three arms, the same three on both
# boxes — `control: a changed test file rebuilds: cache did not grow`,
# `control: the new test actually ran: TestBeta not in output`, and
# `exit status: a red test reds the wrapper: wrapper exited 0`. The last one is
# the hole: a changed package's new tests never run, and a red test greens the
# wrapper. (The poisoned entry is unprunable too — `<slug>-.test` does not
# match the `-[0-9a-f]\{16\}.test` --prune groups by.)
#
# `set -euo pipefail` does not catch it: a command substitution whose command
# is not found yields the empty string and the status of the assignment it
# feeds is discarded (MEASURED 2026-11-09, bash 3.2.57 on darwin; the host's
# own run shows the same, since it got past this line too). So the spelling is
# picked ONCE by probe — like `stat` below, and for the same kind of reason —
# it is PROVEN against a known digest before anything is cached, and a digest
# that is not 64 hex characters is an error and never a key.
if command -v sha256sum >/dev/null 2>&1; then
	sha256_file() { sha256sum "$1"; }
elif command -v shasum >/dev/null 2>&1; then
	sha256_file() { shasum -a 256 "$1"; }
else
	die "neither sha256sum nor shasum is on PATH — the cache key is a sha256 and there is nothing to compute it with (ranger-base-sydnc)"
fi
hash_selfcheck() {
	local probed probe want='9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08' got
	# A DIR WITH THE PROBE INSIDE IT, never a bare `mktemp`
	# (ranger-base-u1z89). A bare one makes a FILE under mktemp(1)'s own
	# `tmp.XXXXXXXXXX` namespace, and while scripts/reap-test-tmp.sh has a
	# class globbing exactly that name, every candidate walk it has is
	# `-type d` — so a file left there by a killed run is reached by no row
	# at all, not even the 24h one that spells its name. The dir is
	# `posse-*`, which the posse-scratch class takes at 120m.
	probed=$(mktemp -d "${TMPDIR:-/tmp}/posse-gotest-hashcheck.XXXXXX") || return 1
	probe="$probed/probe"
	printf '%s' 'test' >"$probe" || { rm -rf -- "${probed:?}"; return 1; }
	got=$(sha256_file "$probe") || { rm -rf -- "${probed:?}"; return 1; }
	rm -rf -- "${probed:?}"
	got=${got%% *}
	[ "$got" = "$want" ]
}
hash_selfcheck || die "the sha256 on this box does not answer a known digest — refusing to key the binary cache on it (ranger-base-sydnc)"

# `stat` is not one program, and the difference is invisible from either box.
# BSD stat (macOS) spells the format `-f` and prints mtime as %m and the
# name as %N; GNU stat (Linux, which is what ubuntu-latest runs) spells it
# `-c` and calls the same two fields %Y and %n. A BSD format string handed
# to GNU stat does not fail in a way any caller here could see: GNU reads
# `-f` as --file-system and prints a filesystem report for a file literally
# named "%m %N", so `find -exec stat -f ...` yielded lines this script then
# parsed as garbage. Measured on ubuntu-latest 2026-09-04 (ranger-base-90y3c):
# --prune deleted NOTHING there for as long as this file has existed, and
# the self-test's own inode arm read a filesystem banner and called it an
# inode — a false pass, on the one arm the whole script exists for.
#
# So the spelling is picked by PROBE, once, and both readers go through it.
# The probe is `-c`, because that is the flag BSD stat does not have at all
# (`stat: illegal option -- c`, exit 1) while GNU answers it; probing `-f`
# instead tells you nothing, since both accept the flag and only the
# MEANING differs.
if stat -c '%Y' . >/dev/null 2>&1; then
	stat_mtime_name() { stat -c '%Y %n' "$@"; }
	stat_inode()      { stat -c '%i' "$@"; }
else
	stat_mtime_name() { stat -f '%m %N' "$@"; }
	stat_inode()      { stat -f '%i' "$@"; }
fi

# slug turns an import path into a filename component: the last two elements,
# non-alphanumerics folded to '-'. Two packages with the same last element
# (cmd/posse and internal/posse) must not collide, and they do not: the sha is
# in the name too, so a collision costs a rebuild, never a wrong binary.
slug() { printf '%s' "$1" | tr -c 'A-Za-z0-9' '-' | sed 's/^-*//; s/-*$//'; }

# reuse <tmpbin> <slug> — put the built binary in the cache under its own
# content hash and echo the path to use. If a byte-identical binary is already
# there, the temp copy is dropped and the EXISTING file is named, so the inode
# the box already assessed is the one that gets exec'd. That is the whole
# trick; everything above is why it is the right one.
reuse() {
	local tmp=$1 sl=$2 sha target
	# An unusable digest is a hard stop, never a key: a cache keyed on a
	# hash that did not happen hands back whatever is already there
	# (ranger-base-sydnc). `die` here only ends the command substitution
	# this runs in, so the CALLER checks what came back as well.
	sha=$(sha256_file "$tmp") || sha=""
	sha=${sha%% *}
	case $sha in *[!0-9a-f]*) sha="" ;; esac
	[ ${#sha} -eq 64 ] || die "no sha256 for the test binary just built for $sl — refusing to cache it under a key that is not a hash (ranger-base-sydnc)"
	sha=${sha:0:16}
	target="$CACHE/$sl-$sha.test"
	if [ -f "$target" ]; then
		# The NAME is a hash of the fresh build, but nothing ties the
		# FILE under that name to it: anything that can write the cache
		# can put other bytes there (ranger-base-g66m3). Reuse only on
		# byte equality; on any difference — or a `cmp` that cannot
		# answer — the fresh build replaces it. The good path is still
		# one inode, so the Gatekeeper saving holds.
		if cmp -s -- "$tmp" "$target"; then
			rm -f "$tmp"
		else
			printf 'gotest.sh: cached %s does not match the build that names it — replacing it (ranger-base-g66m3)\n' "$target" >&2
			mv -f "$tmp" "$target"
		fi
	else
		# Same directory, so this is a rename and not a copy: the file
		# that lands is the one that was just built, one inode, no
		# second write for Gatekeeper to notice.
		mv -f "$tmp" "$target"
	fi
	printf '%s' "$target"
}

prune() {
	[ -d "$CACHE" ] || { echo "gotest.sh: nothing cached at $CACHE"; return 0; }
	local sl n
	# Group by everything before the trailing -<sha>.test.
	for sl in $(find "$CACHE" -maxdepth 1 -name '*.test' -type f 2>/dev/null |
			sed 's|.*/||; s|-[0-9a-f]\{16\}\.test$||' | sort -u); do
		n=0
		# Newest first (both stats print mtime as an epoch), keep $KEEP.
		while read -r _ f; do
			n=$((n + 1))
			[ "$n" -le "$KEEP" ] && continue
			rm -f "$f"
			echo "pruned $(basename "$f")"
		done < <(find "$CACHE" -maxdepth 1 -name "$sl-*.test" -type f |
				while read -r p; do stat_mtime_name "$p"; done | sort -rn)
	done
	echo "gotest.sh: kept $KEEP per package in $CACHE"
}

run() {
	# `go test` argv is packages first, then flags — and a flag's VALUE is a
	# separate word that must not be mistaken for a package (`-run TestFoo`
	# fed straight to `go list` gets "package TestFoo is not in std", which
	# is how this was found). So: everything up to the first word starting
	# with '-' is a package, everything from there on is flags.
	local pkgs=() flags=() seen=0 a
	for a in "$@"; do
		if [ "$a" = "--" ]; then seen=1; continue; fi
		case $a in -*) seen=1 ;; esac
		if [ "$seen" = 1 ]; then flags+=("$a"); else pkgs+=("$a"); fi
	done
	[ ${#pkgs[@]} -gt 0 ] || pkgs=("./...")

	# A compiled test binary spells go test's flags -test.<name>; rewrite
	# the leading dash form and leave flag VALUES and anything already
	# spelled -test.* alone.
	local tflags=()
	for a in "${flags[@]+"${flags[@]}"}"; do
		case $a in
		-test.*) tflags+=("$a") ;;
		-*)      tflags+=("-test.${a#-}") ;;
		*)       tflags+=("$a") ;;
		esac
	done

	mkdir -p "$CACHE" 2>/dev/null && [ -w "$CACHE" ] ||
		die "cannot write the test binary cache $CACHE — a walled seat is not granted it, by design (ranger-base-g66m3); set POSSE_TESTBIN_CACHE to a dir only you write, or use \`make test\`"

	# Resolve the packages BEFORE the loop: a go list that fails inside a
	# process substitution feeds the loop nothing and the wrapper exits 0,
	# which is a silent green over a typo'd package.
	local listing
	if ! listing=$(go list -f '{{.ImportPath}} {{.Dir}}' "${pkgs[@]}" 2>&1); then
		printf '%s\n' "$listing" >&2
		die "go list failed"
	fi
	[ -n "$listing" ] || die "no packages matched: ${pkgs[*]}"

	# Queue AFTER `go list` has agreed the packages exist: waiting twenty
	# minutes for a slot and then dying on a typo'd package is the one
	# ordering this must not have. A `-run` filter or a named package is
	# not queued at all (scripts/suite-lock.sh says why).
	suite_lock_acquire "$@"

	local rc=0 n=0 ip dir sl tmp bin buildout
	while read -r ip dir; do
		[ -n "$ip" ] || continue
		n=$((n + 1))
		sl=$(slug "$ip")
		tmp=$(mktemp "$CACHE/.build.$sl.XXXXXX")
		# Hold the build's own chatter: on success it is the duplicate
		# "[no test files]" line handled below, on failure it is the
		# compile error and the only thing worth reading.
		if ! buildout=$(go test -c -o "$tmp" "$ip" 2>&1); then
			printf '%s\n' "$buildout" >&2
			rm -f "$tmp"
			rc=1
			continue
		fi
		# A package with no test files links nothing and leaves the
		# output empty or absent; say so and move on rather than
		# exec'ing a zero-byte file.
		if [ ! -s "$tmp" ]; then
			rm -f "$tmp"
			echo "ok   $ip [no test files]"
			continue
		fi
		chmod +x "$tmp"
		bin=$(reuse "$tmp" "$sl")
		# reuse()'s own `die` cannot end this script from inside a
		# command substitution, so its answer is checked here too: an
		# empty or unexecutable path means the binary was never cached
		# and there is nothing honest to run (ranger-base-sydnc).
		[ -n "$bin" ] && [ -x "$bin" ] ||
			die "the test binary for $ip was not cached — refusing to report a result for a binary this wrapper cannot name"
		# cwd is the package directory or testdata does not resolve — a
		# hand-run test binary resolves testdata from YOUR shell, not
		# from the package.
		if ( cd "$dir" && "$bin" "${tflags[@]+"${tflags[@]}"}" ); then
			:
		else
			rc=1
		fi
	done <<<"$listing"

	suite_lock_release
	[ "$n" -gt 0 ] || die "no packages built"
	return "$rc"
}

self_test() {
	# The rig has to be shown able to FAIL before either arm is believed, so
	# every arm below has a control that must come out the other way.
	local rc=0 pkg
	# Read back through helpers that cannot abort the run: an arm that finds
	# NO cached binary has to print its own FAIL, or a mutant that guts run()
	# entirely takes the whole self-test down silently and reads as a
	# survivor (it did, the first time this was mutation-checked).
	# `find` stays — asking the filesystem IS the measurement — but nothing
	# downstream of it forks (ranger-base-t07yx). These two helpers decide
	# every reuse arm in this file, and they used to end in `| head -1` and
	# `| wc -l | tr -d ' '`: a `head`, `wc` or `tr` that is signalled or
	# cannot be exec'd under load returned an empty count, and the arms then
	# reported the wrapper as having cached nothing. Counting and taking the
	# first line are things bash does without leaving the process.
	cachedinode() {
		local p
		while IFS= read -r p; do
			stat_inode "$p"
			return 0
		done < <(find "${SELFTEST_TMP}/cache" -name '*.test' 2>/dev/null)
		return 0
	}
	cachedcount() {
		local p c=0
		while IFS= read -r p; do c=$((c + 1)); done \
			< <(find "${SELFTEST_TMP}/cache" -name '*.test' 2>/dev/null)
		printf '%s' "$c"
	}
	SELFTEST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/posse-gotest-selftest.XXXXXX")
	trap 'rm -rf "${SELFTEST_TMP:-}"' EXIT
	local tmp=$SELFTEST_TMP

	# The arms below run single packages, which the queue leaves alone, but
	# an arm added later may not — point it at the throwaway dir so no
	# self-test can ever take a slot a real suite is waiting for, and clear
	# any slot this process inherited from the suite running it.
	export POSSE_SUITE_LOCK_DIR="$tmp/locks"
	unset POSSE_SUITE_LOCK_HELD

	export POSSE_TESTBIN_CACHE="$tmp/cache"
	pkg="$tmp/src"
	mkdir -p "$pkg"
	cat >"$pkg/go.mod" <<-'EOF'
		module gotestselftest

		go 1.24
	EOF
	cat >"$pkg/a_test.go" <<-'EOF'
		package a

		import "testing"

		func TestAlpha(t *testing.T) {}
	EOF

	# House form, the same one scripts/test-times.sh prints and the same one
	# the QA pin requires by arm name: `ok    <arm>` / `FAIL  <arm>: <why>`.
	say() { printf 'ok    %s\n' "$1"; }
	fail() { printf 'FAIL  %s: %s\n' "$1" "$2"; rc=1; }

	# A PLANT OVER A FILE THIS SCRIPT JUST EXEC'D IS ETXTBSY UNDER LOAD, AND
	# NO FD OF OURS IS THE REASON (ranger-base-nywsk). Arm 1b below writes
	# the poison IN PLACE over the cached binary arm 1 ran twice, and on
	# 2026-11-15 at a one-minute load average of 14 that write failed —
	# `cache/gotestselftest-<sha>.test: Text file busy`, under `set -e`, so
	# the whole self-test died after three oks and reddened
	# `make treepins-check` and every `make tree-check` behind it.
	#
	# ETXTBSY on an OPEN FOR WRITE means some process still holds that inode
	# as its executable text. The obvious reading — a writer's fd this
	# script left open, which is the shape ADR 0085 is about from the other
	# side — is the wrong one: the arm's own child was exec'd, waited for and
	# reaped before the write, and this script holds no fd on the cached file
	# at all. It is the kernel's: the final `fput` of an exec'd file, which
	# is what releases the write deny, is deferred past the exit of the
	# process that held it, so a write that follows a reaped exec can lose
	# the race. MEASURED 2026-11-20 on this box (linux 7.2.15, bash 5,
	# one-minute load average 3.7-5.2, scripts/gotest.sh's own temp dir), a
	# tiny Go ELF exec'd and reaped and then written over immediately:
	#
	#   exec-then-write, 300 iterations   7 ETXTBSY   (2.3%)
	#   exec-then-write, 400 iterations   6 ETXTBSY   (1.5%)
	#   the same write while it really is running     ETXTBSY every time
	#
	# and the window is SHORT: retried flat out it cleared after 8-22
	# attempts, 13.9-38.4ms, worst 38.4ms, every time, 6/6. So the fix is a
	# bounded retry — there is nothing to close — and the budget below is
	# two orders of magnitude over the worst drain measured. The arm is then
	# evidence and not a coin toss: a plant that cannot land says so and
	# reds ONE arm, instead of taking the script down from under the arms
	# that have not run yet.
	#
	# The production path never writes over a cached binary — reuse() moves
	# a freshly built file into place, and a rename over a busy inode is not
	# refused — so this is the self-test's own hazard and is fixed here.
	#
	# `sleep` is not one program either: BSD sleep has rejected a fractional
	# argument, so the nap is PROBED like `stat` and the sha are, and a box
	# that will only sleep in whole seconds gets fewer, longer attempts
	# rather than a tighter spin.
	# PLANT_ERR and PLANT_TRIES are plant_over's answer to its caller, and
	# they are declared HERE so bash's dynamic scope keeps them inside this
	# function: plant_over sees them, and nothing outside the self-test
	# inherits them.
	local plant_cap hold_cap PLANT_ERR PLANT_TRIES
	if sleep 0.05 2>/dev/null; then
		nap() { sleep 0.05; }
		plant_cap=200
		hold_cap=200
	else
		nap() { sleep 1; }
		plant_cap=15
		hold_cap=20
	fi
	# plant_over <src> <dst> — write src's bytes over dst IN PLACE, waiting
	# out a dst the kernel still holds as executable text. Sets PLANT_ERR
	# and PLANT_TRIES for the caller to name. Retries on ANY failure and
	# reads no strerror text: the message is localized and the right
	# response to a dst that is genuinely unwritable is the same bounded
	# wait followed by the same named failure.
	plant_over() {
		local src=$1 dst=$2 a=0 said=no
		PLANT_ERR=
		PLANT_TRIES=0
		while :; do
			a=$((a + 1))
			PLANT_TRIES=$a
			if PLANT_ERR=$( { cat "$src" >"$dst"; } 2>&1 ); then
				# The write is not believed on its exit
				# status: a plant that reported success and
				# landed nothing turns arm 1b's whole poison
				# measurement green over a file that was never
				# poisoned. That was the one mutant of this
				# helper that survived its first pass, so the
				# bytes are read back.
				cmp -s -- "$src" "$dst" && return 0
				PLANT_ERR="the write reported success but $dst does not hold the planted bytes"
				return 1
			fi
			[ "$a" -ge "$plant_cap" ] && return 1
			if [ "$said" = no ]; then
				printf 'gotest.sh: %s: %s — waiting for the kernel to release it (ranger-base-nywsk)\n' \
					"$dst" "$PLANT_ERR" >&2
				said=yes
			fi
			# The first few attempts pay no nap: each one is a fork,
			# which is ~1.7ms on this box, and the whole drain
			# measured 13.9-38.4ms.
			[ "$a" -ge 8 ] && nap
		done
	}

	# ARM 1: two runs of an unchanged package reuse ONE inode. This is the
	# property the whole script exists for.
	local i1 i2 out
	out=$( cd "$pkg" && "$SELF" . -run TestAlpha 2>&1 ) || { echo "$out"; fail "arm1 first run" "run failed"; }
	i1=$(cachedinode)
	out=$( cd "$pkg" && "$SELF" . -run TestAlpha 2>&1 ) || { echo "$out"; fail "arm1 second run" "run failed"; }
	i2=$(cachedinode)
	# NUMERIC, not merely non-empty. A reader that is not returning inodes
	# at all returns the same non-inode twice and satisfies `-n` plus
	# equality — which is exactly how the BSD-only `stat -f` above passed
	# this arm on ubuntu-latest while measuring a filesystem banner
	# (ranger-base-90y3c). The arm the whole script exists for must not be
	# green over a reader that reads nothing.
	local numeric=no
	case $i1 in ''|*[!0-9]*) ;; *) numeric=yes ;; esac
	if [ "$numeric" = yes ] && [ "$i1" = "$i2" ]; then
		say "reuse: unchanged package keeps one inode"
	else
		fail "reuse: unchanged package keeps one inode" "$i1 vs $i2"
	fi
	if [ "$(cachedcount)" = 1 ]; then
		say "reuse: no second binary was written"
	else
		fail "reuse: no second binary was written" "cache grew"
	fi

	# ARM 1b: a different FILE planted under the cached name is never
	# exec'd (ranger-base-g66m3). The name is the hash of the fresh build;
	# the file is whatever whoever can write the cache left there. Written
	# IN PLACE over the cached inode, which is what a writer of the dir can
	# do. Control first: the plant, exec'd, really does leave its marker —
	# otherwise "no marker" proves nothing.
	local cached p plant mark
	cached=""
	while IFS= read -r p; do cached=$p; break; done \
		< <(find "${SELFTEST_TMP}/cache" -name '*.test' 2>/dev/null)
	mark="$tmp/planted-ran"
	plant="$tmp/plant"
	printf '#!/bin/sh\n: >"%s"\nexit 0\n' "$mark" >"$plant"
	chmod +x "$plant"
	if [ -z "$cached" ]; then
		fail "poison: a planted file under the cached name is not exec'd" "nothing cached to plant over"
	else
		"$plant"
		if [ -e "$mark" ]; then
			say "control: the plant runs when exec'd"
		else
			fail "control: the plant runs when exec'd" "no marker"
		fi
		rm -f "$mark"
		# NOT a bare `cat "$plant" >"$cached"`: arm 1 exec'd this very
		# inode twice, and the kernel can still be holding it
		# (ranger-base-nywsk — the measurement is above plant_over).
		if ! plant_over "$plant" "$cached"; then
			# Both of this arm's conclusions rest on the plant being
			# THERE, so neither is reported when it is not: a
			# `cmp` that cannot find the plant would otherwise read
			# as "the plant was replaced by the fresh build".
			fail "poison: a planted file under the cached name is not exec'd" \
				"could not plant over $cached after $PLANT_TRIES tries: $PLANT_ERR"
			fail "poison: the plant is replaced by the fresh build" \
				"nothing was planted"
			plant=""
		fi
	fi
	if [ -n "$cached" ] && [ -n "$plant" ]; then
		out=$( cd "$pkg" && "$SELF" . -run TestAlpha -v 2>&1 ) || { echo "$out"; fail "arm1b run" "run failed"; }
		if [ -e "$mark" ]; then
			fail "poison: a planted file under the cached name is not exec'd" "the plant ran"
		else
			case $out in
			*TestAlpha*) say "poison: a planted file under the cached name is not exec'd" ;;
			*) fail "poison: a planted file under the cached name is not exec'd" "TestAlpha not in output" ;;
			esac
		fi
		if cmp -s -- "$plant" "$cached" || [ "$(cachedcount)" != 1 ]; then
			fail "poison: the plant is replaced by the fresh build" "plant still cached or cache grew"
		else
			say "poison: the plant is replaced by the fresh build"
		fi
	fi

	# ARM 2, the control for arm 1: a changed test file MUST produce a
	# different binary, and the new test must actually be in it. Without
	# this arm, arm 1 is equally green over a script that ignores the source
	# entirely and reruns a stale binary forever.
	cat >>"$pkg/a_test.go" <<-'EOF'

		func TestBeta(t *testing.T) {}
	EOF
	out=$( cd "$pkg" && "$SELF" . -run TestBeta -v 2>&1 ) || { echo "$out"; fail "arm2 run" "run failed"; }
	if [ "$(cachedcount)" = 2 ]; then
		say "control: a changed test file rebuilds"
	else
		fail "control: a changed test file rebuilds" "cache did not grow"
	fi
	# `case`, not `printf | grep -q` (ranger-base-t07yx). MEASURED: with a
	# `grep` on PATH whose body is `kill -TERM $$` this was the ONLY arm of
	# the whole self-test that fell over, over an $out that plainly carried
	# TestBeta — so it read as a real regression in the wrapper's caching.
	case $out in
	*TestBeta*) say "control: the new test actually ran" ;;
	*) fail "control: the new test actually ran" "TestBeta not in output" ;;
	esac

	# ARM 3: a failing test is reported as a failure. A wrapper that
	# swallows the child's exit status turns every red into a green, which
	# is the worst thing this script could do.
	cat >>"$pkg/a_test.go" <<-'EOF'

		func TestRed(t *testing.T) { t.Fatal("deliberate") }
	EOF
	if ( cd "$pkg" && "$SELF" . -run TestRed >/dev/null 2>&1 ); then
		fail "exit status: a red test reds the wrapper" "wrapper exited 0"
	else
		say "exit status: a red test reds the wrapper"
	fi
	# ...and its control: the same wrapper is green when the test passes.
	if ( cd "$pkg" && "$SELF" . -run TestAlpha >/dev/null 2>&1 ); then
		say "exit status: a green test greens the wrapper"
	else
		fail "exit status: a green test greens the wrapper" "wrapper exited non-zero"
	fi

	# ARM 4: a package that does not COMPILE is a failure, not a skip. A
	# wrapper that greens a compile error is worse than no wrapper: the
	# build step is the one thing `go test` did for you that this script
	# took over, and swallowing it means a broken tree reports clean.
	cp "$pkg/a_test.go" "$pkg/a_test.go.bak"
	printf 'package a\n\nfunc Broken() { this is not go }\n' >"$pkg/broken.go"
	if ( cd "$pkg" && "$SELF" . -run TestAlpha >/dev/null 2>&1 ); then
		fail "build failure: a package that will not compile reds" "wrapper exited 0"
	else
		say "build failure: a package that will not compile reds"
	fi
	rm -f "$pkg/broken.go" "$pkg/a_test.go.bak"

	# ARM 5: --prune keeps POSSE_TESTBIN_KEEP per package and no more.
	POSSE_TESTBIN_KEEP=1 "$SELF" --prune >/dev/null 2>&1
	if [ "$(cachedcount)" = 1 ]; then
		say "prune: keeps POSSE_TESTBIN_KEEP per package"
	else
		fail "prune: keeps POSSE_TESTBIN_KEEP per package" "$(cachedcount) left"
	fi

	# ARM 6 (ranger-base-nywsk): the retrying plant above is NEEDED, and it
	# WORKS. The bug it fixes showed up once on a loaded box and a re-run in
	# isolation was green, so a fix of it must not be believed on the
	# strength of the flake not coming back — this arm is the deterministic
	# half of the same refusal, and it is the arm that can fail.
	#
	# The holder is a real ELF (ETXTBSY is about an executable TEXT image; a
	# `#!` script's own file is never held, so the plant above cannot stand
	# in for it) that holds its own inode for as long as the arm needs and
	# not one fixed interval longer — it exits when this arm removes the
	# marker it wrote. A sleep of n seconds here would make the arm a race
	# about timing, which is the defect, not the test for it.
	local holder holdsrc hmark hpid hwait
	cat >"$pkg/hold_test.go" <<-'EOF'
		package a

		import (
			"os"
			"testing"
			"time"
		)

		// Run as a BACKGROUND holder of its own executable text by
		// gotest.sh --self-test's arm 6: mark that the exec has
		// landed, then stay alive until that arm removes the marker.
		func TestTextHolder(t *testing.T) {
			p := os.Getenv("GOTEST_SELFTEST_HOLD")
			if p == "" {
				t.Skip("no holder marker requested")
			}
			if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(p); os.IsNotExist(err) {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("holder: the marker was never removed")
		}
	EOF
	holder="$tmp/holder"
	holdsrc="$tmp/holder.src"
	hmark="$tmp/holder-running"
	# Built directly rather than through "$SELF": arm 5 has already
	# measured the cache's contents, and a holder that went through the
	# wrapper would add a binary to it under every later arm's feet.
	# `-timeout 15m` is INERT here — `-c` links a binary and runs no test, so
	# there is nothing for a deadline to cut — and it is written anyway
	# because internal/treepins/suitetimeout_qa_test.go reads every `go test`
	# in scripts/ and requires each to name its own timeout above a 15m floor
	# (ranger-base-2ggb). The alternative was to spell `go` somewhere that
	# pin cannot see it, which is how the build at the top of run() is green:
	# `buildout=$(go test -c ...` hides the word inside `$(go`. Satisfying a
	# tree-wide pin with a harmless flag beats teaching the next reader to
	# route around it.
	if ! out=$( cd "$pkg" && go test -c -timeout 15m -o "$holder" . 2>&1 ); then
		echo "$out"
		fail "control: a write over a live executable is refused" "the holder would not build"
		fail "plant: the in-place plant waits out a live executable" "the holder would not build"
	else
		# DIFFERENT BYTES from the holder, so plant_over's read-back
		# has something to read: a source byte-identical to its
		# destination makes a plant that writes nothing indistinguishable
		# from one that works.
		cp "$holder" "$holdsrc"
		printf '\n# ranger-base-nywsk\n' >>"$holdsrc"
		rm -f "$hmark"
		( cd "$pkg" && GOTEST_SELFTEST_HOLD="$hmark" "$holder" -test.run TestTextHolder >/dev/null 2>&1 ) &
		hpid=$!
		# hold_cap is ten seconds of naps, not one or two, because the
		# holder's FIRST exec is the cold one ADR 0085 prices: macOS
		# charges 156-714ms for assessing a freshly written executable
		# and has been measured at 2.17s. Nothing here is on a deadline
		# — the arm waits — so the budget is set to make a slow box
		# green and a holder that never starts say so.
		hwait=0
		while [ ! -e "$hmark" ] && [ "$hwait" -lt "$hold_cap" ]; do
			hwait=$((hwait + 1))
			nap
		done
		if [ ! -e "$hmark" ]; then
			fail "control: a write over a live executable is refused" "the holder never marked itself"
			fail "plant: the in-place plant waits out a live executable" "the holder never marked itself"
		else
			# The naive write — what this script used to do at the
			# plant above. It must be refused while the holder runs,
			# or ETXTBSY is not enforced here and the arm below
			# proves nothing.
			if ( cat "$holdsrc" >"$holder" ) 2>/dev/null; then
				fail "control: a write over a live executable is refused" "the write landed while it was running"
			else
				say "control: a write over a live executable is refused"
			fi
			# THE ORDER IS THE ARM (ranger-base-7c2py). This used to
			# remove the marker and THEN call plant_over, which left
			# it to the scheduler whether the plant's FIRST attempt
			# was refused — i.e. whether the retry this arm exists
			# to keep alive was exercised at all. MEASURED
			# 2026-11-21 on this box (linux, 16 cores, loadavg
			# 13.6-16.0) over a COPY of this script: PLANT_TRIES at
			# success read 9, 1, 9, 1, 9 over five runs, and the
			# pre-fix mutant — plant_over giving up after ONE
			# attempt — was killed 2 of 5 runs. A green self-test
			# licensed the removal of the retry on a majority of
			# runs, which is not what "the arm that can fail" means.
			#
			# So the plant goes first, in the BACKGROUND, while the
			# holder still holds its text image, and the marker comes
			# off only once the plant has said it is waiting. The
			# first attempt is then refused by construction and
			# nothing is timed: PLANT_TRIES >= 2 is asserted, and it
			# is what the one-attempt mutant cannot produce.
			# MEASURED after the reorder, three batteries of five
			# runs each at loadavg 5.5-21.8: 15/15 green with
			# PLANT_TRIES 9 or 10 every run, and the same mutant
			# 15/15 KILLED, every one of them "after 1 tries".
			local pnote ptriesf pdetail ppid prc ptries pdetailmsg pwait
			pnote="$tmp/plant-note"
			ptriesf="$tmp/plant-tries"
			pdetail="$tmp/plant-detail"
			rm -f "$pnote" "$ptriesf" "$pdetail"
			# plant_over's answer comes back through FILES: a
			# background subshell cannot set PLANT_TRIES in this
			# shell, and the try count is the measurement.
			(
				prc=0
				plant_over "$holdsrc" "$holder" || prc=$?
				printf '%s\n' "$PLANT_TRIES" >"$ptriesf"
				printf '%s\n' "$PLANT_ERR" >"$pdetail"
				exit "$prc"
			) 2>"$pnote" &
			ppid=$!
			# Bounded, and it watches for the plant's EXIT as well as
			# for its notice. The one-attempt mutant returns before
			# printing anything, so a wait on the notice alone would
			# sit here for the whole budget on the one run that
			# matters. Watching the pid is not enough on its own
			# either: a reaped-but-unwaited child still answers
			# `kill -0`, so the plant's own last write is the signal.
			pwait=0
			while [ ! -s "$pnote" ] && [ ! -e "$pdetail" ] && [ "$pwait" -lt "$hold_cap" ]; do
				pwait=$((pwait + 1))
				nap
			done
			# Only now: removing the marker is what lets the holder
			# exit, so the plant's wait ends by the holder's exit and
			# the kernel's release of it.
			rm -f "$hmark"
			prc=0
			wait "$ppid" || prc=$?
			# The notice went to a file instead of this script's
			# stderr, so hand it to the reader the way it used to
			# arrive.
			if [ -s "$pnote" ]; then
				cat "$pnote" >&2
			fi
			ptries=0
			pdetailmsg="the plant wrote no answer"
			if [ -e "$ptriesf" ]; then
				IFS= read -r ptries <"$ptriesf" || :
			fi
			if [ -e "$pdetail" ]; then
				IFS= read -r pdetailmsg <"$pdetail" || :
			fi
			# NUMERIC, for the same reason arm 1 reads its inodes
			# numerically: `-lt` on a non-number is a bash error, and
			# under `set -e` that takes the whole self-test down
			# instead of reddening this arm.
			case $ptries in ''|*[!0-9]*) ptries=0 ;; esac
			if [ "$prc" != 0 ]; then
				fail "plant: the in-place plant waits out a live executable" \
					"$pdetailmsg after $ptries tries"
			elif [ "$ptries" -lt 2 ]; then
				# A plant that landed on its first attempt was
				# never refused, so the retry was not exercised
				# and this arm measured nothing. Saying so is the
				# whole point of the reorder above.
				fail "plant: the in-place plant waits out a live executable" \
					"the first write landed while the holder still held it ($ptries try) — the retry was not exercised"
			else
				say "plant: the in-place plant waits out a live executable"
			fi
		fi
		# The holder's own exit status is not an arm: it is killed by
		# nothing, but a `go test` binary whose stdout went away reports
		# that, and this arm's evidence is the two writes above.
		wait "$hpid" 2>/dev/null || :
	fi
	rm -f "$pkg/hold_test.go"

	if [ "$rc" = 0 ]; then
		echo "gotest.sh --self-test: all arms ok"
	else
		echo "gotest.sh --self-test: FAILED"
	fi
	return "$rc"
}

case ${1:-} in
--self-test) shift; self_test "$@" ;;
--prune)     shift; prune "$@" ;;
-h|--help)   sed -n '2,20p' "$SELF"; exit 0 ;;
*)           run "$@" ;;
esac
