#!/usr/bin/env python3
r"""Doc passages that gate on a bead the store says is closed.

WHY THIS EXISTS (ranger-base-tii7l, from ranger-base-ur2eo). A per-line grep
cannot find "this page describes a bead as pending and bd says it closed".
docs/adr/0019-credential-architecture.md was swept for exactly that class
three times in one day and the class survived twice: ranger-base-vxbfm fixed
six passages, ranger-base-vfx8g fixed three more and wrote a completeness
sentence beside them ("The page now carries no open citation naming a closed
bead"), and ranger-base-ur2eo then found five live sites the second sweep's
census could not see. That census was

    grep -nE "ranger-base-[a-z0-9]+[^)]{0,60}open"

which can only see the class spelled with the word "open", AFTER the id, on
ONE line, with no ")" in between. It returned exactly 2 at the sha that
carried the five, so its sentence was true as written and the class was still
open. The five were spelled "waits behind its number as", "blocked on hs0dl",
"runs only after", "is asked only if B measures dead", and "after hs0dl
measures B dead". None of them is the word "open", two of them are BEFORE the
id, and one of them is on the line above it.

THE METHOD, which is the bead's recipe:

  1. collect every `<prefix>-[a-z0-9]+` mention in the file (104 mentions of
     41 distinct ids on ADR 0019 at 0022e4d);
  2. read each id's LIVE status from `bd show <id>... --json`;
  3. for each mention of a CLOSED id, take a window of WINDOW characters
     either side, snapped OUT to whitespace at both ends;
  4. report the window if it matches any of PHRASES.

THE SNAP IS NOT DECORATION. Without it the window cuts mid-word: "the file is
never opened" truncates to "...never open" and `\bopen\b` matches a boundary
the document does not have. That one line is the difference between 14 hits
and 13 on the control arm below. Snapping OUT (extend to the next whitespace)
rather than IN (retreat to the last one) is the safe direction — it can only
add context, never invent a boundary.

PHRASES ARE MATCHED ACROSS NEWLINES. `\s+` between the words, not a space:
these files are hard-wrapped at 80 and "blocked\non hs0dl" is the same claim
as "blocked on hs0dl". A matcher that reads a line at a time is the
instrument this one replaces.

ARMS, measured 2026-09-04 on docs/adr/0019-credential-architecture.md at
0022e4d — the page still carrying ur2eo's five, which is its state in git as
this lands, so the control arm is reproducible with

    scripts/page-currency-census.py docs/adr/0019-credential-architecture.md

  * 104 mentions of 41 ids, 38 closed, 0 unknown -> 13 windows, and every one
    of the five real sites is among them. (ur2eo's own note recorded 43 ids
    and 39 closed; the page has moved since. The 13 is unchanged.)
  * the old per-line `...open` grep over the same file: 2.
  * with the snap removed: 14. The extra one is line 952, "on exit 0 with an
    envelope the file is never opened" — the raw slice ends after "...never
    open" and `\bopen\b` matches. That is the whole 14-vs-13 difference and
    it is one line of code.

AND ONE PLACE THE METHOD GOT LUCKY, which is worth more than the 13. Of
ur2eo's five spellings, four are in PHRASES and are found by design. The
fifth — "after hs0dl measures B dead", line 1032 — is not in PHRASES and is
reported only because an unrelated "an open interactive window" happens to
sit inside its window. Delete "open" from PHRASES and lines 1032, 40, 41, 43
and 46 all vanish; the arm reads 7. Reword that one sentence and the
instrument goes blind to a live site while still printing 12 others. No
phrase list closes this: "after X measures Y" as a phrase costs nothing here
but a bare "after" takes the page from 13 windows to 22. The list is a net,
not a proof, and this is the measurement that says so.

OVER THE WHOLE DOC TREE (the default with no PATH), same day: 1763 mentions
of 647 ids across 187 files, 616 closed, 2 cited under a retired prefix, 5
unknown, 146 candidate windows, in ~5 s wall — 13 batched `bd show` calls plus
one re-ask per unmatched id. Two of the 5 unknown are the documented
placeholders in HISTORY.md and AGENTS.md; the rest name beads the store no
longer has. That is a different defect from this one and is only listed,
never counted as a hit.

The 2 retired-prefix ids are the reason read_statuses_from_bd() re-asks: this
census reported them `unknown` until its own output stopped balancing (649
keys emitted for 647 ids asked about), and an `unknown` id is skipped, so two
CLOSED beads' windows went unread. Neither turned out to carry a claim shape,
so no live site was hiding there — but the blind spot was the exact one this
instrument exists to close, and it was in the instrument.

THE RESIDUE IS WHY THIS IS AN INSTRUMENT AND NOT A GATE. Those 5 remaining
windows are one passage: the status block's own Corrected sentences, which
have to talk ABOUT beads being open in order to record that they were not
("all three of this page's 'open' citations named a bead that had closed").
A word-matching instrument reading a page that discusses openness cannot tell
that from a live claim, and no phrase list fixes it — the distinguishing fact
is the sentence's mood, not its vocabulary. So this prints candidates a
person reads. It deliberately has NO failing exit code and no --strict: the
first person to wire one owes an answer for that class, and until then a red
build would train the crew to delete true sentences. Exit is 0 for a clean
read and 2 only for an operational failure (unreadable file, bd unusable).

READING BD, AND NOT READING IT. By default this shells out to the live store,
which is right for an instrument a person runs and wrong for anything a suite
executes (see the RHQ_HOME class: a test that asks the live meter measures
the box, not the diff). So the status read is a seam:

    --emit-status F   write the statuses this run read, as JSON
    --status-json F   read statuses from F instead of the store

`--self-test` uses only the second, plus a fake `bd` it writes to a temp dir
and invokes BY PATH — never by putting one on PATH, which would change what
every other program in the process sees — for the arms that grade the store
read itself. One of those arms exists because bd
drops an unknown id SILENTLY: `bd show <good> <bogus> --json` returns only
the good one, puts "Error fetching" on stderr, and exits 0. An id the store
has never heard of is therefore reported as `unknown` and never as "not
closed" — the reconciliation against the requested set is the only thing
standing between a typo'd id and a passage nobody ever checks again.

THE THREE DECISIONS ranger-base-tii7l says nobody had made, and what this
file decides, so the next reader argues with an answer rather than a blank:

  1. WHERE IT LIVES — scripts/, beside gk-inflight-census.py,
     suite-entry-census.py and pattern-kill-census.py, not a `posse gates`
     mode. It reads docs and the bead store and touches nothing posse gates
     is about, and an instrument nobody is required to run does not belong
     behind a verb the crew runs constantly.
  2. WHETHER IT READS BD — live by default, because the caller is a person
     asking a question about right now. The read is behind a seam
     (`--status-json` / `--emit-status`) so nothing hermetic ever has to
     shell out, and `--self-test` proves that path with no real `bd` reachable
     at all.
  3. GATE OR INSTRUMENT — instrument, and see the residue above for why. Not
     "for now": there is no --strict flag to add later without answering the
     residue first, and adding one is a decision with an owner, not a
     follow-up commit.

`--self-test` is 40 arms on planted pages and a planted fake `bd`. It runs
under `env -i PATH=/usr/bin:/bin` — no `bd`, no `git`, no network, nothing a
caged seat has to be granted, which is the property that decides whether the
seat that most needs the rig can run it. 29 mutants of this file were run
against it: 28 die (the phrase list one entry at a time, WINDOW and CHUNK in
both directions, the snap, the status read, the reconciliation, the re-ask
and its attribution, the alias report, the batch's `in want` filter, the id
regex's `+`, `\s+`, `\b` and the case fold) and one survives because it is
equivalent — `if not body: continue` versus `body = "[]"` do the same thing.

Three of those 28 only started dying after the rig was fixed, and the defect
was the same each time: a fixture derived from the constant it was grading.
`"z " * (WINDOW // 2 + 40)` moves when WINDOW moves; `range(CHUNK * 2 + 7)`
moves when CHUNK moves; and five spellings on one small page all sit inside
each other's windows, so `len(hits) == 5` counted mentions and stayed green
with a phrase deleted. The literals in those arms are load-bearing.

Usage:
    scripts/page-currency-census.py [PATH ...]
    scripts/page-currency-census.py --status-json snap.json PATH ...
    scripts/page-currency-census.py --emit-status snap.json PATH ...
    scripts/page-currency-census.py --self-test

With no PATH it reads docs/**/*.md plus the repo root's *.md.
"""

