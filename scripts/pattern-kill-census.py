#!/usr/bin/env python3
"""Who ended whose run: a census of pattern kills across every session on this box.

WHY THIS EXISTS (ranger-base-y2esu, evidence for ranger-base-6nx72).
`pkill -f` and `pgrep -f` match on argv across EVERY process on the machine.
Every session on this box runs a byte-identical suite argv — same Makefile
line, same script, same `go test -timeout 25m` — from a different worktree.
So a seat cleaning up its own run with a pattern ends every other seat's run
at the same instant, and the victim sees a bare `Terminated: 15` with no red,
no failing test, and nothing to bisect. ranger-base-6nx72 shipped the witness
(scripts/test-times.sh records the process table when it is signalled) and the
rule (AGENTS.md, INSTALL.md section 9). This is the instrument that measures
how often it happens, before or after either.

WHAT IT READS. `~/.claude/projects/*/*.jsonl` — every session transcript on
the box, not just the caller's. Each file carries, with ISO-8601 UTC stamps:
every Bash `tool_use` with its verbatim `command`, every `tool_result`, and
every `<task-notification>` (which carries the launching `<tool-use-id>`).
That is enough for cross-session forensics with no `ps`, no unified log and
no root — all three of which are refused inside a caged seat anyway.

THE THREE THINGS IT HAD TO GET RIGHT, each of which was wrong in a draft and
each of which changed the answer:

  1. A BACKGROUNDED RUN'S END IS ITS NOTIFICATION, NOT ITS tool_result. A
     `run_in_background` launch returns a task id at once, so pairing on
     tool_result alone dates every backgrounded suite as ~1s long and drops
     it from the population. It is the commonest shape for the long suite.
     Measured cost of the omission on this corpus: 3 confirmed kills became
     11, and the single best-documented occurrence (2026-09-02 21:28:33Z)
     read as having no victims at all.

  2. COMMAND POSITION, NOT WORD PRESENCE. `grep -rn 'SIGTERM|pkill|kill -'`
     — a CENSUS of the word — is not a kill, and neither is a heredoc that
     appends a runbook warning about pattern kills to an ORDERS.md. Counting
     mere presence made this script's own audit line a "confirmed hit". A
     match is only counted after a line start, `;`, `&`, `&&` or `||`. The
     separator that matters is the BARE `|`, and it is excluded: admitting it
     adds 11 lines to this corpus and every one of them is a grep or a
     heredoc whose alternation happens to list `pkill`. `(` is excluded too,
     though measured on the same corpus it changes nothing either way (144
     lines with or without it) — a real `(pkill ...)` subshell is a shape
     nobody on this box has typed.

  3. A HIT NEEDS A NULL. Runs end all day and busy hours cluster, so some
     sibling run ends within the window by coincidence. The same measurement
     is therefore re-run over kill times jittered inside +/- JITTER, and the
     observed count is reported against that distribution. Without it the
     headline number is unreadable: the all-targets population scores 18
     observed against a null mean of 4.4, but a draft that scored 6 observed
     against a null mean of 3.1 (p=0.08) would have supported the same
     sentence and been wrong to.

MEASURED 2026-09-03 over 85,451 Bash calls / 1,380 sessions / 2026-06-21 to
2026-09-03, window 10s, +/-3h jitter, 400 trials. (The corpus grows while you
work — these move. Re-run rather than quote them at a later date.)

    pkill/killall in command position                        144 lines
      of those, target NOT unique to the typing session      138 lines, 64 seats, 18 days
      of those, pattern could match a sibling's SUITE argv    35 lines, 28 seats,  8 days

    all non-unique kills:  18 of 138 had a sibling run end within 10s
                           null mean 4.28, max 10, p < 0.0025
                           16 killer seats, 7 personas, 22 victim sessions
    suite-pattern subset:  11 of  35 did, hitting 15 victim sessions
                           null mean 1.33, max  5, p < 0.0025
                           11 killer seats, 5 personas, over 5 days

Eleven confirmed cross-session suite kills, by eleven seats belonging to five
different personas, of runs up to 1,373s. It is not one persona's footgun.
Two of the eleven are the two occurrences ranger-base-6nx72 was filed about;
one pair three minutes apart is a kill and a counter-kill between two seats,
each ending the other's suite.

LIMITS, stated rather than discovered.
  * Transcripts see Claude sessions only. Anything the operator runs in their
    own terminal is invisible, so every count here is a LOWER bound.
  * A `tool_result` timestamp is when the RESULT was recorded, 1-2s after the
    process actually died. Hence a window of seconds, not milliseconds.
  * "Confirmed" means a sibling run ENDED inside the window after a kill that
    COULD match it. This reads argv text, not kernel signal delivery: it is
    strong circumstantial evidence, and the null says how strong.
  * A run under 60s is not counted as a run. Below that the population is
    dominated by builds and greps and the null swamps the signal.

USAGE
    scripts/pattern-kill-census.py                  # the whole corpus
    scripts/pattern-kill-census.py --days 7         # the last week
    scripts/pattern-kill-census.py --suite          # only suite-argv patterns
    scripts/pattern-kill-census.py --self-test      # graded on planted fixtures
"""
import argparse
import json
import os
import random
import re
import sys
import tempfile
from collections import defaultdict
from datetime import datetime, timedelta, timezone

