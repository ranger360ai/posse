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
# The blessed form `git commit -F - -- <paths>` refreshes the shared index for
# the paths it NAMES — and that is the whole of its immunity, not the blanket
# clearance this comment used to claim (rangerhq-be7k). A pre-commit hook that
# stages a path the pathspec does NOT name — bd's flush of .beads/issues.jsonl,
# in every repo that carries bd's hook — hits the same stale-index shape: git's
# partial commit writes the real index (refreshed for the pathspec only) BEFORE
# it calls the hook, and hands the hook a separate `next-index-<pid>.lock` to
# add into. So the flush reaches the commit and never reaches the real index,
# which is left holding the pre-flush blob. Measured 2026-08-29, git 2.39.3;
# pinned in internal/posse/staleindex_qa_test.go. The prepare-commit-msg wall
# refuses the private form and both carriers that spring a stale entry (the
# unqualified form and `-i`) for every shell in the checkout since
# rangerhq-lt2w — but it cannot refuse the producer, because the producer is
# the form it prescribes, and nothing stops the class from arriving by another
# route. Hence: detect, don't assume.
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
# exactly that). Merges are excluded. A deletion that git pairs with an add in
# the SAME commit is a move, not a rollback, and is not flagged — see raw_log
# for the similarity threshold and why that number is chosen rather than
# inherited (ranger-base-en75). Only the SOURCE half of a move is excused; the
# destination is compared like any other write.
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
# THE THIRD REJECTED HEURISTIC, and it is the one that keeps getting proposed:
# exempt the PAIR — a path added and then deleted with no other commit touching
# it in between, both inside the scanned range. It looks like it clears nothing
# but throwaway probe fixtures. It does not. That pair is the EXACT shape of
# plant_addonly below, i.e. of the add-only half of the rangerhq-8rtf mechanism:
# the fix lands one new file from a private index, and the next commit off the
# stale shared index deletes it, with nothing in between. Measured 2026-08-29
# (ranger-base-hvbj) by implementing it and running this script's own harness:
# the addonly arm turns into "self-test FAIL: planted addonly revert not
# detected", and three Go pins red with it — TestAuditFlagsAddOnlySilentRevert
# (exit 0, want 1), TestSilentRevertSelfTestStillFires and
# TestSilentRevertSelfTestHasTheStrnumArm. It cannot ship without deleting the
# pin rangerhq-ypn1 landed. It also fails at the job it was proposed for: on
# this repo's own history it cleared ONE of the two probe hits and left the
# other (that path had four states, not two), and it silently un-flagged
# e82338c, a rename+edit already triaged in the allow file. Rejected on the
# measurement, not on taste. (That last clause is spent as of ranger-base-en75
# — e82338c is not flagged at all any more — but the first two objections are
# the load-bearing ones and they are unchanged.)
#
# PROBE FIXTURES, since two pairs landed here on 2026-08-29 (ranger-base-hvbj).
# A fixture bead that has a session commit a file to prove a commit lands is a
# legitimate measurement, and the ADD is never flagged — only the REMOVAL is.
# So a probe fixture that lands STAYS: write it to docs/probes/<bead-id>.md and
# leave it there. Cleaning it up is what costs a triage line, and restoring a
# deleted one costs a second, because re-adding a blob the path already held is
# itself a backwards move. See docs/probes/README.md.
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
#   - A rename that also EDITS the file is NOT flagged any more: the move
#     exception asks git to pair the deletion with the add at >=50% similarity
#     (ranger-base-en75, after 631bda7, e82338c and 2eae58a each bought a
#     triage line for the same shape). That is a deliberate false-NEGATIVE
#     widening and raw_log carries the measurement behind the number. What it
#     costs: a commit that deletes a newly-landed file while adding a
#     >=50%-similar one in the same commit goes quiet. The exact-blob rule it
#     replaced had the same hazard at 100%.
#   - The rename pairing is only as good as git's rename detection. -l0 turns
#     off the renameLimit that would otherwise make git skip it silently on a
#     large commit, and there is no self-test arm for that flag: reproducing it
#     needs a commit with more files than diff.renameLimit (1000 by default),
#     which is not a fixture worth planting. Documented, not pinned.
#   - The deletion rule needs the path's ADD inside the scanned range. On a
#     partial range (`main~10..main`) a deletion of an older file is invisible;
#     a full-history run (the default, and what `make test` runs) has no such
#     gap. It under-reports on short ranges, it does not false-positive.
#
# TRIAGE. Known-and-explained commits live in scripts/silent-reverts.allow, one
# `<sha> [<patch-id>] <reason>` per line. Anything NOT in that file exits 1 — a
# new silent revert is a build failure, and clearing it means writing down why.
#
# THE OPTIONAL SECOND TOKEN is the diff's patch-id (ADR 0054). A persona reads a
# hit in its own session tree and writes the sha this script printed; the
# launcher rebases that tree onto main at landing and mints a NEW sha for the
# same diff, so on main the line names a commit no ref reaches and the landed
# twin reads as untriaged — the gate red on a hit that was read and explained
# (2026-09-04, e8c5e4e's line against its landed self c8adbcc). So a line may
# carry `git diff-tree -p <sha> | git patch-id --stable`'s first field beside
# the sha. A line whose sha did NOT land here triages the one flagged commit
# carrying its patch-id — the oldest, once, and a second twin stays untriaged
# (D3, and it is the whole difference from a pattern). A line whose sha IS an
# ancestor triages by sha alone and its token is inert (D2). Nobody has to know
# any of this: the UNTRIAGED message below prints the whole line to paste, token
# included, every time (D4).
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

