#!/usr/bin/env python3
"""Does any SEAT ever write the store-of-record repo's `.git`? Measured.

WHY THIS EXISTS (ranger-base-kfrpv, from the security lane's verify on
ranger-base-julaf). A seat whose store of record lives in another repository
(`.beads/redirect`, ADR 0012 D3-C) was granted that repo's git dirs WHOLE —
objects, objects/info, info/, packed-refs, refs/heads, refs/replace, index
and logs, all writable from inside the wall with no seat store over them. The
grant's stated reason, written in 2026-08 when the store still lived in the
constitution repo, was that `bd sync` COMMITS the JSONL there.

Two facts decide whether that reason still holds, and only one of them is a
code read. `bd sync --help` on the pinned bd says "Does NOT stage or commit -
that's the user's job", the committing spelling `bd sync --full` is denied by
every crew PID, and since ADR 0015 §4 the committer is the LAUNCHER
(App.CommitQueueJSONL), uncaged and under the launcher lock. The other fact is
what the crew actually TYPES, and that is measurable rather than arguable:
this is the instrument. Re-run it; the corpus grows.

WHAT IT READS. `~/.claude/projects/*/*.jsonl` — every session transcript on
the box, not just the caller's. Each Bash `tool_use` carries the verbatim
command. No `ps`, no unified log, no root, none of which a caged seat can
reach anyway.

THE FOUR THINGS IT HAS TO GET RIGHT, each of which was wrong in a draft and
each of which changed the answer:

  1. SEATS AND THE OPERATOR ARE DIFFERENT QUESTIONS. The operator's own
     session is uncaged and works in the shared checkout; its `git push` and
     `git fetch` in the store repo are the design, not the defect. Project
     directories are split on that line and both halves are printed, because
     a census that pooled them would read the operator's pushes as seat
     writes and "prove" the grant is used.

  2. SEGMENTS, NOT LINES. `cd <store> && bd comments add <id> "... git add
     ..."` is one line whose `git` is inside a bead comment. The line is
     split on the separators a shell would split on, and only a segment whose
     own head word is `git` counts.

  3. WHICH REPO THE git IS AIMED AT. A segment counts for this store only if
     its `-C` names a path under the store repo, or a preceding `cd` in the
     same line put the shell there. A `git -C "$SP/dry/<store>" update-ref`
     against a scratchpad COPY is not a write to the store repo, and two of
     those are the only write verbs any seat session has ever typed at
     something spelled like it.

  4. READ VERBS ARE THE DENOMINATOR. The answer "no seat writes it" is worth
     nothing without "167 seat commands name it and every git verb among them
     is a read". Both are printed, and since ranger-base-7c2py the rule is
     ENFORCED rather than stated: an empty read denominator REFUSES the
     all-clear and exits 2. It was stated and not enforced for one release,
     and that is exactly the hole finding 2 of ranger-base-7c2py came through
     — `--store /home/<user>/src/<name>` matched no path component, so every
     git verb fell out of the classification, reads and writes alike, and the
     script printed "no seat session has ever typed a git write" over an
     empty corpus at exit 0. MEASURED 2026-11-21 against the live store: at
     645 transcripts the path spelling read 111 seat commands and NO git verbs
     at all, where the NAME spelling read 200 commands and 53 read verbs; the
     name spelling re-read the same hour at 689 transcripts read 220 commands,
     the same eight read verbs and the same empty write set. The NAME readings
     are the reading. A census that cannot tell its own silence from an
     all-clear is worse than no census, because the next reader re-runs it to
     re-validate a wall.

WHAT COUNTS AS A WRITE. Any git verb that can write under `.git`: the porcelain
that commits or moves a ref, the plumbing that writes an object, and the
maintenance verbs. `branch` is in that set and `branch --show-current` is not,
so the flags are read before the verb is scored — that distinction is three of
the seat hits.

USAGE
    python3 scripts/store-git-write-census.py --store <dir name>
                                              [--root ~/.claude/projects]
                                              [--self-test]

`--store` is REQUIRED and is matched as a path COMPONENT, so it takes the
store repo's directory name and not a full path: transcripts spell it
`~/src/<name>`, `/home/<user>/src/<name>` and `$HOME/src/<name>`, and a full
path would match one of the three. Required rather than defaulted, because a
default would name one instance's repo in a public tree (ADR 0024 D2). A
`--store` carrying a path separator is REFUSED at exit 2 rather than matched
and missed: a path is the spelling everything else in this shop uses, so the
wrong one has to be loud (ranger-base-7c2py).

EXIT STATUS
    0   the all-clear, over a non-empty read denominator: seats name the store
        repo, every git verb among those commands is a read, and none writes.
    1   at least one seat git write — read the examples before trusting a wall
        that rests on this census.
    2   the census did not answer: a bad --store, no transcripts, or an empty
        read denominator under rule 4 above. NOT an all-clear.
"""

