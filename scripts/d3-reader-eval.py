#!/usr/bin/env python3
"""Offline evaluation of readers for ADR 0066 D3 — the unknown-screen diagnosis.

THE QUESTION (ADR 0066 D3, spike ranger-base-qk9tr). When herdr's readiness
gate refuses because no rule matched, WhatHerdrSaw prints 72 characters of
each region and a human works out which screen it was. D3 proposes a typed
`choice` over the known interstitials plus "none of these". Before any model
is bought, this script answers the cheaper question: over the bytes a D3
record actually carries, how well does each candidate reader name the
screen, and where exactly does today's reader fail?

TWO READERS ARE RUN, and a third is described but never called:

    rules      herdr's own TOML manifest, replayed over the screen with
               `herdr agent explain --file` — the baseline, and the only
               reader whose miss is replayable at the codepoint.
    keywords   a deterministic reader over the registry of known screens:
               does the text carry the screen's own heading? Runs over the
               region PREVIEWS (what a D3 record holds — herdr caps every
               preview at 243 characters, MEASURED herdr 0.9.1) and, for
               comparison, over the full screen.
    model      a typed-decision model (Jev / Kev). NOT called here. Any live
               call is an operator spend ruling (ADR 0066 D4, ADR 0019);
               the question it would be asked is printed by --question so
               the ruling, if it comes, runs the same corpus.

THE CORPUS. Until the fleet runs a posse that writes the readings log
(internal/posse/readingslog.go), the only labelled screens posse owns are
the detection fixtures under etc/herdr/agent-detection/testdata/<agent>/,
labelled by filename. Each is replayed as captured and under the
PERTURBATION classes the incident record names — the ways a known screen has
actually stopped matching its rule, which is the only case where D3 has a
screen to name:

    caret     the selected-row marker glyph swapped (› > → ❯) — one
              codepoint, ranger-base-0sa5a's three misses
    footer    the footer hint reworded — rangerhq-7ia's blocked→idle flip
    depth     extra rows above the content, pushing it past a
              top_non_empty_lines(N) window — ranger-base-k987u's logo
    heading   the screen's own heading reworded — ASSUMED: no incident has
              had this shape; it is the one class a keyword reader cannot
              survive by construction, included so the table shows where a
              model's claimed value would have to live

With --corpus it instead reads a replay corpus exported by
scripts/readings-census.py --export-corpus and runs the keyword reader over
every D3 case's recorded regions, scoring against --labels where there are
any. That is the run that replaces this one once real readings exist.

NO MODEL, NO NETWORK, NO SPEND. Standard library only; the one subprocess
is herdr, read-only, against manifests staged from THIS checkout (the
verify-detection.sh rule: measure the tree, not the install).
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
FIXTURES = REPO / "etc" / "herdr" / "agent-detection"

# The known-screen option set: every screen posse owns a capture of, keyed
# the way a D3 line would name it. `rules` are the herdr rule ids that name
# the screen today (so the rules reader's answer can be scored on the same
# axis); `markers` is what the keyword reader looks for — the screen's own
# heading, lower-cased, whitespace-collapsed. A tuple means every phrase in
# it must be present; a bare string means that phrase alone suffices.
#
# Taken from the fixtures and from internal/posse/interstitial.go's four
# registries; `registered` says whether the Go registry knows the screen,
# because the two sets are not the same and a shipped reader would have to
# live on one of them.
OPTIONS = {
    "codex.update_menu": {"rules": ["update_menu", "startup_update"],
                          "markers": ["update available"], "registered": True},
    "codex.signin_menu": {"rules": ["signin_menu"],
                          "markers": ["sign in with chatgpt", "sign in with device code"],
                          "registered": True},
    "codex.signin_api_key": {"rules": ["signin_api_key"],
                             "markers": ["paste or type your api key", "use your own openai api key"],
                             "registered": True},
    "codex.hooks_review": {"rules": ["hooks_review"],
                           "markers": ["hooks need review"], "registered": False},
    "codex.trust_directory": {"rules": ["trust_directory"],
                              "markers": ["do you trust the contents of this directory"],
                              "registered": False},
    "codex.model_picker": {"rules": ["live_strong_blocker"],
                           "markers": ["select model and effort"], "registered": False},
    "grok.startup_splash": {"rules": ["startup_splash"],
                            "markers": [("new worktree", "resume session")], "registered": True},
    "grok.consent_banner": {"rules": [],
                            "markers": ["help improve grok"], "registered": True},
}

# What each fixture shows, as the SET of known screens on it. The filename
# carries herdr's state; the screens are read off the capture. Two of the
# splash captures also carry the consent banner, and that is two true
# answers on one screen — which a `choice` cannot return and a set can.
EXPECTED = {
    "codex/blocked-update-menu": {"codex.update_menu"},
    "codex/blocked-signin": {"codex.signin_menu"},
    "codex/blocked-signin-narrow": {"codex.signin_menu"},
    "codex/blocked-signin-tall-logo": {"codex.signin_menu"},
    "codex/blocked-signin-api-key": {"codex.signin_api_key"},
    "codex/blocked-signin-api-key-tall-logo": {"codex.signin_api_key"},
    "codex/blocked-hooks-review": {"codex.hooks_review"},
    "codex/blocked-trust-directory": {"codex.trust_directory"},
    "codex/blocked-model-picker": {"codex.model_picker"},
    "codex/idle-composer": set(),
    "grok/idle-startup-splash": {"grok.startup_splash", "grok.consent_banner"},
    "grok/idle-startup-splash-no-consent-banner": {"grok.startup_splash"},
    "grok/idle-startup-splash-plain-footer": {"grok.startup_splash", "grok.consent_banner"},
    "grok/idle-startup-splash-wide-boxed": {"grok.startup_splash"},
    "grok/idle-composer-with-consent-banner": {"grok.consent_banner"},
}

HEADING_REWORDS = [
    ("Update available!", "A newer Codex is available"),
    ("Sign in with ChatGPT", "Log in with ChatGPT"),
    ("Sign in with Device Code", "Log in with a device code"),
    ("Paste or type your API key", "Enter your API key"),
    ("Use your own OpenAI API key", "Bring your own OpenAI key"),
    ("Hooks need review", "Review your hooks"),
    ("Do you trust the contents of this directory?", "Is this directory trusted?"),
    ("Select Model and Effort", "Choose a model"),
    ("New worktree", "Create worktree"),
    ("Resume session", "Continue session"),
    ("Help improve Grok", "Help make Grok better"),
]

FOOTER_REWORDS = [
    ("Press enter to confirm or esc to go back", "Enter to confirm · Esc to go back"),
    ("Press enter to continue", "Press Enter to proceed"),
    ("Press enter to save", "Enter to save"),
    ("Press esc to go back", "Esc to go back"),
    ("Ctrl+.:shortcuts", "Ctrl+.:help"),
]


def perturb(kind: str, text: str) -> str | None:
    """Return the perturbed screen, or None when the class does not apply."""
    out = text
    if kind == "caret":
        out = out.replace("›", "→").replace("❯", ">")
        out = re.sub(r"(?m)^(\s*)>(\s+(?:\d\.|You are in))", r"\1→\2", out)
    elif kind == "footer":
        for a, b in FOOTER_REWORDS:
            out = out.replace(a, b)
    elif kind == "depth":
        out = "\n".join(["  .:'`'::.", "  ::.  .::", "  '::''::'", "  .:'  '::.", "  '::..::'"]) + "\n" + out
    elif kind == "heading":
        for a, b in HEADING_REWORDS:
            out = out.replace(a, b)
    else:
        raise ValueError(kind)
    return None if out == text else out


PERTURBATIONS = ["caret", "footer", "depth", "heading"]


def norm(s: str) -> str:
    return " ".join(s.lower().split())


def keyword_read(texts: list[str]) -> set[str]:
    """The keyword reader: which known screens' headings are in the text."""
    blob = norm("\n".join(texts))
    hits = set()
    for opt, spec in OPTIONS.items():
        for m in spec["markers"]:
            phrases = m if isinstance(m, tuple) else (m,)
            if all(p in blob for p in phrases):
                hits.add(opt)
                break
    return hits