DEFAULT_ROOT = os.path.expanduser("~/.claude/projects")
PAIR_CAP = timedelta(hours=6)          # a resumed session must not invent hours
MIN_RUN_SECONDS = 60                   # below this the null swamps the signal
DEFAULT_WINDOW = 10.0                  # seconds after a kill that count as a hit
DEFAULT_JITTER = 3 * 3600              # the null's displacement, seconds
DEFAULT_TRIALS = 400

# A kill only happens in command position. A bare `|` is NOT in the separator
# set: `grep -rn 'pkill|killall'` is a census, not a kill, and admitting `|`
# adds 11 such lines to this corpus and no real ones. See point 2 above.
SEP = r"(?:^|\n|;|&&|\|\||&)[ \t]*(?:sudo[ \t]+)?"
PKILL_CMD = re.compile(SEP + r"(pkill|killall)\b([^\n;&|]*)")
PKILL_ANY = re.compile(r"\b(pkill|killall)\b")

# What a seat LAUNCHING the suite looks like, and what WAITING on one looks
# like. Both end when the suite dies, so both are evidence, but they are
# labelled apart: a waiter proves the suite died, a launcher IS the suite.
LAUNCH_RE = re.compile(r"(?:^|\n|;|&&|\|\|)\s*(?:[A-Za-z_]+=\S+\s+)*(?:time\s+)?"
                       r"(?:make\s+(?:-\w+\s+)*test\b|\S*test-times\.sh|go\s+test\b)")
WAIT_RE = re.compile(r"\b(pgrep|kill -0|until |while \[|seq 1 \d+|tail -f)\b")

# A pattern is session-unique when it CANNOT match another seat's process: a
# pid, our own scratchpad or worktree path, a mktemp root, `-P $$`.
UNIQUE_HINT = re.compile(
    r"\$\$|\$ROOT\b|claude-\d+/|scratchpad|\.posse/worktrees|/private/tmp/claude|"
    r"XXXXXX|\$TMPDIR|\$S/|-P\s")

# A pattern that can match another seat's SUITE. This is the population the
# bead is about; `killall yes` dilutes it and is counted separately.
SUITE_TARGET = re.compile(r"go\s+test|test-times\.sh|make\s+test|"
                          r"\bposse\.test\b|\brhq\.test\b|/[a-z0-9_]+\.test\b")


def parse_ts(s):
    try:
        return datetime.fromisoformat(str(s).replace("Z", "+00:00")).astimezone(timezone.utc)
    except Exception:
        return None