import argparse
import contextlib
import io
import json
import os
import re
import sys
import tempfile
from collections import Counter, defaultdict

DEFAULT_ROOT = os.path.expanduser("~/.claude/projects")

def operator_dir():
    """The project directory of the OPERATOR's own session.

    Claude Code names a project directory after the cwd with the separators
    replaced, so the operator's own session — which works in the home, in the
    shared checkout every crew worktree is a worktree of — is the one named
    after the home itself. DERIVED and never written down: a literal here
    would be this box's username in a public tree (ADR 0024 D2), and it would
    be wrong on any other box.
    """
    return "-" + os.path.expanduser("~").strip("/").replace("/", "-")

# git verbs that can write anything under `.git`. Spelled out rather than
# derived as "not in a read list", so a verb git adds tomorrow is scored as a
# read and undercounts, which is the direction that cannot manufacture the
# answer this census was run to get.
WRITE_VERBS = frozenset(
    """
    add am apply branch checkout cherry-pick clean commit commit-tree config
    fetch gc hash-object index-pack init maintenance merge mv notes
    pack-objects pack-refs prune pull push read-tree rebase reflog remote
    repack replace reset restore revert rm sparse-checkout stash switch
    symbolic-ref tag update-index update-ref worktree write-tree
    """.split()
)

# Flags that turn a write verb into a read. `git branch --show-current` and
# `git branch -a` are reads of the ref store; `git remote -v` is a read of the
# config. Without this the census scores six seat reads as writes.
READ_ONLY_FLAGS = {
    "branch": {"--show-current", "-a", "--all", "-l", "--list", "-v", "--verbose", "--contains", "--merged", "--no-merged"},
    "remote": {"-v", "--verbose", "get-url", "show"},
    "config": {"--get", "--get-all", "--get-regexp", "--list", "-l"},
    "notes": {"list", "show"},
    "stash": {"list", "show"},
    "worktree": {"list"},
    "tag": {"-l", "--list"},
    "reflog": {"show"},
}

SEGMENT = re.compile(r"\n|;|&&|\|\||\|")
CD = re.compile(r"(?:cd|pushd)\s+(\S+)")
GIT = re.compile(r"(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*git\s+(.*)$", re.S)
DASH_C = re.compile(r"-C\s+((?:\"[^\"]*\")|(?:'[^']*')|\S+)\s+(.*)$", re.S)
VERB = re.compile(r"((?:-{1,2}[A-Za-z][\w-]*\s+)*)([a-z][a-z-]*)")


def names_store(path, store):
    """Is `path` inside the store repo, by path COMPONENT?

    Not a substring test: a scratchpad copy at `$SP/dry/<store>` carries the
    name too, and finding 3 above is what that distinction is worth. A
    segment's `-C` is accepted only when the component is the LAST one before
    the rest of the path — i.e. the path is the store repo or inside it — and
    never when something else follows a `dry/` style prefix of its own.
    """
    parts = [p for p in path.strip("\"'").split("/") if p and p != "."]
    if store not in parts:
        return False
    i = parts.index(store)
    # `~/src/<store>` and `/home/x/src/<store>/...` count; `$SP/dry/<store>`
    # does not, because its parent is a scratchpad the seat made.
    return i > 0 and parts[i - 1] in ("src", "repos", "code")


