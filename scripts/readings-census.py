#!/usr/bin/env python3
"""Census the readings log, and export a replay corpus from it.

ADR 0066 D1 — the instrument the log exists for. The spike behind the ADR
(docs/notes.d/ranger-base-our1e.md §5, R0) found that posse counts its
screen-reading incidents by bead and its readings not at all, so the error
rate of today's readers is UNKNOWN and no reader can be shown better than
another. This turns the log into the two things that fixes:

    readings per day, by decision and by verdict     the denominator
    a replay corpus (the bytes + the verdict)        the labelled cases

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

    print("\nregions read")
    for name, n in c["regions"].items():
        print(f"  {n:6d}  {name}")


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
    `whole_recent` is in the shipped manifests.
    """
    out.mkdir(parents=True, exist_ok=True)
    regions_root = out / "regions"
    cases = 0
    files = 0
    with (out / "corpus.jsonl").open("w", encoding="utf-8") as jf:
        for i, rec in enumerate(records):
            case_id = f"{day_of(rec)}-{i:05d}-{rec.get('decision', 'D?')}"
            herdr = rec.get("herdr") or {}
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
            jf.write(json.dumps(case, ensure_ascii=False, sort_keys=True) + "\n")
            cases += 1
            for reg in case["regions"]:
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
