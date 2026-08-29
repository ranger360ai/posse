#!/usr/bin/env bash
# Find commits on this branch that silently put a file's content BACK to an
# older version — a revert nobody labelled a revert (rangerhq-8rtf).
#
# Usage: scripts/audit-silent-reverts.sh [rev-range]     (default: HEAD)
#        scripts/audit-silent-reverts.sh --self-test     (prove the detector works)
#        scripts/audit-silent-reverts.sh --quiet         (summary only unless it fails)
#
# WHY THIS EXISTS. rangerhq-8rtf: ef8d35f (a landed P1 fix) was undone by
# dcca7b5 "bd sync: afternoon batch" and re-landed 3h52m later by 1cc432e.
# Nothing reported it. `go test ./...` stayed GREEN across the window, because
# the same commit that removed the fix also restored the `t.Skip` on its
# regression pin — the one state in which the suite is green and the defect is
# live. It was found by accident, by a human reading history for another reason.
# No test, no gate and no status line covers this class, so this script does.
#
# THE MECHANISM it is watching for (measured, scratch repos, 2026-08-22):
# a commit that bypasses the SHARED .git/index — `GIT_INDEX_FILE=<private> …`,
# the workaround rangerhq-2f5r recorded — lands correctly in HEAD and leaves
# `.git/index` holding the PRE-fix blobs for every path it committed. The next
# commit taken from that shared index therefore commits them back. That is what
# a `bd sync` is. Both personas involved read it as leftover work, not a revert.
# The blessed form `git commit -F - -- <paths>` does NOT have this property
# (measured: it refreshes the shared index for the named paths), and the
# prepare-commit-msg wall refuses the private form for personas — but the wall
# keys on RHQ_PERSONA, so it is silent for the operator's own commits, and
# nothing stops the class from arriving by another route. Hence: detect, don't
# assume.
#
# THE MECHANISM HAS TWO HALVES, and this script covers both (rangerhq-ypn1).
# When the landed change MODIFIES a file, the stale shared index holds the older
# blob and the next commit from it rolls the content back — dcca7b5's shape.
# When the landed change ADDS a file, the stale index has no entry for that path
# at all, so the next commit from it writes a tree WITHOUT the file: the undo is
# a DELETION. Same cause, same silence, and worse, because in that shape the
# regression pin does not need re-skipping to keep the suite green — it stops
# existing, and `go test ./...` reports "[no test files]" and exits 0. This
# script used to skip deletions on the rationale that "a removal is visible in
# review". rangerhq-8rtf is the disproof of that rationale: nobody reviewed
# dcca7b5. 14 of the first 411 commits on main are add-only and the top six are
# `test:` commits — regression pins, landed as one new file.
#
# HOW IT DETECTS. One `git log --raw` pass. For each path it keeps the ordered
# list of states that path has held, where ABSENCE IS A STATE: a path begins
# absent, an add moves it to a blob, a deletion moves it back to absent. A
# commit is flagged when it sets a path to a state that path held BEFORE its
# immediately preceding change — i.e. it went backwards over at least one landed
# change. That one rule covers a content rollback, the deletion of a file that
# was added earlier in the range, and the re-landing of either (the repair half
# reads as a rollback of the revert, by construction — 1cc432e is triaged for
# exactly that). Merges are excluded. A deletion whose exact blob is added at
# another path in the SAME commit is a move, not a rollback, and is not flagged.
#
# COST OF THE DELETION RULE, measured on this repo's real history (447 commits,
# 2026-08-23) before it was turned on: 30 deletions, of which 8 are exact moves
# and drop out. What remains flags five commits — 21653f9 (already triaged for a
# content rollback in the same pivot), 9daf91f and 631bda7. Two new triage lines
# across 447 commits. Proximity heuristics were measured and rejected: the real
# incident's revert landed 2 commits after its fix while the nearest benign
# deletion is 3, so any window wide enough to hold the incident holds the benign
# ones too. Subject-mention was measured and rejected outright: ZERO of the 30
# real deletions name the deleted path's basename in the subject, so "flag a
# deletion the commit does not mention" is just "flag every deletion" wearing a
# hat.
#
# STATES ARE COMPARED AS STRINGS, DELIBERATELY (ranger-base-hhcu). ci.yml gave
# two different verdicts for the same 422 commits on the same tree: ubuntu-latest
# flagged b26975f (cmd/posse/cockpit.go "-> content of 1fdf9da"), macos-latest
# did not. Neither blob was ever repeated — the three states are 6e51571afa45…,
# 6e44262db4c1… and 05e20b38efc9…, all distinct. But the first two ABBREVIATE to
# `6e51571` and `6e44262`, both valid scientific notation, and a field (or an
# element split out of one) is a STRNUM: awk compares two strnums NUMERICALLY
# when both look like numbers. Both overflow to +inf, +inf == +inf, and the path
# "went backwards" to a state it never held.
#
# Measured 2026-08-29 over this repo's own 433 commits, one captured raw log fed
# to four awks: gawk 5.3.2 flags b26975f; mawk 1.3.4, busybox awk and darwin's
# BWK awk 20200816 do not. mawk and BWK reject an overflowed string as a strnum,
# gawk does not — `printf '6e51571 6e44262\n' | gawk '{print ($1==$2)}'` is 1 and
# is 0 everywhere else. gawk is therefore the only one of the four that
# reproduces ubuntu-latest's verdict, over the same history, naming the same
# commit and the same path — the runner's awk was not probed directly, but
# nothing else measured produces that output. That is the whole of the split,
# and it is why nobody could triage their way out of it from a mac. Note also
# that `make test-linux` cannot see it: its golang:1.26 image is debian, whose
# /usr/bin/awk is mawk (measured), so the container agreed with darwin.
# Note the direction that matters more than the false positive: the same coercion
# can HIDE a real rollback, so on a coercing awk a clean run was worth less than
# it read. Hence raw_log's --no-abbrev and the `""` in states_awk's capture, with
# the `numeric` self-test arm pinning one layer each.
#
# LIMITS, so nobody reads a clean run as more than it is:
#   - Exact-blob only. A PARTIAL revert (some hunks of a file) is invisible here.
#   - A legitimate revert commit is flagged too; that is why triage is a file
#     rather than a heuristic on the subject line.
#   - It sees only what is committed. Work reverted in the working tree before
#     it ever landed leaves no trace to find.
#   - A rename that also EDITS the file is flagged. The move exception is
#     exact-blob, like the rest of the tool; a 90%-similar file at a new path is
#     git's heuristic, not this one's. Answer it with a triage line (631bda7).
#   - The deletion rule needs the path's ADD inside the scanned range. On a
#     partial range (`main~10..main`) a deletion of an older file is invisible;
#     a full-history run (the default, and what `make test` runs) has no such
#     gap. It under-reports on short ranges, it does not false-positive.
#
# TRIAGE. Known-and-explained commits live in scripts/silent-reverts.allow, one
# `<sha> <reason>` per line. Anything NOT in that file exits 1 — a new silent
# revert is a build failure, and clearing it means writing down why.
set -uo pipefail

