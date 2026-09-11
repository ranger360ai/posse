#!/usr/bin/env bash
# shell-syntax.sh — every shell script this repo tracks still PARSES, under
# the shell its own shebang names (ranger-base-g4z8m).
#
#   scripts/shell-syntax.sh              sweep this repo
#   scripts/shell-syntax.sh --census     path<TAB>interpreter, one row per file
#   scripts/shell-syntax.sh --self-test  prove the sweep can still say no
#   scripts/shell-syntax.sh --root DIR   sweep DIR instead (--self-test uses it)
#
# WHY THIS EXISTS. scripts/verify-box.sh carries three tables of PROSE —
# ROSTER, EXCLUDED, UNTARGETED — and each one is a quoted heredoc inside a
# command substitution:
#
#     EXCLUDED=$(cat <<'EXCLUDED_EOF'
#     verify-gotest	tree check of the wrapper
#     EXCLUDED_EOF
#     )
#
# bash scans $( ) for its closing paren while tracking quote state, and the
# heredoc quoting does not stop that. So a single apostrophe in a reason
# field — the natural English for a possessive, and every row there is English
# written by whoever added a check — opens a quote that never closes and
# swallows the rest of the file. MEASURED 2026-09-11, darwin arm64, under
# bash 3.2.57 and bash 5 alike: the parser reports a syntax error at line 318,
# hundreds of lines below the table, on a case-statement token nobody touched.
# Row 203 of that table reads "this repo own seed PIDs", ungrammatical, which
# is what meeting this hazard once and working around it without writing it
# down looks like.
#
# WHY A SWEEP AND NOT A COMMENT ABOVE THE TABLES. boxcheck_qa_test.go fails
# when a verify-* target is in neither table, so those tables WILL keep being
# edited by hand, and this is the failure mode of editing them. A warning in a
# header is read by whoever already opened the file; `bash -n` is read by
# whoever is about to break it. And the class is wider than one file: a shell
# script either parses or it does not, and until this landed nothing in the
# tree ever asked.
#
# THE CORPUS IS TRACKED FILES, TWO RULES. Every tracked *.sh, plus every
# tracked file whose first line is a shell shebang. Either rule alone has a
# hole, and the hole is the one verify-box.sh's own UNTARGETED table exists
# for: a check keyed on a NAME cannot see a file that arrives under another
# one. A .sh with no shebang is a finding rather than a skip — nothing can say
# which parser applies to it, so nothing can say it parses.
#
# Tracked, not found: git ls-files is how every other tree-wide pin here
# defines the tree, and it is what ships. It also lists the staged half of a
# work in progress, so `git add` then `make test` covers a script the same
# commit is about to carry. A script that is in neither is covered by nobody,
# by the same rule that covers nothing else untracked.
#
# THE SHEBANG PICKS THE PARSER, because the shebang is what picks the parser
# at run time. `#!/bin/sh` is checked with sh — which is dash on the linux
# runners and bash in sh mode on darwin, i.e. on each box the thing that will
# actually run it. MEASURED 2026-09-11: all 13 of this repo's `#!/bin/sh`
# scripts parse under /bin/dash as well as under sh, so the split costs
# nothing today and will name the box it disagrees on if it ever does.
#
# EXIT STATUS, in the house form:
#   0  every script in the corpus parsed
#   1  at least one FINDING — a script that does not parse, or a .sh whose
#      parser cannot be identified
#   2  NOTHING WAS MEASURED, or an ERROR: an empty corpus, a root that is no
#      git work tree, or a shebang naming an interpreter this box lacks. An
#      empty corpus is not a pass; it is a green over an empty room.
#
# READ-ONLY. `-n` parses and does not execute, and the self-test has an arm
# that says so: a fixture whose body would drop a canary file leaves none.
set -uo pipefail

self=$(basename "$0")
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd) || {
  echo "$self: cannot locate the repo from $0" >&2; exit 2; }

usage() {
  echo "usage: $self [--census|--self-test] [--root DIR]" >&2
}

