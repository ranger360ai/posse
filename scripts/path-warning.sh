#!/bin/sh
# Say so when the binary `make install` just wrote is not the one the shell
# will find (ranger-base-88m).
#
# Usage: scripts/path-warning.sh <bindir>
#
# BINDIR defaults to ~/.local/bin, which is on no default macOS or Linux PATH.
# `install` exits 0, prints "installed: ~/.local/bin/posse", and the next line
# the README tells you to type — `posse init` — is `command not found`. Debian's
# skel .profile prepends ~/.local/bin only when the directory already exists at
# login, and `make install` creates it mid-session, so even there the shell you
# are standing in does not see it. herdr's own installer, which lands `herdr` in
# the same directory, prints this warning; posse's install did not.
#
# Three cases, in the order they matter:
#   nothing on PATH        -> the ranger-base-88m failure: name the export
#   a DIFFERENT posse      -> the ranger-base-253 ambiguity: an older install,
#                             or a brew one, answers for the fresh binary
#   the binary just written -> silent
#
# This warns; it does not fail. `make install` promoting the fleet's binary on a
# box whose PATH is already right must stay exit 0, and a PATH the installer
# cannot edit is not a broken build. Warnings go to stderr so they survive a
# caller that keeps only one stream.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <bindir>" >&2
	exit 2
fi

bindir=$1
installed=$bindir/posse

resolved=$(command -v posse 2>/dev/null) || resolved=

if [ -z "$resolved" ]; then
	cat >&2 <<WARN
  WARNING : $bindir is not in your PATH — the next command is \`posse: command not found\`
            add it to this shell, and to your shell profile (~/.zshrc, ~/.bashrc):

              export PATH="$bindir:\$PATH"

WARN
	exit 0
fi

if [ "$resolved" != "$installed" ] && ! [ "$resolved" -ef "$installed" ]; then
	echo "  WARNING : PATH resolves posse to $resolved, not the binary just installed" >&2
fi