def git_segments(cmd, store):
    """Every git verb in `cmd` aimed at the store repo, with its flags."""
    cwd_in_store = False
    for seg in SEGMENT.split(cmd):
        s = seg.strip()
        if not s:
            continue
        m = CD.match(s)
        if m:
            cwd_in_store = names_store(m.group(1), store)
            continue
        m = GIT.match(s)
        if not m:
            continue
        rest = m.group(1)
        aimed = cwd_in_store
        c = DASH_C.match(rest)
        if c:
            aimed = names_store(c.group(1), store)
            rest = c.group(2)
        if not aimed:
            continue
        v = VERB.match(rest)
        if not v:
            continue
        yield v.group(2), rest


def is_write(verb, rest):
    if verb not in WRITE_VERBS:
        return False
    for flag in READ_ONLY_FLAGS.get(verb, ()):
        if re.search(r"(?<![\w-])" + re.escape(flag) + r"(?![\w-])", rest):
            return False
    return True


def commands(path):
    with open(path, "r", errors="replace") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            msg = rec.get("message")
            if not isinstance(msg, dict):
                continue
            content = msg.get("content")
            if not isinstance(content, list):
                continue
            for c in content:
                if isinstance(c, dict) and c.get("type") == "tool_use" and c.get("name") == "Bash":
                    cmd = (c.get("input") or {}).get("command")
                    if isinstance(cmd, str):
                        yield cmd


def census(root, store):
    op = operator_dir()
    seen = Counter()
    reads = defaultdict(Counter)
    writes = defaultdict(Counter)
    examples = defaultdict(list)
    files = 0
    for d in sorted(os.listdir(root)):
        pdir = os.path.join(root, d)
        if not os.path.isdir(pdir):
            continue
        kind = "operator" if d == op else "seat"
        for name in sorted(os.listdir(pdir)):
            if not name.endswith(".jsonl"):
                continue
            files += 1
            for cmd in commands(os.path.join(pdir, name)):
                if store not in cmd:
                    continue
                seen[kind] += 1
                for verb, rest in git_segments(cmd, store):
                    if is_write(verb, rest):
                        writes[kind][verb] += 1
                        if len(examples[kind + "/" + verb]) < 3:
                            examples[kind + "/" + verb].append((d, ("git " + rest)[:160]))
                    else:
                        reads[kind][verb] += 1
    return files, seen, reads, writes, examples


def report(root, store, out):
    files, seen, reads, writes, examples = census(root, store)
    print(f"transcripts read: {files} under {root}", file=out)
    print(f"store repo matched by path component: {store}\n", file=out)
    for kind in ("seat", "operator"):
        print(f"=== {kind} sessions — {seen[kind]} Bash command(s) naming {store}", file=out)
        print(f"    read git verbs : {sorted(reads[kind].items(), key=lambda r: (-r[1], r[0])) or 'none'}", file=out)
        w = sorted(writes[kind].items(), key=lambda r: (-r[1], r[0]))
        print(f"    WRITE git verbs: {w or 'NONE'}", file=out)
        for verb, _ in w:
            for d, line in examples[kind + "/" + verb]:
                print(f"      [{verb}] [{d}] {line}", file=out)
        print("", file=out)
    if writes["seat"]:
        print(f"ANSWER: {sum(writes['seat'].values())} seat git write(s) — read the examples before trusting the wall.", file=out)
        return 1
    # RULE 4, ENFORCED (ranger-base-7c2py). An empty write set over an empty
    # read set is the instrument saying nothing, and it is indistinguishable
    # from the all-clear by every signal a reader has: same report, same
    # exit 0. The denominator is the READ verbs and not `seen`, because a
    # --store that matches no path component still matches as a substring of
    # the command and keeps `seen` looking healthy — 111 commands and no verbs
    # is how finding 2 presented.
    if not reads["seat"]:
        print(
            f"ANSWER: NOT PROVEN — no seat session names {store} with a git verb aimed at it, "
            "so the empty write set is this census's silence and not a reading "
            "(rule 4: read verbs are the denominator). Check the --store spelling: "
            "it is the store repo's directory NAME, matched as a path component.",
            file=out,
        )
        return 2
    print(f"ANSWER: no seat session has ever typed a git write aimed at {store}.", file=out)
    return 0