import argparse
import glob
import json
import os
import re
import subprocess
import sys

# Characters either side of a mention. 200 is the bead's number and it is
# roughly two and a half hard-wrapped lines each way, which is what it takes
# to reach the verb in "ranger-base-hs0dl ... runs only after" when the id and
# the verb are separated by a parenthetical.
WINDOW = 200

# The claim shapes. Every one of these was found on a real page in one of the
# three ADR 0019 sweeps except `to be filed` and `awaiting`, which are the
# house spellings for the same thing elsewhere in docs/. Words are joined by
# \s+ so a hard wrap between them does not hide the phrase.
PHRASES = (
    "open",
    "pending",
    "held by",
    "blocked on",
    "waits on",
    "waits behind",
    "not yet",
    "awaiting",
    "runs only after",
    "asked only if",
    "to be filed",
    "decided on",
)

# bd's own statuses. Anything not in CLOSED is live and is not this
# instrument's business; `unknown` is the store's silence and is reported
# separately because it is a different defect (a typo'd or purged id).
CLOSED = ("closed",)

DEFAULT_PREFIX = "ranger-base"

# Ids per `bd show` call. Not a guess about argv limits — `bd list --all
# --json` pages at 50 rows in its bare form, and an id this instrument asked
# for and did not get back is reported as `unknown`, which is precisely how a
# silent page boundary would disguise itself as 600 typos. Small batches keep
# the reconciliation honest and cost nothing measurable: 647 ids over the doc
# tree is 13 calls.
CHUNK = 50


