#!/bin/sh
# Build the posse release artifacts for GitHub Releases — the tarballs and the
# Homebrew bottles the tap's formula downloads (rangerhq-i0n0, ranger-base-9vg3).
#
# Usage: scripts/release-artifacts.sh [--rev <commit>] [--version vX.Y.Z] [--out <dir>]
#   --rev      commit to build (default: HEAD)
#   --version  the release tag; default is the annotated/lightweight tag that
#              points exactly at --rev. Must agree with internal/rhq.Version.
#   --out      output directory (default: dist). It is EMPTIED before use, so
#              it is refused unless it is absent, empty, or holds nothing but
#              this build's own output (posse_*.tar.gz, posse-*.bottle.tar.gz,
#              checksums.txt, posse.rb) — and / , $HOME and the repo root are
#              refused outright. There is no --force (ranger-base-9hyc).
#   GOBIN=<go> which go to build with (default: go)
#
# Blast radius: writes only inside <out>, and removes only an <out> that holds
# nothing it did not write; $PWD/.git is opened read-only (one temp worktree,
# removed on exit) and no network is touched. It does not tag, does not
# publish, and does not talk to GitHub.
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
	-h|--help) sed -n '2,25p' "$0"; exit 0 ;;
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

# >>> out-guard (ranger-base-9hyc) — the QA control arm excises exactly this
# region, between these two markers, and puts the two unguarded lines back.
#
# `rm -rf "$OUT"` on a path a human typed by hand, at the runbook step next to
# the irreversible one. The wipe has to stay — a tarball left from the previous
# version would be picked up by tap-formula.sh's checksums and by the upload
# glob — so the guard is on WHAT may be wiped: a directory this script could
# plausibly have written, and nothing else. There is deliberately no --force:
# the way to wipe a directory holding your own things is to type the rm
# yourself, so the blast radius is on a command line you wrote.
case $OUT in /*) ;; *) OUT=$PWD/$OUT ;; esac

# Canonicalise before comparing, or the identity refusals below are string
# games: `--out .`, `--out $HOME/x/..`, a trailing slash and a symlink pointing
# into $HOME must all land on the same path the comparison uses.
if [ -L "$OUT" ] && [ ! -d "$OUT" ]; then
	echo "release-artifacts: --out is a symlink to a non-directory: $OUT" >&2
	exit 2
fi
if [ -e "$OUT" ] && [ ! -d "$OUT" ]; then
	echo "release-artifacts: --out exists and is not a directory: $OUT" >&2
	exit 2
fi
if [ -d "$OUT" ]; then
	OUT=$(cd "$OUT" && pwd -P) || { echo "release-artifacts: cannot enter --out: $OUT" >&2; exit 2; }
else
	out_parent=$(dirname "$OUT")
	out_leaf=$(basename "$OUT")
	if [ ! -d "$out_parent" ]; then
		echo "release-artifacts: --out parent directory does not exist: $out_parent" >&2
		exit 2
	fi
	out_parent=$(cd "$out_parent" && pwd -P) ||
		{ echo "release-artifacts: cannot enter --out parent: $out_parent" >&2; exit 2; }
	case $out_parent in
	*/) OUT=$out_parent$out_leaf ;;
	*) OUT=$out_parent/$out_leaf ;;
	esac
fi

# Three paths are refused by identity, however clean they look: / and $HOME
# because no build output lives there, and the repo root because that is where
# an --out that expanded empty in a caller lands (OUT=$PWD, and the wipe takes
# the checkout). The content check below would catch all three on any real
# machine; these say so in one line instead of listing a stray file.
repo_path=$(cd "$repo" && pwd -P)
if [ "$OUT" = "/" ]; then
	echo "release-artifacts: refusing --out / — that is the filesystem root" >&2
	exit 2
fi
if [ "$OUT" = "$repo_path" ]; then
	echo "release-artifacts: refusing --out $OUT — that is the repository root" >&2
	exit 2
fi
if [ -n "${HOME:-}" ] && [ -d "$HOME" ] && [ "$OUT" = "$(cd "$HOME" && pwd -P)" ]; then
	echo "release-artifacts: refusing --out $OUT — that is \$HOME" >&2
	exit 2
fi

# An --out that already exists may be wiped only if everything in it is
# something this script put there (plus posse.rb, which tap-formula.sh writes
# beside them, and .DS_Store, which Finder writes and which is nobody's work).
# Anything else is the operator's, and we do not know what it is.
if [ -d "$OUT" ]; then
	stray=
	for entry in "$OUT"/* "$OUT"/.*; do
		name=${entry##*/}
		case $name in .|..) continue ;; esac
		# An unmatched glob stays literal in POSIX sh; -e/-L skips it.
		[ -e "$entry" ] || [ -L "$entry" ] || continue
		case $name in
		posse_*.tar.gz|posse-*.bottle.tar.gz|checksums.txt|posse.rb|.DS_Store)
			# Allowed as a plain entry only. A DIRECTORY named
			# posse_x.tar.gz would be removed whole, contents unseen,
			# so the name alone does not license the wipe.
			[ -d "$entry" ] || continue
			;;
		esac
		stray=$name
		break
	done
	if [ -n "$stray" ]; then
		cat >&2 <<EOF