# interp_of <first line> — the SHELL basename that line names, or nothing.
# Answers for `#!/bin/sh`, `#!/usr/bin/env bash`, `#!/bin/bash -e` and
# `#!/usr/bin/env -S zsh`; answers nothing for a python shebang and for a file
# with no shebang at all. Written with parameter expansion rather than word
# splitting so a path with a glob character in it cannot expand.
interp_of() {
  local rest=${1%$'\r'} word
  case "$rest" in '#!'*) rest=${rest#'#!'} ;; *) return 0 ;; esac
  while [ -n "$rest" ]; do
    case "$rest" in
      ' '*|$'\t'*) rest=${rest#?}; continue ;;
    esac
    word=${rest%%[$' \t']*}
    rest=${rest#"$word"}
    word=${word##*/}
    case "$word" in
      env|-*|*=*) continue ;;   # `env`, its own flags, and VAR=value
    esac
    case "$word" in
      sh|bash|dash|ksh|zsh) printf '%s\n' "$word" ;;
    esac
    return 0
  done
}

# first_line <path> — read with the builtin, no fork per file. A file with no
# trailing newline still yields its first line, which is why the read's
# non-zero status is ignored.
first_line() {
  local line=""
  IFS= read -r line < "$1" 2>/dev/null || true
  printf '%s\n' "$line"
}

# corpus <root> — the tracked paths to check, one per line.
corpus() {
  local root=$1 f
  git -C "$root" ls-files -z 2>/dev/null | while IFS= read -r -d '' f; do
    [ -f "$root/$f" ] || continue
    case "$f" in
      *.sh) printf '%s\n' "$f"; continue ;;
    esac
    [ -n "$(interp_of "$(first_line "$root/$f")")" ] && printf '%s\n' "$f"
  done
}

is_worktree() {
  git -C "$1" rev-parse --is-inside-work-tree >/dev/null 2>&1
}

census() {
  local root=$1 f interp
  is_worktree "$root" || { echo "$self: $root is not a git work tree" >&2; return 2; }
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    interp=$(interp_of "$(first_line "$root/$f")")
    printf '%s\t%s\n' "$f" "${interp:-?}"
  done < <(corpus "$root")
}

sweep() {
  local root=$1 f line interp out n=0 findings=0 errors=0
  is_worktree "$root" || { echo "$self: $root is not a git work tree" >&2; return 2; }
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    n=$((n + 1))
    line=$(first_line "$root/$f")
    interp=$(interp_of "$line")
    if [ -z "$interp" ]; then
      printf 'FINDING %s\n  no shell shebang — nothing can say which parser applies, so nothing can say it parses\n' "$f"
      findings=$((findings + 1))
      continue
    fi
    if ! command -v "$interp" >/dev/null 2>&1; then
      printf 'ERROR %s\n  its shebang names %s, which is not on PATH on this box\n' "$f" "$interp"
      errors=$((errors + 1))
      continue
    fi
    if ! out=$("$interp" -n "$root/$f" 2>&1); then
      printf 'FINDING %s (%s)\n' "$f" "$interp"
      printf '%s\n' "$out" | sed 's|^|  |'
      findings=$((findings + 1))
    fi
  done < <(corpus "$root")

  if [ "$n" -eq 0 ]; then
    echo "$self: no shell script found under $root — a sweep over nothing is not a pass" >&2
    return 2
  fi
  if [ "$findings" -gt 0 ]; then
    cat <<'NOTE_EOF'

The line a shell names is where its parse gave up, not where the file broke.
The usual cause is a quote that never closes, and the one this check was
written for is an apostrophe inside a quoted heredoc that is itself inside a
command substitution: bash tracks quote state while hunting the closing paren,
so the heredoc quoting does not make the apostrophe literal. Build the table
with `read -r -d ''` or keep it in a file (ranger-base-g4z8m).
NOTE_EOF
  fi
  [ "$errors" -gt 0 ] && return 2
  [ "$findings" -gt 0 ] && return 1
  printf '%s: %d shell scripts parse\n' "$self" "$n"
  return 0
}

