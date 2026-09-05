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
#   POSSE_TESTBIN_CACHE   where the binaries live (default ~/.cache/posse/testbin)
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

CACHE=${POSSE_TESTBIN_CACHE:-$HOME/.cache/posse/testbin}
KEEP=${POSSE_TESTBIN_KEEP:-3}

die() { printf 'gotest.sh: %s\n' "$*" >&2; exit 2; }

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
	sha=$(shasum -a 256 "$tmp" | cut -c1-16)
	target="$CACHE/$sl-$sha.test"
	if [ -f "$target" ]; then
		rm -f "$tmp"
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

	mkdir -p "$CACHE"

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
	SELFTEST_TMP=$(mktemp -d)
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
