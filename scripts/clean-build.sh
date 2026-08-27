#!/bin/sh
# Build ./cmd/posse from a pristine worktree of HEAD — never from the working tree.
#
# Personas share this checkout, so at any moment someone else's half-finished
# edits may be sitting in it. A build of the working tree is neither
# reproducible nor guaranteed to compile, and the resulting binary is what the
# whole fleet then runs. So: check HEAD out somewhere else, build there, stamp
# the real sha (no "-dirty" — we did not build the dirt).
#
# Usage: scripts/clean-build.sh <output-path>
#   GOBIN=<go>   which go to build with (default: go)
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <output-path>" >&2
	exit 2
fi

# go build is run from inside the temp worktree, so the output path must be
# absolute or it lands in the worktree and vanishes with it.
out=$1
case $out in
/*) ;;
*) out=$PWD/$out ;;
esac

if ! repo=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "clean-build: not a git repository — refusing to build" >&2
	echo "  a build that cannot name its commit must not become the fleet's binary" >&2
	exit 1
fi
if ! sha=$(git -C "$repo" rev-parse --short HEAD 2>/dev/null); then
	echo "clean-build: no commits on HEAD — refusing to build" >&2
	exit 1
fi

# Not an error: the whole point is that we build HEAD regardless. But say out
# loud which changes are being left behind, so nobody installs a binary they
# think contains their edits.
dirty=$(git -C "$repo" status --porcelain)
if [ -n "$dirty" ]; then
	echo "clean-build: working tree is dirty — building HEAD ($sha), NOT these:" >&2
	printf '%s\n' "$dirty" | sed 's/^/    /' >&2
	echo "  commit them first if they belong in the installed binary." >&2
fi

# `git worktree add` below fires the repo's bd post-checkout hook, and that is
# precisely how the 08-26 lock storm broke `make install`: a bd write that hung
# on a lock held by an orphaned daemon, reported as an unexplained stall in the
# middle of a build. Say the cause out loud BEFORE the hook runs. Advisory
# only — the pin's state does not make the build wrong, and a failed check must
# never be what stands between the operator and a working binary. Exit 1 is a
# real failed check and gets the full report; exit 2 is "cannot check" (no bd
# on this machine, e.g. Linux CI) and gets one line.
pincheck=$(dirname "$0")/verify-bd-pin.sh
if [ -x "$pincheck" ]; then
	# NOT `out` — that is this script's output path, set from $1 at the top.
	# Clobbering it makes `make release` write the binary to the report text and
	# still exit 0 (MEASURED while building this check).
	if pinreport=$("$pincheck" 2>&1); then
		:
	else
		rc=$?
		if [ "$rc" = 2 ]; then
			echo "clean-build: bd pin not checkable on this machine (verify-bd-pin exit 2)" >&2
		else
			echo "clean-build: BD PIN CHECK FAILED — the post-checkout hook below talks to bd:" >&2
			printf '%s\n' "$pinreport" | sed 's/^/    /' >&2
			echo "  building anyway; this is advisory. If the build stalls, that is why." >&2
		fi
	fi
fi

# Explicit template — `mktemp -t <prefix>` is BSD-only and GNU coreutils
# rejects it, which would break `make release`/`make install` for any Linux
# reader following INSTALL.md. Same form as scripts/verify-prune-guard.sh.
tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-clean-build.XXXXXX")
src=$tmp/src # git worktree add refuses an existing directory
cleanup() {
	git -C "$repo" worktree remove --force "$src" 2>/dev/null || :
	rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

git -C "$repo" worktree add --detach --quiet "$src" HEAD
(cd "$src" && "${GOBIN:-go}" build -ldflags "-X github.com/ranger360ai/posse/internal/rhq.Build=$sha" -o "$out" ./cmd/posse)

echo "clean-build: built $out from $sha"