cd "$(git rev-parse --show-toplevel)" || exit 2
ALLOW=scripts/silent-reverts.allow

# --- self-test fixtures -----------------------------------------------------
# Each plants one shape in a throwaway repo and leaves cwd inside it.

plant_repo() {
  local r; r=$(mktemp -d)/r; mkdir -p "$r"; cd "$r" || return 2
  git init -q .; git config user.email t@t; git config user.name t
  printf 'v1\n' > fix.go; printf 'o\n' > other.txt
  env -u RHQ_PERSONA git add -A; env -u RHQ_PERSONA git commit -qm base
}

# MODIFY half — rangerhq-8rtf verbatim: the fix edits an existing file.
plant_modify() {
  plant_repo || return 2
  printf 'v2-THE-FIX\n' > fix.go
  ( export GIT_INDEX_FILE="$(mktemp -d)/index"
    env -u RHQ_PERSONA git read-tree HEAD
    env -u RHQ_PERSONA git add -- fix.go
    env -u RHQ_PERSONA git commit -qm "the fix" )
  printf 'synced\n' > other.txt
  env -u RHQ_PERSONA git add other.txt
  env -u RHQ_PERSONA git commit -qm "bd sync: batch"   # reverts fix.go, silently
  [ "$(git show HEAD:fix.go)" = "v1" ] || return 2
}

