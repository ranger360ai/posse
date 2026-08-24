#!/bin/sh
# Build the posse release artifacts for GitHub Releases — the tarballs the
# Homebrew tap's formula downloads (rangerhq-i0n0).
#
# Usage: scripts/release-artifacts.sh [--rev <commit>] [--version vX.Y.Z] [--out <dir>]
#   --rev      commit to build (default: HEAD)
#   --version  the release tag; default is the annotated/lightweight tag that
#              points exactly at --rev. Must agree with internal/rhq.Version.
#   --out      output directory (default: dist), wiped before use
#   GOBIN=<go> which go to build with (default: go)
#
# Blast radius: writes only inside <out>; $PWD/.git is opened read-only (one
# temp worktree, removed on exit) and no network is touched. It does not tag,
# does not publish, and does not talk to GitHub.
#
# Why not goreleaser: it would be a fifth pinned substrate to keep honest (see
# NOTES.md on what pinning costs here) for four `go build` invocations that
# already cross-compile clean with CGO off. The operator left the choice open
# in the bead; this is the version that can be RUN on the machine that wrote
# it, which is the only kind of release config worth having.
#
# THE VERSION IS THE CODE'S, NOT THE TAG'S. internal/rhq.Version is a const, so
# it cannot be stamped from outside; a tag that disagrees with it would ship a
# binary whose `posse version` contradicts its own download URL. So the tag is
# checked AGAINST the source and the build refuses on a mismatch. Bumping a
# release therefore means editing app.go first, then tagging — in that order.
set -eu

REV=HEAD
VERSION=
OUT=dist
PLATFORMS='darwin/arm64 darwin/amd64 linux/amd64 linux/arm64'

while [ $# -gt 0 ]; do
	case $1 in
	--rev) REV=${2:?--rev needs a commit}; shift 2 ;;
	--version) VERSION=${2:?--version needs a tag}; shift 2 ;;
	--out) OUT=${2:?--out needs a directory}; shift 2 ;;
	-h|--help) sed -n '2,20p' "$0"; exit 0 ;;
	*) echo "release-artifacts: unknown argument: $1" >&2; exit 2 ;;
	esac
done

if ! repo=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "release-artifacts: not a git repository — refusing to build" >&2
	exit 1
fi
sha=$(git -C "$repo" rev-parse --short "$REV^{commit}") ||
	{ echo "release-artifacts: not a commit: $REV" >&2; exit 1; }

# The tag, if we were not given one. --exact-match on purpose: `git describe`
# without it happily reports the LAST tag plus a distance, which would name an
# artifact after a release it is not.
if [ -z "$VERSION" ]; then
	VERSION=$(git -C "$repo" describe --tags --exact-match "$REV" 2>/dev/null) || {
		echo "release-artifacts: $REV carries no exact tag — pass --version vX.Y.Z" >&2
		exit 1
	}
fi
case $VERSION in
v[0-9]*) ;;
*) echo "release-artifacts: --version must look like vX.Y.Z, got: $VERSION" >&2; exit 2 ;;
esac
bare=${VERSION#v}

# The preflight described in the header. Read from the COMMIT, not the working
# tree: someone else's uncommitted app.go edit must not decide what ships.
src_version=$(git -C "$repo" show "$REV:internal/rhq/app.go" |
	sed -n 's/^[[:space:]]*Version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
if [ -z "$src_version" ]; then
	echo "release-artifacts: could not read internal/rhq.Version at $sha" >&2
	exit 1
fi
if [ "$src_version" != "$bare" ]; then
	cat >&2 <<EOF
release-artifacts: tag/source version mismatch — refusing to build
  tag    : $VERSION  (-> $bare)
  app.go : $src_version   (internal/rhq.Version at $sha)
  A binary whose \`posse version\` says $src_version must not ship from a URL
  that says $bare. Fix internal/rhq.Version, commit, re-tag.
EOF
	exit 1
fi

case $OUT in /*) ;; *) OUT=$PWD/$OUT ;; esac
rm -rf "$OUT"
mkdir -p "$OUT"

# Same discipline as scripts/clean-build.sh, and for the same reason: personas
# share this checkout, so the working tree is never what ships.
# Explicit template, not `mktemp -t <prefix>`: the -t form is BSD, and GNU
# coreutils rejects a template with no X's ("too few X's in template"). This
# script runs on ubuntu-latest in .github/workflows/release.yml, so the BSD
# spelling failed only in CI, on a tag, where it is most expensive to discover.
# Same form as scripts/verify-prune-guard.sh.
tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-release.XXXXXX")
src=$tmp/src
cleanup() {
	git -C "$repo" worktree remove --force "$src" 2>/dev/null || :
	rm -rf "$tmp"
}
trap cleanup EXIT INT TERM
git -C "$repo" worktree add --detach --quiet "$src" "$REV"

echo "release-artifacts: $VERSION from $sha"

# CGO off everywhere. It is what makes one machine able to build all four
# targets, and it is what makes the linux tarballs work on a musl box as well
# as a glibc one — verified cross-compiling all four from darwin/arm64.
for platform in $PLATFORMS; do
	goos=${platform%/*}
	goarch=${platform#*/}
	stage=$tmp/stage/$goos-$goarch
	mkdir -p "$stage"
	(cd "$src" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" "${GOBIN:-go}" build \
		-trimpath \
		-ldflags "-s -w -X github.com/ranger360ai/posse/internal/rhq.Build=$sha" \
		-o "$stage/posse" ./cmd/posse)
	for doc in LICENSE README.md INSTALL.md; do
		[ -f "$src/$doc" ] && cp "$src/$doc" "$stage/" || :
	done
	tarball=$OUT/posse_${bare}_${goos}_${goarch}.tar.gz
	# Sorted, owner-stripped, so two builds of the same commit differ only by
	# gzip's timestamp — not reproducible, but comparably diffable.
	(cd "$stage" && tar -czf "$tarball" $(ls | sort))
	echo "  $(basename "$tarball")  $(wc -c <"$tarball" | tr -d ' ') bytes"
done

# checksums.txt is what the formula renderer reads and what a human verifies a
# download against, so it is plain `sha256  name` in the OUT directory's own
# terms — no paths, so `shasum -c` works from inside the download dir.
sum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
(cd "$OUT" && sum posse_${bare}_*.tar.gz > checksums.txt)
echo "release-artifacts: wrote $OUT/checksums.txt"
cat "$OUT/checksums.txt" | sed 's/^/  /'
