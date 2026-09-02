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

CACHE=${POSSE_TESTBIN_CACHE:-$HOME/.cache/posse/testbin}
KEEP=${POSSE_TESTBIN_KEEP:-3}

die() { printf 'gotest.sh: %s\n' "$*" >&2; exit 2; }

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
		# Newest first (BSD stat prints mtime as an epoch), keep $KEEP.
		while read -r _ f; do
			n=$((n + 1))
			[ "$n" -le "$KEEP" ] && continue
			rm -f "$f"
			echo "pruned $(basename "$f")"
		done < <(find "$CACHE" -maxdepth 1 -name "$sl-*.test" -type f \
				-exec stat -f '%m %N' {} \; | sort -rn)
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

	local rc=0 n=0 ip dir sl tmp bin
	while read -r ip dir; do
		[ -n "$ip" ] || continue
		n=$((n + 1))
		sl=$(slug "$ip")
		tmp=$(mktemp "$CACHE/.build.$sl.XXXXXX")
		if ! go test -c -o "$tmp" "$ip"; then
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
	cachedinode() { find "${SELFTEST_TMP}/cache" -name '*.test' -exec stat -f '%i' {} \; 2>/dev/null | head -1 || true; }
	cachedcount() { find "${SELFTEST_TMP}/cache" -name '*.test' 2>/dev/null | wc -l | tr -d ' '; }
	SELFTEST_TMP=$(mktemp -d)
	trap 'rm -rf "${SELFTEST_TMP:-}"' EXIT
	local tmp=$SELFTEST_TMP

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
	if [ -n "$i1" ] && [ "$i1" = "$i2" ]; then
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
	if printf '%s' "$out" | grep -q 'TestBeta'; then
		say "control: the new test actually ran"
	else
		fail "control: the new test actually ran" "TestBeta not in output"
	fi

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

	# ARM 4: --prune keeps POSSE_TESTBIN_KEEP per package and no more.
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
