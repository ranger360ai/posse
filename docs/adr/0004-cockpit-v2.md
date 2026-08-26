# ADR 0004 — Cockpit v2: width-aware rows, an IN PROGRESS section, scrolling

*Status: accepted 2026-08-18 · amended 2026-08-19 (§2 holder join) · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> The fleet observations in Context are that instance's; the mechanism
> assumes only the section shapes described below.

## Context

`posse cockpit` (`cmd/posse/cockpit.go`, ~500 lines, raw-mode + ANSI, only
dep `golang.org/x/term`) draws two sections — SESSIONS blocked-first,
READY WORK beneath — with `%-16s`/`%-36s` fixed widths and no notion of
the terminal's size. Three consequences on a real fleet night: rows wrap
on narrow popups and leave slack on wide ones; the list of claimed beads
(what every persona is *doing*) is invisible — a bead vanishes from view
the moment it's claimed, and the only way to see who holds what is
`bd list --status in_progress` in another pane; and with 7 personas plus
14+ ready beads the list runs past the popup and the cursor with it.

The cockpit is oversight, not a workbench: herdr is the layout surface;
the cockpit answers "who is stuck, who holds what, what's waiting" and
lets the operator act on the answer (focus, prompt, peek, kill, claim,
dispatch). v2 keeps that scope exactly.

## Decision

**1. One view model, drawn to a size.** `draw()` becomes
`render(w, h int) string` over a *row model* built by `refresh()`: a flat
slice of rows (section headings, items, empty markers), each item tagged
with its section and key. Terminal size comes from `term.GetSize` on every
draw plus `SIGWINCH` → redraw; the non-tty fallback renders at 80×∞. Every
row is composed from columns of two kinds:

| kind | rule |
|---|---|
| fixed | mark, emoji, id, priority, status, holder — printed at their natural width, never truncated |
| flex | exactly one per row (title / session name+persona) — gets `w − fixed − gaps`, truncated with `…` on rune count |
| droppable | trailing dim context (repo dir, `(focused)`, `@runtime`) — dropped whole when `w < 100`; holder column dropped when `w < 70` |

Rune count, not display width: the flex column is last and emoji live in
fixed columns at the left, so a wide glyph misaligns nothing the eye
reads. No width library — that is the deliberate cost of "no new dep".

**2. Three sections, one cursor.** SESSIONS · IN PROGRESS · READY WORK,
each heading with its count. `tab` cycles sections in that order; `j/k`
still walk the whole list; the `selection` re-anchor gains the
third section. Sorting: sessions blocked-first (unchanged); in progress
**stalled-first** — holder session blocked, then no live session, then
idle, then working (the operator's eye goes to what needs a hand); ready
as `bd ready` returns it.

An IN PROGRESS row is a bead with `status: in_progress` from every
`beads:` repo (`Bd.InProgressAll`, `bd list --status in_progress --json`),
joined to its holder: `SessionForBead(assignee, dir, id)` looked up in
the live sessions, then `SessionFor(assignee, dir)` as the pre-Dial-F
fallback — the same two names dispatch's held-bead/`--resume` path
checks. *(Amended 2026-08-19: as accepted this line named
only the slot, but under Dial F (ADR 0003) the dispatcher names sessions
per bead, so the slot alone matches almost nothing live.)* Columns: `id · p · holder · holder-state · age · title · repo`,
where holder-state is the session's herdr status or `no session` and age
is since `updated_at` (`3m`, `2h`, `1d`). Ready rows drop in_progress
beads (today `bd ready` may include them; the cockpit filters — a bead
appears in one section only).

**3. Keys per section.** Unchanged for sessions and ready. On an
in-progress row: `enter` focuses the holder's session, `p` prompts it,
`v` peeks it (all "act on the holder"; no session → status line says so),
`d` re-dispatches (dispatch's `--resume` semantics: re-prompt the holder,
or launch it if gone), `u` unclaims (y confirms; `Bd.Unclaim`, actor =
none — the operator's hand, attributed as such). Footer shows the keys of
the *selected* section only.

**4. Scrolling.** A single viewport over the row model: header (2 lines)
and footer (status + keys, up to 3 lines, mode-dependent) are fixed;
rows in between get `h − 5`. Scroll offset keeps the cursor visible with
a 2-row margin; section headings scroll with their rows (no sticky
headers — they'd cost rows and complexity for a list that is rarely more
than two screens). Row model edges show `↑ n more` / `↓ n more` in dim.
`ctrl-d`/`ctrl-u` page; `g`/`G` top/bottom. Peek mode shows the tail in
the same viewport, clipped to `h`, instead of appending below the list.
*(Amended 2026-08-26: the fixed 2+3 chrome holds only down to `h = 6` —
below that `h − 5` leaves no viewport, and as accepted the render
overflowed the terminal (rangerhq-5qm). Below the floor the viewport
keeps one row — the cursor's — and the chrome sheds one line per lost
`h`, least-carrying first: the header's blank spacer, the cost line, the
status line, then the header title; the footer's action line (keys,
prompt, or the y/n question) survives to `h = 2`, and at `h = 1` only
the cursor's row remains. `chromeFor`/`trimFooter` in
`cmd/posse/cockpit.go`, pinned by `TestCockpitShortTerminalShedsChrome`
and `TestCockpitFitsShortTerminal`.)*

**5. Testable by construction.** `render(w,h)` is a pure function of
`(rows, cursor, offset, mode, status)`; `cockpit_test.go` gains golden
tests at 60×20, 80×24, 140×40 with a fixed clock, and a truncation test
per column kind. `displayOnly` reuses the row model at 80 wide.

**Out of scope (named so nobody argues them in):** mouse; filtering or
search; per-repo tabs; editing beads (title, priority, labels) — `bd` is
one pane away; multi-select or bulk kill; sticky section headers; a
tier/cost column — ADR 0003 adds it as a *droppable* column of
the sessions row once it exists, which this layout already accommodates.

## Consequences

- `cockpit.go` grows a `rowModel` + `render(w,h)` and loses the inline
  `fmt.Fprintf` drawing; net +150 lines, most of it the in-progress join
  and scrolling. `beads.go` gains `InProgressAll` (mirror of `ReadyAll`).
- Refresh cost: one more `bd` invocation per repo per 2s tick. Acceptable
  at today's 2–3 repos; if it shows, the tick for beads becomes 6s while
  sessions stay at 2s (a knob, not a redesign).
- `dispatch --resume` from a key: `LaunchBead` already handles an
  in_progress bead whose holder is gone; the cockpit passes `Resume: true`
  for that one call.
- The `--watch` loop and dispatch output are untouched.

## Alternatives rejected

- **A TUI framework (bubbletea/tview).** Solves width and scrolling for
  free and costs a dependency tree and a rewrite of a 500-line file that
  works; the row model is ~80 lines.
- **Per-section fixed heights / independent scroll per section.** Sessions
  starve on a small popup or waste rows on a big one; one viewport, one
  cursor is what the operator already has in muscle memory.
- **Sticky headings.** Cost rows and edge cases; the count in the heading
  and `↑ n more` tell the operator where they are.
- **`go-runewidth` for exact alignment.** Right answer in general; wrong
  trade here (one dep for a column that is last anyway).
- **Show in_progress inside READY WORK with a marker.** Mixes "waiting" with
  "being done"; the sort keys differ (priority vs stalled-first) and so
  do the actions.
