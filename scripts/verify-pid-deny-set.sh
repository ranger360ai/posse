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
# EXIT CODES. 0 every PID carries the set and no superseded rule - 1 at least
# one PID is missing a required rule or still carries a superseded one - 2
# nothing was measured (no home, no agents dir, no PIDs, unreadable file),
# which is not a pass and must never be read as one.
set -uo pipefail

# ADR 0015 section 3, in the ADR's own order. The bd list is the one staged and
# measured on ranger-base-az93 / ranger-base-3bqn; `posse promote` is the older
# "Fence, spelled twice" bullet whose authority the bd amendment reuses.
#
# The hook rows are NARROWED, and the narrowing is load-bearing (operator
# ruling 2026-08-29 evening, ranger-base-y5g7, promoted; the ADR's own copy of
# the list is amended on ranger-base-i6do). u9ud wrote them as the whole verb
# in both spellings, and the whole verb is not something a persona types - it
# is what beads' OWN git hooks run. `.git/hooks/pre-commit` ends by exec-ing
# the singular hook verb and the prepare-commit-msg chain ends by exec-ing the
# plural one, so those two rows refused every commit and every
# `git worktree add` for all eleven crew PIDs at once: a fleet that closes
# beads by committing could not close one (ranger-base-c7ek). `--no-verify` is
# not a way around it - it skips the pre-commit slot and lands on the
# prepare-commit-msg slot one step later, which is why BOTH spellings had to
# narrow or the wall just moves. What the deny was FOR - a persona
# reconfiguring bd's hooks by hand - is exactly install and uninstall, so these
# four rows keep that and hand back the run/slot forms git needs.
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