# RENAME-THAT-EDITS control — ranger-base-en75. The move exception used to be
# exact-blob, so a rename that also edited the file was reported as a deletion
# and cost a triage line; three commits in ~460 paid it. moved.go is 20 lines
# with 5 of them rewritten in the same commit as the move, which git scores
# R065 (git 2.50.1) — deliberately the shape of the real strike e82338c
# (etc/cleanroom/{Dockerfile => Dockerfile.debian}, R060) and not of the easy
# one 2eae58a (R097), so this arm also reds if the threshold is raised to 75%.
plant_renameedit() {
  plant_repo || return 2
  local i
  for i in $(seq 1 20); do echo "original line $i of the moved file"; done > mod.go
  env -u RHQ_PERSONA git add -A; env -u RHQ_PERSONA git commit -qm "add mod.go"
  git mv mod.go moved.go
  for i in $(seq 1 20); do
    if [ "$i" -le 5 ]; then echo "EDITED line $i after the move, quite different text here"
    else echo "original line $i of the moved file"; fi
  done > moved.go
  env -u RHQ_PERSONA git add -A
  env -u RHQ_PERSONA git commit -qm "move it and edit it"
  # Fixture witness. This arm asserts an ABSENCE, so it has to show that the
  # hazard is present (ranger-base-z4vx): the top commit must hold exactly one
  # rename, and that rename must be INEXACT. An R100 here would be excused by
  # the OLD exact-blob exception too, i.e. the arm would measure nothing.
  git show --no-abbrev --raw --format= -M50% HEAD |
    awk '$5 ~ /^R0*[0-9]+$/ { sim=substr($5,2)+0; if (sim>=50 && sim<100) n++ }
         END { exit !(n==1) }' || return 2
}

# DELETE-PLUS-UNRELATED-ADD control — ranger-base-en75, and it is the wrong arm
# the arm above needs. Widening the move exception from exact-blob to git's rename
# pairing is a false-NEGATIVE widening: the failure it can cause is silence. So
# one commit here deletes a file a previous commit added AND adds an unrelated
# one, which is the add-only half of the rangerhq-8rtf mechanism wearing exactly
# the coat the new exception hands out. It must still fire, or the exception is
# not an exception, it is an off switch.
plant_delplusadd() {
  plant_repo || return 2
  printf 'package x // the regression pin\n' > newpin_test.go
  env -u RHQ_PERSONA git add -A
  env -u RHQ_PERSONA git commit -qm "the fix: add newpin_test.go"
  env -u RHQ_PERSONA git rm -q newpin_test.go
  printf 'title: notes\nthese bytes have nothing whatever in common with a go\npin; they exist to be an add in the same commit as a\ndeletion, and to sit far below any similarity threshold\nthis tool would ever choose.\n' > unrelated.md
  env -u RHQ_PERSONA git add -A
  env -u RHQ_PERSONA git commit -qm "bd sync: batch"
  # Fixture witness, and the chosen threshold's own control: the top commit must
  # be exactly one deletion and one addition that git did NOT pair. If some
  # future threshold ever pairs these two, the rig stops being the shape it
  # claims and says so here, rather than quietly turning the arm into a
  # tautology.
  git show --no-abbrev --raw --format= -M50% HEAD |
    awk '$5 ~ /^D/ {d++} $5 ~ /^A/ {a++} $5 ~ /^[RC]/ {r++}
         END { exit !(d==1 && a==1 && r+0==0) }' || return 2
}

