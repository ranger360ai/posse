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
# command. It writes nothing, and it runs no beads.
#
# THREE READERS, because the fence has three carriers and a PID is only the
# first of them (ranger-base-9ix7):
#
#   default          check_home     the PIDs under <home>/agents carry the set.
#   --live           check_live     every LIVE persona session's argv fence is
#                                   the fence its PID spells now. The argv is
#                                   rendered at launch and frozen for the life
#                                   of the process, while the L1 shim beside it
#                                   re-renders at every dispatch - so an edited
#                                   PID reaches a running session by one carrier
#                                   and not the other, in both directions.
#   --settings <r>   check_settings that repo's COMMITTED .claude/settings.json
#                                   carries no rule the ruling superseded. No
#                                   renderer writes that file; git checks it out
#                                   and the operator's hand is what changes it.
#
# Each reader's own header says what it does and does not assert. The one thing
# that is deliberately NOT duplicated is the rule list: REQUIRED and FORBIDDEN
# are declared once, below, and a second copy of a fence is the failure mode
# this whole script exists to notice.
#
# WHAT IT IS NOT. A rule present in a PID is not a rule enforced. Whether it is
# REALIZED on a given runtime and cage is `posse gates <persona>`, which probes
# the render; whether it is enforced rather than cooperative is ADR 0025's
# question, and for both layers here the answer is cooperative. Green here
# means the rules are spelled where the launch will read them, no more.
#
# EXIT CODES. 0 clean - 1 findings (a PID missing a required rule or keeping a
# superseded one, a live session whose argv is not its PID's fence, a settings
# file still carrying a superseded rule) - 2 nothing was measured (no home, no
# agents dir, no PIDs, no persona session in the process table, no settings
# file, an unreadable file), which is not a pass and must never be read as one.
# With more than one reader asked for, the exit is the worst of them and 2
# outranks 1: a run half of which was blind must not exit on the quieter
# answer.
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
  # pkill/killall: operator ruling 2026-09-03 (ranger-base-jjx19), staged as
  # ranger-base docs/rca/jjx19-pkill-deny.diff. A pattern kill matches every
  # seat's identical argv; the census confirmed 11 sibling suites ended that way.
  'Bash(pkill:*)' 'Bash(killall:*)'
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

# --- reader 2: the live sessions ---------------------------------------
#
# WHY A SECOND READER (ranger-base-9ix7). A PID's deny: reaches a session by
# three carriers, and they do not refresh alike:
#
#   L1 PATH shim   state/gates/<persona>/bin/*  per-PERSONA. RenderGates
#                  removes the bin dir and writes it fresh at EVERY dispatch,
#                  so a session ALREADY RUNNING picks a change up at its next
#                  exec - it looks the shim up on PATH each time.
#   --disallowedTools argv                      per-SESSION. Rendered at launch
#                  and frozen for the life of the process: nothing can rewrite
#                  a running process's argv.
#   .claude/settings.json                       per-REPO, hand-maintained,
#                  constitution class. Reader 3 below.
#
# So the reader above answers "does the PID carry the rule" and `posse gates`
# answers "is the rule realizable here", and BETWEEN them sits a question
# neither asks: is the fence the sessions are actually running on the fence the
# PID spells today. MEASURED 2026-08-29, and it was not. One session had been
# up since 00:25 with four rules in its argv against the twenty-six its PID now
# carries - every rule of the u9ud amendment absent - while two others were
# still carrying the two broad hook rows the y5g7 ruling superseded, hours
# after their shims had narrowed. Both directions at once: sessions LOOSER than
# the constitution and sessions STRICTER than it (the ranger-base-c7ek
# breakage class, one layer up from the shim that was fixed).
#
# The fix for a flagged session is `posse relaunch <name>`, which lands the
# work first. That is why this is a detective control and not a re-render:
# there is nothing to re-render - argv is not rewritable - and a session
# mid-bead is not something to restart behind the operator's back.