def option_of_rule(rule: str) -> str | None:
    for opt, spec in OPTIONS.items():
        if rule in spec["rules"]:
            return opt
    return None


class Herdr:
    """`herdr agent explain --file` against the checkout's manifests."""

    def __init__(self, stage: Path):
        self.cfg = stage / "config"
        self.state = stage / "state"
        staged = self.cfg / "herdr" / "agent-detection"
        staged.mkdir(parents=True)
        self.state.mkdir()
        for toml in FIXTURES.glob("*.toml"):
            shutil.copy(toml, staged / toml.name)
        self.staged = staged
        self.calls = 0
        self.wall = 0.0

    def explain(self, screen: Path, agent: str) -> dict:
        env = dict(os.environ, XDG_CONFIG_HOME=str(self.cfg), XDG_STATE_HOME=str(self.state))
        t0 = time.perf_counter()
        p = subprocess.run(["herdr", "agent", "explain", "--file", str(screen), "--agent", agent, "--json"],
                           capture_output=True, text=True, env=env)
        self.wall += time.perf_counter() - t0
        self.calls += 1
        if p.returncode != 0 or not p.stdout.strip():
            return {"error": (p.stderr or p.stdout).strip()}
        d = json.loads(p.stdout)
        src = str(d.get("manifest_source") or "")
        if str(self.staged) not in src:
            d["warning_source"] = f"manifest answered from {src or 'none'}, not the checkout"
        return d


