#!/usr/bin/env python3
"""How every full suite on this box was started: the queue's coverage, measured.

WHY THIS EXISTS (ranger-base-uvzjk). scripts/suite-lock.sh serializes full
`go test ./...` runs across crew worktrees, and it can only serialize the runs
that go through a wrapper that calls it — `make test`, scripts/test-times.sh,
scripts/gotest.sh. A bare `go test ./...` typed at a seat bypasses the queue
entirely, and three of the five concurrent suites in the 2026-09-04 incident
were exactly that. So the question "is the queue worth anything" is a question
about how the crew actually starts a suite, and that is measurable rather than
arguable. This is the instrument. Re-run it; the corpus grows.

WHAT IT READS. `~/.claude/projects/*/*.jsonl` — every session transcript on
the box, not just the caller's. Each Bash `tool_use` carries the verbatim
command and an ISO-8601 UTC stamp. No `ps`, no unified log, no root, none of
which a caged seat can reach anyway.

THE THREE THINGS IT HAD TO GET RIGHT, each of which was wrong in a draft and
each of which changed the answer:

  1. SEGMENTS, NOT LINES. `go build ./... && go test -count=1 ./` is one line
     carrying a `./...` that belongs to the BUILD. Counting per line scored it
     as a full suite; it is not one. The line is split on the separators a
     shell would split on and each segment is classified by its own head word.

  2. A `-run` FILTER IS NOT A SUITE. `go test -run TestFoo ./...` walks every
     package and runs almost nothing; it costs seconds, and suite-lock.sh
     deliberately does not queue it. Counting it would inflate both the
     problem and the queue's coverage.

  3. THE TREND IS THE ANSWER, NOT THE TOTAL. `make test` became the house
     command partway through this corpus. The lifetime split is dominated by
     the weeks before that and understates today's coverage by a factor of
     three. Both are printed, and the recent window is the one that bears on
     whether the queue reaches the crew.

WHAT COUNTS AS A RUN. An unfiltered package TREE — `./...`, `all`, or a
subtree like `./internal/posse/...` — which is exactly what
suite_lock_wanted() queues, so this instrument and the thing it reports on
read an argv the same way. The whole-module and subtree halves are printed
separately because that split is what would decide a narrower queue: measured
here, subtrees are 211 of 1851 runs (11%), and 145 of those 211 are
`./internal/posse/...`, `./internal/rhq/...` or `./internal/...` — the
expensive half of the suite under another name. A queue that let those through
would let most of the load through.

MEASURED 2026-09-04 over 91,710 Bash calls / 1,554 transcripts / 2026-06-21 to
2026-09-04: 1851 tree runs, 22% through a wrapper the queue reaches and 78%
bypassing it — but the corpus predates `make test` becoming the house command,
and over the last three days it is 75% reached, 25% bypassing. (These move.
Re-run rather than quote them at a later date.)

Usage: scripts/suite-entry-census.py [--days N] [--glob PATTERN]
"""

import argparse
import collections
import glob
import json
import re
import sys

# A command's separators are `\n ; && || | & ( )`, and `|` is included here
# and excluded from the pattern-kill census for opposite reasons: there a bare
# pipe was a grep's alternation, here `go test ./... | grep -E ...` is the
# house form and its head word is the thing being classified. They are matched
# by split_outside_quotes() rather than by a regex, because a regex splits
# inside quotes too — see the docstring there.
ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=\S*\s+")
PREFIX = ("env ", "time ", "nohup ", "command ")

# A FULL package tree, by the spellings go accepts. `./...`, `all`, and
# anything ending `/...` — the same reading suite_lock_wanted() makes, and it
# has to stay the same reading or this instrument measures a different thing
# from the one it is reporting on.
TREE = re.compile(r"(?:^|\s)(?:\./\.\.\.|\.\.\.|all|[\w./@-]+/\.\.\.)(?:\s|$)")
# ...and the whole-module half of it, reported separately because it is the
# number that would decide a narrower queue. See "WHAT COUNTS AS A RUN".
ROOT = re.compile(r"(?:^|\s)(?:\./\.\.\.|\.\.\.|all)(?:\s|$)")
FILTER = re.compile(r"-{1,2}(?:test\.)?run[= ]")

WRAPPERS = ("make test", "scripts/test-times.sh", "scripts/gotest.sh")


def split_outside_quotes(cmd):
    """Split on shell separators that are actually separators.

    A naive re.split() cuts inside quoted strings and heredoc bodies, and the
    fragments it makes there start with real-looking words: `bd comments add
    <id> "... go test ./... green ..."` yielded a `go test ./...` segment and
    was counted as a suite run. Measured on this corpus, that mistake added
    ~40 runs to the `bare go test` column, all of them prose ABOUT running
    the suite. So: track the quote state, and drop heredoc bodies, which are
    data whatever they look like."""
    out, buf = [], []
    i, n = 0, len(cmd)
    quote = None            # "'" or '"' while inside one
    heredoc = None          # the delimiter we are skipping to
    while i < n:
        c = cmd[i]
        if heredoc is not None:
            j = cmd.find("\n", i)
            line = cmd[i:] if j < 0 else cmd[i:j]
            if line.strip() == heredoc:
                heredoc = None
            i = n if j < 0 else j + 1
            continue
        if quote:
            if c == "\\" and quote == '"' and i + 1 < n:
                i += 2
                continue
            if c == quote:
                quote = None
            buf.append(c)
            i += 1
            continue
        if c == "\\" and i + 1 < n:
            i += 2
            continue
        if c in "'\"":
            quote = c
            buf.append(c)
            i += 1
            continue
        if cmd.startswith("<<", i):
            m = re.match(r"<<-?\s*([\"\']?)([A-Za-z_][A-Za-z0-9_]*)\1", cmd[i:])
            if m:
                heredoc = m.group(2)
                # the body starts on the next line; the rest of THIS line is
                # still part of the current segment
                j = cmd.find("\n", i)
                if j < 0:
                    break
                buf.append(cmd[i + m.end():j])
                i = j + 1
                out.append("".join(buf))
                buf = []
                continue
        if c in "\n;|&()":
            out.append("".join(buf))
            buf = []
            i += 2 if cmd.startswith(("&&", "||"), i) else 1
            continue
        buf.append(c)
        i += 1
    out.append("".join(buf))
    return out