# ADD-ONLY half — rangerhq-ypn1: the fix is a NEW file, so the stale index
# DELETES it rather than rolling it back.
plant_addonly() {
  plant_repo || return 2
  printf 'package x // the regression pin\n' > newpin_test.go
  ( export GIT_INDEX_FILE="$(mktemp -d)/index"
    env -u RHQ_PERSONA git read-tree HEAD
    env -u RHQ_PERSONA git add -- newpin_test.go
    env -u RHQ_PERSONA git commit -qm "the fix: add newpin_test.go" )
  printf 'synced\n' > other.txt
  env -u RHQ_PERSONA git add other.txt
  env -u RHQ_PERSONA git commit -qm "bd sync: batch"   # deletes the pin, silently
  git ls-tree --name-only HEAD | grep -q newpin_test.go && return 2
  return 0
}

# NEGATIVE control — a plain move. The deletion rule must NOT fire here, or it
# is not a detector, it is a tax on renaming files.
plant_move() {
  plant_repo || return 2
  printf 'body\n' > moved.go
  env -u RHQ_PERSONA git add -A; env -u RHQ_PERSONA git commit -qm "add moved.go"
  git mv moved.go elsewhere.go
  env -u RHQ_PERSONA git commit -qm "move it"
}

# NUMERIC-ID control — ranger-base-hhcu. n.txt holds three distinct states, and
# the first and third abbreviate to <digit>e<digits>, which is a NUMBER to awk.
# Nothing here went backwards, so the detector must stay silent. The two
# contents are chosen rather than generated so the shape is deterministic:
# `numeric-shaped blob 1059` hashes to 7e1599240849…, `…1259` to 5e004138631…,
# and both keep ten digits after the `e`, so any abbreviation git picks between
# 6 and 12 characters is still scientific notation with an exponent that
# overflows a double. At 40 hex they are plainly unequal; at 7 they are not.
plant_numeric() {
  local r; r=$(mktemp -d)/r; mkdir -p "$r"; cd "$r" || return 2
  git init -q .; git config user.email t@t; git config user.name t
  printf 'numeric-shaped blob 1059\n' > n.txt; printf 'o\n' > other.txt
  env -u RHQ_PERSONA git add -A
  env -u RHQ_PERSONA git commit -qm "add n.txt"            # state 1: 7e15992…
  printf 'plain\n' > n.txt
  env -u RHQ_PERSONA git commit -qam "edit n.txt"          # state 2
  printf 'numeric-shaped blob 1259\n' > n.txt
  env -u RHQ_PERSONA git commit -qam "edit n.txt again"    # state 3: 5e00413…
  # Fixture witness: the hazard has to actually BE here, or this arm is a
  # negative control that measured nothing (ranger-base-z4vx). Exactly two of
  # the three abbreviated destination ids must read as scientific notation.
  git log --reverse --no-merges --raw --format= HEAD -- n.txt |
    awk '$4 ~ /^[0-9]e[0-9]+$/ {n++} END {exit !(n==2)}' || return 2
}

# The synthetic raw log arm (iii) reads: three commits over one path, whose
# states are `0000100`, `0000042` and then $1. Both `0000100` and `00001e2` are
# the number 100 to every awk measured, so passing 00001e2 is a stream where a
# strnum comparison sees a rollback and a string comparison does not; passing
# 0000100 is a real repeat, which must be flagged either way. Hand-built because
# git will not produce ids of that shape on demand.
numeric_stream() {
  printf '__C__\t1111111111111111111111111111111111111111\tt\tadd n.txt\n'
  printf ':000000 100644 0000000 0000100 A\tn.txt\n'
  printf '__C__\t2222222222222222222222222222222222222222\tt\tedit n.txt\n'
  printf ':100644 100644 0000100 0000042 M\tn.txt\n'
  printf '__C__\t3333333333333333333333333333333333333333\tt\tedit n.txt again\n'
  printf ':100644 100644 0000042 %s M\tn.txt\n' "$1"
}

