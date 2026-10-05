#!/usr/bin/env python3
"""Census the readings log, and export a replay corpus from it.

ADR 0066 D1 — the instrument the log exists for. The spike behind the ADR
(docs/notes.d/ranger-base-our1e.md §5, R0) found that posse counts its
screen-reading incidents by bead and its readings not at all, so the error
rate of today's readers is UNKNOWN and no reader can be shown better than
another. This turns the log into the two things that fixes:

    readings per day, by decision and by verdict     the denominator
    a replay corpus (the bytes + the verdict)        the labelled cases
    D5 records whose settle gate read seen-idle      the false-IDLE rate

NO MODEL, NO NETWORK, NO BINARY. It reads JSONL files and writes JSONL
files. It does not shell out to `posse` and it does not import anything
outside the standard library, so a census can be taken from any seat,
cage or not, and from a tree whose `go build` is broken.

WHERE IT LOOKS. The log lives inside each session tree's own git dir
(internal/posse/readingslog.go): per session tree, invisible to
`git status`, and removed by git when the tree is removed. This resolves
that the way git does — read the tree's `.git` file, follow its
`gitdir:` line — so the script and the writer agree on the location
without either calling the other.

    scripts/readings-census.py                       every session tree
    scripts/readings-census.py --root DIR            a different worktree root
    scripts/readings-census.py --repo ~/src/posse    a shared checkout too
    scripts/readings-census.py --log PATH            one log, named outright
    scripts/readings-census.py --by rule             group the table by rule
    scripts/readings-census.py --export-corpus DIR   write the replay corpus
    scripts/readings-census.py --json                the table as JSON

EXIT STATUS. 0 whatever it finds, including nothing: an empty census is a
reading about the fleet ("no consequential readings were taken"), not a
failure of the script. 2 only for a usage error or a path that cannot be
read when it was named outright.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from collections import Counter, defaultdict
from pathlib import Path

LOG_NAME = "posse-readings.jsonl"

# The decision ids, in the spike's own order, with the one-line gloss a
# census row needs. Kept here rather than read off the records so a table
# names a decision that took no readings today — an absent row and a zero
# row are different facts, and the zero is the one that says the
# instrument is wired.
DECISIONS = {
    "D1": "pane state (idle/working/blocked)",
    "D2": "dialog (is this screen a permission prompt)",
    "D3": "unknown screen (what was herdr looking at)",
    "D4": "composer hold (working/typed/sent/ghost)",
    "D5": "stall verdict (rewait/keep/hand back)",
    "D6": "turn delivered",
}

# The five consequences ADR 0066 D1 names. A record carrying anything else
# is counted under its own name and flagged, because the set is the ADR's
# and a sixth value is either a new decision nobody wrote down or a log
# from a newer posse than this script.
CONSEQUENCES = ["refusal", "hand-back", "settle-open", "ghost-retirement", "hold"]

# THE FALSE-IDLE COUNT (ADR 0066 D1 as amended 2026-10-04, ranger-base-o1aoi;
# the prices and the rejected alternatives are in
# docs/notes.d/ranger-base-o1aoi.md).
#
# The five consequences above are five ways of NOT acting, so the reading
# that TYPES leaves none of them and was invisible to this census. It is not
# invisible to the log: typed text that starts no turn ends in the D5 stall
# verdict, which is already counted here, and since that amendment a D5
# record carries a second evidence block — `gate` — holding the pane-state
# reading the settle gate opened on, plus a pane capture from each side of
# the keystrokes.
#
# A CANDIDATE is a D5 record whose gate reading was SEEN and IDLE: herdr
# said it recognized a settled screen, posse typed on that, and no turn
# started. That is the shape of a false IDLE and the numerator ADR 0066
# opened with as UNKNOWN; the denominator is the typed-prompt count, which
# this log does not hold and the watch log does.
#
# It is NOT a sixth consequence and not a new record kind. The D5 row counts
# exactly what it counted before and carries more, so every rate above is
# unchanged — which is the one property the amendment promised.
#
# `idle` ALONE, and the other states are visible rather than counted. The
# settle gate accepts `done` as well (awaitSettled's `until`), so a gate that
# read a rule-matched `done` over the wrong screen is the same class and is
# NOT in this numerator — it is in the `d5` and `with_gate` figures the line
# prints beside it, which is why both are printed. The rule is the
# amendment's own words ("seen and idle") and widening it is a decision for
# the record that reads the first fourteen days, not for this script: a
# numerator nobody decided would price the error rate of a reading that was
# never named.
GATE_SEEN_IDLE_STATES = ["idle"]


def gate_of(rec: dict) -> dict:
    """The gate evidence block, or {} for a record that typed nothing.

    Absent on every decision but D5, and within D5 on the launch-line path:
    the work prompt rode in as argv there, so there is no reading that
    typed (ADR 0013 §2).
    """
    g = rec.get("gate")
    return g if isinstance(g, dict) else {}


def is_false_idle_candidate(rec: dict) -> bool:
    """A D5 record whose settle gate read a screen it had SEEN, as idle."""
    if str(rec.get("decision", "")) != "D5":
        return False
    g = gate_of(rec)
    return bool(g.get("seen")) and str(g.get("state", "")) in GATE_SEEN_IDLE_STATES


def git_dir_of(tree: Path) -> Path | None:
    """Resolve a work tree's git dir the way git resolves it.

    A worktree's `.git` is a FILE holding `gitdir: <abs path>`; a main
    checkout's is the directory itself. Anything else — no `.git` at all, a
    `gitdir:` line pointing nowhere — is not a tree this script can place a
    log for, and the caller skips it.
    """
    dot = tree / ".git"
    if dot.is_dir():
        return dot
    if not dot.is_file():
        return None
    try:
        text = dot.read_text(encoding="utf-8", errors="replace").strip()
    except OSError:
        return None
    if not text.startswith("gitdir:"):
        return None
    target = Path(text[len("gitdir:"):].strip())
    if not target.is_absolute():
        target = (tree / target).resolve()
    return target if target.is_dir() else None


def logs_under_root(root: Path) -> list[Path]:
    """Every readings log reachable from a worktree root: <root>/<repo>/<session>."""
    found: list[Path] = []
    if not root.is_dir():
        return found
    for repo in sorted(root.iterdir()):
        if not repo.is_dir():
            continue
        for tree in sorted(repo.iterdir()):
            if not tree.is_dir():
                continue
            gd = git_dir_of(tree)
            if gd is None:
                continue
            log = gd / LOG_NAME
            if log.is_file():
                found.append(log)
    return found


def read_log(path: Path) -> tuple[list[dict], int]:
    """Parse one log. Returns the records and how many lines would not parse.

    A line that will not parse is SKIPPED and counted, never fatal: several
    posse processes append to one log, and the one shape that can go wrong
    is a torn line. A census that died on one of those would be an
    instrument that stops working exactly when the fleet is busiest.
    """
    out: list[dict] = []
    torn = 0
    try:
        with path.open(encoding="utf-8", errors="replace") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    rec = json.loads(line)
                except json.JSONDecodeError:
                    torn += 1
                    continue
                if isinstance(rec, dict):
                    out.append(rec)
                else:
                    torn += 1
    except OSError as exc:
        print(f"readings-census: {path}: {exc}", file=sys.stderr)
    return out, torn


def day_of(rec: dict) -> str:
    """The UTC day a record was written, as the log stamps it (RFC 3339)."""
    at = str(rec.get("at", ""))
    return at[:10] if len(at) >= 10 else "unknown"


def truncated_regions(regs: list) -> list[str]:
    """The names of the regions herdr cut down, in the order they were read.

    THE RECORD'S OWN FLAG, never a comparison of `bytes` against the text.
    Two things stand between the two numbers after the writer has taken the
    flag, and both would make a derived answer wrong in a direction nobody
    could see: the data ceiling rewrites the text AFTER the flag is set
    (internal/posse/readingslog.go, AppendReading), so a redacted region's
    text no longer has the length the flag was taken from; and `bytes` is
    herdr's count of BYTES while a Python `len` over the same string counts
    characters, so every region carrying a `·` or a `→` — which is most of
    them — would read as truncated by one or two.
    """
    return [str(reg.get("region", "?")) for reg in regs if reg.get("truncated")]


def group_key(rec: dict, by: str) -> str:
    if by == "decision":
        return str(rec.get("decision", "?"))
    if by == "rule":
        return str(rec.get("rule", "?"))
    if by == "session":
        return str(rec.get("session", "?"))
    if by == "runtime":
        return str(rec.get("runtime", "?"))
    if by == "posse":
        return str(rec.get("posse", "?"))
    return str(rec.get("decision", "?"))


def census(records: list[dict], by: str) -> dict:
    per_day: dict[str, Counter] = defaultdict(Counter)
    per_group: dict[str, Counter] = defaultdict(Counter)
    verdicts: dict[str, Counter] = defaultdict(Counter)
    regions: Counter = Counter()
    redacted = 0
    truncated = 0
    replayable = 0
    d5 = 0
    gated = 0
    candidates = 0
    for rec in records:
        day = day_of(rec)
        cons = str(rec.get("consequence", "?"))
        per_day[day][cons] += 1
        per_day[day]["total"] += 1
        per_group[group_key(rec, by)][cons] += 1
        per_group[group_key(rec, by)]["total"] += 1
        verdicts[str(rec.get("decision", "?"))][str(rec.get("verdict", ""))] += 1
        if rec.get("redacted"):
            redacted += 1
        herdr = rec.get("herdr") or {}
        regs = herdr.get("regions") or []
        for reg in regs:
            regions[str(reg.get("region", "?"))] += 1
        # The gate block's regions are regions the record carries, so they
        # are counted in the regions table like every other — which is how
        # a reader sees that the two captures are really there, under the
        # two names the writer gives them.
        gate = gate_of(rec)
        for reg in gate.get("regions") or []:
            regions[str(reg.get("region", "?"))] += 1
        if str(rec.get("decision", "")) == "D5":
            d5 += 1
            if gate:
                gated += 1
            if is_false_idle_candidate(rec):
                candidates += 1
        cut = truncated_regions(regs)
        if cut:
            truncated += 1
        # "Replayable" is the one quality claim a census may make about a
        # case: it carries region bytes, none of them was cut down on the
        # way in, and no data-ceiling class was taken out of them. Both
        # lossy cases still count in every rate above — they happened —
        # and neither can reproduce a verdict byte for byte.
        #
        # TRUNCATION IS THE COMMON ONE, and it was counted as replayable
        # until ranger-base-r5546. herdr previews a region at 243
        # characters (internal/posse/panework.go), and `whole_recent` is in
        # the shipped manifests, so the richest records in the log — a D3
        # that evaluated every region — routinely carry a few per cent of
        # the bytes of their biggest region. Redaction is the rarer one and
        # is the trade ADR 0050 makes on purpose.
        #
        # KEYED ON `herdr` ALONE, and the gate block does not make a case
        # replayable. The claim is that THIS record's verdict can be
        # re-decided from the bytes it carries, and a D5 verdict cannot: it
        # is a reading of a herdr wait and a commit count, and the gate's
        # captures are two screens some OTHER reader looked at. Counting
        # them here would turn the one quality number into a count of
        # records that happen to hold bytes. `cut` is read over `regs`
        # alone for the same reason.
        if regs and not cut and not rec.get("redacted"):
            replayable += 1
    return {
        "readings": len(records),
        "per_day": {d: dict(c) for d, c in sorted(per_day.items())},
        f"per_{by}": {g: dict(c) for g, c in sorted(per_group.items())},
        "verdicts": {d: dict(c) for d, c in sorted(verdicts.items())},
        "regions": dict(regions.most_common()),
        "redacted": redacted,
        "truncated": truncated,
        "replayable": replayable,
        "false_idle": {"d5": d5, "with_gate": gated, "candidates": candidates},
    }


def print_table(c: dict, by: str, logs: list[Path], torn: int) -> None:
    print(f"readings: {c['readings']} in {len(logs)} log(s)"
          f" · {c['replayable']} replayable · {c['truncated']} truncated"
          f" · {c['redacted']} redacted")
    if c["readings"] == 0:
        print("  (no consequential readings logged — see ADR 0066 D1 for what is recorded)")
        return
    if torn:
        print(f"  {torn} line(s) would not parse and were skipped")

    print("\nper day")
    head = ["day", "total"] + CONSEQUENCES
    rows = []
    extra: set[str] = set()
    for day, counts in c["per_day"].items():
        extra |= set(counts) - {"total"} - set(CONSEQUENCES)
        rows.append([day, str(counts.get("total", 0))] +
                    [str(counts.get(k, 0)) for k in CONSEQUENCES])
    print_rows(head, rows)
    if extra:
        print(f"  consequences outside ADR 0066 D1's five: {', '.join(sorted(extra))}")

    print(f"\nper {by}")
    head = [by, "total"] + CONSEQUENCES
    rows = []
    for key, counts in c[f"per_{by}"].items():
        label = key
        if by == "decision":
            label = f"{key}  {DECISIONS.get(key, '')}".rstrip()
        rows.append([label, str(counts.get("total", 0))] +
                    [str(counts.get(k, 0)) for k in CONSEQUENCES])
    print_rows(head, rows)
    print_false_idle(c)

    print("\nregions read")
    for name, n in c["regions"].items():
        print(f"  {n:6d}  {name}")


def print_false_idle(c: dict) -> None:
    """The one line ADR 0066 D1's amendment exists to make a number.

    Printed BESIDE THE D5 ROW rather than as a table of its own, because it
    is a reading OF that row: the same records, counted by what their gate
    block says. Both figures are given — the candidates and how many D5
    records carry a gate block at all — so the residual is visible rather
    than folded in: a D5 record with no gate block is a prompt that was
    never typed (it rode in on a launch line), and one whose gate was NOT
    seen-idle is a stall the gate cannot be blamed for.

    Silent where there are no D5 records at all. A zero over zero is not a
    rate, and an empty census already says so once.
    """
    f = c.get("false_idle") or {}
    if not f.get("d5"):
        return
    print(f"\n  false-IDLE candidates: {f.get('candidates', 0)} of {f.get('d5', 0)} D5 record(s)"
          f" ({f.get('with_gate', 0)} carrying a settle-gate reading)")
    print("    a candidate is a stalled prompt whose gate read a screen it had SEEN, as idle")
    print("    — the screen from each side of the keystrokes is in its gate regions (ADR 0066 D1)")


def print_rows(head: list[str], rows: list[list[str]]) -> None:
    width = [len(h) for h in head]
    for row in rows:
        for i, cell in enumerate(row):
            width[i] = max(width[i], len(cell))
    fmt = "  " + "  ".join(f"{{:<{w}}}" if i == 0 else f"{{:>{w}}}"
                           for i, w in enumerate(width))
    print(fmt.format(*head))
    for row in rows:
        print(fmt.format(*row))


def export_corpus(records: list[dict], out: Path) -> tuple[int, int]:
    """Write the replay corpus: the bytes and the verdict, per reading.

    TWO SHAPES, because there are two readers to replay against and neither
    can read the other's input:

      corpus.jsonl        one line per case — the verdict, the rule, the
                          herdr reading and every region's bytes. This is
                          what an offline evaluation of ANY reader scores
                          itself against, and what posse's own Go-side
                          replay (Reading.ReplayHold) re-decides.
      regions/<id>/<region>.txt
                          each region's bytes as a file, so a reader that
                          takes a file can be pointed at one — `herdr agent
                          explain --file <capture> --agent <name>` is the
                          case that exists today, and it is how the D1-D3
                          rules (herdr's TOML manifest, not Go) get re-run
                          over a case at all.

    A LOSSY CASE IS EXPORTED AND MARKED, in both the ways a case can be
    lossy. It is still a labelled reading and still counts, and a corpus
    that silently dropped it would make the rates and the corpus disagree
    about the same fleet. What it cannot do is prove a rule right or wrong
    byte for byte, and the case's two quality fields say which half is
    missing: `redacted` names the data-ceiling classes taken out of it
    (ADR 0050), and `truncated` names the regions herdr handed over cut
    down — the common one, since a preview is capped at 243 characters and
    `whole_recent` is in the shipped manifests. Both fields are read over
    `herdr`'s regions, the way `replayable` is.

    THE GATE BLOCK IS EXPORTED THE SAME WAY, because its captures are the
    ones a false IDLE is diagnosed from and `herdr agent explain --file` is
    what they are diagnosed with (ADR 0066 D1 as amended, ranger-base-o1aoi).
    So a D5 case carries `gate` beside `herdr`, and both blocks' regions
    land in `regions/<id>/<region>.txt` — no collision, because the writer
    names the gate's two captures apart from each other and from herdr's
    manifest vocabulary (`pane_capture_at_prompt`, `pane_capture`).
    """
    out.mkdir(parents=True, exist_ok=True)
    regions_root = out / "regions"
    cases = 0
    files = 0
    with (out / "corpus.jsonl").open("w", encoding="utf-8") as jf:
        for i, rec in enumerate(records):
            case_id = f"{day_of(rec)}-{i:05d}-{rec.get('decision', 'D?')}"
            herdr = rec.get("herdr") or {}
            gate = gate_of(rec)
            case = {
                "id": case_id,
                "at": rec.get("at"),
                "decision": rec.get("decision"),
                "verdict": rec.get("verdict"),
                "consequence": rec.get("consequence"),
                "rule": rec.get("rule"),
                "posse": rec.get("posse"),
                "redacted": rec.get("redacted") or [],
                "truncated": truncated_regions(herdr.get("regions") or []),
                "herdr": {
                    "state": herdr.get("state"),
                    "matched_rule": herdr.get("matched_rule"),
                    "fallback_reason": herdr.get("fallback_reason"),
                    "reported": herdr.get("reported"),
                    "seen": herdr.get("seen"),
                },
                "regions": herdr.get("regions") or [],
            }
            if gate:
                case["gate"] = {
                    "state": gate.get("state"),
                    "matched_rule": gate.get("matched_rule"),
                    "fallback_reason": gate.get("fallback_reason"),
                    "reported": gate.get("reported"),
                    "seen": gate.get("seen"),
                    "regions": gate.get("regions") or [],
                    "false_idle_candidate": is_false_idle_candidate(rec),
                }
            jf.write(json.dumps(case, ensure_ascii=False, sort_keys=True) + "\n")
            cases += 1
            regs = list(case["regions"])
            if gate:
                regs += list(case["gate"]["regions"])
            for reg in regs:
                name = str(reg.get("region", "region"))
                safe = "".join(ch if ch.isalnum() or ch in "-_." else "_" for ch in name)
                d = regions_root / case_id
                d.mkdir(parents=True, exist_ok=True)
                (d / f"{safe}.txt").write_text(str(reg.get("text", "")), encoding="utf-8")
                files += 1
    return cases, files


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(
        prog="readings-census.py",
        description="Census the ADR 0066 D1 readings log and export a replay corpus.")
    ap.add_argument("--root", default=os.path.expanduser("~/.posse/worktrees"),
                    help="worktree root to walk (default: ~/.posse/worktrees)")
    ap.add_argument("--repo", action="append", default=[],
                    help="a shared checkout to include as well; repeatable")
    ap.add_argument("--log", action="append", default=[],
                    help="a log file to read outright, skipping discovery; repeatable")
    ap.add_argument("--by", default="decision",
                    choices=["decision", "rule", "session", "runtime", "posse"],
                    help="what the second table groups by (default: decision)")
    ap.add_argument("--since", default="",
                    help="keep readings on or after this UTC day (YYYY-MM-DD)")
    ap.add_argument("--export-corpus", metavar="DIR", default="",
                    help="write the replay corpus (the bytes + the verdict) to DIR")
    ap.add_argument("--json", action="store_true", help="print the census as JSON")
    args = ap.parse_args(argv)

    logs: list[Path] = []
    if args.log:
        for p in args.log:
            path = Path(os.path.expanduser(p))
            if not path.is_file():
                print(f"readings-census: {path}: no such log", file=sys.stderr)
                return 2
            logs.append(path)
    else:
        logs = logs_under_root(Path(os.path.expanduser(args.root)))
        for r in args.repo:
            gd = git_dir_of(Path(os.path.expanduser(r)))
            if gd is None:
                print(f"readings-census: {r}: not a git work tree", file=sys.stderr)
                return 2
            log = gd / LOG_NAME
            if log.is_file():
                logs.append(log)

    records: list[dict] = []
    torn = 0
    for log in logs:
        recs, t = read_log(log)
        records.extend(recs)
        torn += t
    if args.since:
        records = [r for r in records if day_of(r) >= args.since]
    records.sort(key=lambda r: str(r.get("at", "")))

    c = census(records, args.by)
    if args.export_corpus:
        cases, files = export_corpus(records, Path(os.path.expanduser(args.export_corpus)))
        c["corpus"] = {"dir": args.export_corpus, "cases": cases, "region_files": files}

    if args.json:
        print(json.dumps(c, ensure_ascii=False, indent=2, sort_keys=True))
    else:
        print_table(c, args.by, logs, torn)
        if "corpus" in c:
            print(f"\ncorpus: {c['corpus']['cases']} case(s), "
                  f"{c['corpus']['region_files']} region file(s) → {c['corpus']['dir']}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