# RE-LAND-THROUGH-A-RENAME control — ranger-base-en75. Only the SOURCE half of a
# rename is excused; the destination is a write like any other and is still
# compared against every state its path has held. fix.go moves out to stale.go
# and back, so its third state is a blob it already held, and that is a rollback
# whichever way the bytes travelled. This is the arm that pins states_awk's R
# handling: the rename-that-edits arm above does NOT, because deleting the R
# branch outright makes a rename invisible, and invisible is indistinguishable
# from excused when all you assert is silence.
plant_reland() {
  plant_repo || return 2
  git mv fix.go stale.go
  env -u RHQ_PERSONA git commit -qm "move the fix out of the way"
  git mv stale.go fix.go
  env -u RHQ_PERSONA git commit -qm "bd sync: batch"
  [ "$(git show HEAD:fix.go)" = "v1" ] || return 2
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

# --- ADR 0054 fixtures: the patch-id twin -----------------------------------
# The four plants below exercise the TRIAGE half rather than the detector half:
# each plants a shape the detector already flags and then writes the fixture's
# OWN scripts/silent-reverts.allow. This script cds to the toplevel of the repo
# it is run in, so that file is the one the triage reads. It is deliberately not
# committed — the scan reads git log, so an untracked allow file leaves the
# planted commit count alone and every arm keeps its positive witness.
#
# `deadbee` is the stand-in for a sha that did not land here: the retired
# session tree's, which on a fresh clone was never fetched at all and on the box
# that minted it survives only until gc. The twin plant asserts it does not
# resolve, because an arm whose "non-resolving" sha resolves is measuring the
# sha path and saying nothing about the token.

# Write the fixture's allow file. Called after the plant so the shas are real.
plant_allow() {
  mkdir -p scripts
  printf '%s\n' "$@" > scripts/silent-reverts.allow
}

# TWIN — the shape the arm exists for. The line names a sha that does not
# resolve here and carries the flagged commit's patch-id. Must triage, exit 0.
plant_twin() {
  plant_modify || return 2
  plant_allow "deadbee $(patch_id HEAD) the launcher rebased this one; the token is what survived"
  # Fixture witness (ranger-base-z4vx): the hazard has to be present. The line's
  # sha must NOT resolve, and its second field must be a real 40-hex patch-id
  # rather than the empty string a broken patch_id would leave behind.
  git cat-file -e deadbee^{commit} 2>/dev/null && return 2
  awk 'NR==1 && $2 ~ /^[0-9a-f]{40}$/ {ok=1} END {exit !ok}' scripts/silent-reverts.allow || return 2
  return 0
}

# INERT — the WRONG ARM for twin, and the one that keeps the token from becoming
# a pattern (D2). The same patch-id sits beside the sha of an ANCESTOR that is
# not the flagged commit, so the line triages by sha alone, its token says
# nothing, and the flagged commit is still UNTRIAGED: exit 1.
plant_inert() {
  plant_modify || return 2
  plant_allow "$(git rev-parse --short=7 HEAD~1) $(patch_id HEAD) an ancestor's line — its token is inert"
  # Fixture witness: the line's sha really is an ancestor of the scanned tip and
  # really is NOT the flagged commit, or this arm is the mismatch arm again.
  git merge-base --is-ancestor HEAD~1 HEAD || return 2
  [ "$(git rev-parse --short=7 HEAD~1)" != "$(git rev-parse --short=7 HEAD)" ] || return 2
  return 0
}

# MISMATCH — the second wrong arm. A non-resolving sha beside a patch-id that is
# real but is not this commit's (the fix commit's, so the arm is refused by the
# comparison and not by a malformed token). Nothing matches: exit 1.
plant_mismatch() {
  plant_modify || return 2
  plant_allow "deadbee $(patch_id HEAD~1) a real patch-id, but not this commit's"
  [ -n "$(patch_id HEAD~1)" ] || return 2
  [ "$(patch_id HEAD~1)" != "$(patch_id HEAD)" ] || return 2
  return 0
}

# ONE CLAIM — D3. fix, revert, fix again, revert again: the two reverts carry the
# same patch-id, and one token may claim only the OLDER of them. Five commits,
# so this arm carries its own count rather than bending PLANTED.
#
# DIVERGENCE from ADR 0054 Verification 1, which reads "(two flagged commits,
# one patch-id) and one line". THREE commits are flagged here, and the ADR's
# count cannot be built: putting fix.go back to v2 so the second revert can have
# the first's diff is itself a move back to a state the path already held, so
# the re-land BETWEEN the twins is flagged by construction — 1cc432e's shape,
# which this script's header and the allow file both already record. The middle
# commit therefore gets a plain sha line: fixture plumbing, an ancestor, no
# token. What the ADR asks for holds exactly — one token, one claim, exactly one
# UNTRIAGED, and it is the SECOND twin.
plant_oneclaim() {
  plant_repo || return 2
  printf 'v2-THE-FIX\n' > fix.go
  env -u RHQ_PERSONA git commit -qam "the fix"
  printf 'v1\n' > fix.go
  env -u RHQ_PERSONA git commit -qam "revert the fix"      # flagged; patch-id P
  printf 'v2-THE-FIX\n' > fix.go
  env -u RHQ_PERSONA git commit -qam "re-land the fix"     # flagged; 1cc432e's shape
  printf 'v1\n' > fix.go
  env -u RHQ_PERSONA git commit -qam "revert it again"     # flagged; patch-id P again
  plant_allow \
    "deadbee $(patch_id HEAD~2) the launcher rebased the FIRST revert — one claim, not two" \
    "$(git rev-parse --short=7 HEAD~1) the re-land between the twins — fixture plumbing, not a twin"
  # Fixture witness: the two reverts must really be patch-id twins and the
  # re-land between them must really not be, or the arm measures nothing.
  [ -n "$(patch_id HEAD)" ] || return 2
  [ "$(patch_id HEAD)" = "$(patch_id HEAD~2)" ] || return 2
  [ "$(patch_id HEAD~1)" != "$(patch_id HEAD)" ] || return 2
  return 0
}

# The one-claim plant is five commits, not PLANTED's three.
PLANTED_ONECLAIM=5

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
  local rc=0 prc n out scanned shape d fdir nonhex an want arc; d=$(mktemp -d)
  for shape in modify addonly move numeric renameedit delplusadd reland; do
    ( set -e; "plant_$shape" >/dev/null 2>&1; pwd > "$d/$shape" )
    prc=$?
    [ "$prc" -eq 0 ] || {
        echo "self-test: $shape rig did not reproduce the mechanism"; return 2; }
  done
  for shape in modify addonly move numeric renameedit delplusadd reland; do
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
      renameedit)
        # ranger-base-en75. The exact-blob move exception could not see this
        # shape, so every rename that also edited a file cost a triage line.
        # Reds if raw_log stops asking git to pair renames, if the threshold is
        # raised past the fixture R065, or if the source half of an R stops
        # being suppressed.
        if [ "$n" -eq 0 ]; then echo "self-test PASS: a rename that also EDITS is not flagged (over $scanned planted commits)"
        else echo "self-test FAIL: a rename that also edits was flagged as a silent revert"; rc=1; fi ;;
      delplusadd)
        # The WRONG ARM for the one above. Widening the exception is a
        # false-NEGATIVE widening, so the thing to prove is that it did not
        # widen onto a real deletion that merely shares a commit with an add.
        if [ "$n" -ge 1 ]; then echo "self-test PASS: a deletion plus an UNRELATED add in one commit still fires"
        else echo "self-test FAIL: a deletion plus an unrelated add was silenced by the move exception"; rc=1; fi ;;
      reland)
        # The pin states_awk's R handling actually has: only the SOURCE of a
        # rename is excused, the destination is still compared, so content that
        # travels back to a path by rename is still a rollback.
        if [ "$n" -ge 1 ]; then echo "self-test PASS: a re-land through a rename is still caught"
        else echo "self-test FAIL: a re-land through a rename was not caught"; rc=1; fi ;;
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
  # --- ADR 0054: the patch-id twin arm --------------------------------------
  # These four read the TRIAGE, not the detector, so they run the whole audit
  # over the fixture rather than scan() alone — the allow file, the patch-id
  # arm and the hint all live past scan(). Same two-sided fixture discipline as
  # above: the plant's status on its own line, and every arm demands the scan
  # report the commit count its plant means to build.
  for shape in twin inert mismatch oneclaim; do
    ( set -e; "plant_$shape" >/dev/null 2>&1; pwd > "$d/$shape" )
    prc=$?
    [ "$prc" -eq 0 ] || {
        echo "self-test: $shape rig did not reproduce the mechanism"; return 2; }
  done
  for shape in twin inert mismatch oneclaim; do
    want=$PLANTED
    [ "$shape" = oneclaim ] && want=$PLANTED_ONECLAIM
    out=$( cd "$(cat "$d/$shape")" && audit HEAD ); arc=$?
    scanned=$(printf '%s\n' "$out" | awk '/^scanned [0-9]+ commits/ {print $2+0}')
    [ -n "$scanned" ] || scanned=0
    if [ "$scanned" -ne "$want" ]; then
      echo "self-test FAIL: $shape rig scanned $scanned commits, want $want — the fixture was never built"
      rc=1; continue
    fi
    case "$shape" in
      twin)
        # D2's positive half: the line's sha did not land, its token did.
        if [ "$arc" -eq 0 ] && printf '%s\n' "$out" | grep -q 'triaged (patch-id twin of deadbee):'; then
          echo "self-test PASS: a line whose sha did not land triages its patch-id twin"
        else
          echo "self-test FAIL: the patch-id twin arm did not triage the landed twin (exit $arc)"; rc=1
        fi
        # D1's other half, and it has nowhere else to be measured: the triage
        # print strips the token, so a reason never starts with 40 hex. Without
        # the strip this line reads "… — 77e50340…8 the launcher rebased …".
        if printf '%s\n' "$out" | grep -q 'twin of deadbee): [0-9a-f]* — the launcher rebased'; then
          echo "self-test PASS: the triage print strips the patch-id token"
        else
          echo "self-test FAIL: the triage print did not strip the patch-id token"; rc=1
        fi ;;
      inert)
        if [ "$arc" -eq 1 ] && printf '%s\n' "$out" | grep -q 'UNTRIAGED:'; then
          echo "self-test PASS: the token on an ANCESTOR's line is inert"
        else
          echo "self-test FAIL: an ancestor's token triaged another commit's diff (exit $arc)"; rc=1
        fi ;;
      mismatch)
        if [ "$arc" -eq 1 ] && printf '%s\n' "$out" | grep -q 'UNTRIAGED:'; then
          echo "self-test PASS: a token that is not this commit's patch-id triages nothing"
        else
          echo "self-test FAIL: the twin arm fired on a patch-id that does not match (exit $arc)"; rc=1
        fi
        # D4: the hint prints the line to paste, and the patch-id in it is the
        # one git computes from the recipe the ADR names — asked here, of git,
        # rather than of this script's own patch_id.
        fdir=$(cat "$d/mismatch")
        an=$( cd "$fdir" && git diff-tree -p HEAD | git patch-id --stable | awk 'NR==1 {print $1}' )
        if [ -n "$an" ] && printf '%s\n' "$out" | grep -q " $an <reason>"; then
          echo "self-test PASS: the UNTRIAGED hint carries the commit's real patch-id"
        else
          echo "self-test FAIL: the UNTRIAGED hint did not carry the commit's patch-id"; rc=1
        fi ;;
      oneclaim)
        n=$(printf '%s\n' "$out" | grep -c 'UNTRIAGED:')
        an=$( cd "$(cat "$d/oneclaim")" && git rev-parse --short=7 HEAD )
        if [ "$n" -eq 1 ] && printf '%s\n' "$out" | grep -q "UNTRIAGED: $an"; then
          echo "self-test PASS: one patch-id claims one commit; the second twin stays UNTRIAGED"
        else
          echo "self-test FAIL: one token left $n commit(s) untriaged, want exactly the second twin"; rc=1
        fi ;;
    esac
  done

  [ "$rc" -eq 0 ] && echo "self-test PASS: detector flags the rangerhq-8rtf mechanism"
  return $rc
}