release-artifacts: refusing to wipe $OUT
  it holds "$stray", which this script did not write.
  --out is emptied before the build, so it may name only a directory that is
  absent, empty, or holds nothing but posse_*.tar.gz, posse-*.bottle.tar.gz,
  checksums.txt, posse.rb.
  If that directory is yours to lose, remove it yourself and re-run.
EOF
		exit 2
	fi
fi

rm -rf "$OUT"
mkdir -p "$OUT"
# <<< out-guard (ranger-base-9hyc)

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

# ---------------------------------------------------------------------------
# Bottles (ranger-base-9vg3)
# ---------------------------------------------------------------------------
# A bottle is what stops `brew install` entering its build-from-source path,
# and that path is fatal on a Mac whose Command Line Tools are behind its
# macOS: brew runs `fatal_build_from_source_checks` — Xcode, the CLT version,
# the SDK — BEFORE it unpacks anything, so the one route INSTALL.md sells as
# "a release binary, no Go needed" was the one route that needed a toolchain.
# Measured on Homebrew 6.0.20 / macOS 26.4.1 arm64, both arms: with a bottle
# brew prints `Pouring posse-0.3.0.arm64_sonoma.bottle.tar.gz` and installs;
# with the bottle block deleted, the same install on the same box dies with
# `Your Command Line Tools are too outdated`.
#
# A bottle is NOT a build. It is the keg — what `def install` would have left
# in the Cellar — tarred up as `<name>/<version>/...`. So it is made here, from
# the binary we just cross-compiled, on whatever box cuts the release: no brew,
# no Mac, no `brew bottle`, and by construction the same bytes as the tarball
# beside it. `brew bottle` would additionally stamp an INSTALL_RECEIPT.json;
# brew tolerates its absence (`Tab.for_keg` returns an empty tab) and writes
# its own at pour time — verified by pouring one of these.
#
# THE KEG LAYOUT IS THE FORMULA'S `def install`, AND THE TWO MUST AGREE.
# scripts/tap-formula.sh renders `bin.install "posse"` and
# `doc.install "README.md", "INSTALL.md"`, so the keg is bin/posse plus
# share/doc/posse/{README.md,INSTALL.md} and nothing else — LICENSE ships in
# the tarball and is not installed by the formula. A bottle that disagrees
# with `def install` gives a poured install different contents from a source
# one, silently. tapformula_qa_test.go pins the two against each other.
#
# THE FILENAME IS BREW'S, NOT OURS. brew asks the root_url for
# `#{name}-#{version}.#{tag}.bottle.tar.gz` — ONE dash (Bottle::Filename
# #url_encode). Its own cache spells the same file with two (`posse--0.3.0…`),
# which is the spelling every doc and every `brew bottle` output shows, and
# uploading THAT name 404s at install time on a box nobody tested. Measured.
bottle_tag() { # $1=goos $2=goarch -> the brew bottle tag
	case $1/$2 in
	# One macOS tag per arch, at HOMEBREW_MACOS_OLDEST_SUPPORTED (sonoma, 14).
	# brew falls back to a bottle built for an OLDER macOS
	# (OS::Mac::Bottles::Collector#find_older_compatible_tag), so sonoma covers
	# sequoia, tahoe and whatever comes next without a new asset per release
	# of macOS — verified by pouring an arm64_sonoma bottle on macOS 26 Tahoe.
	# Anything older than sonoma is a Homebrew that brew itself calls
	# unsupported; it builds from source, as it did before this bead.
	darwin/arm64) printf arm64_sonoma ;;
	darwin/amd64) printf sonoma ;;
	# Linux has NO older-version fallback — the collector override is
	# macOS-only — so these two tags are exact and complete.
	linux/arm64) printf arm64_linux ;;
	linux/amd64) printf x86_64_linux ;;
	*) echo "release-artifacts: no bottle tag for $1/$2" >&2; exit 1 ;;
	esac
}

bottle_from() { # $1=stage dir $2=goos $3=goarch
	_tag=$(bottle_tag "$2" "$3")
	_keg=$tmp/keg/$2-$3/posse/$bare
	mkdir -p "$_keg/bin" "$_keg/share/doc/posse"
	cp "$1/posse" "$_keg/bin/posse"
	for _doc in README.md INSTALL.md; do
		cp "$1/$_doc" "$_keg/share/doc/posse/$_doc"
	done
	_bottle=$OUT/posse-${bare}.${_tag}.bottle.tar.gz
	(cd "$tmp/keg/$2-$3" && tar -czf "$_bottle" posse)
	echo "  $(basename "$_bottle")  $(wc -c <"$_bottle" | tr -d ' ') bytes"
}

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

	bottle_from "$stage" "$goos" "$goarch"
done

# checksums.txt is what the formula renderer reads and what a human verifies a
# download against, so it is plain `sha256  name` in the OUT directory's own
# terms — no paths, so `shasum -c` works from inside the download dir.
sum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
(cd "$OUT" && sum posse_${bare}_*.tar.gz posse-${bare}.*.bottle.tar.gz > checksums.txt)
echo "release-artifacts: wrote $OUT/checksums.txt"
cat "$OUT/checksums.txt" | sed 's/^/  /'