def harvest(root):
    """Every Bash call on the box, with the interval its processes were alive."""
    calls = []
    if not os.path.isdir(root):
        return calls
    for proj in sorted(os.listdir(root)):
        pdir = os.path.join(root, proj)
        if not os.path.isdir(pdir):
            continue
        for fn in sorted(os.listdir(pdir)):
            if not fn.endswith(".jsonl"):
                continue
            pending = {}
            try:
                fh = open(os.path.join(pdir, fn), "r", errors="replace")
            except OSError:
                continue
            with fh:
                for ln in fh:
                    if ('"Bash"' not in ln and '"tool_result"' not in ln
                            and "task-notification" not in ln):
                        continue
                    try:
                        rec = json.loads(ln)
                    except Exception:
                        continue
                    when = parse_ts(rec.get("timestamp"))
                    if rec.get("type") == "queue-operation":
                        m = re.search(r"<tool-use-id>([^<]+)</tool-use-id>",
                                      rec.get("content") or "")
                        if m and when:
                            c = pending.get(m.group(1))
                            if c and when - c["t"] <= PAIR_CAP:
                                c["end"] = when
                                pending.pop(m.group(1), None)
                        continue
                    content = (rec.get("message") or {}).get("content")
                    if not isinstance(content, list):
                        continue
                    for blk in content:
                        if not isinstance(blk, dict):
                            continue
                        if blk.get("type") == "tool_use" and blk.get("name") == "Bash":
                            cmd = (blk.get("input") or {}).get("command")
                            if not isinstance(cmd, str) or not when:
                                continue
                            c = {"proj": proj, "sess": fn[:-6], "t": when, "end": None,
                                 "cmd": cmd,
                                 "bg": bool((blk.get("input") or {}).get("run_in_background"))}
                            calls.append(c)
                            if blk.get("id"):
                                pending[blk["id"]] = c
                        elif blk.get("type") == "tool_result":
                            tid = blk.get("tool_use_id")
                            c = pending.get(tid)
                            if c and c["bg"]:
                                continue  # its end is the notification, not this
                            pending.pop(tid, None)
                            if c and when and when - c["t"] <= PAIR_CAP:
                                c["end"] = when
    return calls


def runs_of(calls):
    out = []
    for c in calls:
        if not c["end"] or (c["end"] - c["t"]).total_seconds() < MIN_RUN_SECONDS:
            continue
        if LAUNCH_RE.search(c["cmd"]):
            out.append(dict(c, kind="launch"))
        elif WAIT_RE.search(c["cmd"]) and re.search(r"test|suite", c["cmd"]):
            out.append(dict(c, kind="wait"))
    return out


def kill_patterns(cmd):
    """(tool, flags, pattern) for every pkill/killall in command position."""
    out = []
    for m in PKILL_CMD.finditer(cmd):
        toks = re.findall(r"\"[^\"]*\"|'[^']*'|\S+", m.group(2).strip())
        pats = [t for t in toks if not t.startswith("-")]
        if not pats:
            continue
        flags = " ".join(t for t in toks if t.startswith("-"))
        out.append((m.group(1), flags, pats[0].strip("\"'")))
    return out


def hits_for(seat, when, runs, window):
    """Runs belonging to ANOTHER seat that ended in the window after a kill.

    The unit of exclusion is the seat (the project dir), not the session id.
    Resuming a session opens a NEW transcript file in the SAME project dir, so
    excluding by session id alone charges a seat with killing its own earlier
    run across a resume — which is a seat ending its own work, the one thing
    every seat is entitled to do. Measured on this corpus: session-level
    exclusion scores 11 confirmed suite-pattern kills, seat-level scores 11,
    so nothing here rests on it; it is right anyway and the self-test pins it.
    """
    return [r for r in runs
            if r["proj"] != seat and r["t"] <= when <= r["end"]
            and 0 <= (r["end"] - when).total_seconds() <= window]


def confirmed(kill_times, runs, window):
    n, victims = 0, set()
    for seat, when in kill_times:
        h = hits_for(seat, when, runs, window)
        if h:
            n += 1
            victims.update(r["sess"] for r in h)
    return n, victims


def null_distribution(kill_times, runs, window, jitter, trials, seed):
    rng = random.Random(seed)
    out = []
    for _ in range(trials):
        shifted = [(seat, t + timedelta(seconds=rng.uniform(-jitter, jitter)))
                   for seat, t in kill_times]
        out.append(confirmed(shifted, runs, window)[0])
    out.sort()
    return out


def one_line(cmd, n=170):
    return " ".join(cmd.split())[:n]


