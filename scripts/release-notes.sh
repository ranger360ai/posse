#!/bin/sh
# Print one CHANGELOG section — the text that LEADS a GitHub Release's notes,
# above the commit list --generate-notes builds (ranger-base-5356).
#
# Usage: scripts/release-notes.sh [--version vX.Y.Z] [--file <changelog>] [--require]
#   --version  the tag being cut. Default: the exact tag pointing at HEAD.
#   --file     changelog to read (default: CHANGELOG.md beside this repo root)
#   --require  exit 1 unless the version has a section OF ITS OWN. Without it,
#              a missing section is a warning on stderr and an empty stdout.
#   --cap N    keep whole `### ` subsections while the output fits N characters
#              (default 60000; 0 = no cap) and close with a pointer to the rest.
#              GitHub refuses a release body over 125,000 characters with HTTP
#              422, AFTER the tag is pushed (v0.5.0's first run, ranger-base-597lo).
#
# THE DEFAULT IS LENIENT ON PURPOSE. This runs inside the release workflow,
# AFTER the tag is pushed, and a version number cannot be reused: a release
# that dies here is a burned number and no release at all. So nothing in this
# script may be the reason a tag fails. The strict arm is the precondition —
# `make release-notes VERSION=vX.Y.Z`, run while the number is still free.
#
# The `## Unreleased` fallback exists for the same reason. A cutter who forgot
# to rename the heading still ships the entry; --require is what tells them,
# before the tag, that the rename is outstanding.
#
# Blast radius: reads one file, writes stdout. No network, no writes, no git.
set -eu

VERSION=
FILE=
REQUIRE=0
CAP=60000

while [ $# -gt 0 ]; do
	case $1 in
	--version) VERSION=${2:?--version needs a tag}; shift 2 ;;
	--file) FILE=${2:?--file needs a path}; shift 2 ;;
	--require) REQUIRE=1; shift ;;
	--cap) CAP=${2:?--cap needs a character count}; shift 2 ;;
	-h|--help) sed -n '2,22p' "$0"; exit 0 ;;
	*) echo "release-notes: unknown argument: $1" >&2; exit 2 ;;
	esac
done

if [ -z "$FILE" ]; then
	if repo=$(git rev-parse --show-toplevel 2>/dev/null); then
		FILE=$repo/CHANGELOG.md
	else
		FILE=CHANGELOG.md
	fi
fi
if [ ! -f "$FILE" ]; then
	echo "release-notes: no changelog at $FILE" >&2
	exit 1
fi

if [ -z "$VERSION" ]; then
	VERSION=$(git describe --tags --exact-match 2>/dev/null) || VERSION=
fi
if [ -n "$VERSION" ]; then
	# The same allowlist the workflow's tag guard uses, for the same reason:
	# this string becomes a regex below, and `v0.4.0|^## ` would otherwise
	# match every heading in the file.
	case $VERSION in
	v[0-9]*) ;;
	*) echo "release-notes: --version must look like vX.Y.Z, got: $VERSION" >&2; exit 2 ;;
	esac
	case $VERSION in
	*[!v0-9.]*) echo "release-notes: --version must look like vX.Y.Z, got: $VERSION" >&2; exit 2 ;;
	esac
fi

# One section's body: everything after a matching `## ` heading, up to the next
# `## ` heading, with the blank lines top and bottom trimmed off.
section() {
	awk -v want="$1" '
		/^## / { if (inSec) exit; inSec = ($0 ~ want); next }
		inSec { body[++n] = $0 }
		END {
			s = 1; while (s <= n && body[s] ~ /^[ \t]*$/) s++
			e = n; while (e >= s && body[e] ~ /^[ \t]*$/) e--
			for (i = s; i <= e; i++) print body[i]
		}
	' "$FILE"
}

# Whole `### ` subsections while they fit CAP characters — the text before the
# first subsection always ships — and one closing line naming what was left
# in CHANGELOG.md. The section in the tagged tree is the record; the release
# body is its opening. With CAP=0 the section passes through untouched.
capped() {
	if [ "${CAP:-0}" -le 0 ]; then cat; return; fi
	awk -v lim="$CAP" -v ver="$VERSION" '
		{ line[NR] = $0; if ($0 ~ /^### /) { sec++ } part[NR] = sec; size[sec] += length($0) + 1; total++ }
		END {
			keep = 0; used = 0
			for (k = 0; k <= sec; k++) { if (k > 0 && used + size[k] > lim) break; used += size[k]; keep = k }
			for (i = 1; i <= NR; i++) if (part[i] <= keep) print line[i]
			if (keep < sec) {
				dropped = 0; for (i = 1; i <= NR; i++) if (part[i] > keep) dropped++
				printf "\n_This is the opening of the `%s` section: %d of its %d lines. The whole section is CHANGELOG.md in the tagged tree; a release body is capped at 125,000 characters._\n", ver, NR - dropped, NR
			}
		}
	'
}

out=
if [ -n "$VERSION" ]; then
	esc=$(printf '%s' "$VERSION" | sed 's/\./\\./g')
	# Anchored on both ends so `v0.4` does not match `## v0.4.1`; a heading may
	# still carry a date or a title after the version.
	out=$(section "^## ${esc}([^0-9.]|\$)")
fi

if [ -n "$out" ]; then
	printf '%s\n' "$out" | capped
	exit 0
fi

fallback=$(section '^## [Uu]nreleased')
if [ -n "$fallback" ]; then
	if [ "$REQUIRE" -eq 1 ]; then
		echo "release-notes: $FILE has no section for ${VERSION:-this version} — rename '## Unreleased' to '## $VERSION' before tagging" >&2
		exit 1
	fi
	echo "release-notes: no section for ${VERSION:-this version}; using '## Unreleased' (rename it — docs/runbooks/release.md)" >&2
	printf '%s\n' "$fallback" | capped
	exit 0
fi

echo "release-notes: $FILE has no section for ${VERSION:-this version} and no '## Unreleased' — the release notes will be the generated commit list only" >&2
[ "$REQUIRE" -eq 0 ] || exit 1
exit 0
