#!/usr/bin/env python3
"""How deep do verify/fix chains run: a census of one bead store's lineages.

WHY THIS EXISTS (ranger-base-ewnas). verify-after files a verify bead for
every configured close (ADR 0006 §3); the verifier files one findings bundle
from it (§6); the fix closes; verify-after files a bead about THAT close.
Nothing in that cycle counts the rounds. One project crew ran about twenty
rounds on a single config-test pin before a human stopped it, and every
round was locally reasonable. `verify_chain_max:` bounds the depth, and this
is the instrument that says what the depth IS — before the bound, to pick
its default, and after, to see whether it still fits.

WHAT IT READS. One store's `issues.jsonl`: the path given as the first
argument, else $BEADS_DIR/issues.jsonl, else ./.beads/issues.jsonl. It never
calls bd, so it cannot hand bd a store-selecting flag (AGENTS.md's rule
about reading another repo's queue), and it never writes.

THE TWO KEYS, and why the shipped one is the lineage.

  LINEAGE (what verifyChainRounds counts): a close was filed
  `discovered-from` a verify bead, and that verify bead answered the close
  before it. A verify bead answering N closes — `verify_batch: N` — carries
  N markers and the walk cannot know which of the N the findings bundle was
  about, so this census stops there exactly as the harness does.

  PATH (the artifact key ranger-base-ewnas proposed, measured and rejected):
  a round continues only when the two closes' commits touch a path in common
  — and a batched verify bead is no longer ambiguous, because the path says
  which of its N closes this round answered, which is the whole appeal of
  the key. Needs the repo, so pass it as the second argument to get this
  half; the trail is every commit whose subject or body names a bead this
  store holds, taken in one pass over the log.

The path key reads a hot file as an artifact. In a tree where most closes
touch one or two large files, it chains unrelated subjects together and
reports depths nothing on the queue has ever worked at. Run both halves and
compare the deepest lineages by eye: that is the comparison the rejection
rests on, and it is re-runnable rather than asserted.

  FAN-OUT (the batch repair, measured and rejected under ranger-base-bu8rz):
  continue through ALL N closes of a batched verify bead and take the
  deepest chain, with no artifact test to say which of the N this round
  answered. It is the obvious way to make the bound reachable at a
  `verify_batch:` above 1 — see verifyChainBound, which instead turns the
  bound off there — and it fails for the PATH key's reason one layer up: a
  batch is no more an artifact than a hot file is. This half needs no repo
  and always runs, so the comparison that rejected it is re-runnable too.
  Read its deepest chains by eye; they are not one subject.

The REACHABILITY line each report prints is the question the shipped walk's
own census does not answer: how many closes a given `verify_chain_max:`
would escalate under that key. The lineage key answers 0 above a batch of 1,
which is the defect ranger-base-bu8rz was filed for.

Usage:
    scripts/chain-depth-census.py [issues.jsonl] [repo-with-the-commits]
"""

import collections
import json
import os
import re
import subprocess
import sys

MARKER = "Verify the close of "
MARKER_AFTER = " ("
BEAD_ID = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)+$")
TOKEN = re.compile(r"\b[a-z][a-z0-9]*(?:-[a-z0-9]+)+\b")


def store_path(argv):
    if len(argv) > 1 and argv[1]:
        return argv[1]
    d = os.environ.get("BEADS_DIR") or os.path.join(os.getcwd(), ".beads")
    return os.path.join(d, "issues.jsonl")


def load(path):
    beads = {}
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            d = json.loads(line)
            if d.get("id"):
                beads[d["id"]] = d
    return beads


def parents(beads):
    """id -> its discovered-from parents, in order: the handoff edge ADR 0006 §1 names."""
    out = collections.defaultdict(list)
    for b in beads.values():
        for dep in b.get("dependencies") or []:
            if dep.get("type") == "discovered-from" and dep.get("depends_on_id"):
                out[b["id"]].append(dep["depends_on_id"])
    return out


def verify_parent(beads, par, i, seen):
    """The first discovered-from parent of i that is a verify bead, as the harness picks it.

    Not simply the first parent: that makes the count depend on the order bd
    lists edges in, and one bead in the census store carries two parents with
    the verify bead second.
    """
    for p in par.get(i, ()):
        if p not in seen and is_verify(beads, p):
            return p
    return None


def is_verify(beads, i):
    return "qa" in (beads.get(i, {}).get("labels") or [])


def verify_sources(beads, q):
    """The closes a verify bead's description says it answers, one marker per line."""
    out = []
    for line in (beads.get(q, {}).get("description") or "").split("\n"):
        if not line.startswith(MARKER):
            continue
        rest = line[len(MARKER):]
        i = rest.find(MARKER_AFTER)
        if i > 0 and BEAD_ID.match(rest[:i]):
            out.append(rest[:i])
    return out


