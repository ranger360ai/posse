#!/usr/bin/env python3
"""How long a dispatch --watch pass takes on this box: the distribution, measured.

WHY THIS EXISTS (ranger-base-rb05v, ADR 0063). The operator asked for "a
watchdog that acts, not logs". A threshold that ACTS has to clear the slowest
healthy pass this shop actually produces, and nobody had measured that
distribution — the pass budget (2 x (max-interval + gather window), 12m at
3m/3m) was derived from config, not from passes. This is the instrument.
Re-run it; the corpus grows. ADR 0063 D4 names the observation that reopens
the decision, and this script is half of the recipe.

WHAT IT READS. Both generations of $RHQ_HOME/state/dispatch-watch.log (the
live file and the .1 the autostart hook rotates to), or the paths given. No
ps, no unified log, no root, nothing a caged seat cannot reach.

WHAT IT CAN AND CANNOT KNOW. Only two lines in the log carry a time: the
generation banner and the pass header. So for each pass with a successor:

  U = next header - this header = the pass PLUS the wait after it. This is
      what the pass-stall watchdog reads (notePass to notePass, shifted by
      one pass), so the budget is compared against U.
  L = U - the printed "next pass in" wait = the pass itself. Exact when the
      wait ran out; a LOWER BOUND when a carried leg's settle woke the loop
      early (watch.go, the settled() arm), because the real wait was shorter.

A rotated file whose banner rotated away gets a dummy date and is kept.
Midnight is a header reading earlier than its predecessor. The last pass of
every generation has no successor and is excluded — including every pass
that hung, which is why this census measures healthy passes and the log's
watchdog lines measure the other kind.

A LONG PASS IS NOT NECESSARILY A SLOW PASS. The 09-20 generation holds four
passes of 33-60m that were a lid-closed laptop (docs/notes.d/ranger-base-
rb05v.md §4); the tell is in the SAME log — the pulse and guard-clock lines
inside a pass are tickers, and a 2m pulse that printed four times in an hour
was not awake for the hour. --classify prints those counts beside the long
passes so the next reader can make that call without this file's author.
"""

import argparse
import datetime as dt
import os
import re
import sys
from collections import defaultdict

HDR = re.compile(r"^── pass (\d+) · (\d\d):(\d\d):(\d\d)")
GEN = re.compile(
    r"^== dispatch --watch armed (\d{4})-(\d\d)-(\d\d) (\d\d):(\d\d):(\d\d)(?: · pid (\d+))?"
)
NEXT = re.compile(r"next pass in (\S+) \(ctrl-c")
SUSPEND = "was SUSPENDED"
OVERLOAD = "over load_guard"
STALL = "watchdog: no pass has completed"


def parse_dur(s):
    m = re.fullmatch(r"(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?", s)
    if not m:
        return None
    h, mi, se = m.groups()
    return int(h or 0) * 3600 + int(mi or 0) * 60 + float(se or 0)


def fmt(s):
    s = int(round(s))
    return f"{s // 60}m{s % 60:02d}s"


def read(paths):
    rows = []
    for path in paths:
        date = dt.date(2000, 1, 1)
        gen = f"{os.path.basename(path)} head (banner rotated away, date unknown)"
        cur = None
        with open(path, errors="replace") as fh:
            for line in fh:
                m = GEN.match(line)
                if m:
                    y, mo, d, _h, _mi, _s, pid = m.groups()
                    date = dt.date(int(y), int(mo), int(d))
                    gen = f"{date} pid{pid}"
                    if cur:
                        rows.append(cur)
                        cur = None
                    continue
                m = HDR.match(line)
                if m:
                    n, h, mi, s = m.groups()
                    t = dt.datetime.combine(date, dt.time(int(h), int(mi), int(s)))
                    if cur:
                        if t < cur["t"]:
                            date = date + dt.timedelta(days=1)
                            t = t + dt.timedelta(days=1)
                        cur["next"] = t
                        rows.append(cur)
                    cur = {
                        "gen": gen, "n": int(n), "t": t, "next": None, "wait": None,
                        "pulse": 0, "guard": 0, "stall": 0, "suspend": 0,
                    }
                    continue
                if cur:
                    m = NEXT.search(line)
                    if m:
                        cur["wait"] = parse_dur(m.group(1))
                    if line.startswith("pulse:"):
                        cur["pulse"] += 1
                    if OVERLOAD in line:
                        cur["guard"] += 1
                    if STALL in line:
                        cur["stall"] += 1
                    if SUSPEND in line:
                        cur["suspend"] += 1
        if cur:
            rows.append(cur)
    return rows