def report(args, out=sys.stdout):
    calls = harvest(args.root)
    if not calls:
        print(f"no transcripts under {args.root}", file=out)
        return 1
    if args.days:
        floor = max(c["t"] for c in calls) - timedelta(days=args.days)
        calls = [c for c in calls if c["t"] >= floor]
    runs = runs_of(calls)

    kills, prose = [], 0
    for c in calls:
        p = kill_patterns(c["cmd"])
        if p:
            kills.append((c, p))
        elif PKILL_ANY.search(c["cmd"]):
            prose += 1

    def nonunique(pats):
        return [p for p in pats if not UNIQUE_HINT.search(p[2])]

    pop = [(c, nonunique(p)) for c, p in kills if nonunique(p)]
    if args.suite:
        pop = [(c, [p for p in pats if SUITE_TARGET.search(p[2])])
               for c, pats in pop]
        pop = [(c, pats) for c, pats in pop if pats]

    print(f"corpus  {len(calls)} Bash calls / {len({c['sess'] for c in calls})} sessions"
          f" / {min(c['t'] for c in calls).date()}..{max(c['t'] for c in calls).date()}",
          file=out)
    print(f"runs    {sum(1 for r in runs if r['kind'] == 'launch')} suite launches + "
          f"{sum(1 for r in runs if r['kind'] == 'wait')} waits on one, "
          f"each over {MIN_RUN_SECONDS}s with a paired end", file=out)
    print(f"kills   {len(kills)} pkill/killall in command position "
          f"({prose} further lines only mention the word)", file=out)
    label = "can match a sibling's SUITE argv" if args.suite else "not unique to the typing session"
    print(f"        {len(pop)} whose target {label} — "
          f"{len({c['proj'] for c, _ in pop})} seats, "
          f"{len({c['t'].date() for c, _ in pop})} days", file=out)
    if not pop:
        return 0

    counts, seats = defaultdict(int), defaultdict(set)
    for c, pats in pop:
        for tool, flags, pat in pats:
            key = " ".join(x for x in (tool, flags, pat) if x)
            counts[key] += 1
            seats[key].add(c["proj"])
    print("\nmost-typed targets", file=out)
    for k, n in sorted(counts.items(), key=lambda kv: -kv[1])[:args.top]:
        print(f"  {n:4d}  {len(seats[k]):2d} seats   {k}", file=out)

    kt = [(c["proj"], c["t"]) for c, _ in pop]
    obs, victims = confirmed(kt, runs, args.window)
    null = null_distribution(kt, runs, args.window, args.jitter, args.trials, args.seed)
    ge = sum(1 for x in null if x >= obs)
    print(f"\nOBSERVED  {obs} of {len(kt)} kills had another session's run end "
          f"within {args.window:.0f}s ({len(victims)} distinct victim sessions)", file=out)
    print(f"NULL      times jittered +/-{args.jitter / 3600:.0f}h, {args.trials} trials: "
          f"mean {sum(null) / len(null):.2f}, median {null[len(null) // 2]}, "
          f"max {null[-1]}, p(null >= observed) = {ge / len(null):.4f}", file=out)

    print("\nthe confirmed lines", file=out)
    for c, pats in sorted(pop, key=lambda cp: cp[0]["t"]):
        h = hits_for(c["proj"], c["t"], runs, args.window)
        if not h:
            continue
        print(f"\n  {c['t'].isoformat()[:19]}Z  killer: {c['proj']}", file=out)
        print(f"    {one_line(c['cmd'])}", file=out)
        for r in h:
            print(f"    -> {r['kind']} in {r['proj']}: alive "
                  f"{(r['end'] - r['t']).total_seconds():.0f}s, ended "
                  f"+{(r['end'] - c['t']).total_seconds():.1f}s", file=out)
            print(f"       {one_line(r['cmd'], 140)}", file=out)
    return 0


# ---------------------------------------------------------------------------
# self-test — planted transcripts, and every arm carries the case that would
# read the same way if the arm were absent (a rig that cannot fail pins
# nothing). Run: scripts/pattern-kill-census.py --self-test
# ---------------------------------------------------------------------------
def _rec_use(tid, when, cmd, bg=False):
    return {"timestamp": when, "message": {"content": [
        {"type": "tool_use", "name": "Bash", "id": tid,
         "input": {"command": cmd, "run_in_background": bg}}]}}


def _rec_result(tid, when):
    return {"timestamp": when, "message": {"content": [
        {"type": "tool_result", "tool_use_id": tid}]}}


def _rec_notify(tid, when):
    return {"type": "queue-operation", "timestamp": when,
            "content": f"<task-notification>\n<tool-use-id>{tid}</tool-use-id>\n"
                       f"<status>completed</status>\n</task-notification>"}


def _write(path, recs):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as fh:
        for r in recs:
            fh.write(json.dumps(r) + "\n")


