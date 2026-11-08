#!/bin/sh
# The bwrap tier as DESIGNED (ADR 0080, amendment "The bwrap tier",
# ranger-base-rwcqg, D16-D22). 0080-linux-cage.probe.sh measured the
# mechanism; this measures the parts of the design that rest on a runtime or
# on git rather than on the kernel. Written 2026-11-08 on a darwin laptop, so
# none of it has run on Linux yet: the first run on the D1 host is the
# measurement. Paste its output on the bead that runs it.
#
#   sh docs/adr/0080-bwrap-tier.probe.sh
#
# Needs: Linux, bwrap, git, claude; codex optional. Run as the posse user.
# Everything is under one mktemp dir, removed on exit. No model is called:
# `claude mcp add` writes the user config and makes no request, and
# `codex sandbox` runs a shell command under codex's own sandbox, no turn.
#
# Each line prints PROBE EXPECT GOT. A row whose GOT differs from EXPECT is
# a finding. Do not rerun to make it match.
set -u
W=$(mktemp -d)
trap 'chmod -R u+w "$W" 2>/dev/null; rm -rf "$W"' EXIT
row() { printf '%-50s %-10s %s\n' "$1" "$2" "$3"; }
try() { if "$@" >/dev/null 2>&1; then echo allowed; else echo refused; fi; }
BWRAP=$(command -v bwrap) || { echo "bwrap: absent"; exit 1; }
# The D17 base: the host read-only, /dev live, no namespace but the user and
# mount ones (pid, net and /tmp stay the host's).
BASE="$BWRAP --die-with-parent --ro-bind / / --dev-bind /dev /dev"

echo "== host =="
uname -sr; "$BWRAP" --version
for b in claude codex git; do
  printf '%s: ' "$b"; command -v "$b" >/dev/null && "$b" --version 2>&1 | head -1 || echo absent
done

echo "== T1 claude's config save inside a dir grant (D19) =="
# MEASURED on darwin (ADR 0080 D19): with only CLAUDE_CONFIG_DIR writable the
# save lands; with its .claude.json.* names denied it is lost. Here, $HOME is
# read-only and only the config dir is bound, which is the shipped wall.
mkdir -p "$W/cfg"
if command -v claude >/dev/null; then
  : > "$W/t1.mark"
  CLAUDE_CONFIG_DIR="$W/cfg" $BASE --bind "$W/cfg" "$W/cfg" \
    claude mcp add -s user t1probe -- /bin/true >"$W/t1.out" 2>&1
  rc=$?
  row "T1 claude mcp add, only the config dir writable" allowed \
    "$( [ $rc -eq 0 ] && echo allowed || echo "refused: $(tail -1 "$W/t1.out" | cut -c1-60)")"
  row "T1b the save reached the file" saved \
    "$(grep -q '"t1probe"' "$W/cfg/.claude.json" 2>/dev/null && echo saved || echo lost)"
  # A .lock or .tmp.<pid>.<hex> left behind means a sibling was refused.
  row "T1c no .claude.json.* sibling left behind" none \
    "$(ls -A "$W/cfg" | grep '^\.claude\.json\.' | tr '\n' ' ' | sed 's/^$/none/')"
  row "T1d no .claude.json.* written in HOME" none \
    "$(find "$HOME" -maxdepth 1 -name '.claude.json.*' -newer "$W/t1.mark" | tr '\n' ' ' | sed 's/^$/none/')"
else
  row "T1 claude mcp add, only the config dir writable" allowed absent
fi

echo "== T2-T5 git in a linked worktree under the D20 grants =="
R="$W/repo"; WT="$W/wt"
git init -q -b main "$R" && git -C "$R" -c user.name=p -c user.email=p@x commit -q --allow-empty -m root
git -C "$R" worktree add -q -b posse/s1 "$WT"
git -C "$R" branch -q posse/s2          # a peer session's ref
C=$(cd "$R/.git" && pwd -P)
: > "$C/refs/heads/posse/.posse-keep"   # D20's keep file
GITW="$BASE --bind $WT $WT --bind $C/objects $C/objects --bind $C/logs $C/logs
  --bind $C/worktrees/wt $C/worktrees/wt --bind $C/refs/heads/posse $C/refs/heads/posse"