# norm_rule <rule> - a rule reduced to what it MEANS, so a PID spelling and the
# argv spelling rendered from it compare equal.
#
# This exists because the argv is not a copy of the list. L0Spellings
# (internal/posse/gates.go) widens each rule on its way to claude: it adds an
# option-blind twin (`Bash(x -* verb sub *)`) so a global option placed before
# the verb cannot walk past the rule, and it rewrites a NEGATIVE rule entirely
# - claude's dialect has no negation, so `Bash(git commit unless --)` is
# rendered as the bare `Bash(git commit)` plus its twin. A literal comparison
# therefore reports the negative rule missing from every session that is
# perfectly current: MEASURED, it did, on all eleven live sessions before this
# function existed.
#
# The reduction: drop the tool wrapper, drop a negation tail, drop the `-*` and
# trailing `*` tokens the widening adds, and drop a rule's trailing `:*`, which
# is the prefix marker rather than a word. What is left is the command and its
# words, which is the pair both matchers actually key on. A non-Bash rule
# (Edit, Write, Edit(.claude/**)) has no widening and passes through whole.
#
# NOT a reimplementation of L0Spellings, deliberately: it never SYNTHESIZES a
# spelling, so it cannot drift into disagreeing with the renderer about what a
# rule expands to. It only erases the three decorations the renderer adds, and
# the self-test carries the arm that fails if any of the three stops being
# erased.
norm_rule() {
  local r=$1 tool rest out="" t restore=0
  case "$r" in
    *'('*')') tool=${r%%(*}; rest=${r#*(}; rest=${rest%)} ;;
    *) printf '%s\n' "$r"; return 0 ;;
  esac
  if [ "$tool" != "Bash" ]; then
    printf '%s(%s)\n' "$tool" "$rest"
    return 0
  fi
  rest=${rest%% unless *}
  # The token loop word-splits, so a bare `*` would glob against the cwd.
  case $- in *f*) ;; *) set -f; restore=1 ;; esac
  for t in $rest; do
    [ "$t" = "-*" ] && continue
    [ "$t" = "*" ] && continue
    t=${t%":*"}
    [ -n "$t" ] || continue
    out="$out $t"
  done
  [ "$restore" = 1 ] && set +f
  printf 'Bash:%s\n' "$out"
}

# argv_rules <deny region> - the rules out of a rendered launch line's deny
# region, one per line.
#
# ps space-joins argv, so a two-word rule arrives indistinguishable from two
# words. The parentheses are what makes it recoverable and every command rule
# has them; a bare tool name (Edit, Write) is matched separately. Anything the
# region holds that is neither is not a rule this reader knows, and it is left
# out rather than guessed at.
argv_rules() {
  printf '%s\n' "$1" | grep -o '[A-Za-z_][A-Za-z0-9_]*([^)]*)'
  printf '%s\n' "$1" | tr ' ' '\n' | grep -E -x 'Edit|Write'
}

# The sets are delimited strings because macOS ships bash 3.2, which has no
# associative arrays.
setadd() {
  case "$1" in *"|$2|"*) printf '%s' "$1" ;; *) printf '%s|%s|' "$1" "$2" ;; esac
}
sethas() {
  case "$1" in *"|$2|"*) return 0 ;; *) return 1 ;; esac
}