# --- self-test --------------------------------------------------------------
#
# Each arm plants a throwaway git work tree and sweeps it with this same
# sweep(), so the fixture is measured by the producer rather than by a second
# reading of it. `git init` and `git add` only — nothing here commits, and no
# arm touches the repo it is run from.
st_repo() {
  local d=$st_tmp/$1
  mkdir -p "$d" || return 2
  git -C "$d" init -q >/dev/null 2>&1 || return 2
  printf '%s\n' "$d"
}

st_add() {   # st_add <dir> <path>, body on stdin
  local d=$1 p=$2
  mkdir -p "$d/$(dirname "$p")" || return 2
  cat > "$d/$p" || return 2
  chmod +x "$d/$p"
  git -C "$d" add -- "$p" >/dev/null 2>&1 || return 2
}

st_say() {   # st_say <ok|no> <message>
  st_ran=$((st_ran + 1))
  if [ "$1" = ok ]; then
    printf 'self-test PASS: %s\n' "$2"
  else
    printf 'self-test FAIL: %s\n' "$2"
    st_rc=1
  fi
}

selftest() {
  local d out rc
  st_tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-shell-syntax.XXXXXX") || return 2
  trap 'rm -rf "${st_tmp:?}"' EXIT
  st_rc=0
  st_ran=0

  # Arm 1 — the shape this check was written for. A quoted heredoc inside a
  # command substitution, one apostrophe in the body.
  d=$(st_repo bead) || return 2
  st_add "$d" table.sh <<'FIXTURE_EOF'
#!/usr/bin/env bash
T=$(cat <<'T_EOF'
verify-gotest	tree check of the wrapper's own reuse
T_EOF
)
case "$T" in
  *gotest*) echo yes ;;
esac
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    1:*FINDING*table.sh*) st_say ok "an apostrophe in a heredoc inside \$( ) is caught (exit 1)" ;;
    *) st_say no "the bead shape went unreported: exit $rc, output: $out" ;;
  esac

  # Arm 2 — the control that must come out the other way: the same file, same
  # structure, no apostrophe. Without this the arm above passes on a sweep
  # that reds everything.
  d=$(st_repo control) || return 2
  st_add "$d" table.sh <<'FIXTURE_EOF'
#!/usr/bin/env bash
T=$(cat <<'T_EOF'
verify-gotest	tree check of the wrapper
T_EOF
)
case "$T" in
  *gotest*) echo yes ;;
esac
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    0:*"1 shell scripts parse"*) st_say ok "the same table without the apostrophe is clean (exit 0)" ;;
    *) st_say no "a clean table was not reported clean: exit $rc, output: $out" ;;
  esac

  # Arm 3 — the wider class, so this is a syntax check and not one pattern.
  d=$(st_repo plain) || return 2
  st_add "$d" broken.sh <<'FIXTURE_EOF'
#!/usr/bin/env bash
if true; then
  echo x
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    1:*FINDING*broken.sh*) st_say ok "an unterminated if is caught" ;;
    *) st_say no "an unterminated if went unreported: exit $rc, output: $out" ;;
  esac

  # Arm 4 — an empty corpus exits 2, not 0. A lister that stopped matching
  # would otherwise turn every arm above, and the tree sweep itself, green on
  # nothing at all.
  d=$(st_repo empty) || return 2
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc" in
    2) st_say ok "a corpus of nothing exits 2 rather than passing" ;;
    *) st_say no "an empty corpus exited $rc: $out" ;;
  esac

  # Arm 5 — the shebang picks the parser, and the census says which. Pinning
  # the MAPPING rather than a parse difference on purpose: on darwin sh is
  # bash in sh mode, so no fixture can tell the two parsers apart on the box
  # this is most often typed from.
  d=$(st_repo interp) || return 2
  st_add "$d" posix.sh <<'FIXTURE_EOF'
#!/bin/sh
echo posix
FIXTURE_EOF
  st_add "$d" bashy.sh <<'FIXTURE_EOF'