def commit_paths(repo, known):
    """bead id -> the paths of every commit whose message names it.

    A message token counts only when the store HOLDS that id: `--name-only`
    bodies are full of hyphenated words, and a loose match hands paths to
    ids that are not beads at all.
    """
    if not repo:
        return {}
    out = collections.defaultdict(set)
    log = subprocess.run(
        ["git", "-C", repo, "log", "--no-merges", "--format=\x01%H\x02%s %b", "--name-only"],
        capture_output=True, text=True, check=True).stdout
    ids = []
    for line in log.split("\n"):
        if line.startswith("\x01"):
            ids = [i for i in TOKEN.findall(line.split("\x02", 1)[-1]) if i in known]
            continue
        p = line.strip()
        if p:
            for i in ids:
                out[i].add(p)
    return out


def depth(beads, par, paths, leaf, key, seen=None):
    """Verify rounds already spent on leaf's lineage, newest first.

    The LINEAGE key walks one forced hop at a time and stops at a batched
    verify bead, which is the harness's own walk. The PATH and FAN-OUT keys
    branch: a batch's N closes are all candidates and the deepest chain
    through any of them is the answer. PATH then requires a shared commit
    path, which is what it claims disambiguates the batch; FAN-OUT requires
    nothing, which is why it fuses unrelated subjects. That branching is each
    proposal as proposed, not a liberty taken with it.
    """
    if seen is None:
        seen = {leaf}
    q = verify_parent(beads, par, leaf, seen)
    if q is None:
        return []
    seen = seen | {q}
    srcs = verify_sources(beads, q)
    if key == "lineage":
        if len(srcs) != 1:
            return [(q, None)]          # a batch: count the round, stop the walk
        src = srcs[0]
        if src in seen:
            return [(q, src)]
        return [(q, src)] + depth(beads, par, paths, src, key, seen | {src})
    best = [(q, None)]
    for src in srcs:
        if src in seen:
            continue
        if key == "path" and paths.get(leaf) and paths.get(src) and not (paths[leaf] & paths[src]):
            continue                     # a different artifact: not this chain
        cand = [(q, src)] + depth(beads, par, paths, src, key, seen | {src})
        if len(cand) > len(best):
            best = cand
    return best


def report(label, beads, par, paths, key):
    rows = []
    for i in beads:
        if is_verify(beads, i):
            continue
        rows.append((len(depth(beads, par, paths, i, key)), i))
    hist = collections.Counter(d for d, _ in rows)
    print(f"\n=== {label}: rounds already spent, over {len(rows)} non-verify beads")
    for k in sorted(hist):
        print(f"   {k} round(s): {hist[k]:5d}  ({100.0 * hist[k] / len(rows):.1f}%)")
    # REACHABILITY: what a configured bound would actually escalate under this
    # key. 0 at the shipped default is not a quiet queue, it is a bound that
    # cannot fire (ranger-base-bu8rz).
    for bound in (3, 5, 8):
        n = sum(1 for d, _ in rows if d >= bound)
        print(f"   at verify_chain_max: {bound} this key escalates {n} close(s) ({100.0 * n / len(rows):.2f}%)")
    rows.sort(reverse=True)
    print(f"--- deepest {min(5, len(rows))}, by eye: is each one SUBJECT?")
    for d, i in rows[:5]:
        print(f"  depth={d} {i} — {(beads[i].get('title') or '')[:78]}")
        for q, src in depth(beads, par, paths, i, key):
            t = (beads.get(src, {}).get("title") or "") if src else "(a batch of closes)"
            print(f"        verify {q} of {src or '-'}: {t[:66]}")
    return hist


def main():
    path = store_path(sys.argv)
    repo = sys.argv[2] if len(sys.argv) > 2 else ""
    beads = load(path)
    par = parents(beads)
    print(f"store: {path}\nbeads: {len(beads)}, {len(par)} of them with a discovered-from parent")
    report("LINEAGE key (what the harness counts)", beads, par, {}, "lineage")
    report("FAN-OUT key (the batch repair, measured and rejected)", beads, par, {}, "fanout")
    if repo:
        paths = commit_paths(repo, set(beads))
        print(f"\ncommit trails: {len(paths)} beads are named by at least one commit in {repo}")
        report("PATH key (measured and rejected)", beads, par, paths, "path")
    else:
        print("\n(no repo given, so the PATH key half was not measured — pass it as argument 2)")


if __name__ == "__main__":
    main()
