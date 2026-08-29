#!/usr/bin/env bash
# Detective control for ADR 0015 section 3's fence: does every PID in a
# promoted posse home actually carry the deny set the ADR says it carries?
# (ranger-base-d866.)
#
# WHY THIS EXISTS. ranger-base-d866 found the fence for bd's destructive and
# egress verbs living only in one repo's .claude/settings.json, which a persona
# dispatched into any other tree does not read. The fix (ADR 0015 section 3,
# amended 2026-08-29 by ranger-base-u9ud) is to carry the rules in the PID
# instead: a PID's deny: is what becomes the L1 PATH shim and, on claude, the
# --disallowedTools argv. Both travel with the session's environment and are
# indifferent to which repo the session is standing in. A settings file is
# neither.
#
# What that fix depends on is every PID being edited by hand and staying
# edited, and that dependency is the thing nothing was checking. On 2026-08-29
# the nine shipped example PIDs carried the bd rules within hours of the
# amendment while the eleven PIDs of the crew that actually dispatches carried
# none of them, and no command on this box would have said so. This is that
# command. It reads PIDs. It writes nothing and it runs no bd.
#
# WHAT IT IS NOT. A rule present in a PID is not a rule enforced. Whether it is
# REALIZED on a given runtime and cage is `posse gates <persona>`, which probes
# the render; whether it is enforced rather than cooperative is ADR 0025's
# question, and for both layers here the answer is cooperative. Green here
# means the rules are spelled where the launch will read them, no more.
#
# EXIT CODES. 0 every PID carries the set - 1 at least one is missing a rule -
# 2 nothing was measured (no home, no agents dir, no PIDs, unreadable file),
# which is not a pass and must never be read as one.
set -uo pipefail

# ADR 0015 section 3, in the ADR's own order. The bd list is the one staged and
# measured on ranger-base-az93 / ranger-base-3bqn; `posse promote` is the older
# "Fence, spelled twice" bullet whose authority the bd amendment reuses.
#
# The four hook rules name a SUBVERB. That is not tidiness, it is the
# ranger-base-c7ek amendment, and the whole-verb spellings they replace are in
# FORBIDDEN below because carrying one is a defect, not a stricter fence.
REQUIRED=(
  'Bash(posse promote:*)'
  'Bash(bd daemon:*)' 'Bash(bd daemons:*)' 'Bash(bd admin:*)'
  'Bash(bd delete:*)' 'Bash(bd doctor:*)'
  'Bash(bd hook install:*)' 'Bash(bd hook uninstall:*)'
  'Bash(bd hooks install:*)' 'Bash(bd hooks uninstall:*)'
  'Bash(bd import:*)' 'Bash(bd init:*)'
  'Bash(bd migrate:*)' 'Bash(bd rename:*)' 'Bash(bd rename-prefix:*)'
  'Bash(bd repair:*)' 'Bash(bd repo:*)' 'Bash(bd federation:*)'
  'Bash(bd config set:*)' 'Bash(bd config unset:*)'
  'Bash(bd dep relate:*)' 'Bash(bd relate:*)' 'Bash(bd sync --full:*)'
  'Bash(bd jira:*)' 'Bash(bd linear:*)' 'Bash(bd setup:*)'
)

# Rules a PID must NOT carry. Presence is a finding of the same weight as an
# absence above, and this is the only list in the fence where that is true.
#
# WHY. The L1 shim a deny renders sits on PATH and is therefore matched by
# EVERY execve of `bd`, not only the ones a persona types. `bd hook pre-commit`,
# `bd hook post-checkout`, `bd hook post-merge`, `bd hooks run pre-push` and
# `bd hooks run prepare-commit-msg` are what beads' OWN installed git hooks
# exec. A whole-verb deny on hook/hooks therefore refuses beads' hooks, the
# hooks exit non-zero, and git aborts: a persona carrying it cannot commit and
# cannot check out AT ALL in any repo where bd installed hooks. Measured
# 2026-08-29 across three personas (ranger-base-c7ek), which is how it was
# found - it blocked the verifier's own commit.
#
# The hazard the whole-verb rule was reaching for is install/uninstall, which
# REQUIRED still denies. Nothing is given up by narrowing: typed `bd hook ...`
# is refused one layer up by scripts/bd-argv-gate.py, which is an ALLOW-list
# and so walls every spelling of the verb - including ones this list cannot
# enumerate. L2 covers what a persona types; L1 must stay narrow precisely
# because it also sees what git spawns.
FORBIDDEN=(
  'Bash(bd hook:*)'
  'Bash(bd hooks:*)'
)