def phrase_re(phrase):
    """`blocked on` -> /\\bblocked\\s+on\\b/i — see "ACROSS NEWLINES" above."""
    return re.compile(r"\b" + r"\s+".join(re.escape(w) for w in phrase.split()) + r"\b",
                      re.IGNORECASE)


PHRASE_RES = [(p, phrase_re(p)) for p in PHRASES]


def id_re(prefix):
    return re.compile(re.escape(prefix) + r"-[a-z0-9]+")


def snap(text, start, end):
    """Widen [start, end) OUT to whitespace at both ends.

    OUT, never in: retreating to the previous whitespace would drop the
    partial word, and a partial word is exactly how "never opened" becomes
    "never open". Widening can only ever add context.
    """
    while start > 0 and not text[start - 1].isspace():
        start -= 1
    while end < len(text) and not text[end].isspace():
        end += 1
    return start, end


def _bd_rows(bd, batch):
    """The top-level rows `bd show <batch> --json` returns, or [].

    Only top-level rows: a row carries nested `dependencies`/`dependents`
    arrays whose entries have their own `id` and `status`, and reading those
    would answer for beads nobody asked about.
    """
    try:
        out = subprocess.run([bd, "show"] + list(batch) + ["--json"],
                             capture_output=True, text=True)
    except OSError as exc:
        raise SystemExit("cannot run %s: %s" % (bd, exc))
    body = out.stdout.strip()
    if not body:
        return []
    try:
        parsed = json.loads(body)
    except ValueError:
        raise SystemExit("%s printed non-JSON (rc=%d): %.200s"
                         % (bd, out.returncode, body))
    if isinstance(parsed, list):
        return [r for r in parsed if isinstance(r, dict) and r.get("id")]
    if isinstance(parsed, dict) and not parsed.get("error"):
        raise SystemExit("%s returned an unexpected JSON shape" % bd)
    return []