def rule_id(d: dict) -> str:
    m = d.get("matched_rule")
    if isinstance(m, dict):
        return str(m.get("id") or "")
    return str(m or "")


def regions_of(d: dict) -> list[dict]:
    seen = {}
    for r in d.get("evaluated_rules") or []:
        if r["region"] in seen:
            continue
        ev = r.get("evidence") or {}
        seen[r["region"]] = {"region": r["region"], "bytes": ev.get("region_bytes", 0),
                             "text": ev.get("region_preview", "")}
    return list(seen.values())


def fixtures() -> list[tuple[str, str, Path]]:
    out = []
    for agent_dir in sorted((FIXTURES / "testdata").iterdir()):
        if not agent_dir.is_dir():
            continue
        for f in sorted(agent_dir.glob("*.txt")):
            out.append((agent_dir.name, f"{agent_dir.name}/{f.stem}", f))
    return out


def run_fixtures(h: Herdr, work: Path) -> list[dict]:
    rows = []
    for agent, key, path in fixtures():
        expected = EXPECTED.get(key)
        if expected is None:
            print(f"d3-reader-eval: {key}: no expected screens listed — add it to EXPECTED", file=sys.stderr)
            continue
        base = path.read_text(encoding="utf-8")
        for kind in ["none"] + PERTURBATIONS:
            text = base if kind == "none" else perturb(kind, base)
            if text is None:
                continue
            screen = work / f"{key.replace('/', '__')}.{kind}.txt"
            screen.write_text(text, encoding="utf-8")
            d = h.explain(screen, agent)
            if "error" in d:
                rows.append({"fixture": key, "perturb": kind, "error": d["error"]})
                continue
            rid = rule_id(d)
            seen = bool(rid) or bool(d.get("visible_idle"))
            regs = regions_of(d)
            t0 = time.perf_counter()
            kw_prev = keyword_read([r["text"] for r in regs])
            kw_us = (time.perf_counter() - t0) * 1e6
            kw_full = keyword_read([text])
            rows.append({
                "fixture": key, "perturb": kind, "expected": sorted(expected),
                "herdr_state": d.get("state"), "herdr_rule": rid or None,
                "herdr_fallback": d.get("fallback_reason"),
                "d3_fires": not seen,
                "rules_option": option_of_rule(rid) if rid else None,
                "kw_previews": sorted(kw_prev), "kw_full": sorted(kw_full),
                "kw_us": round(kw_us, 1),
                "preview_chars": max((len(r["text"]) for r in regs), default=0),
                "regions": len(regs),
                "warning": d.get("warning_source"),
            })
    return rows