# The raw log every scan reads.
#
# --find-renames=50% -l0 is the move exception (ranger-base-en75). It used to be
# --no-renames, and the move exception lived entirely in states_awk as an
# EXACT-BLOB rule: a deletion was excused only when the identical blob appeared
# at another path in the same commit. A rename that also EDITS the file is a
# different blob, so the exception could not see it and the deletion half was
# reported. Three commits in ~460 paid a triage line for that (631bda7,
# e82338c, 2eae58a) and the rate was not falling, so the question is asked of
# git now instead of guessed at.
#
# THE THRESHOLD IS 50%, CHOSEN, NOT INHERITED. This is a false-NEGATIVE
# widening and the number is the whole of its width, so it does not get to be
# git's default by accident:
#   - 50% is the LOWEST value at which git pairs anything, so it is the widest
#     this exception can be. It is picked at the wide end deliberately: the two
#     live strikes measure R097 (examples/agents/{ranger.md => ops.md}) and
#     R060 (etc/cleanroom/{Dockerfile => Dockerfile.debian}), and 60% is the
#     tightest threshold that clears both — i.e. zero margin, and the next
#     rename+edit at 55% buys back the triage line this bead exists to stop.
#   - What it costs is measured, not argued. Over this repo's 504 commits git
#     pairs exactly THREE deletions with an add even at -M30% — the two above
#     and one R100 exact move — and all three are real renames. There is no
#     commit in this history where a lower threshold silences something that is
#     not a rename, so the observed false-pairing rate at the chosen number is
#     0/504.
#   - What is still at risk, stated plainly: a stale-index commit that deletes
#     a newly-landed file while adding a >=50%-similar one in the SAME commit
#     goes quiet. The old exact-blob exception had the identical hazard at
#     100%. Only the SOURCE half of a rename is excused; the destination is
#     still compared against every state that path has held, so a re-land
#     through a rename is still caught (the reland self-test arm).
#   - -l0 is not decoration. git skips inexact rename detection entirely once
#     the file count passes diff.renameLimit, whose default has changed across
#     git versions, and it does so with a warning on stderr that nothing here
#     reads. That is the ranger-base-hhcu shape again: the same history, two
#     verdicts, decided by the toolchain. 0 means unlimited, and it costs 0.55s
#     over this repo's full history (measured 2026-08-29, git 2.50.1). It has no
#     self-test arm — see LIMITS.
# Keep this flag and states_awk's R handling in step, in both directions. -M
# WITHOUT the R branch is worse than either half alone: the rename line parses
# as one path literally named "src<tab>dst", so the deletion is never recorded
# at all and the audit under-reports in silence. The R branch WITHOUT -M is
# dead code. Both directions are pinned in
# internal/posse/silentrevert_qa_test.go.
#
# --no-abbrev is load-bearing (ranger-base-hhcu):
# git's default abbreviation is 7 hex, which is short enough that a blob id
# lands on the <digit>e<digits> shape roughly once in 270, and that shape is
# a NUMBER to awk. Full 40-hex ids are not immune in principle, but the shape
# needs all 40 characters to cooperate, which is ~6e-8 rather than 0.4%.
# The string comparison in states_awk is the actual fix; this is the cheap
# second layer, and each is pinned by its own assertion in the numeric arm.
raw_log() {
  git log --reverse --no-merges --find-renames=50% -l0 --raw --no-abbrev \
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
        p=epath[i]; st=est[i]; d=edst[i]; s=esrc[i]; moved=emoved[i]
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
      rest=substr($0,t+1)
      split(substr($0,1,t-1),m," ")
      # A RENAME IS TWO PATHS ON ONE LINE (ranger-base-en75). raw_log asks git
      # to pair deletions with adds, so a rename arrives as
      #   :<mode> <mode> <src> <dst> R<sim>\t<srcpath>\t<dstpath>
      # which the single-tab path parse below would read as one path literally
      # named "src<tab>dst". Decompose it into the two entries it stands for:
      # the source LEAVES (a deletion, but a MOVED one, so not a rollback) and
      # the destination ARRIVES with the possibly-edited blob. The destination
      # is deliberately a plain write, compared against every state that path
      # has held, so a re-land through a rename is still caught. C<sim> (a
      # copy) is handled the same way minus the deletion; raw_log does not
      # enable -C today, so that arm is defensive.
      if (m[5] ~ /^[RC]/) {
        tt=index(rest,"\t"); if (tt==0) next
        if (m[5] ~ /^R/) {
          ne++; epath[ne]=substr(rest,1,tt-1)
          esrc[ne]=m[3] ""; edst[ne]=m[4] ""; est[ne]="D"; emoved[ne]=1
        }
        ne++; epath[ne]=substr(rest,tt+1)
        esrc[ne]=m[3] ""; edst[ne]=m[4] ""; est[ne]="A"; emoved[ne]=0
        next
      }
      ne++
      epath[ne]=rest
      emoved[ne]=0
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

# --- TRIAGE (ADR 0054) ------------------------------------------------------
# The allow file's grammar is `<sha> [<patch-id>] <reason>` (D1). Everything
# below is what the optional second token buys; with no token in the file the
# behaviour is exactly what it was, because the sha match is tried first and the
# arm runs only over what the sha match left.

# The patch-id of one commit's diff — the first field of
# `git diff-tree -p <sha> | git patch-id --stable`, which is the recipe the
# UNTRIAGED hint tells the reader to run, spelled the same way here so the two
# cannot drift. Empty when there is no diff to hash (a root commit).
patch_id() {
  git diff-tree -p "$1" 2>/dev/null | git patch-id --stable 2>/dev/null | awk 'NR==1 {print $1}'
}

# The reason on the allow line(s) naming this sha, with the sha and the optional
# patch-id token stripped. THE COST OF MAKING THE TOKEN OPTIONAL is that a
# reason may not begin with 40 hex — the triage lines in the file today all
# begin "Benign", "THE INCIDENT" or "The REPAIR", and the allow file's header
# states the rule under ranger-base-ltari, which owns that file. sub() rebuilds
# $0's fields, so the second sub sees whatever followed the sha.
allow_reason() {
  awk -v s="$1" '$1 ~ "^"s { sub($1" *", ""); if ($1 ~ /^[0-9a-f]{40}$/) sub($1" *", ""); print }' "$ALLOW"
}

# D2's predicate: did this allow line's sha land here? BOTH halves are
# load-bearing. On a fresh clone (ci.yml, fetch-depth: 0) an object on no ref is
# never transferred, so a session sha does not resolve at all; on the box that
# minted it the object still EXISTS for a fortnight after the rebase orphaned
# it, on zero refs, and only merge-base can say it is not on the branch.
line_sha_landed() {
  git rev-parse --verify --quiet "$1^{commit}" >/dev/null 2>&1 || return 1
  git merge-base --is-ancestor "$1" "$2" 2>/dev/null
}

# The allow line whose token claims this patch-id, or nothing. A line whose sha
# landed here triages by sha alone and its token is inert — that is the whole of
# what keeps this from being a pattern that excuses the next commit with the
# same diff.
#
# FORCE STRING on both sides, for the reason states_awk does (ranger-base-hhcu):
# a field is a STRNUM and so is a -v assignment, and awk compares two strnums
# NUMERICALLY when both look like numbers. A 40-hex patch-id of the form
# <digit>e<digits> is valid scientific notation whose value overflows to +inf,
# and +inf == +inf, so a coercing awk would hand one commit's token to another
# commit's diff. Rare at 40 hex (~6e-8) and not rare enough to leave to luck in
# the one comparison that decides whether a hit is excused.
twin_line_for() {
  local pid=$1 tip=$2 s
  [ -f "$ALLOW" ] || return 0
  for s in $(awk -v p="$pid" '$1 !~ /^#/ && ($2 "") == (p "") { print $1 }' "$ALLOW"); do
    line_sha_landed "$s" "$tip" && continue
    printf '%s\n' "$s"
    return 0
  done
}

# The whole audit over one range, from the current directory: scan, triage,
# report. Split out of the main body so a self-test arm can run the TRIAGE half
# over a planted fixture (ADR 0054 Verification 1) — the allow file, the twin
# arm and the hint all live past scan(). Reads QUIET. Returns 1 when something
# is untriaged, 2 when the scan itself broke.
audit() {
  local range=${1:-HEAD} out scanned detail untriaged untriaged_shas sha pid tip
  local twin_lines hints lsha spent remaining
  out=$(scan "$range") || return 2
  scanned=$(printf '%s\n' "$out" | awk -F'\t' '/^SCANNED/{print $2}')
  detail=$(printf '%s\n' "$out" | grep -v $'^SHA\t' | grep -v '^SCANNED')
  [ "$QUIET" -eq 1 ] || printf '%s\n' "$detail"

  untriaged=0; untriaged_shas=""
  while IFS=$'\t' read -r _ sha; do
    if [ -f "$ALLOW" ] && grep -q "^$sha" "$ALLOW"; then
      [ "$QUIET" -eq 1 ] || printf '  triaged: %s — %s\n' "$sha" "$(allow_reason "$sha")"
    else
      untriaged_shas="$untriaged_shas $sha"
      untriaged=$((untriaged+1))
    fi
  done < <(printf '%s\n' "$out" | grep $'^SHA\t')

  # The patch-id twin arm (D2/D3) and the hint's token (D4). It runs ONLY when
  # something is untriaged — which is the state the gate is red in anyway — so a
  # clean run makes zero extra git calls, and there is a pin in
  # internal/posse/silentrevert_qa_test.go that counts them.
  twin_lines=""; hints=""
  if [ "$untriaged" -gt 0 ]; then
    tip=${range##*..}
    [ -n "$tip" ] || tip=HEAD
    git rev-parse --verify --quiet "$tip^{commit}" >/dev/null 2>&1 || tip=HEAD
    spent=" "; remaining=""; untriaged=0
    # Scan order is oldest-first (raw_log --reverse), so the first commit a
    # token matches IS the oldest flagged one carrying it: D3's one claim, in
    # one pass, with `spent` spending the token on it.
    for sha in $untriaged_shas; do
      pid=$(patch_id "$sha"); lsha=""
      case "$spent" in
        *" $pid "*) ;;
        *) [ -n "$pid" ] && lsha=$(twin_line_for "$pid" "$tip") ;;
      esac
      if [ -n "$lsha" ]; then
        spent="$spent$pid "
        twin_lines="$twin_lines  triaged (patch-id twin of $lsha): $sha — $(allow_reason "$lsha")
"
      else
        remaining="$remaining $sha"
        hints="$hints$sha $pid
"
        untriaged=$((untriaged+1))
      fi
    done
    untriaged_shas=$remaining
    [ "$QUIET" -eq 1 ] || printf '%s' "$twin_lines"
  fi

  if [ "$untriaged" -gt 0 ] && [ "$QUIET" -eq 1 ]; then
    printf '%s\n' "$detail"; printf '%s' "$twin_lines"
  fi
  for sha in $untriaged_shas; do
    # Strings on both sides again: two 7-hex shas land on the <digit>e<digits>
    # shape about once in 270 each, which is the abbreviation hazard the whole
    # of ranger-base-hhcu is about, and here it would print one commit's
    # patch-id under another commit's sha.
    pid=$(printf '%s' "$hints" | awk -v s="$sha" '($1 "") == (s "") {print $2}')
    printf '  UNTRIAGED: %s — a silent revert nobody has explained. Read it, then\n' "$sha"
    printf '             either fix it or paste this line into %s:\n' "$ALLOW"
    # D4: the line to paste, patch-id included, every time. Nobody is asked to
    # know the recipe or to guess whether their commit will be rebased.
    if [ -n "$pid" ]; then printf '               %s %s <reason>\n' "$sha" "$pid"
    else printf '               %s <reason>\n' "$sha"; fi
  done

  [ "$QUIET" -eq 1 ] && [ "$untriaged" -eq 0 ] \
    && { echo "silent-revert audit: $scanned commits, 0 untriaged"; return 0; }
  echo
  echo "scanned $scanned commits; $untriaged untriaged silent revert(s)"
  [ "$untriaged" -eq 0 ] || return 1
  return 0
}

QUIET=0
[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }

# --quiet: one summary line when clean, full detail when something is untriaged.
[ "${1:-}" = "--quiet" ] && { QUIET=1; shift; }

audit "${1:-HEAD}"
exit $?