def read_statuses_from_bd(ids, bd="bd"):
    """({requested id: status}, {requested id: the id bd answered under}).

    Two things bd does that a naive read gets wrong, both measured on this
    store 2026-09-04:

    IT DROPS AN ID IT DOES NOT KNOW — absent from the array, "Error fetching
    <id>" on stderr, rc 0. So the array is reconciled against the requested
    set here rather than trusted, and a silence is `unknown`, never "not
    closed". (Asked for ONLY unknown ids it exits 1 and returns an
    {"error": ...} OBJECT instead of an array; that is not a failure either.)

    IT ANSWERS UNDER A DIFFERENT ID. `bd show ranger-base-fm4p` returns a row
    whose id is `rangerhq-fm4p` — the retired pre-publication prefix
    (HISTORY.md) — status closed. Keying the result by the RETURNED id left
    the requested one `unknown`, and an `unknown` id is skipped, so a CLOSED
    bead's windows were never read. That is precisely the miss this whole
    instrument exists to prevent, found by this function's own output not
    balancing: 649 keys emitted for 647 ids asked about.

    A batch cannot attribute a renamed row on its own — the array is not
    positional and two requests could both resolve elsewhere. So any request
    left unmatched after a batch is re-asked ALONE, where one row is
    unambiguous, and the alias is returned beside the status so the caller
    can say the page cites a retired id.
    """
    ids = list(ids)
    if not ids:
        return {}, {}
    statuses = {}
    aliases = {}
    for i in range(0, len(ids), CHUNK):
        batch = ids[i:i + CHUNK]
        want = set(batch)
        rows = _bd_rows(bd, batch)
        for row in rows:
            if row["id"] in want:
                statuses[row["id"]] = row.get("status") or "unknown"
        for one in [b for b in batch if b not in statuses]:
            solo = _bd_rows(bd, [one])
            if len(solo) == 1:
                statuses[one] = solo[0].get("status") or "unknown"
                if solo[0]["id"] != one:
                    aliases[one] = solo[0]["id"]
    for i in ids:
        statuses.setdefault(i, "unknown")
    return statuses, aliases


def census(path, statuses, prefix=DEFAULT_PREFIX, window=WINDOW):
    """Every mention of a CLOSED id whose snapped window carries a claim shape.

    Returns (hits, mentions, ids). One hit per MENTION, not per phrase and not
    per id: the unit a person has to go and read is the passage around one
    occurrence.
    """
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    hits = []
    mentions = 0
    ids = set()
    for m in id_re(prefix).finditer(text):
        bid = m.group(0)
        ids.add(bid)
        mentions += 1
        if statuses.get(bid) not in CLOSED:
            continue
        s, e = snap(text, max(0, m.start() - window), min(len(text), m.end() + window))
        win = text[s:e]
        matched = [p for p, rx in PHRASE_RES if rx.search(win)]
        if matched:
            hits.append({
                "file": path,
                "line": text.count("\n", 0, m.start()) + 1,
                "id": bid,
                "status": statuses.get(bid),
                "phrases": matched,
                "window": " ".join(win.split()),
            })
    return hits, mentions, ids


def default_paths():
    return sorted(set(glob.glob("docs/**/*.md", recursive=True)) | set(glob.glob("*.md")))