# Rules a PID must NOT carry. Presence-only would enforce half the ruling: a
# PID that gains the four narrowed rows and KEEPS the superseded broad one
# satisfies every REQUIRED test and still cannot commit, because deny wins
# (ADR 0001) and the broad row is the one bd's own pre-commit hook trips. That
# state is one careless merge away - the broad spelling is what all twenty PIDs
# carried until the evening of 2026-08-29, and what an un-amended ADR still
# prints for someone to copy - so the superseded rows are named and checked for
# ABSENCE rather than left to a reader to notice. This is NOT a general "no
# rule outside REQUIRED" check: a PID's own extra denies are its business, and
# only these two are known to break the fleet.
FORBIDDEN=(
  'Bash(bd hook:*)' 'Bash(bd hooks:*)'
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
  local home=$1 agents pid name rule have rules found
  local pids=0 bad=0 checked=0 flagged
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
    local -a missing=()
    for rule in "${REQUIRED[@]}"; do
      checked=$((checked + 1))
      found=0
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then found=1; break; fi
      done <<<"$rules"
      [ "$found" = 1 ] || missing+=("$rule")
    done
    local -a superseded=()
    for rule in "${FORBIDDEN[@]}"; do
      checked=$((checked + 1))
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then superseded+=("$rule"); break; fi
      done <<<"$rules"
    done
    flagged=0
    if [ ${#superseded[@]} -gt 0 ]; then
      flagged=1
      # Never elided. There are two of these at most, and each one is a PID
      # that cannot commit.
      echo "SUPERSEDED  $name carries ${#superseded[@]} rule(s) the y5g7 ruling removed:"
      printf '           %s\n' "${superseded[@]}"
    fi
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
    [ "$flagged" = 0 ] || bad=$((bad + 1))
  done
  if [ "$pids" -eq 0 ]; then
    echo "nothing measured: no PIDs in $agents" >&2
    return 2
  fi
  # The positive witness. An assertion of pure absence is satisfied by
  # measuring nothing (ranger-base-fm4p), so say what was actually read.
  echo "scanned $pids PIDs in $agents against ${#REQUIRED[@]} required and ${#FORBIDDEN[@]} superseded rules ($checked comparisons)"
  if [ "$bad" -ne 0 ]; then
    echo "$bad PID(s) do not carry the ADR 0015 section 3 fence"
    return 1
  fi
  echo "every PID carries the ADR 0015 section 3 fence"
  return 0
}

# Every arm below has a wrong answer that fails: two complete PIDs that must
# come back clean (one per list spelling), one PID that must be flagged, one
# PID that carries the whole list AND the superseded broad rows, one arm per
# narrowed hook alternative, one list-shape arm, and two homes with nothing in
# them that must exit 2 rather than 0.
self_test() {
  local d rc=0 out r full_block full_flow alt slug alts=0
  d=$(mktemp -d) || return 2
  mkdir -p "$d/block/agents" "$d/flow/agents" "$d/gap/agents" "$d/empty/agents" \
           "$d/superseded/agents"

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

  # ranger-base-t2v2. The regression the narrowing exists to prevent, and the
  # one a presence-only control cannot see: this PID carries every REQUIRED
  # rule, four narrowed hook rows included, and ALSO the two superseded broad
  # rows u9ud wrote. Nothing is missing, so the REQUIRED half calls it clean —
  # and deny wins (ADR 0001), so the broad rows are live and this persona
  # cannot commit or add a worktree at all (ranger-base-c7ek). Delete the
  # FORBIDDEN loop and this arm goes green over that.
  {
    echo '---'
    echo 'name: still-broad'
    echo 'deny:'
    echo "$full_block"
    printf -- '  - %s\n' "${FORBIDDEN[@]}"
    echo '---'
    echo body
  } > "$d/superseded/agents/a.md"

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

  out=$(check_home "$d/superseded"); r=$?
  if [ $r -eq 1 ] && [[ $out == *'SUPERSEDED'* ]] \
     && [[ $out == *'Bash(bd hook:*)'* ]] && [[ $out == *'Bash(bd hooks:*)'* ]] \
     && [[ $out != *'MISSING'* ]]; then
    echo "self-test PASS: a PID keeping the superseded broad hook rows is flagged, with nothing missing"
  else
    echo "self-test FAIL: the superseded broad hook rows were not caught (rc=$r): $out"; rc=1
  fi

  # One arm per narrowed alternative (ranger-base-t2v2, the h6fx lesson: a
  # wholesale rename can collapse two arms into one and leave the test green
  # over the bug it guards). The four hook rows differ by a single character in
  # two places — hook/hooks, install/uninstall — so each is dropped on its own
  # and the report must name THAT row and count exactly one missing. A prefix
  # or substring comparison lets `hooks install` answer for `hook install` and
  # the count arm catches it; a fixture built from REQUIRED itself cannot, and
  # that is what the count is for.
  for alt in 'Bash(bd hook install:*)' 'Bash(bd hook uninstall:*)' \
             'Bash(bd hooks install:*)' 'Bash(bd hooks uninstall:*)'; do
    alts=$((alts + 1))
    slug="alt$alts"
    mkdir -p "$d/$slug/agents"
    {
      echo '---'
      echo "name: $slug"
      echo 'deny:'
      printf -- '  - %s\n' "${REQUIRED[@]}" | grep -vxF -- "  - $alt"
      echo '---'
      echo body
    } > "$d/$slug/agents/a.md"
    out=$(check_home "$d/$slug"); r=$?
    if [ $r -eq 1 ] && [[ $out == *"$alt"* ]] \
       && [[ $out == *"(1 of ${#REQUIRED[@]})"* ]]; then
      echo "self-test PASS: dropping $alt is caught, and only it"
    else
      echo "self-test FAIL: dropping $alt was not caught as exactly one gap (rc=$r): $out"; rc=1
    fi
  done

  # Why the comparison above is allowed to be a plain string equality, stated
  # rather than assumed: MEASURED on ranger-base-t2v2, mutating it to a
  # substring test leaves every arm here green — an EQUIVALENT mutant, because
  # no rule in either list is a proper substring of another. Every rule closes
  # with `:*)`, so `bd hook` cannot answer for `bd hook install` and `bd daemon`
  # cannot answer for `bd daemons`. That property is what makes the mutant
  # equivalent, so it is checked here rather than left as a claim: add a rule
  # that is a prefix of another and this arm reds, and the substring mutant
  # stops being equivalent the same moment.
  local a b overlap=0
  for a in "${REQUIRED[@]}" "${FORBIDDEN[@]}"; do
    for b in "${REQUIRED[@]}" "${FORBIDDEN[@]}"; do
      [ "$a" = "$b" ] && continue
      case "$b" in *"$a"*)
        echo "self-test FAIL: '$a' is a proper substring of '$b'"; overlap=1;;
      esac
    done
  done
  if [ "$overlap" = 0 ]; then
    echo "self-test PASS: no rule is a proper substring of another, so exact match is enough"
  else
    rc=1
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