# deny_rules <pid> - one rule per line, from either spelling of the list. Both
# are live in the fleet: the shipped examples write a block sequence, the crew
# PIDs were written as a flow sequence on one line. A rule containing a comma
# would split wrong in the flow arm; none of the required ones does, and
# Bash(git commit unless --) is the closest call - no comma, and the awk below
# keeps it whole. Only the frontmatter is read: the body of a PID discusses its
# own deny rules in prose, and prose is not a fence. That is guarded twice, by
# the fm filter and by the exit at the closing ---, and the two are redundant:
# MEASURED, removing either one alone leaves the self-test green and only
# removing both turns the gap arm red. Redundant, not unpinned - do not read
# the surviving single mutants as a hole.
deny_rules() {
  awk '
    /^---[[:space:]]*$/ { fm++; if (fm == 2) exit; next }
    fm != 1 { next }
    /^deny:[[:space:]]*\[/ {
      line = $0
      sub(/^deny:[[:space:]]*\[/, "", line)
      sub(/\][[:space:]]*$/, "", line)
      n = split(line, parts, /,[[:space:]]*/)
      for (i = 1; i <= n; i++) {
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", parts[i])
        if (parts[i] != "") print parts[i]
      }
      next
    }
    /^deny:[[:space:]]*$/ { block = 1; next }
    block && /^[[:space:]]+#/ { next }
    block && /^[[:space:]]+-[[:space:]]/ {
      rule = $0
      sub(/^[[:space:]]+-[[:space:]]+/, "", rule)
      sub(/[[:space:]]+$/, "", rule)
      print rule
      next
    }
    block { block = 0 }
  ' "$1"
}

verbose=0
ELIDE_OVER=4

check_home() {
  local home=$1 agents pid name rule have rules found flagged
  local pids=0 bad=0 checked=0
  agents="$home/agents"
  if [ ! -d "$agents" ]; then
    echo "nothing measured: no agents/ under $home" >&2
    return 2
  fi
  for pid in "$agents"/*.md; do
    [ -e "$pid" ] || continue
    if [ ! -r "$pid" ]; then
      echo "nothing measured: $pid is unreadable" >&2
      return 2
    fi
    pids=$((pids + 1))
    name=$(basename "$pid" .md)
    rules=$(deny_rules "$pid")
    flagged=0
    local -a missing=()
    for rule in "${REQUIRED[@]}"; do
      checked=$((checked + 1))
      found=0
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then found=1; break; fi
      done <<<"$rules"
      [ "$found" = 1 ] || missing+=("$rule")
    done
    if [ ${#missing[@]} -gt 0 ]; then
      flagged=1
      echo "MISSING  $name (${#missing[@]} of ${#REQUIRED[@]}):"
      # A whole crew missing a whole list is the expected FIRST reading of this
      # script, and 250 lines of it buries the summary. Elide by default.
      if [ "$verbose" = 1 ] || [ ${#missing[@]} -le "$ELIDE_OVER" ]; then
        printf '           %s\n' "${missing[@]}"
      else
        printf '           %s\n' "${missing[@]:0:$ELIDE_OVER}"
        echo "           ... and $((${#missing[@]} - ELIDE_OVER)) more (--verbose for all)"
      fi
    fi
    # The other direction. A PID carrying a FORBIDDEN rule is broken even when
    # it carries every REQUIRED one - the two lists are not alternatives, and
    # the set that holds BOTH spellings is the shape a presence-only check
    # calls clean while its persona cannot commit (ranger-base-c7ek). Never
    # elided: the list is short and each entry is a live outage.
    local -a carries=()
    for rule in "${FORBIDDEN[@]}"; do
      checked=$((checked + 1))
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then carries+=("$rule"); break; fi
      done <<<"$rules"
    done
    if [ ${#carries[@]} -gt 0 ]; then
      flagged=1
      echo "CARRIES  $name (${#carries[@]} rule(s) that wall beads' own git hooks):"
      printf '           %s\n' "${carries[@]}"
    fi
    bad=$((bad + flagged))
  done
  if [ "$pids" -eq 0 ]; then
    echo "nothing measured: no PIDs in $agents" >&2
    return 2
  fi
  # The positive witness. An assertion of pure absence is satisfied by
  # measuring nothing (ranger-base-fm4p), so say what was actually read.
  echo "scanned $pids PIDs in $agents against ${#REQUIRED[@]} required and ${#FORBIDDEN[@]} forbidden rules ($checked comparisons)"
  if [ "$bad" -ne 0 ]; then
    echo "$bad PID(s) do not carry the ADR 0015 section 3 fence as written"
    return 1
  fi
  echo "every PID carries the ADR 0015 section 3 fence and none walls beads' hooks"
  return 0
}

# Every arm below has a wrong answer that fails: two complete PIDs that must
# come back clean (one per list spelling), two PIDs that must be flagged - one
# for an absence and one for a presence - and two homes with nothing in them
# that must exit 2 rather than 0.
self_test() {
  local d rc=0 out r full_block full_flow
  d=$(mktemp -d) || return 2
  mkdir -p "$d/block/agents" "$d/flow/agents" "$d/gap/agents" "$d/broad/agents" "$d/empty/agents"

  full_block=$(printf -- '  - %s\n' "${REQUIRED[@]}")
  full_flow=$(printf '%s, ' "${REQUIRED[@]}")
  full_flow=${full_flow%, }

  {
    echo '---'
    echo 'name: complete-block'
    echo 'deny:'
    echo '  - Edit'
    echo '  # a comment inside the list must not end the block'
    echo "$full_block"
    echo '---'
    echo 'body: deny: [Bash(nonsense:*)] in prose must not be read as a rule'
  } > "$d/block/agents/a.md"

  {
    echo '---'
    echo 'name: complete-flow'
    echo "deny: [Bash(git commit unless --), $full_flow]"
    echo '---'
    echo body
  } > "$d/flow/agents/a.md"

  # The discriminating arm, carrying two mutants at once: `daemons` present and
  # `daemon` absent, which a substring test passes and the fence does not; and
  # the absent rule written out in the BODY, where a PID's prose really does
  # discuss its own rules, so a parser that reads past the frontmatter calls
  # this PID clean and this arm fails.
  {
    echo '---'
    echo 'name: plural-only'
    echo 'deny:'
    printf -- '  - %s\n' "${REQUIRED[@]}" | grep -vxF -- '  - Bash(bd daemon:*)'
    echo '---'
    echo 'Daemon lifecycle belongs to the operator. This PID is fenced by'
    echo 'deny:'
    echo '  - Bash(bd daemon:*)'
  } > "$d/gap/agents/a.md"

  # The presence arm. This PID carries EVERY required rule, so the missing-rule
  # half is silent, and it also carries the whole-verb hook denies - the exact
  # shape that shipped to eleven PIDs and left none of them able to commit. An
  # audit that only asks "is the rule there?" calls this file clean.
  {
    echo '---'
    echo 'name: keeps-broad-too'
    echo 'deny:'
    printf -- '  - %s\n' "${REQUIRED[@]}"
    printf -- '  - %s\n' "${FORBIDDEN[@]}"
    echo '---'
    echo body
  } > "$d/broad/agents/a.md"

  out=$(check_home "$d/block"); r=$?
  if [ $r -eq 0 ] && [[ $out == *"scanned 1 PIDs"* ]]; then
    echo "self-test PASS: a complete block-sequence PID is clean, and 1 PID was read"
  else
    echo "self-test FAIL: complete block PID flagged (rc=$r): $out"; rc=1
  fi

  out=$(check_home "$d/flow"); r=$?
  if [ $r -eq 0 ]; then
    echo "self-test PASS: a complete flow-sequence PID is clean"
  else
    echo "self-test FAIL: complete flow PID flagged (rc=$r): $out"; rc=1
  fi

  out=$(check_home "$d/gap"); r=$?
  if [ $r -eq 1 ] && [[ $out == *'Bash(bd daemon:*)'* ]]; then
    echo "self-test PASS: a PID carrying only daemons is flagged for daemon"
  else
    echo "self-test FAIL: the daemon/daemons gap was not caught (rc=$r): $out"; rc=1
  fi

  out=$(check_home "$d/broad"); r=$?
  if [ $r -eq 1 ] && [[ $out == *'CARRIES'* ]] && [[ $out == *'Bash(bd hook:*)'* ]]; then
    echo "self-test PASS: a PID carrying every required rule AND the broad hook denies is flagged"
  else
    echo "self-test FAIL: the whole-verb hook deny was not caught (rc=$r): $out"; rc=1
  fi

  out=$(check_home "$d/empty" 2>&1); r=$?
  if [ $r -eq 2 ]; then
    echo "self-test PASS: an agents/ holding no PIDs exits 2, not 0"
  else
    echo "self-test FAIL: empty agents/ returned rc=$r, want 2"; rc=1
  fi

  out=$(check_home "$d/nosuch" 2>&1); r=$?
  if [ $r -eq 2 ]; then
    echo "self-test PASS: a home with no agents/ exits 2, not 0"
  else
    echo "self-test FAIL: missing home returned rc=$r, want 2"; rc=1
  fi

  rm -rf "$d"
  return $rc
}

home=""
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test)
      self_test
      exit $?
      ;;
    --verbose)
      verbose=1
      ;;
    -h|--help)
      echo "usage: $(basename "$0") [--verbose] [<posse-home>]"
      echo "       $(basename "$0") --self-test"
      exit 0
      ;;
    -*)
      echo "usage: $(basename "$0") [--verbose] [<posse-home>|--self-test]" >&2
      exit 2
      ;;
    *)
      home=$1
      ;;
  esac
  shift
done

home=${home:-${RHQ_HOME:-$HOME/.config/posse}}
check_home "$home"
exit $?