def score(rows: list[dict]) -> dict:
    """The numbers the ADR needs, derived from the rows and nothing else."""
    s = {"cases": 0, "errors": 0, "by_perturb": {}}
    for r in rows:
        if "error" in r:
            s["errors"] += 1
            continue
        s["cases"] += 1
        exp = set(r["expected"])
        b = s["by_perturb"].setdefault(r["perturb"], {
            "cases": 0, "rule_names_it": 0, "d3_fires": 0,
            "kw_previews_exact": 0, "kw_full_exact": 0,
            "kw_previews_false_positive": 0, "kw_previews_miss": 0,
            "d3_residue": 0, "kw_previews_exact_on_residue": 0, "kw_full_exact_on_residue": 0,
        })
        b["cases"] += 1
        # The rules reader "names it" when its matched rule maps to one of the
        # expected screens, or when nothing is expected and D3 does not fire.
        if (r["rules_option"] and r["rules_option"] in exp) or (not exp and not r["d3_fires"]):
            b["rule_names_it"] += 1
        if r["d3_fires"]:
            b["d3_fires"] += 1
        kp, kf = set(r["kw_previews"]), set(r["kw_full"])
        if kp == exp:
            b["kw_previews_exact"] += 1
        if kf == exp:
            b["kw_full_exact"] += 1
        if kp - exp:
            b["kw_previews_false_positive"] += 1
        if exp - kp:
            b["kw_previews_miss"] += 1
        # The residue is the only place D3 has anything to say: the rule did
        # NOT name a screen that is there. Scoring a second reader anywhere
        # else credits it for work the first reader already did.
        if exp and not (r["rules_option"] and r["rules_option"] in exp):
            b["d3_residue"] += 1
            if kp == exp:
                b["kw_previews_exact_on_residue"] += 1
            if kf == exp:
                b["kw_full_exact_on_residue"] += 1
    return s


def print_rows(rows: list[dict]) -> None:
    head = ["fixture", "perturb", "herdr", "rule", "D3?", "kw(previews)", "kw(full)", "expected", "ok?"]
    table = []
    for r in rows:
        if "error" in r:
            table.append([r["fixture"], r["perturb"], "ERROR", r["error"][:40], "", "", "", "", ""])
            continue
        exp = set(r["expected"])
        ok = "=" if set(r["kw_previews"]) == exp else ("+" if set(r["kw_previews"]) - exp else "-")
        short = lambda xs: ",".join(x.split(".", 1)[1] for x in xs) or "∅"
        table.append([r["fixture"], r["perturb"], r["herdr_state"] or "?", r["herdr_rule"] or "none",
                      "D3" if r["d3_fires"] else "seen", short(r["kw_previews"]), short(r["kw_full"]),
                      short(r["expected"]), ok])
    width = [max(len(h), *(len(row[i]) for row in table)) for i, h in enumerate(head)]
    fmt = "  ".join(f"{{:<{w}}}" for w in width)
    print(fmt.format(*head))
    for row in table:
        print(fmt.format(*row))
    print("\n  ok? column: = keyword reader over the previews names exactly the screens present;"
          " + names one that is not there (false positive); - misses one that is")


def print_summary(s: dict, h: Herdr | None) -> None:
    print(f"\ncases: {s['cases']}  errors: {s['errors']}")
    if h:
        print(f"rules reader: {h.calls} herdr calls, {h.wall / max(h.calls, 1) * 1000:.1f} ms mean wall each")
    head = ["perturb", "cases", "rule names it", "D3 fires", "residue", "kw prev on residue", "kw full on residue", "kw prev FP", "kw prev exact"]
    table = []
    for kind in ["none"] + PERTURBATIONS:
        b = s["by_perturb"].get(kind)
        if not b:
            continue
        table.append([kind, b["cases"], b["rule_names_it"], b["d3_fires"], b["d3_residue"],
                      f"{b['kw_previews_exact_on_residue']}/{b['d3_residue']}",
                      f"{b['kw_full_exact_on_residue']}/{b['d3_residue']}",
                      b["kw_previews_false_positive"], f"{b['kw_previews_exact']}/{b['cases']}"])
    width = [max(len(h), *(len(str(row[i])) for row in table)) for i, h in enumerate(head)]
    fmt = "  ".join(f"{{:<{w}}}" for w in width)
    print(fmt.format(*head))
    for row in table:
        print(fmt.format(*[str(c) for c in row]))
    print("\n  residue = cases where a known screen is present and herdr's rule did not name it:"
          " the only cases D3 has anything to say about.")