G="git -c user.name=p -c user.email=p@x -C $WT"
row "T2 commit in the session's worktree" allowed "$(try $GITW $G commit -q --allow-empty -m t2)"
row "T3 move main" refused "$(try $GITW $G update-ref refs/heads/main HEAD)"
row "T3b write the shared hooks dir" refused "$(try $GITW sh -c "echo x > $C/hooks/pre-commit")"
row "T3c write the shared config" refused "$(try $GITW sh -c "echo x >> $C/config")"
row "T3d move a peer session's ref (D20 widening)" allowed "$(try $GITW $G update-ref refs/heads/posse/s2 HEAD)"
# T4: a gc OUTSIDE the wall packs every loose ref while the seat is up. With
# the keep file, the bound dir survives and the seat's next commit lands.
rm -f "$W/go"
$GITW sh -c "while [ ! -f $W/go ]; do sleep 0.1; done; $G commit -q --allow-empty -m t4" >/dev/null 2>&1 &
P=$!
sleep 1; git -C "$R" pack-refs --all --prune; : > "$W/go"
wait $P && T4=allowed || T4=refused
row "T4 commit after an outside pack-refs --all" allowed "$T4"
row "T4b the keep file kept the dir" present "$( [ -d "$C/refs/heads/posse" ] && echo present || echo pruned)"
row "T4c git ignores the keep file (for-each-ref)" clean \
  "$(git -C "$R" for-each-ref 2>&1 | grep -q posse-keep && echo listed || echo clean)"
row "T4d git fsck says nothing of it" clean \
  "$(git -C "$R" fsck 2>&1 | grep -q posse-keep && echo flagged || echo clean)"
# T4e is the control: same pack, no keep file, so the empty dir is pruned
# under a live mount. EXPECT refused is what makes the keep file load-bearing.
rm -f "$C/refs/heads/posse/.posse-keep" "$W/go"
git -C "$R" branch -q -D posse/s2
$GITW sh -c "while [ ! -f $W/go ]; do sleep 0.1; done; $G commit -q --allow-empty -m t4e" >/dev/null 2>&1 &
P=$!
sleep 1; git -C "$R" pack-refs --all --prune; : > "$W/go"
wait $P && T4E=allowed || T4E=refused
row "T4e control: no keep file, commit after the pack" refused "$T4E"
# T5: the ADR 0059 residual. No packed-refs.lock grant, so a sequencer that
# ends by deleting a pseudo-ref leaves it behind (AGENTS.md, ranger-base-71g2f).
git -C "$R" worktree remove --force "$WT" 2>/dev/null; rm -rf "$WT"
git -C "$R" worktree add -q -B posse/s1 "$WT" main
mkdir -p "$C/refs/heads/posse"; : > "$C/refs/heads/posse/.posse-keep"
git -C "$R" -c user.name=p -c user.email=p@x commit -q --allow-empty -m pick
PICK=$(git -C "$R" rev-parse main)
$GITW $G cherry-pick --allow-empty "$PICK" >/dev/null 2>&1
row "T5 cherry-pick exits, CHERRY_PICK_HEAD state" leaks \
  "$( [ -f "$C/worktrees/wt/CHERRY_PICK_HEAD" ] && echo leaks || echo clean)"

echo "== T6 codex's own sandbox nested in ours (D21) =="
if command -v codex >/dev/null; then
  mkdir -p "$W/out"
  CX="$BASE --bind $WT $WT --bind $W/out $W/out"
  row "T6 codex sandbox: write in its workspace" allowed \
    "$(try $CX codex sandbox -c sandbox_mode=workspace-write -C "$WT" -- sh -c "echo x > $WT/t6")"
  row "T6b codex sandbox: write our wall grants, codex does not" refused \
    "$(try $CX codex sandbox -c sandbox_mode=workspace-write -C "$WT" -- sh -c "echo x > $W/out/t6")"
  row "T6c codex sandbox: write neither grants" refused \
    "$(try $CX codex sandbox -c sandbox_mode=workspace-write -C "$WT" -- sh -c "echo x > $W/t6")"
else
  row "T6 codex sandbox: write in its workspace" allowed absent
fi

echo "== T7 the launch line's --args fd (D22) =="
# The args file is NUL-separated, as posse writes it. The redirection sits
# BEFORE the command name, which is where SeatbeltPrefix's slot puts it.
printf '%s\0' --die-with-parent --ro-bind / / --dev-bind /dev /dev --bind "$W/out" "$W/out" > "$W/args"
row "T7 bwrap --args 3 reads the rendered file" allowed \
  "$(try sh -c "3<'$W/args' '$BWRAP' --args 3 -- sh -c 'echo x > $W/out/t7'")"
row "T7b the wall in the file holds" refused \
  "$(try sh -c "3<'$W/args' '$BWRAP' --args 3 -- sh -c 'echo x > $W/t7'")"
row "T7c fd 3 inside the seat" closed \
  "$(sh -c "3<'$W/args' '$BWRAP' --args 3 -- sh -c '[ -e /proc/self/fd/3 ] && echo open || echo closed'" 2>&1)"
echo "== done; GOT != EXPECT rows are the findings =="