check_live() {
  local home=$1 fixture=${2:-} agents
  local src line pid args persona pidfile region r n
  local sessions=0 measured=0 unmeasured=0 bad=0 comparisons=0
  agents="$home/agents"
  if [ -n "$fixture" ]; then
    if [ ! -r "$fixture" ]; then
      echo "nothing measured: $fixture is unreadable" >&2
      return 2
    fi
    src=$(cat "$fixture")
  else
    src=$(ps -Ao pid=,args= 2>/dev/null)
  fi
  if [ -z "$src" ]; then
    echo "nothing measured: the process table read back empty" >&2
    return 2
  fi
  while IFS= read -r line; do
    # ps right-pads the pid column.
    line=${line#"${line%%[![:space:]]*}"}
    [ -n "$line" ] || continue
    pid=${line%% *}
    args=${line#* }
    # posse's persona launch, on every runtime: the PID rides on the line.
    case "$args" in *--append-system-prompt*) ;; *) continue ;; esac
    sessions=$((sessions + 1))
    persona=$(printf '%s' "$args" | grep -o 'name: [A-Za-z0-9_-][A-Za-z0-9_-]*' | head -1)
    persona=${persona#name: }
    pidfile="$agents/$persona.md"
    if [ -z "$persona" ] || [ ! -r "$pidfile" ]; then
      echo "UNMEASURED  pid $pid carries a system prompt no PID under $agents answers for (read '${persona:-?}')"
      unmeasured=$((unmeasured + 1))
      continue
    fi
    case "$args" in
      *'--disallowedTools '*) region=${args##*--disallowedTools } ;;
      *)
        # grok renders one --deny per rule and codex renders none at all; both
        # freeze the same way and neither argv shape is read here. Named, not
        # skipped: an unmeasured session must not read as a current one.
        echo "UNMEASURED  pid $pid ($persona) renders no --disallowedTools - not a claude launch line"
        unmeasured=$((unmeasured + 1))
        continue
        ;;
    esac
    local pidset="" argvset="" extraset=""
    local -a missing=() extra=()
    while IFS= read -r r; do
      [ -n "$r" ] || continue
      pidset=$(setadd "$pidset" "$(norm_rule "$r")")
    done <<<"$(deny_rules "$pidfile")"
    while IFS= read -r r; do
      [ -n "$r" ] || continue
      argvset=$(setadd "$argvset" "$(norm_rule "$r")")
    done <<<"$(argv_rules "$region")"
    if [ -z "$pidset" ]; then
      echo "UNMEASURED  pid $pid ($persona) - its PID carries no deny: list, so there is nothing to be stale against"
      unmeasured=$((unmeasured + 1))
      continue
    fi
    measured=$((measured + 1))
    while IFS= read -r r; do
      [ -n "$r" ] || continue
      n=$(norm_rule "$r")
      comparisons=$((comparisons + 1))
      sethas "$argvset" "$n" || missing+=("$r")
    done <<<"$(deny_rules "$pidfile")"
    while IFS= read -r r; do
      [ -n "$r" ] || continue
      n=$(norm_rule "$r")
      comparisons=$((comparisons + 1))
      if ! sethas "$pidset" "$n" && ! sethas "$extraset" "$n"; then
        extraset=$(setadd "$extraset" "$n")
        extra+=("$r")
      fi
    done <<<"$(argv_rules "$region")"
    if [ ${#missing[@]} -eq 0 ] && [ ${#extra[@]} -eq 0 ]; then
      continue
    fi
    bad=$((bad + 1))
    echo "STALE  $persona pid $pid - its argv fence is not the fence its PID spells now:"
    if [ ${#missing[@]} -gt 0 ]; then
      echo "       the PID has ${#missing[@]} the session lacks (running LOOSER than the constitution):"
      if [ "$verbose" = 1 ] || [ ${#missing[@]} -le "$ELIDE_OVER" ]; then
        printf '           %s\n' "${missing[@]}"
      else
        printf '           %s\n' "${missing[@]:0:$ELIDE_OVER}"
        echo "           ... and $((${#missing[@]} - ELIDE_OVER)) more (--verbose for all)"
      fi
    fi
    if [ ${#extra[@]} -gt 0 ]; then
      # Never elided. This is the c7ek shape: a rule the ruling removed, still
      # refusing inside a live session hours after the shim let it go.
      echo "       the session has ${#extra[@]} the PID dropped (running STRICTER than the constitution):"
      printf '           %s\n' "${extra[@]}"
    fi
    echo "       fix: posse relaunch <that session> - it lands the work first"
  done <<<"$src"
  if [ "$sessions" -eq 0 ]; then
    echo "nothing measured: no persona sessions in the process table" >&2
    return 2
  fi
  if [ "$measured" -eq 0 ]; then
    echo "nothing measured: $sessions persona session(s) found, none readable against a PID under $agents" >&2
    return 2
  fi
  # The positive witness, with the unmeasured half on its face rather than
  # rounded into the pass (ranger-base-fm4p).
  echo "read $sessions persona session(s): $measured compared against their PIDs ($comparisons rule comparisons), $unmeasured not measured"
  if [ "$bad" -ne 0 ]; then
    echo "$bad live session(s) are running on a fence their PID no longer spells"
    return 1
  fi
  echo "every measured session's argv fence matches the PID it was launched from"
  return 0
}

# --- reader 3: a repo's committed settings fence -----------------------
#
# WHY (ranger-base-9ix7). d866 found the fence living only in one repo's
# .claude/settings.json and ADR 0015 section 3 moved it into the PIDs, where it
# travels with the session. It moved; it did not DELETE the copy. posse's own
# .claude/settings.json still carries that deny list, it is written by the
# operator's hand (b100b60), no renderer touches it - `git worktree add` checks
# it out and nothing in the binary writes it - and every posse worktree gets it
# at checkout. MEASURED 2026-08-29: it still carried the two broad hook rows
# the y5g7 ruling superseded, and was missing `Bash(posse promote:*)` besides,
# with nothing on the box able to say so, because the reader above reads PIDs
# and this file is not one.
#
# WHAT IS AND IS NOT ASSERTED, because inventing a requirement here would be
# worse than the drift. ADR 0015 section 3 does not say a repo's settings file
# must carry the list - the whole point of the amendment is that the PID
# carries it instead. So:
#   FORBIDDEN rows here are a FINDING (exit 1). A superseded row fences a verb
#   the promoted ruling permits, in every worktree of that repo, for every
#   persona, and that is the c7ek class: a fence refusing work the constitution
#   allows. Claude Code matches only the top-level command, so a verb reached
#   through a git hook is untouched and commits still land - which is exactly
#   why this one went a day unnoticed.
#   REQUIRED rows absent are a NOTE, never a failure. The PID is the carrier;
#   an absent row here is a copy being shorter, not a fence with a hole.
# Changing that split is an ADR's decision, not this script's.
#
# The file is constitution class (ADR 0015 section 3, fourth spelling): a
# persona session is refused at the commit if it edits one. This reader
# therefore only ever reports - the repair is the operator's hand.
settings_deny() {
  # permissions.deny, one rule per line. Reads the FIRST "deny" array in the
  # file, one-line or block; a rule containing a `]` or an escaped quote would
  # be misread and none does.
  awk '
    {
      if (!indeny) {
        if (match($0, /"deny"[[:space:]]*:[[:space:]]*\[/) == 0) next
        indeny = 1
        $0 = substr($0, RSTART + RLENGTH)
      }
      line = $0
      if ((j = index(line, "]")) > 0) { line = substr(line, 1, j - 1); done = 1 }
      while (match(line, /"[^"]*"/)) {
        print substr(line, RSTART + 1, RLENGTH - 2)
        line = substr(line, RSTART + RLENGTH)
      }
      if (done) exit
    }
  ' "$1"
}

check_settings() {
  local repo=$1 f rules rule have found files=0 bad=0 comparisons=0 shown=0
  for f in "$repo/.claude/settings.json" "$repo/.claude/settings.local.json"; do
    [ -e "$f" ] || continue
    if [ ! -r "$f" ]; then
      echo "nothing measured: $f is unreadable" >&2
      return 2
    fi
    rules=$(settings_deny "$f")
    if [ -z "$rules" ]; then
      echo "nothing measured: $f declares no permissions.deny this reader can parse" >&2
      return 2
    fi
    files=$((files + 1))
    local -a superseded=() absent=()
    for rule in "${FORBIDDEN[@]}"; do
      comparisons=$((comparisons + 1))
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then superseded+=("$rule"); break; fi
      done <<<"$rules"
    done
    for rule in "${REQUIRED[@]}"; do
      comparisons=$((comparisons + 1))
      found=0
      while IFS= read -r have; do
        if [ "$have" = "$rule" ]; then found=1; break; fi
      done <<<"$rules"
      [ "$found" = 1 ] || absent+=("$rule")
    done
    if [ ${#superseded[@]} -gt 0 ]; then
      bad=$((bad + 1))
      echo "SUPERSEDED  ${f#"$repo"/} carries ${#superseded[@]} rule(s) the y5g7 ruling removed:"
      printf '            %s\n' "${superseded[@]}"
    fi
    if [ ${#absent[@]} -gt 0 ]; then
      shown=1
      echo "note  ${f#"$repo"/} does not carry ${#absent[@]} of the ${#REQUIRED[@]} ADR 0015 section 3 rules. The PID is the carrier, so this is a shorter copy and not a hole:"
      if [ "$verbose" = 1 ] || [ ${#absent[@]} -le "$ELIDE_OVER" ]; then
        printf '        %s\n' "${absent[@]}"
      else
        printf '        %s\n' "${absent[@]:0:$ELIDE_OVER}"
        echo "        ... and $((${#absent[@]} - ELIDE_OVER)) more (--verbose for all)"
      fi
    fi
  done
  if [ "$files" -eq 0 ]; then
    echo "nothing measured: no .claude/settings.json or .claude/settings.local.json under $repo" >&2
    return 2
  fi
  echo "scanned $files settings file(s) under $repo against ${#FORBIDDEN[@]} superseded and ${#REQUIRED[@]} required rules ($comparisons comparisons)"
  if [ "$bad" -ne 0 ]; then
    echo "$bad settings file(s) still fence a verb the promoted ruling permits - the repair is the operator's hand (constitution class, ADR 0015 section 3)"
    return 1
  fi
  if [ "$shown" = 1 ]; then
    echo "no settings file carries a superseded rule"
  else
    echo "every settings file carries the ADR 0015 section 3 fence and no superseded rule"
  fi
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


  # --- the live-session reader (ranger-base-9ix7) ------------------------
  #
  # Every arm below has a wrong answer that fails. The composite one is
  # `live-current`: a session whose argv is the CORRECT render of its PID,
  # carrying all three decorations L0Spellings adds. Each decoration also gets
  # an arm of its own, because a composite arm dies to any one mutation and
  # then cannot say which - and the three isolating arms are what keep
  # norm_rule from being quietly narrowed to two.
  #
  # The decoration that mattered most in practice is the negation rewrite:
  # before norm_rule existed, a literal comparison called
  # `Bash(git commit unless --)` missing from all eleven live sessions, every
  # one of which was current in that rule. An always-red control is a control
  # nobody runs.
  local psdir="$d/ps"
  mkdir -p "$psdir" "$d/live/agents"
  {
    echo '---'
    echo 'name: livecrew'
    echo 'deny:'
    echo '  - Bash(git push:*)'
    echo '  - Bash(git commit unless --)'
    echo '  - Bash(security:*)'
    echo '  - Bash(bd hook install:*)'
    echo '---'
    echo body
  } > "$d/live/agents/livecrew.md"

  # Built in pieces rather than edited by parameter expansion: the rules are
  # full of glob metacharacters, and a fixture whose difference from its
  # sibling depends on a `*` being read literally is a fixture that will
  # eventually measure something other than what it says.
  local pre='  4321 claude --model claude-opus-5 --permission-mode auto --append-system-prompt ---\012name: livecrew\012deny:\012 --add-dir /m --settings {} --disallowedTools '
  local base='Bash(git push:*) Bash(git -* push *) Bash(git commit) Bash(git -* commit) Bash(security:*)'
  local narrowed='Bash(bd hook install:*) Bash(bd -* hook install *)'
  local broadrow='Bash(bd hook:*) Bash(bd -* hook *)'

  echo "$pre$base $narrowed" > "$psdir/current"
  out=$(check_live "$d/live" "$psdir/current"); r=$?
  if [ $r -eq 0 ] && [[ $out == *"read 1 persona session(s): 1 compared"* ]] \
     && [[ $out != *STALE* ]]; then
    echo "self-test PASS: a session whose argv is the correct render of its PID is clean, and 1 session was read"
  else
    echo "self-test FAIL: a correctly-rendered session was called stale (rc=$r): $out"; rc=1
  fi

  # Decoration 1: the option-blind twin. Drop the `-*` skip in norm_rule and
  # the twin becomes a rule the PID does not carry - a false STRICTER finding.
  {
    echo '---'; echo 'name: twin'; echo 'deny:'
    echo '  - Bash(git push:*)'
    echo '---'; echo body
  } > "$psdir/twin.md"
  mkdir -p "$d/twin/agents"; cp "$psdir/twin.md" "$d/twin/agents/twin.md"
  echo '  11 claude --append-system-prompt ---\012name: twin\012 --disallowedTools Bash(git push:*) Bash(git -* push *)' > "$psdir/twin"
  out=$(check_live "$d/twin" "$psdir/twin"); r=$?
  if [ $r -eq 0 ] && [[ $out != *STALE* ]]; then
    echo "self-test PASS: the option-blind twin is not read as a rule the PID dropped"
  else
    echo "self-test FAIL: the option-blind twin was read as drift (rc=$r): $out"; rc=1
  fi

  # Decoration 2: the negation rewrite. Drop the `unless` tail-strip and this
  # session is called stale in the one rule it is not stale in.
  mkdir -p "$d/neg/agents"
  {
    echo '---'; echo 'name: neg'; echo 'deny:'
    echo '  - Bash(git commit unless --)'
    echo '---'; echo body
  } > "$d/neg/agents/neg.md"
  echo '  12 claude --append-system-prompt ---\012name: neg\012 --disallowedTools Bash(git commit) Bash(git -* commit)' > "$psdir/neg"
  out=$(check_live "$d/neg" "$psdir/neg"); r=$?
  if [ $r -eq 0 ] && [[ $out != *STALE* ]]; then
    echo "self-test PASS: a negative rule and the bare form rendered from it compare equal"
  else
    echo "self-test FAIL: the negative rule was read as drift (rc=$r): $out"; rc=1
  fi

  # Decoration 3: the prefix marker. A wordless rule renders as itself PLUS the
  # `:*` form; drop the strip and the second is a phantom finding.
  mkdir -p "$d/bare/agents"
  {
    echo '---'; echo 'name: bare'; echo 'deny:'
    echo '  - Bash(security)'
    echo '---'; echo body
  } > "$d/bare/agents/bare.md"
  echo '  13 claude --append-system-prompt ---\012name: bare\012 --disallowedTools Bash(security) Bash(security:*)' > "$psdir/bare"
  out=$(check_live "$d/bare" "$psdir/bare"); r=$?
  if [ $r -eq 0 ] && [[ $out != *STALE* ]]; then
    echo "self-test PASS: a wordless rule and its :* prefix form compare equal"
  else
    echo "self-test FAIL: the :* prefix form was read as drift (rc=$r): $out"; rc=1
  fi

  # A PID TIGHTENED after the session launched: the session is LOOSER than the
  # constitution. Exactly one rule, named, and no STRICTER half.
  echo "$pre$base" > "$psdir/loose"
  out=$(check_live "$d/live" "$psdir/loose"); r=$?
  if [ $r -eq 1 ] && [[ $out == *"the PID has 1 the session lacks"* ]] \
     && [[ $out == *'Bash(bd hook install:*)'* ]] && [[ $out != *STRICTER* ]]; then
    echo "self-test PASS: a session missing one rule its PID gained is flagged, and only it"
  else
    echo "self-test FAIL: the tightened-PID gap was not caught as exactly one (rc=$r): $out"; rc=1
  fi

  # The ranger-base-c7ek shape, and the one a one-directional control cannot
  # see: the session still carries the BROAD row the ruling narrowed. Both
  # halves must be named - the narrowed rule it lacks and the broad rule it
  # kept - and the broad rule must be reported ONCE, not once per spelling.
  echo "$pre$base $broadrow" > "$psdir/strict"
  out=$(check_live "$d/live" "$psdir/strict"); r=$?
  if [ $r -eq 1 ] && [[ $out == *"the session has 1 the PID dropped"* ]] \
     && [[ $out == *'Bash(bd hook:*)'* ]] \
     && [[ $out == *"the PID has 1 the session lacks"* ]] \
     && [[ $out == *'Bash(bd hook install:*)'* ]]; then
    echo "self-test PASS: a session still carrying the superseded broad row is flagged in both directions, the broad row once"
  else
    echo "self-test FAIL: the superseded-row session was not caught in both directions (rc=$r): $out"; rc=1
  fi

  # A session this home has no PID for is NAMED, never counted as current.
  echo '  99 claude --append-system-prompt ---\012name: stranger\012 --disallowedTools Bash(git push:*)' > "$psdir/mixed"
  cat "$psdir/current" >> "$psdir/mixed"
  out=$(check_live "$d/live" "$psdir/mixed"); r=$?
  if [ $r -eq 0 ] && [[ $out == *UNMEASURED* ]] && [[ $out == *stranger* ]] \
     && [[ $out == *"1 not measured"* ]]; then
    echo "self-test PASS: a session with no PID in this home is named unmeasured, not passed"
  else
    echo "self-test FAIL: the unknown-persona session was not named (rc=$r): $out"; rc=1
  fi

  # Nothing measured is not a pass, here too: a process table with no persona
  # session in it answers 2.
  echo '  1 /sbin/launchd' > "$psdir/none"
  out=$(check_live "$d/live" "$psdir/none" 2>&1); r=$?
  if [ $r -eq 2 ]; then
    echo "self-test PASS: a process table with no persona session exits 2, not 0"
  else
    echo "self-test FAIL: an empty process table returned rc=$r, want 2"; rc=1
  fi

  # --- the settings reader (ranger-base-9ix7) ---------------------------
  local sd
  for sd in clean broad flow bare_repo; do mkdir -p "$d/$sd/.claude"; done

  {
    echo '{ "permissions": { "allow": ["Read"], "deny": ['
    printf '    "%s",\n' "${REQUIRED[@]}" | sed '$ s/,$//'
    echo '  ] } }'
  } > "$d/clean/.claude/settings.json"
  out=$(check_settings "$d/clean"); r=$?
  if [ $r -eq 0 ] && [[ $out == *"scanned 1 settings file(s)"* ]] && [[ $out != *SUPERSEDED* ]]; then
    echo "self-test PASS: a settings file carrying the whole set and no superseded row is clean"
  else
    echo "self-test FAIL: a clean settings file was flagged (rc=$r): $out"; rc=1
  fi

  # The live shape on 2026-08-29: the whole set AND the two rows the ruling
  # removed. A presence-only reader calls this clean.
  {
    echo '{ "permissions": { "deny": ['
    printf '    "%s",\n' "${REQUIRED[@]}"
    printf '    "%s",\n' "${FORBIDDEN[@]}" | sed '$ s/,$//'
    echo '  ] } }'
  } > "$d/broad/.claude/settings.json"
  out=$(check_settings "$d/broad"); r=$?
  if [ $r -eq 1 ] && [[ $out == *SUPERSEDED* ]] \
     && [[ $out == *'Bash(bd hook:*)'* ]] && [[ $out == *'Bash(bd hooks:*)'* ]]; then
    echo "self-test PASS: a settings file keeping the superseded broad rows is flagged"
  else
    echo "self-test FAIL: the superseded rows in a settings file were not caught (rc=$r): $out"; rc=1
  fi

  # The one-line array shape, which the block-shape reader must not need.
  # A missing REQUIRED row here is a NOTE and must NOT turn the exit red - the
  # PID is the carrier, and asserting otherwise would invent a requirement no
  # ADR wrote.
  echo '{"permissions":{"deny":["Bash(make install:*)","Bash(bd delete:*)"]}}' > "$d/flow/.claude/settings.json"
  out=$(check_settings "$d/flow"); r=$?
  if [ $r -eq 0 ] && [[ $out == *"does not carry"* ]] && [[ $out == *'Bash(bd daemon:*)'* ]]; then
    echo "self-test PASS: a one-line deny array parses, and a short copy is a note rather than a failure"
  else
    echo "self-test FAIL: the one-line array or the note/failure split is wrong (rc=$r): $out"; rc=1
  fi

  out=$(check_settings "$d/bare_repo" 2>&1); r=$?
  if [ $r -eq 2 ]; then
    echo "self-test PASS: a repo with no settings file exits 2, not 0"
  else
    echo "self-test FAIL: a repo with no settings file returned rc=$r, want 2"; rc=1
  fi

  rm -rf "$d"
  return $rc
}

usage() {
  local me
  me=$(basename "$0")
  echo "usage: $me [--verbose] [<posse-home>]"
  echo "           the PIDs under <home>/agents carry the ADR 0015 section 3 fence"
  echo "       $me --live [--live-from <ps capture>] [<posse-home>]"
  echo "           every live persona session's argv fence is the fence its PID spells NOW"
  echo "       $me --settings <repo>"
  echo "           <repo>'s committed .claude/settings.json carries no superseded rule"
  echo "       $me --pids ...   run the PID reader alongside the readers above"
  echo "       $me --self-test"
  echo "exit: 0 clean · 1 findings · 2 nothing measured, which is never a pass"
}

home=""
live=0
live_from=""
settings_repo=""
pids_arm=0
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test)
      self_test
      exit $?
      ;;
    --verbose)
      verbose=1
      ;;
    --pids)
      pids_arm=1
      ;;
    --live)
      live=1
      ;;
    --live-from)
      # A captured `ps -Ao pid=,args=` instead of this box's process table:
      # what the self-test drives, and what reads a capture taken elsewhere.
      live=1
      live_from=${2:-}
      if [ -z "$live_from" ]; then echo "--live-from needs a file" >&2; exit 2; fi
      shift
      ;;
    --settings)
      settings_repo=${2:-}
      if [ -z "$settings_repo" ]; then echo "--settings needs a repo path" >&2; exit 2; fi
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      usage >&2
      exit 2
      ;;
    *)
      home=$1
      ;;
  esac
  shift
done

home=${home:-${RHQ_HOME:-$HOME/.config/posse}}
# No reader asked for is the reader this script was born as.
if [ "$live" = 0 ] && [ -z "$settings_repo" ]; then pids_arm=1; fi

# Worst-of, ranked 2 > 1 > 0: an arm that measured nothing outranks an arm that
# found something, because a run half of which was blind must not exit on the
# quieter of the two answers.
rc=0
worst() {
  case "$1" in
    2) rc=2 ;;
    1) [ "$rc" = 2 ] || rc=1 ;;
  esac
}

if [ "$pids_arm" = 1 ]; then
  check_home "$home"
  worst $?
fi
if [ "$live" = 1 ]; then
  [ "$pids_arm" = 1 ] && echo
  check_live "$home" "$live_from"
  worst $?
fi
if [ -n "$settings_repo" ]; then
  { [ "$pids_arm" = 1 ] || [ "$live" = 1 ]; } && echo
  check_settings "$settings_repo"
  worst $?
fi
exit $rc