def segments(cmd):
    """The commands a shell would run, one at a time, stripped of leading
    variable assignments and of the wrappers that do not change the verb."""
    for raw in split_outside_quotes(cmd):
        s = raw.strip()
        changed = True
        while changed and s:
            changed = False
            if ASSIGN.match(s):
                s = ASSIGN.sub("", s, count=1)
                changed = True
            for p in PREFIX:
                if s.startswith(p):
                    s = s[len(p):].lstrip()
                    changed = True
        if s:
            yield s


def entry_point(s):
    """Which door this segment came through, or None if it is not a test run."""
    if re.match(r"make\b", s) and re.search(r"(?:^|\s)test(?:\s|$)", s):
        return "make test"
    if re.search(r"(?:^|/)test-times\.sh\b", s):
        return None if "--self-test" in s else "scripts/test-times.sh"
    if re.search(r"(?:^|/)gotest\.sh\b", s):
        return None if ("--self-test" in s or "--prune" in s) else "scripts/gotest.sh"
    if re.match(r"go\s+test\b", s):
        return "bare go test"
    return None


def is_full_suite(kind, s):
    """A full, unfiltered package tree. `make test` is one by definition: its
    recipe carries `./...` and this census is about the door, not the flag."""
    if FILTER.search(s):
        return False
    return kind == "make test" or bool(TREE.search(s))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--days", type=int, default=7,
                    help="size of the recent window reported beside the total")
    ap.add_argument("--glob", default="~/.claude/projects/*/*.jsonl")
    args = ap.parse_args()

    import os
    pattern = os.path.expanduser(args.glob)

    runs = []          # (day, entry point, "root" | "subtree")
    sessions = collections.defaultdict(set)
    n_bash = 0
    seen_files = 0
    corpus_days = set()   # every day the corpus covers, run or no run
    for path in glob.glob(pattern):
        seen_files += 1
        try:
            fh = open(path, encoding="utf-8", errors="replace")
        except OSError:
            continue
        with fh:
            for raw in fh:
                if '"Bash"' not in raw:
                    continue
                try:
                    rec = json.loads(raw)
                except Exception:
                    continue
                msg = rec.get("message") or {}
                content = msg.get("content")
                if not isinstance(content, list):
                    continue
                day = (rec.get("timestamp") or "")[:10]
                for blk in content:
                    if not isinstance(blk, dict) or blk.get("name") != "Bash":
                        continue
                    cmd = (blk.get("input") or {}).get("command")
                    if not isinstance(cmd, str):
                        continue
                    n_bash += 1
                    if day:
                        corpus_days.add(day)
                    for s in segments(cmd):
                        kind = entry_point(s)
                        if kind and is_full_suite(kind, s):
                            scope = "root" if (kind == "make test" or ROOT.search(s)) else "subtree"
                            runs.append((day, kind, scope))
                            sessions[kind].add(path)

    if not seen_files:
        print(f"no transcripts matched {pattern} — this census measured nothing",
              file=sys.stderr)
        return 2
    if not runs:
        print(f"{n_bash} Bash calls in {seen_files} transcripts, and not one full "
              f"suite run — either the corpus is wrong or the classifier is",
              file=sys.stderr)
        return 2

    days = sorted({d for d, _, _ in runs if d})
    recent = set(days[-args.days:])

    def table(rows, title):
        counts = collections.Counter(k for _, k, _ in rows)
        scopes = collections.Counter(sc for _, _, sc in rows)
        total = sum(counts.values())
        print(f"\n{title} — {total} unfiltered package-tree runs "
              f"({scopes['root']} whole module, {scopes['subtree']} a subtree)")
        for k, n in counts.most_common():
            print(f"  {n:6d}  {100 * n / total:5.1f}%  {k}")
        through = sum(counts[w] for w in WRAPPERS)
        print(f"  through a wrapper the queue reaches: {through} "
              f"({100 * through / total:.0f}%)")
        print(f"  bypassing it entirely:               {counts['bare go test']} "
              f"({100 * counts['bare go test'] / total:.0f}%)")

    span = f"{min(corpus_days)} to {max(corpus_days)}" if corpus_days else "undated"
    print(f"{n_bash} Bash calls in {seen_files} transcripts, {span}")
    table(runs, "WHOLE CORPUS")
    table([r for r in runs if r[0] in recent],
          f"LAST {args.days} DAYS WITH ANY RUN ({min(recent)} to {max(recent)})")

    print("\nby day:")
    per = collections.defaultdict(collections.Counter)
    for d, k, _ in runs:
        per[d][k] += 1
    for d in days[-args.days:]:
        r = per[d]
        print(f"  {d}  bare={r['bare go test']:4d}  make={r['make test']:4d}  "
              f"test-times={r['scripts/test-times.sh']:3d}  "
              f"gotest={r['scripts/gotest.sh']:3d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
