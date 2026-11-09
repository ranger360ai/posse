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

THE KEYS, and why the shipped one is the MIN.

  MIN (what verifyChainRounds counts since 2026-11-09, ranger-base-rffxq): a
  close was filed `discovered-from` a verify bead; that bead answered one
  close or N; and the count is one round plus the FEWEST any of those closes
  has spent. Every one of the N is a close the findings bundle could have
  been about, so the smallest is a lower bound on the true count, and short
  is the direction the harness errs in. Over a batch filed like with like —
  verify-after groups its pending closes by the rounds they have spent — all
  N have the same count and the minimum is exact.

  LINEAGE (what it counted before that): the same walk, except that it
  STOPPED at a verify bead answering more than one close rather than guess
  which of the N to continue through. At `verify_batch: 1` that cost one
  round. Above it every verify bead is batched, so every walk counted exactly
  one round however deep the lineage ran and no `verify_chain_max:` could
  fire at all — the defect ranger-base-bu8rz was filed for. It is still
  measured here because the amendment rests on the two keys agreeing about
  the closes that reach the bound.

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
  continue through all N closes of a batched verify bead and take the
  DEEPEST chain, with no artifact test to say which of the N this round
  answered. It is the obvious way to make the bound reachable at a
  `verify_batch:` above 1 — the MIN key above is the shipped one, and the
  two differ in that one word — and it fails for the PATH key's reason one
  layer up: a batch is no more an artifact than a hot file is. This half
  needs no repo and always runs, so the comparison that rejected it is
  re-runnable too. Read its deepest chains by eye; they are not one subject.

The REACHABILITY line each report prints is the question the shipped walk's
own census does not answer: how many closes a given `verify_chain_max:`
would escalate under that key. The lineage key answered 0 above a batch of 1,
which is the defect ranger-base-bu8rz was filed for; the MIN key is what
answers it now.

`--replay` prints the other half of the amendment's cost: what the GROUPING
does to the filing. It replays every close a verify bead answers, in close
order, at `verify_batch:` 1 and 4 with a 24h age, under four ways of grouping
the pending closes — one group, grouped by rounds spent (what ships), round 0
against round 1-or-more, and each deep close on its own — and prints the
verify beads each files and how long its closes waited.

Usage:
    scripts/chain-depth-census.py [issues.jsonl] [repo-with-the-commits]
    scripts/chain-depth-census.py [issues.jsonl] --replay [--since=YYYY-MM-DD]