def run_corpus(path: Path, labels: Path | None) -> list[dict]:
    """The keyword reader over a readings-census replay corpus's D3 cases."""
    want = {}
    if labels:
        for line in labels.read_text(encoding="utf-8").splitlines():
            if line.strip():
                rec = json.loads(line)
                want[rec["id"]] = set(rec.get("expected") or [])
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        c = json.loads(line)
        if c.get("decision") != "D3":
            continue
        regs = c.get("regions") or []
        kw = keyword_read([str(r.get("text", "")) for r in regs])
        row = {"id": c["id"], "verdict": c.get("verdict"), "regions": len(regs),
               "bytes": sum(int(r.get("bytes") or 0) for r in regs),
               "redacted": bool(c.get("redacted")), "kw": sorted(kw)}
        if c["id"] in want:
            row["expected"] = sorted(want[c["id"]])
            row["exact"] = kw == want[c["id"]]
        rows.append(row)
    return rows


def question() -> dict:
    """The typed question, as a System One `choice`, for the run a spend
    ruling would authorize. Options first, in a FIXED order the evaluators
    found the model leans toward (so "none of these" is deliberately last
    and the reorder test is the evaluator's, not ours); criteria text is the
    screen's own heading, which is what the keyword reader keys on too."""
    opts = {}
    for opt, spec in OPTIONS.items():
        words = [" + ".join(m) if isinstance(m, tuple) else m for m in spec["markers"]]
        opts[opt] = {"criteria": f"the screen carries the heading: {' or '.join(repr(w) for w in words)}"}
    opts["none_of_these"] = {"criteria": "none of the headings above is on the screen"}
    return {
        "type": "choice",
        "state": "<the region previews of one D3 record, joined by newlines — never the whole pane>",
        "question": "Which known first-run screen is this terminal showing?",
        "options": opts,
        "note": "a `choice` returns ONE option; three of the fifteen fixtures show two true screens",
    }


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(prog="d3-reader-eval.py",
                                 description="Offline evaluation of ADR 0066 D3 readers against labelled screens.")
    ap.add_argument("--corpus", metavar="corpus.jsonl",
                    help="score the keyword reader over a readings-census replay corpus instead of the fixtures")
    ap.add_argument("--labels", metavar="labels.jsonl",
                    help="with --corpus: {id, expected:[...]} per line for the cases somebody has labelled")
    ap.add_argument("--question", action="store_true", help="print the typed question a model run would ask, as JSON")
    ap.add_argument("--json", action="store_true", help="rows and summary as JSON")
    args = ap.parse_args(argv)

    if args.question:
        print(json.dumps(question(), indent=2, ensure_ascii=False))
        return 0

    if args.corpus:
        rows = run_corpus(Path(args.corpus), Path(args.labels) if args.labels else None)
        if args.json:
            print(json.dumps(rows, indent=2, ensure_ascii=False))
            return 0
        if not rows:
            print("no D3 cases in the corpus")
            return 0
        for r in rows:
            tail = f"  expected={r['expected']} exact={r['exact']}" if "expected" in r else "  (unlabelled)"
            print(f"{r['id']}  regions={r['regions']} bytes={r['bytes']} redacted={r['redacted']}  kw={r['kw'] or '∅'}{tail}")
        labelled = [r for r in rows if "expected" in r]
        print(f"\nD3 cases: {len(rows)}  labelled: {len(labelled)}  exact: {sum(1 for r in labelled if r['exact'])}")
        return 0

    if not shutil.which("herdr"):
        print("d3-reader-eval: herdr not on PATH", file=sys.stderr)
        return 2
    with tempfile.TemporaryDirectory(prefix="d3-reader-eval.") as tmp:
        stage = Path(tmp) / "stage"
        work = Path(tmp) / "screens"
        stage.mkdir()
        work.mkdir()
        h = Herdr(stage)
        rows = run_fixtures(h, work)
    s = score(rows)
    if args.json:
        print(json.dumps({"rows": rows, "summary": s, "herdr_calls": h.calls,
                          "herdr_ms_mean": h.wall / max(h.calls, 1) * 1000}, indent=2, ensure_ascii=False))
        return 0
    print_rows(rows)
    print_summary(s, h)
    warnings = sorted({r["warning"] for r in rows if r.get("warning")})
    for w in warnings:
        print(f"\nWARNING: {w}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