# Every shape above builds exactly three commits, and every arm below checks
# that it got three. An arm that reads a fixture nobody built is the failure
# this number exists to catch (ranger-base-z4vx).
PLANTED=3

self_test() {
  # Prove the detector fires on the real mechanism — BOTH halves — before
  # trusting a clean run against main, and prove it stays quiet on a move.
  #
  # Each arm is only as good as the fixture it reads, so each fixture is proved
  # TWICE — once from the plant's side and once from the detector's.
  #
  # From the plant's side: take the plant's exit status ON ITS OWN LINE. The
  # earlier spelling `( set -e; "plant_$shape" …; pwd > … ) || { … }` could
  # never fire, because errexit is suppressed for the LEFT OPERAND of `||` and
  # that suppression is inherited into the subshell: a plant returning non-zero
  # did not abort it, `pwd` ran anyway, and this script's own toplevel was
  # written as the fixture path (ranger-base-z4vx).
  #
  # From the detector's side: every arm requires the scan to report $PLANTED
  # commits. The two positive arms fail safe without it (they want n>=1 and an
  # empty fixture gives 0), but the NEGATIVE control does not — it asserts an
  # ABSENCE, and an absence is exactly what a fixture that was never built
  # hands it. A control that only counts absences needs a positive witness
  # that it looked at something; scan()'s own SCANNED line is that witness.
  local rc=0 prc n out scanned shape d fdir nonhex an; d=$(mktemp -d)
  for shape in modify addonly move numeric; do
    ( set -e; "plant_$shape" >/dev/null 2>&1; pwd > "$d/$shape" )
    prc=$?
    [ "$prc" -eq 0 ] || {
        echo "self-test: $shape rig did not reproduce the mechanism"; return 2; }
  done
  for shape in modify addonly move numeric; do
    out=$( cd "$(cat "$d/$shape")" && scan HEAD )
    n=$(printf '%s\n' "$out" | grep -c 'path(s) went backwards')
    scanned=$(printf '%s\n' "$out" | awk -F'\t' '/^SCANNED/{print $2+0}')
    [ -n "$scanned" ] || scanned=0
    if [ "$scanned" -ne "$PLANTED" ]; then
      echo "self-test FAIL: $shape rig scanned $scanned commits, want $PLANTED — the fixture was never built"
      rc=1; continue
    fi
    case "$shape" in
      move)
        if [ "$n" -eq 0 ]; then echo "self-test PASS: a plain move is not flagged (over $scanned planted commits)"
        else echo "self-test FAIL: plain move flagged as a silent revert"; rc=1; fi ;;
      numeric)
        # Three assertions, because the fix has two independent layers and an
        # arm that only checks the outcome is green when EITHER survives.
        fdir=$(cat "$d/numeric")
        # (i) the outcome: on this history, as production runs it, nothing moved
        #     backwards. Red today without both layers, on gawk.
        if [ "$n" -eq 0 ]; then echo "self-test PASS: a <digit>e<digits> blob id is not a silent revert (over $scanned planted commits)"
        else echo "self-test FAIL: a <digit>e<digits> blob id read as a silent revert"; rc=1; fi
        # (ii) layer one: raw_log hands states_awk full 40-hex ids. Dropping
        #      --no-abbrev reds this and nothing else.
        nonhex=$( cd "$fdir" && raw_log HEAD |
                  awk '/^:/ && $4 !~ /^[0-9a-f]{40}$/ {n++} END {print n+0}' )
        if [ "${nonhex:-1}" -eq 0 ]; then echo "self-test PASS: raw_log emits full 40-hex blob ids"
        else echo "self-test FAIL: raw_log emitted ${nonhex:-?} abbreviated blob id(s)"; rc=1; fi
        # (iii) layer two: states_awk compares states as STRINGS, pinned on a
        #       SYNTHETIC stream rather than on the fixture's real ids. That is
        #       deliberate: the +inf collision only happens on an awk that takes
        #       an OVERFLOWED numeric string as a strnum, and of the four awks
        #       measured only gawk does, so (i) and (ii) above are undiscriminating
        #       on darwin, mawk and busybox — exactly the blind spot that let this
        #       ship. `0000100` and `00001e2` are both plainly 100, no overflow
        #       involved, and ALL FOUR awks compare them equal as strnums
        #       (measured 2026-08-29). So this arm reds on every platform the
        #       moment the `""` leaves states_awk's capture, and it is the pin
        #       that layer actually has. states_awk is split out of scan() so it
        #       can be fed a stream raw_log would never emit.
        an=$( numeric_stream 00001e2 | states_awk )
        if [ "$(printf '%s\n' "$an" | awk -F'\t' '/^SCANNED/{print $2+0}')" != "3" ]; then
          echo "self-test FAIL: the synthetic strnum stream did not parse — nothing was measured"; rc=1
        elif printf '%s\n' "$an" | grep -q 'path(s) went backwards'; then
          echo "self-test FAIL: states_awk coerced two distinct blob ids to one number"; rc=1
        else
          echo "self-test PASS: states_awk compares ids as strings, not numbers"
        fi
        # (iii-control) the same rig with a genuine repeat MUST fire, or the
        # assertion above is an absence nobody proved could be a presence.
        an=$( numeric_stream 0000100 | states_awk )
        if printf '%s\n' "$an" | grep -q 'path(s) went backwards'; then
          echo "self-test PASS: the strnum rig does fire on a real repeat"
        else
          echo "self-test FAIL: the strnum rig cannot detect anything — arm (iii) proves nothing"; rc=1
        fi ;;
      *)
        if [ "$n" -ge 1 ]; then echo "self-test PASS: detector flags the $shape half of the rangerhq-8rtf mechanism"
        else echo "self-test FAIL: planted $shape revert not detected"; rc=1; fi ;;
    esac
  done
  [ "$rc" -eq 0 ] && echo "self-test PASS: detector flags the rangerhq-8rtf mechanism"
  return $rc
}