#!/usr/bin/env bash
echo bashy
FIXTURE_EOF
  out=$(census "$d" 2>&1); rc=$?
  if [ "$rc" -eq 0 ] &&
     printf '%s\n' "$out" | grep -q "^posix\.sh	sh$" &&
     printf '%s\n' "$out" | grep -q "^bashy\.sh	bash$"; then
    st_say ok "each file is parsed by the shell its own shebang names"
  else
    st_say no "the census does not follow the shebang: exit $rc, output: $out"
  fi

  # Arm 6 — a shebang naming a shell this box lacks is an ERROR that names it,
  # not a silent pass and not a quiet fallback to bash. Reached by scrubbing
  # PATH down to the two tools the sweep itself needs rather than by naming a
  # rare shell: every shell this script recognises ships on darwin, so an arm
  # keyed on one being absent would skip on the box it is most often typed
  # from, and a skipped arm is not an arm.
  d=$(st_repo missing) || return 2
  st_add "$d" odd.sh <<'FIXTURE_EOF'
#!/usr/bin/env bash
echo hello
FIXTURE_EOF
  mkdir -p "$st_tmp/nosh" || return 2
  ln -sf "$(command -v git)" "$st_tmp/nosh/git" || return 2
  ln -sf "$(command -v sed)" "$st_tmp/nosh/sed" || return 2
  out=$(PATH=$st_tmp/nosh; sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    2:*ERROR*odd.sh*bash*) st_say ok "a shebang naming a shell this box lacks is an ERROR (exit 2)" ;;
    *) st_say no "a missing interpreter was not reported: exit $rc, output: $out" ;;
  esac

  # Arm 7 — a .sh with no shebang is named, not skipped.
  d=$(st_repo noshebang) || return 2
  st_add "$d" bare.sh <<'FIXTURE_EOF'
echo no shebang here
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    1:*FINDING*bare.sh*) st_say ok "a .sh with no shebang is a finding, not a skip" ;;
    *) st_say no "a .sh with no shebang was skipped: exit $rc, output: $out" ;;
  esac

  # Arm 8 — the corpus is not keyed on the extension. A tracked file with a
  # shell shebang and no .sh is swept too, which is the hole a name-keyed
  # census leaves.
  d=$(st_repo noext) || return 2
  st_add "$d" bin/hook <<'FIXTURE_EOF'
#!/usr/bin/env bash
case x in
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  case "$rc:$out" in
    1:*FINDING*bin/hook*) st_say ok "a shell shebang without a .sh is in the corpus" ;;
    *) st_say no "a shell script without a .sh was not swept: exit $rc, output: $out" ;;
  esac

  # Arm 9 — the sweep PARSES and does not RUN. `-n` is the whole of that
  # promise, and a change from `-n` to a source or an exec would be green
  # everywhere above.
  d=$(st_repo readonly) || return 2
  st_add "$d" effect.sh <<FIXTURE_EOF
#!/usr/bin/env bash
: > "$st_tmp/canary"
FIXTURE_EOF
  out=$(sweep "$d" 2>&1); rc=$?
  if [ "$rc" -eq 0 ] && [ ! -e "$st_tmp/canary" ]; then
    st_say ok "a swept script is parsed and not executed"
  elif [ -e "$st_tmp/canary" ]; then
    st_say no "the sweep EXECUTED a script it was only asked to parse"
  else
    st_say no "the read-only arm could not run: exit $rc, output: $out"
  fi

  # The reader on the arms themselves. st_ran counts the arms that actually
  # REPORTED, so an arm that returned early — a fixture that would not build,
  # a case that fell through its patterns — is the difference between this
  # number and the one below, rather than a line of output nobody counted.
  if [ "$st_ran" -ne "$st_arms" ]; then
    echo "self-test FAIL: $st_ran of $st_arms arms reported; the rest never ran"
    st_rc=1
  fi
  [ "$st_rc" -eq 0 ] && printf 'self-test: %d arms, all clean\n' "$st_ran"
  return $st_rc
}
st_arms=9

mode=sweep
root=$repo
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test) mode=selftest ;;
    --census)    mode=census ;;
    --root)      shift; root=${1:-}; [ -n "$root" ] || { usage; exit 2; } ;;
    -h|--help)   usage; exit 0 ;;
    *)           usage; exit 2 ;;
  esac
  shift
done

case "$mode" in
  census)   census "$root" ;;
  selftest) selftest ;;
  *)        sweep "$root" ;;
esac