def q(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(p * len(xs)))]


def stats(label, xs):
    print(f"  {label}: p50 {fmt(q(xs, .5))}  p90 {fmt(q(xs, .9))}  p95 {fmt(q(xs, .95))}"
          f"  p99 {fmt(q(xs, .99))}  max {fmt(q(xs, 1))}")


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("logs", nargs="*", help="watch logs, oldest first (default: $RHQ_HOME/state/dispatch-watch.log.1 and .log)")
    ap.add_argument("--budget", type=float, default=720, help="the pass clock's budget in seconds to count against (default 720 = 12m at 3m/3m)")
    ap.add_argument("--top", type=int, default=20, help="how many of the longest passes to list")
    ap.add_argument("--classify", action="store_true", help="beside each long pass, print the pulse/guard-clock/watchdog/suspend line counts inside it")
    args = ap.parse_args()
    paths = args.logs
    if not paths:
        home = os.environ.get("RHQ_HOME") or os.environ.get("POSSE_HOME") or os.path.expanduser("~/.config/posse")
        live = os.path.join(home, "state", "dispatch-watch.log")
        paths = [p for p in (live + ".1", live) if os.path.exists(p)]
    if not paths:
        print("no watch log found; pass the paths", file=sys.stderr)
        return 2
    rows = read(paths)
    complete = [r for r in rows if r["next"] and r["wait"] is not None]
    if not complete:
        print("no pass with a successor and a wait line in", paths, file=sys.stderr)
        return 2
    for r in complete:
        r["U"] = (r["next"] - r["t"]).total_seconds()
        r["L"] = max(r["U"] - r["wait"], 0.0)
    U = [r["U"] for r in complete]
    L = [r["L"] for r in complete]
    gens = len({r["gen"] for r in rows})
    print(f"{len(complete)} passes with a successor and a wait line, of {len(rows)} headers in {gens} generations")
    print("U = header to header = pass + wait (what the pass clock reads); L = U - printed wait = the pass itself (exact, or a lower bound after a settle wake)")
    stats("U", U)
    stats("L", L)
    b = args.budget
    print(f"  U > budget ({fmt(b)}): {sum(1 for x in U if x > b)}   U > 2x budget: {sum(1 for x in U if x > 2 * b)}   L > budget: {sum(1 for x in L if x > b)}")
    print("  waits seen:", ", ".join(sorted({fmt(r['wait']) for r in complete})))
    print(f"\nTOP {args.top} by L:")
    for r in sorted(complete, key=lambda r: -r["L"])[: args.top]:
        extra = ""
        if args.classify:
            extra = (f" · inside it: pulse {r['pulse']} guard {r['guard']} stall {r['stall']} suspend {r['suspend']}"
                     f" (a 2m pulse writes ~{int(r['U'] // 120)} ticks awake)")
        print(f"  {r['gen']} · pass {r['n']} · {r['t'].strftime('%m-%d %H:%M:%S')} · L {fmt(r['L'])} · U {fmt(r['U'])} · wait {fmt(r['wait'])}{extra}")
    print("\nPER GENERATION (passes · waits · max L · L over 6m / 9m / budget):")
    by = defaultdict(list)
    for r in complete:
        by[r["gen"]].append(r)
    for g, rs in by.items():
        waits = "/".join(sorted({fmt(r["wait"]) for r in rs}))
        print(f"  {g} · {len(rs)} · {waits} · {fmt(max(r['L'] for r in rs))} · "
              f"{sum(1 for r in rs if r['L'] > 360)} / {sum(1 for r in rs if r['L'] > 540)} / {sum(1 for r in rs if r['L'] > b)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