def main(argv=None):
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("paths", nargs="*")
    ap.add_argument("--prefix", default=DEFAULT_PREFIX)
    ap.add_argument("--window", type=int, default=WINDOW)
    ap.add_argument("--status-json", help="read statuses from this file, not the store")
    ap.add_argument("--emit-status", help="write the statuses this run read to this file")
    ap.add_argument("--json", action="store_true", help="print the hits as JSON")
    ap.add_argument("--bd", default=os.environ.get("BD", "bd"))
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args(argv)

    if args.self_test:
        return self_test()

    paths = args.paths or default_paths()
    missing = [p for p in paths if not os.path.isfile(p)]
    if missing:
        print("no such file: %s" % ", ".join(missing), file=sys.stderr)
        return 2
    if not paths:
        print("nothing to read", file=sys.stderr)
        return 2

    # One pass to collect ids, one store read for all of them, then the census.
    rx = id_re(args.prefix)
    ids = set()
    for p in paths:
        with open(p, encoding="utf-8") as fh:
            ids.update(rx.findall(fh.read()))

    aliases = {}
    if args.status_json:
        with open(args.status_json, encoding="utf-8") as fh:
            statuses = json.load(fh)
        for i in ids:
            statuses.setdefault(i, "unknown")
    else:
        statuses, aliases = read_statuses_from_bd(sorted(ids), args.bd)

    if args.emit_status:
        with open(args.emit_status, "w", encoding="utf-8") as fh:
            json.dump(statuses, fh, indent=2, sort_keys=True)

    all_hits = []
    mentions = 0
    for p in paths:
        hits, n, _ = census(p, statuses, args.prefix, args.window)
        all_hits.extend(hits)
        mentions += n

    if args.json:
        print(json.dumps(all_hits, indent=2))
    else:
        n_closed = sum(1 for i in ids if statuses.get(i) in CLOSED)
        unknown = sorted(i for i in ids if statuses.get(i) == "unknown")
        print("%d mentions of %d ids in %d file(s); %d closed, %d unknown"
              % (mentions, len(ids), len(paths), n_closed, len(unknown)))
        if aliases:
            print("  cited under a retired id, answered under another: %s"
                  % ", ".join("%s -> %s" % kv for kv in sorted(aliases.items())))
        if unknown:
            print("  unknown to the store (typo, or purged): %s" % ", ".join(unknown))
        print("%d window(s) name a CLOSED bead beside a claim shape:" % len(all_hits))
        for h in all_hits:
            print("\n  %s:%d  %s [%s]  %s"
                  % (h["file"], h["line"], h["id"], h["status"], "/".join(h["phrases"])))
            print("      %s" % h["window"])
        print("\nCandidates, not a verdict — a page that DISCUSSES openness reads "
              "the same as one that claims it. Read each window. (No failing exit "
              "code by design; see the module docstring.)")
    return 0


# ---------------------------------------------------------------------------
# self-test — planted pages, and every arm carries the case that would read
# the same way if the arm were absent. A rig that cannot fail pins nothing:
# the snap arm, the status arm, the window-bound arm and the reconciliation
# arm each run their own WRONG arm and assert it reads differently.
# Run: scripts/page-currency-census.py --self-test
# ---------------------------------------------------------------------------
FAKE_BD = r"""#!/usr/bin/env python3
import json, os, sys
# Records one line per invocation in $FAKE_BD_LOG, answers from $FAKE_BD_DB
# (a {id: status} JSON file), and DROPS ids it does not know exactly the way
# the real bd does: absent from the array, "Error fetching" on stderr, rc 0.
db = json.load(open(os.environ["FAKE_BD_DB"]))
ids = [a for a in sys.argv[2:] if not a.startswith("-")]
if os.environ.get("FAKE_BD_LOG"):
    with open(os.environ["FAKE_BD_LOG"], "a") as fh:
        fh.write(" ".join(ids) + "\n")
alias = json.load(open(os.environ["FAKE_BD_ALIAS"])) if os.environ.get("FAKE_BD_ALIAS") else {}
if os.environ.get("FAKE_BD_SHAPE"):       # a future bd that returns an object
    print(json.dumps({"issues": []})); sys.exit(0)
if os.environ.get("FAKE_BD_WHOLE"):      # the wrong arm: answers everything
    out = [{"id": i, "status": db.get(i, "closed")} for i in ids]
    print(json.dumps(out)); sys.exit(0)
known = [i for i in ids if i in db]
for i in ids:
    if i not in db:
        print('Error fetching %s: no issue found matching "%s"' % (i, i), file=sys.stderr)
if not known:
    print(json.dumps({"error": "no issues found matching the provided IDs"}))
    sys.exit(1)
# ...and answers under the OTHER id where the store has renamed one, which is
# what `bd show ranger-base-fm4p` -> `rangerhq-fm4p` does on the real store.
print(json.dumps([{"id": alias.get(i, i), "status": db[i]} for i in known]))
"""