# The raw log every scan reads. --no-abbrev is load-bearing (ranger-base-hhcu):
# git's default abbreviation is 7 hex, which is short enough that a blob id
# lands on the <digit>e<digits> shape roughly once in 270, and that shape is
# a NUMBER to awk. Full 40-hex ids are not immune in principle, but the shape
# needs all 40 characters to cooperate, which is ~6e-8 rather than 0.4%.
# The string comparison in states_awk is the actual fix; this is the cheap
# second layer, and each is pinned by its own assertion in the numeric arm.
raw_log() {
  git log --reverse --no-merges --no-renames --raw --no-abbrev \
      --format="__C__%x09%H%x09%an%x09%s" "$1"
}

# The state machine, reading a raw log on stdin. Split out of scan() so a
# self-test arm can feed it a stream raw_log itself would never emit.
states_awk() {
  awk '
    # A path holds an ordered list of STATES. ABSENT is one of them: a path
    # starts absent, an add moves it to a blob, a delete moves it back. Going
    # backwards to any state older than the immediately preceding one is a
    # silent revert, whichever direction it travelled.
    function flush(   i,p,st,d,s,n,j,back,kk,moved) {
      if (ci==0) { ne=0; return }
      split("", addedhere)
      for (i=1; i<=ne; i++) if (est[i] ~ /^A/) addedhere[edst[i]]=epath[i]
      for (i=1; i<=ne; i++) {
        p=epath[i]; st=est[i]; d=edst[i]; s=esrc[i]; moved=0
        if (st ~ /^D/) {
          d=ABSENT
          if (s in addedhere) moved=1           # exact-content move, not a rollback
        } else if (cnt[p]==0 && st ~ /^A/) {
          cnt[p]=1; blob[p,1]=ABSENT; cm[p,1]="(absent)"   # the state before it existed
        }
        n=cnt[p]; back=""
        for (j=1; j<=n-1; j++) if (blob[p,j]==d) { back=cm[p,j]; break }
        if (back != "" && !moved) {
          kk=++nreg[ci]
          reg[ci,kk]= p "\t" back "\t" cm[p,n] "\t" (d==ABSENT ? "D" : "M")
        }
        cnt[p]=n+1; blob[p,n+1]=d; cm[p,n+1]=substr(sha[ci],1,7)
      }
      ne=0
    }
    BEGIN { ABSENT="absent"; ne=0 }        # cannot collide: blob ids are 40 hex
    /^__C__\t/ { flush(); split($0,a,"\t"); ci++; sha[ci]=a[2]; auth[ci]=a[3]; subj[ci]=a[4]; next }
    /^:/ {
      t=index($0,"\t"); if (t==0) next
      ne++
      epath[ne]=substr($0,t+1)
      split(substr($0,1,t-1),m," ")
      # FORCE STRING, and do it HERE so there is exactly one place that can be
      # got wrong (ranger-base-hhcu). A field, and every element split out of
      # one, is a STRNUM: awk compares two strnums NUMERICALLY when both look
      # like numbers. A blob id of the form <digit>e<digits> is valid
      # scientific notation whose value overflows to +inf, and +inf == +inf, so
      # an awk that coerces calls two unrelated blobs EQUAL and reports that a
      # path went back to a state it never held. Concatenating "" makes these
      # plain strings once, and every comparison below inherits it.
      esrc[ne]=m[3] ""; edst[ne]=m[4] ""; est[ne]=m[5]
      next
    }
    END {
      flush()
      for (i=1; i<=ci; i++) {
        if (nreg[i]+0 == 0) continue
        printf "%s  %d path(s) went backwards  %s\n", substr(sha[i],1,7), nreg[i], substr(subj[i],1,72)
        for (k=1; k<=nreg[i]; k++) { split(reg[i,k],f,"\t")
          if (f[4]=="D")
            printf "           %s  -> DELETED, undoing %s that put it there\n", f[1], f[3]
          else
            printf "           %s  -> content of %s, undoing %s\n", f[1], f[2], f[3] }
        printf "SHA\t%s\n", substr(sha[i],1,7)
      }
      printf "SCANNED\t%d\n", ci
    }'
}