def self_test():
    ok = fail = 0

    def check(cond, what):
        nonlocal ok, fail
        if cond:
            ok += 1
            print(f"  ok    {what}")
        else:
            fail += 1
            print(f"  BAD   {what}")

    # --- unit arms: what counts as a kill, and what does not ---------------
    check(kill_patterns("pkill -f 'go test'") == [("pkill", "-f", "go test")],
          "a bare pkill is a kill")
    check(kill_patterns("cd /x && pkill -9 -f foo") == [("pkill", "-9 -f", "foo")],
          "a pkill after && is a kill")
    census = ("cd /x && grep -rn \"SIGTERM|syscall.Kill|pkill|kill -\" "
              "--include=*.go . | head -20")
    check(kill_patterns(census) == [],
          "a grep alternation listing pkill is NOT a kill — the bare `|` is "
          "not a separator (this exact line is what an earlier cut reported "
          "as a confirmed cross-session kill)")
    check(kill_patterns("grep -rnE '(pkill|killall)' ORDERS.md") == [],
          "nor is one in parentheses")
    check(kill_patterns("echo 'never pkill -f a suite pattern' >> ORDERS.md") == [],
          "prose inside a quoted string is NOT a kill")
    check(PKILL_ANY.search("grep -rnE '(pkill|killall)' ORDERS.md") is not None,
          "...and the loose matcher DOES see it, so the two really differ")
    check(bool(UNIQUE_HINT.search("/private/tmp/claude-501/x/scratchpad/load.sh")),
          "a scratchpad path is session-unique")
    check(not UNIQUE_HINT.search("go test -timeout 25m"),
          "the shared suite argv is NOT session-unique")
    check(bool(SUITE_TARGET.search("test-times.sh")) and not SUITE_TARGET.search("yes"),
          "the suite subset admits the suite script and excludes `killall yes`")

    # --- end-to-end arms over planted transcripts --------------------------
    root = tempfile.mkdtemp(prefix="pkcensus-selftest.")
    day = "2026-09-01T%s.000Z"
    # victim A: a FOREGROUND suite, 300s, ending 2s after the kill
    _write(os.path.join(root, "proj-victim-a", "sess-a.jsonl"), [
        _rec_use("t1", day % "12:00:00", "make test 2>&1 | tail -40"),
        _rec_result("t1", day % "12:05:02")])
    # victim B: a BACKGROUNDED suite, 600s, ending 1s after the kill. Its
    # tool_result lands immediately; only the notification dates it. Drop the
    # notification arm and this row vanishes.
    _write(os.path.join(root, "proj-victim-b", "sess-b.jsonl"), [
        _rec_use("t2", day % "11:55:00", "go test -timeout 25m ./... > /tmp/x.log", bg=True),
        _rec_result("t2", day % "11:55:01"),
        _rec_notify("t2", day % "12:05:01")])
    # the killer, plus a control kill 4h earlier naming its OWN scratchpad
    _write(os.path.join(root, "proj-killer", "sess-k.jsonl"), [
        _rec_use("t3", day % "08:00:00", "pkill -f /private/tmp/claude-501/k/scratchpad/fan.sh"),
        _rec_result("t3", day % "08:00:01"),
        _rec_use("t4", day % "12:05:00", "pkill -f 'go test -timeout 25m'; pkill -f 'make test'"),
        _rec_result("t4", day % "12:05:01")])

    calls = harvest(root)
    runs = runs_of(calls)
    check(len(calls) == 4, f"harvest found the first four planted calls (got {len(calls)})")
    bg = [c for c in calls if c["bg"]][0]
    check(bg["end"] is not None
          and abs((bg["end"] - bg["t"]).total_seconds() - 600) < 2,
          "a backgrounded run is dated by its notification (600s), not its "
          "instant tool_result (1s)")
    check(sorted(r["kind"] for r in runs) == ["launch", "launch"],
          f"both victims are runs (got {[r['kind'] for r in runs]})")

    # victim C ended 5s BEFORE the kill: a run already over is not a victim,
    # and only an arm that checks the SIGN of the interval says so.
    _write(os.path.join(root, "proj-victim-c", "sess-c.jsonl"), [
        _rec_use("t5", day % "11:59:00", "make test 2>&1 | tail -5"),
        _rec_result("t5", day % "12:04:55")])
    # the killer's OWN suite, ending in-window: ending your own run is the
    # thing every seat is entitled to do, and must not be counted.
    _write(os.path.join(root, "proj-killer", "sess-k2.jsonl"), [
        _rec_use("t6", day % "12:00:00", "make test 2>&1 | tail -40"),
        _rec_result("t6", day % "12:05:03")])
    # victim D: a 6s `go test` in another seat, ending 1s after the kill. Real
    # and matched by the pattern, but under the run floor.
    _write(os.path.join(root, "proj-victim-d", "sess-d.jsonl"), [
        _rec_use("t7", day % "12:04:55", "go test -timeout 25m ./internal/x"),
        _rec_result("t7", day % "12:05:01")])
    # victim E: a stale pair — the tool_result lands 8h later, which is what a
    # resumed session looks like. Unpaired, so not a run of any length.
    # victim F: a line that only NAMES the suite. Command position, or every
    # grep for "go test" becomes a 20-minute run.
    _write(os.path.join(root, "proj-victim-e", "sess-e.jsonl"), [
        _rec_use("t8", day % "12:04:00", "make test 2>&1 | tail -5"),
        _rec_result("t8", "2026-09-01T20:05:00.000Z")])
    _write(os.path.join(root, "proj-victim-f", "sess-f.jsonl"), [
        _rec_use("t9", day % "11:00:00", "grep -rn 'go test -timeout 25m' Makefile scripts/"),
        _rec_result("t9", day % "12:05:02")])
    calls = harvest(root)
    runs = runs_of(calls)

    stale = [c for c in calls if c["sess"] == "sess-e"][0]
    check(stale["end"] is None,
          f"a tool_result {PAIR_CAP.total_seconds() / 3600:.0f}h+ after its "
          f"tool_use does not date the call, so a resumed session invents no run")
    named = [c for c in calls if c["sess"] == "sess-f"][0]
    check(named["end"] is not None and not any(r["sess"] == "sess-f" for r in runs),
          "a grep that merely NAMES the suite argv is paired but is not a run")

    killer = [c for c in calls if "pkill -f 'go test" in c["cmd"]][0]
    control = [c for c in calls if "fan.sh" in c["cmd"]][0]
    h = hits_for(killer["proj"], killer["t"], runs, DEFAULT_WINDOW)
    check(len(h) == 2, f"the pattern kill is charged with BOTH victims (got {len(h)})")
    check(len(runs) == 4, f"four of the five planted calls are runs "
                          f"(got {len(runs)}) — so the two below are exclusions, "
                          f"not absences")
    check(not any(r["sess"] == "sess-c" for r in h),
          "a run that ended 5s BEFORE the kill is not charged to it (secured "
          "by the containment test, which is why the `0 <=` beside it is "
          "belt-and-braces rather than the arm that does the work)")
    check(not any(r["sess"] == "sess-d" for r in h),
          f"a 6s run in another seat, ending 1s after the kill, is under the "
          f"{MIN_RUN_SECONDS}s floor and is not a victim")
    check(any(c["sess"] == "sess-d" for c in calls),
          "...and that 6s run IS in the corpus, so the line above is a floor "
          "and not a missing fixture")
    check(not any(r["sess"] == "sess-k2" for r in h),
          "the killer's own suite in a RESUMED session of the same seat, "
          "ending 3s after, is not charged to it")
    check(kill_patterns(control["cmd"]) and
          all(UNIQUE_HINT.search(p[2]) for p in kill_patterns(control["cmd"])),
          "the control kill IS a kill and IS session-unique, so it is a real "
          "wrong arm and not an absent one")
    check(not hits_for(control["proj"], control["t"], runs, DEFAULT_WINDOW),
          "the control kill, four hours off, is charged with nothing")

    # the null must be able to score above zero on this fixture, or the
    # comparison it exists for is decorative
    kt = [(killer["proj"], killer["t"])]
    null = null_distribution(kt, runs, DEFAULT_WINDOW, DEFAULT_JITTER, 200, 7)
    check(sum(null) == 0,
          "on a fixture with two runs and one kill the null scores 0, so the "
          "observed 1 is not something any displacement would have produced")

    print(f"\n{ok} ok, {fail} bad")
    return 1 if fail else 0


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--root", default=DEFAULT_ROOT, help="transcript root")
    ap.add_argument("--days", type=float, default=0, help="only the last N days")
    ap.add_argument("--window", type=float, default=DEFAULT_WINDOW,
                    help="seconds after a kill in which a run's end counts")
    ap.add_argument("--jitter", type=float, default=DEFAULT_JITTER,
                    help="the null's displacement, in seconds")
    ap.add_argument("--trials", type=int, default=DEFAULT_TRIALS)
    ap.add_argument("--seed", type=int, default=20260903)
    ap.add_argument("--top", type=int, default=14)
    ap.add_argument("--suite", action="store_true",
                    help="only patterns that can match another seat's suite argv")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)
    if args.self_test:
        return self_test()
    return report(args)


if __name__ == "__main__":
    sys.exit(main())