def self_test():
    import tempfile
    ok = fail = 0

    def check(cond, what):
        nonlocal ok, fail
        if cond:
            ok += 1
            print("  ok    %s" % what)
        else:
            fail += 1
            print("  BAD   %s" % what)

    tmp = tempfile.mkdtemp(prefix="page-currency-selftest-")

    def page(name, body):
        path = os.path.join(tmp, name)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(body)
        return path

    CLOSED_ID = "ranger-base-aaaa1"
    LIVE_ID = "ranger-base-bbbb2"
    st = {CLOSED_ID: "closed", LIVE_ID: "in_progress"}
    pad = "x " * 200          # >WINDOW chars of filler, whitespace-separated

    # --- 1. the class itself, in the spellings that escaped ----------------
    # ONE PAGE PER SPELLING, and each asserts the phrase it matched ON. A
    # first cut put all five on one small page and asserted `len(hits) == 5`;
    # every window there spanned the whole file, so the arm counted MENTIONS
    # and stayed green with a phrase deleted from the list.
    spellings = [
        ("V17 runs only after %s.", "runs only after"),
        ('"ask the owner to refresh" waits behind its number as %s.', "waits behind"),
        ("Priced and parked as %s, blocked on hs0dl.", "blocked on"),
        ("It is asked only if B measures dead (%s).", "asked only if"),
        ("The probe is held by %s.", "held by"),
        ("%s is still pending.", "pending"),
        ("Awaiting %s.", "awaiting"),
        ("%s has not yet landed.", "not yet"),
        ("The shape waits on %s.", "waits on"),
        ("A bead is to be filed against %s.", "to be filed"),
        ("It is decided on %s.", "decided on"),
        ("%s is still open.", "open"),
    ]
    for i, (tmpl, phrase) in enumerate(spellings):
        pg = page("spell%02d.md" % i, tmpl % CLOSED_ID)
        h, _, _ = census(pg, st)
        check(len(h) == 1 and phrase in h[0]["phrases"],
              "%-16r is found, on that phrase" % phrase)
    # ...and the census the second sweep ran finds almost none of them, which
    # is the whole reason this file exists.
    old_grep = re.compile(r"ranger-base-[a-z0-9]+[^)]{0,60}open")
    n_old = sum(1 for tmpl, _ in spellings if old_grep.search(tmpl % CLOSED_ID))
    check(n_old == 1,
          "...and the per-line `...open` grep finds %d of the %d (only the "
          "one literally spelled `open` after the id)" % (n_old, len(spellings)))

    # --- 1b. what counts as an id ------------------------------------------
    bare = page("bare.md", "the prefix ranger-base- alone is still open")
    check(census(bare, st)[0] == [], "a bare `ranger-base-` with no suffix is not a hit")
    check(id_re(DEFAULT_PREFIX).findall("the prefix ranger-base- alone") == [],
          "...and it is not EXTRACTED as an id either — a `*` suffix would "
          "read it as one, send it to bd and file it under `unknown`, which "
          "the hit-count arm above cannot see")
    longer = page("longer.md", "%sx is still open" % CLOSED_ID)
    check(census(longer, st)[0] == [],
          "...and `%sx` is a DIFFERENT id, not a match on `%s` (a non-greedy "
          "suffix would conflate two beads' statuses)" % (CLOSED_ID, CLOSED_ID))

    # --- 2. status is read, not assumed ------------------------------------
    live = page("live.md", "V17 runs only after %s." % LIVE_ID)
    check(census(live, st)[0] == [], "a LIVE id beside the same claim is not a hit")
    check(len(census(live, {LIVE_ID: "closed"})[0]) == 1,
          "...and the identical page IS a hit once that id closes (so the arm "
          "above is grading the status read, not the prose)")

    # --- 3. the snap, and the sentence it saves -----------------------------
    # "opened" is planted so that the RAW slice — mention end + WINDOW — lands
    # between its "open" and its "ed". Nothing here is approximate: the filler
    # is sized so the word starts at exactly the offset that does it.
    lead = "the file is never "
    want = len(CLOSED_ID) + WINDOW - len("open")   # offset "opened" must start at
    filler = "q" * (want - len(CLOSED_ID) - 1 - len(lead) - 1)
    snapped_page = page("snap.md",
                        "%s %s %s%s and more words after" % (CLOSED_ID, filler, lead, "opened"))
    with open(snapped_page) as fh:
        _t = fh.read()
    assert _t[len(CLOSED_ID) + WINDOW - len("open"):][:6] == "opened", "fixture misplaced"
    with_snap = census(snapped_page, st)[0]
    saved = globals()["snap"]
    globals()["snap"] = lambda text, s, e: (s, e)
    without_snap = census(snapped_page, st)[0]
    globals()["snap"] = saved
    check(len(without_snap) == 1 and without_snap[0]["phrases"] == ["open"],
          "WRONG ARM: an unsnapped window ends mid-word and `never opened` "
          "reads as `never open`")
    check(with_snap == [],
          "...and the snap removes it — the same 14-vs-13 difference the "
          "control arm on ADR 0019 shows")

    # --- 4. hard wraps, which is how a per-line matcher goes blind ----------
    wrapped = page("wrap.md", "Priced and parked as %s, blocked\non hs0dl." % CLOSED_ID)
    check(len(census(wrapped, st)[0]) == 1, "`blocked\non` across a hard wrap is a hit")
    check(not any(phrase_re("blocked on").search(l)
                  for l in open(wrapped).read().split("\n")),
          "...and no single LINE of that page contains the phrase, so a "
          "line-at-a-time matcher cannot see it")

    # --- 5. the two other shapes the old grep could not read ---------------
    before = page("before.md", "Still pending: the probe on %s." % CLOSED_ID)
    check(len(census(before, st)[0]) == 1, "a claim BEFORE the id is a hit")
    paren = page("paren.md", "The probe (%s) is still open." % CLOSED_ID)
    check(len(census(paren, st)[0]) == 1, "a `)` between the id and the claim is a hit")
    check(not old_grep.search(open(paren).read()),
          "...which the old grep's [^)]{0,60} could not be (it reads 0 here)")

    # --- 6. the window is BOUNDED ------------------------------------------
    # 300 and 20 are LITERAL, not WINDOW-derived. A first cut wrote
    # `"z " * (WINDOW // 2 + 40)` and the fixture then moved with the constant
    # it was supposed to be grading: WINDOW 200 -> 2000 left this arm green.
    # These two numbers straddle the shipped 200 and are a bless of it — the
    # ADR 0019 arms in the docstring were measured at that reach, so widening
    # it is a change to what the instrument claims and this arm should say so.
    far = page("far.md", "%s %s still pending" % (CLOSED_ID, "z" * 300))
    near = page("near.md", "%s %s still pending" % (CLOSED_ID, "z" * 20))
    check(census(far, st)[0] == [], "a claim 300 chars out is NOT a hit at WINDOW=%d" % WINDOW)
    check(len(census(near, st)[0]) == 1,
          "...and the same words 20 chars out are (so the arm above is "
          "measuring the distance, not a broken matcher)")

    # --- 7. the residue: the documented blind spot, pinned ------------------
    residue = page("residue.md",
                   'Corrected 2026-09-04 (%s): all three of this page\'s "open" '
                   'citations named a bead that had closed.' % CLOSED_ID)
    check(len(census(residue, st)[0]) == 1,
          "a sentence recording that a citation WAS open is reported too — "
          "this instrument cannot tell mood from vocabulary, which is why it "
          "has no failing exit code")

    # --- 8. the store read: bd's silent drop, and the reconciliation -------
    bd_path = os.path.join(tmp, "bd")
    with open(bd_path, "w") as fh:
        fh.write(FAKE_BD)
    os.chmod(bd_path, 0o755)
    db_path = os.path.join(tmp, "db.json")
    with open(db_path, "w") as fh:
        json.dump(st, fh)
    os.environ["FAKE_BD_DB"] = db_path
    log_path = os.path.join(tmp, "calls.log")
    os.environ["FAKE_BD_LOG"] = log_path

    got, _ = read_statuses_from_bd([CLOSED_ID, "ranger-base-nope9"], bd_path)
    check(got.get("ranger-base-nope9") == "unknown",
          "an id bd drops at rc 0 is reported `unknown`, never `not closed`")
    check(got.get(CLOSED_ID) == "closed",
          "...and the id it DID answer for still comes back (the drop is "
          "silent, so the batch must not be discarded with it)")
    os.environ["FAKE_BD_WHOLE"] = "1"
    try:
        whole, _ = read_statuses_from_bd([CLOSED_ID, "ranger-base-nope9"], bd_path)
    finally:
        del os.environ["FAKE_BD_WHOLE"]
    check(whole.get("ranger-base-nope9") == "closed",
          "WRONG ARM: a bd that answers for everything reports 0 unknown — "
          "so the arm above is grading the reconciliation and not the fake")

    os.environ["FAKE_BD_SHAPE"] = "1"
    try:
        shaped = None
        try:
            read_statuses_from_bd([CLOSED_ID], bd_path)
        except SystemExit as exc:
            shaped = str(exc)
    finally:
        del os.environ["FAKE_BD_SHAPE"]
    check(shaped is not None and "unexpected JSON shape" in shaped,
          "a bd that returns a bare OBJECT instead of an array STOPS the run "
          "— silently reading zero rows would mark every id `unknown`")

    # --- 8b. the row bd answers under another id ---------------------------
    ALIASED = "ranger-base-fm4p9"
    RENAMED = "rangerhq-fm4p9"
    with open(db_path, "w") as fh:
        json.dump({CLOSED_ID: "closed", ALIASED: "closed"}, fh)
    alias_path = os.path.join(tmp, "alias.json")
    with open(alias_path, "w") as fh:
        json.dump({ALIASED: RENAMED}, fh)
    os.environ["FAKE_BD_ALIAS"] = alias_path
    try:
        st2, al2 = read_statuses_from_bd([CLOSED_ID, ALIASED], bd_path)
    finally:
        del os.environ["FAKE_BD_ALIAS"]
    check(st2.get(ALIASED) == "closed",
          "a row bd answers under ANOTHER id is attributed to the id that "
          "was ASKED for (keying by the returned id left it `unknown`, and "
          "an unknown id's windows are never read — a closed bead invisible)")
    check(RENAMED not in st2 and al2 == {ALIASED: RENAMED},
          "...and the rename is REPORTED, not silently absorbed")
    check(st2.get(CLOSED_ID) == "closed",
          "...and the rest of the batch is unaffected")
    st3, al3 = read_statuses_from_bd([CLOSED_ID], bd_path)
    check(al3 == {}, "WRONG ARM: a batch with no rename reports no alias, so "
                     "the arm above is grading the re-ask and not the fake")

    only_bogus, _ = read_statuses_from_bd(["ranger-base-nope9"], bd_path)
    with open(db_path, "w") as fh:
        json.dump(st, fh)
    check(only_bogus == {"ranger-base-nope9": "unknown"},
          "bd's rc-1 {\"error\"} OBJECT (all ids unknown) is not a crash")

    # --- 9. chunking, which is where a paging store would hide -------------
    # 107 is LITERAL for the same reason as the two above: `CHUNK * 2 + 7`
    # made the batch grow with the batch size, and CHUNK 50 -> 1000 left this
    # arm green at one call.
    many = ["ranger-base-c%04d" % i for i in range(107)]
    with open(db_path, "w") as fh:
        json.dump({i: "closed" for i in many}, fh)
    open(log_path, "w").close()
    got, _ = read_statuses_from_bd(many, bd_path)
    calls = [l for l in open(log_path).read().split("\n") if l.strip()]
    check(len(calls) == 3,
          "107 KNOWN ids go out in 3 calls (CHUNK=%d) — exactly 3, so a batch "
          "that lines up costs no per-id re-asks" % CHUNK)
    check(all(len(c.split()) <= CHUNK for c in calls),
          "...and no call exceeds CHUNK")
    check(sum(1 for v in got.values() if v == "unknown") == 0 and len(got) == len(many),
          "every id in a multi-batch read comes back — a page boundary that "
          "swallowed a batch would show up here as %d unknowns" % CHUNK)

    print("\n%d ok, %d BAD" % (ok, fail))
    return 1 if fail else 0


if __name__ == "__main__":
    sys.exit(main())