scan() { raw_log "$1" | states_awk; }

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }

# --quiet: one summary line when clean, full detail when something is untriaged.
QUIET=0
[ "${1:-}" = "--quiet" ] && { QUIET=1; shift; }

out=$(scan "${1:-HEAD}") || exit 2
scanned=$(printf '%s\n' "$out" | awk -F'\t' '/^SCANNED/{print $2}')
detail=$(printf '%s\n' "$out" | grep -v $'^SHA\t' | grep -v '^SCANNED')
[ "$QUIET" -eq 1 ] || printf '%s\n' "$detail"

untriaged=0
while IFS=$'\t' read -r _ sha; do
  if [ -f "$ALLOW" ] && grep -q "^$sha" "$ALLOW"; then
    [ "$QUIET" -eq 1 ] || printf '  triaged: %s — %s\n' "$sha" "$(awk -v s="$sha" '$1 ~ "^"s {sub($1" *",""); print}' "$ALLOW")"
  else
    untriaged_shas="${untriaged_shas:-} $sha"
    untriaged=$((untriaged+1))
  fi
done < <(printf '%s\n' "$out" | grep $'^SHA\t')

if [ "$untriaged" -gt 0 ] && [ "$QUIET" -eq 1 ]; then printf '%s\n' "$detail"; fi
for sha in ${untriaged_shas:-}; do
  printf '  UNTRIAGED: %s — a silent revert nobody has explained. Read it, then\n' "$sha"
  printf '             either fix it or write the reason in %s\n' "$ALLOW"
done

[ "$QUIET" -eq 1 ] && [ "$untriaged" -eq 0 ] \
  && { echo "silent-revert audit: $scanned commits, 0 untriaged"; exit 0; }
echo
echo "scanned $scanned commits; $untriaged untriaged silent revert(s)"
[ "$untriaged" -eq 0 ] || exit 1