"""

import collections
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timedelta

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

    The MIN key is the harness's own walk: at a batched verify bead it
    continues through the SHALLOWEST of the closes that bead answers, so the
    count is one round plus the smallest. The LINEAGE key is what it did
    before — one forced hop at a time, stopping at a batched verify bead. The
    PATH and FAN-OUT keys branch the other way and take the DEEPEST chain:
    PATH requires a shared commit path, which is what it claims disambiguates
    the batch; FAN-OUT requires nothing, which is why it fuses unrelated
    subjects. Each key is that proposal as proposed, not a liberty taken
    with it.
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
    if key == "min":
        best = None
        for src in srcs:
            if src in seen:
                continue
            cand = [(q, src)] + depth(beads, par, paths, src, key, seen | {src})
            if best is None or len(cand) < len(best):
                best = cand
            if len(best) == 1:
                break                    # a source with no round above it
        # No source this walk can continue through: the round counts and the
        # lineage ends at it, exactly as the harness reads it.
        return best if best is not None else [(q, None)]
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
            t = (beads.get(src, {}).get("title") or "") if src else "(no close of its own to continue through)"
            print(f"        verify {q} of {src or '-'}: {t[:66]}")
    return hist


def close_times(beads, cands):
    """(closed_at, id) for every candidate the store dates, in close order."""
    out = []
    for i in cands:
        b = beads.get(i) or {}
        if b.get("closed_at"):
            out.append((parse_time(b["closed_at"]), i))
    out.sort()
    return out


def parse_time(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00")[:19])


def replay(cl, n, age, key, n_for=None):
    """One filing schedule: verify beads filed, and the hours closes waited.

    Event-driven, as a dispatch pass is: a group that reaches its batch size
    files at once, and a partial group is held until its OLDEST close reaches
    `age` and then files as it stands (DefaultVerifyBatchAge). `key` is what
    verify-after groups by; `n_for` gives a group its own batch size, which
    is what "deep unbatched" needs — a strategy that files each deep close on
    its own must file it on ARRIVAL, not wait 24h for a fifth close that the
    key cannot produce. (That flaw is in the sim.py this is ported from,
    docs/notes.d/ranger-base-rffxq.md: its deep-unbatched waits are wrong and
    its counts are right.)
    """
    pend = collections.defaultdict(list)
    filed, waits = 0, []
    size = n_for or (lambda k: n)
    for t, i in cl + [(cl[-1][0] + timedelta(days=30), None)]:
        for k in list(pend):
            while pend[k] and t - pend[k][0][0] >= age:
                flush = pend[k][0][0] + age
                g, pend[k] = pend[k][:size(k)], pend[k][size(k):]
                filed += 1
                waits += [(flush - x[0]).total_seconds() / 3600 for x in g]
        if i is None:
            break
        k = key(i)
        pend[k].append((t, i))
        if len(pend[k]) >= size(k):
            g, pend[k] = pend[k][:size(k)], pend[k][size(k):]
            filed += 1
            waits += [(t - x[0]).total_seconds() / 3600 for x in g]
    waits.sort()
    return filed, waits[len(waits) // 2], waits[int(len(waits) * 0.9)]


def replay_report(beads, par, since):
    """What the GROUPING costs: the filing, under four ways to group.

    The grouping is the other half of the bound (ADR 0006 §3, amended
    ranger-base-rffxq): without it a batch mixes lineage depths, the
    shallowest-source walk reads the shallowest member's count for all of
    them, and no bound fires. What it costs is verify beads and wait, and
    this is where that is read.
    """
    mixed = homog = 0
    byround = collections.Counter()
    for q in beads:
        if not is_verify(beads, q):
            continue
        srcs = [s for s in verify_sources(beads, q) if s in beads]
        if len(srcs) < 2:
            continue
        ds = {len(depth(beads, par, {}, s, "min")) for s in srcs}
        if len(ds) > 1:
            mixed += 1
        else:
            homog += 1
            byround[next(iter(ds))] += 1
    print(f"\n=== batched verify beads: homogeneous {homog} {dict(byround)}, mixed {mixed}")
    print("   (a mixed batch is one whose closes have spent DIFFERENT rounds: the")
    print("    shape the grouping exists to stop, and the shape that made the")
    print("    shallowest-source walk read shallow before it)")

    cands = set()
    for q in beads:
        if is_verify(beads, q):
            cands.update(verify_sources(beads, q))
    cl = [x for x in close_times(beads, cands) if x[0].isoformat() >= since]
    if not cl:
        print(f"   no closes a verify bead answers, closed at or after {since}")
        return
    d = {i: len(depth(beads, par, {}, i, "min")) for _, i in cl}
    print(f"\n=== replay: {len(cl)} closes a verify bead answers, since {since}")
    print(f"   rounds already spent (MIN key): {dict(sorted(collections.Counter(d.values()).items()))}")
    age = timedelta(hours=24)
    strategies = (
        ("one group", lambda i: 0, None),
        ("by rounds (SHIPPED)", lambda i: d[i], None),
        ("round 0 vs >=1", lambda i: min(d[i], 1), None),
        ("deep unbatched", lambda i: 0 if d[i] == 0 else ("solo", i),
         lambda k: None if k == 0 else 1),
    )
    for n in (1, 4):
        for name, key, solo in strategies:
            n_for = (lambda k: n if solo(k) is None else solo(k)) if solo else None
            filed, med, p90 = replay(cl, n, age, key, n_for)
            print(f"   verify_batch: {n}  {name:20s} verify beads {filed:5d}  wait h median {med:5.1f} p90 {p90:5.1f}")
        print()


def main():
    argv = [a for a in sys.argv if not a.startswith("--")]
    flags = [a for a in sys.argv if a.startswith("--")]
    since = "2000"
    for f in flags:
        if f.startswith("--since="):
            since = f.split("=", 1)[1]
        elif f != "--replay":
            sys.exit(f"unknown flag {f}; see the head of this file")
    path = store_path(argv)
    repo = argv[2] if len(argv) > 2 else ""
    beads = load(path)
    par = parents(beads)
    print(f"store: {path}\nbeads: {len(beads)}, {len(par)} of them with a discovered-from parent")
    if "--replay" in flags:
        replay_report(beads, par, since)
        return
    report("MIN key (what the harness counts)", beads, par, {}, "min")
    report("LINEAGE key (the single-close walk MIN replaced)", beads, par, {}, "lineage")
    report("FAN-OUT key (the batch repair, measured and rejected)", beads, par, {}, "fanout")
    if repo:
        paths = commit_paths(repo, set(beads))
        print(f"\ncommit trails: {len(paths)} beads are named by at least one commit in {repo}")
        report("PATH key (measured and rejected)", beads, par, paths, "path")
    else:
        print("\n(no repo given, so the PATH key half was not measured — pass it as argument 2)")


if __name__ == "__main__":
    main()