def self_test(out):
    """The classifier, over lines this corpus really carries.

    Every arm is a line that was MISREAD by a draft of this script, so an arm
    that starts passing for the wrong reason is one the next draft has to
    break on purpose.
    """
    store = "beadqueue"
    cases = [
        # (command, expected (verb, is_write) pairs)
        ("cd ~/src/beadqueue; git push -q origin main", [("push", True)]),
        ("git -C /home/u/src/beadqueue branch --show-current 2>&1", [("branch", False)]),
        ("cd /home/u/src/beadqueue; git branch -a | head", [("branch", False)]),
        ("git -C \"$SP/dry/beadqueue\" update-ref refs/remotes/origin/main x", []),
        ("cd ~/src/beadqueue && bd comments add x \"... git add . ...\"", []),
        ("cd ~/src/beadqueue && git status -sb | head -1", [("status", False)]),
        ("cd ~/src/project && git commit -m x -- f; cd ~/src/beadqueue && git log -1", [("log", False)]),
        ("git -C ~/src/beadqueue remote -v", [("remote", False)]),
        ("cd ~/src/beadqueue && git fetch -q origin", [("fetch", True)]),
    ]
    bad = 0
    for cmd, want in cases:
        got = [(v, is_write(v, rest)) for v, rest in git_segments(cmd, store)]
        if got != want:
            bad += 1
            print(f"MISCLASSIFIED: {cmd}\n  got  {got}\n  want {want}", file=out)
    print(f"self-test: {len(cases) - bad}/{len(cases)} classifier arm(s) pass", file=out)
    ibad, itotal = instrument_arms(out)
    print(f"self-test: {itotal - ibad}/{itotal} instrument arm(s) pass", file=out)
    return 1 if (bad or ibad) else 0


# Any name but the operator's, which is DERIVED from $HOME — a corpus planted
# in the operator's half would be scored as the operator's, and every seat arm
# below would then read zero for the wrong reason. The arms assert the
# difference rather than assuming it.
PLANTED_SEAT_DIR = "-seat-session"


def plant_corpus(root, cmds):
    """One seat session's transcript under a fresh root, carrying `cmds`."""
    pdir = os.path.join(root, PLANTED_SEAT_DIR)
    os.makedirs(pdir, exist_ok=True)
    with open(os.path.join(pdir, "a.jsonl"), "w") as f:
        for c in cmds:
            rec = {"message": {"content": [
                {"type": "tool_use", "name": "Bash", "input": {"command": c}},
            ]}}
            f.write(json.dumps(rec) + "\n")
    return root


def instrument_arms(out):
    """The ARGUMENT and the ANSWER, end to end — not the classifier.

    Every arm in self_test feeds git_segments directly, which is why all nine
    of them were green over a --store that matched no path component and over
    an all-clear printed with an empty read denominator (ranger-base-7c2py
    finding 2): the one thing the self-test never ran was the script. These
    arms run report() and main() whole over a planted corpus, and the last
    one is the blindness control — the same corpus, spelled as a NAME, has to
    answer all-clear, or the refusals above are refusing everything.
    """
    store = "beadqueue"
    reader = "cd ~/src/beadqueue && git status -sb | head -1"
    writer = "cd ~/src/beadqueue && git commit -m x -- f"
    # NAMES the store and aims no git verb at it. This is the shape that kept
    # `seen` looking healthy while both verb sets were empty, so it is the
    # shape the denominator rule has to catch.
    namer = 'cd ~/src/beadqueue && bd comments add x "... git add . ..."'
    arms = []
    # The prefix is posse's temp namespace and not a convenience: a root
    # outside `posse-*` is collected by no class in scripts/reap-test-tmp.sh
    # at any age, and this box's /tmp has hit 100% of its inodes
    # (internal/treepins/temproot_qa_test.go).
    with tempfile.TemporaryDirectory(prefix="posse-store-git-write-census.") as tmp:
        roots = {}
        for name, cmds in (("read", [reader]), ("write", [reader, writer]), ("quiet", [namer])):
            roots[name] = plant_corpus(os.path.join(tmp, name), cmds)

        if PLANTED_SEAT_DIR == operator_dir():
            print("ARM DID NOT RUN: the planted session dir is this box's operator dir", file=out)
            return 1, 1

        def answer(root, arg_store):
            buf = io.StringIO()
            rc = report(root, arg_store, buf)
            return rc, buf.getvalue()

        def run_main(argv):
            buf = io.StringIO()
            err = io.StringIO()
            with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(err):
                rc = main(argv)
            return rc, buf.getvalue(), err.getvalue()

        rc, said = answer(roots["read"], store)
        arms.append(("a corpus of seat reads answers all-clear", (rc, "has ever typed a git write" in said), (0, True)))

        rc, said = answer(roots["write"], store)
        arms.append(("a corpus with one seat write exits 1", (rc, "1 seat git write(s)" in said), (1, True)))

        # The finding itself: an empty read denominator must not answer.
        rc, said = answer(roots["quiet"], store)
        arms.append((
            "an empty read denominator refuses the all-clear",
            (rc, "NOT PROVEN" in said, "has ever typed a git write" in said),
            (2, True, False),
        ))

        # The argument. A path matches no component, so it reads the quiet
        # corpus out of ANY corpus — this one has a read in it.
        rc, said, cried = run_main(["--store", "/home/u/src/" + store, "--root", roots["read"]])
        arms.append((
            "--store as a path is refused before it reads anything",
            (rc, "ANSWER" in said, "not a path" in cried),
            (2, False, True),
        ))

        # Blindness control for the two refusals above.
        rc, said, _ = run_main(["--store", store, "--root", roots["read"]])
        arms.append((
            "control: the same corpus, --store as a NAME, answers all-clear",
            (rc, "has ever typed a git write" in said),
            (0, True),
        ))

    bad = 0
    for label, got, want in arms:
        if got != want:
            bad += 1
            print(f"ARM FAILED: {label}\n  got  {got}\n  want {want}", file=out)
    return bad, len(arms)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--root", default=DEFAULT_ROOT, help="transcript root")
    ap.add_argument("--store", default="", help="the store repo's directory NAME (not a path)")
    ap.add_argument("--self-test", action="store_true", help="run the classifier's own arms and exit")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test(sys.stdout)
    # Asked for here rather than by argparse, so --self-test needs no store:
    # the classifier's arms carry their own, and a DEFAULT store would name
    # one instance's repo in a public tree (ADR 0024 D2).
    if not args.store:
        print("--store <dir name> is required: the store repo's directory name, not a path", file=sys.stderr)
        return 2
    # A PATH IS REFUSED, NOT MATCHED AND MISSED (ranger-base-7c2py). The store
    # is matched as a path COMPONENT, so a path matches no component, drops
    # every git verb out of the classification and used to answer all-clear
    # over nothing. The message names the spelling that works rather than
    # only the one that does not.
    if "/" in args.store or os.sep in args.store:
        name = os.path.basename(args.store.rstrip("/" + os.sep)) or args.store
        print(
            f"--store takes the store repo's directory NAME, not a path: {args.store}\n"
            f"    it is matched as a path COMPONENT, so a path matches nothing — try: --store {name}",
            file=sys.stderr,
        )
        return 2
    if not os.path.isdir(args.root):
        print(f"no transcripts under {args.root}", file=sys.stderr)
        return 2
    return report(args.root, args.store, sys.stdout)


if __name__ == "__main__":
    sys.exit(main())
