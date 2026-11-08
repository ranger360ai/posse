## claude's first-run notices: one row, and why it is read by nothing yet (ranger-base-72oz4)

The optional half of ranger-base-yk5qb. That bead's remedy was a bring-up step
(INSTALL.md §1 and §10); this one was to put the screen's NAME in the
unrecognized-pane failure line, by giving `internal/posse/knownscreen.go` a
`KnownScreens` row — which it could not have without a capture, because every
marker in that table is read off a shipped fixture (ADR 0066 D3).

The capture exists now and the row is in. What the capture also settled is that
the row does not do what this bead was filed believing it would.

### 1. The screen is a RECOGNIZED blocker, not an unrecognized pane

MEASURED 2026-11-08. herdr 0.9.1, claude 2.1.319 darwin-arm64. The shipped
capture `etc/herdr/agent-detection/testdata/claude/blocked-fullscreen-upsell.txt`
replayed through `herdr agent explain --file --agent claude`, with this
checkout staged into a throwaway `XDG_CONFIG_HOME` (verify-detection.sh's own
rig):

```
state: blocked
manifest: bundled 2026.09.11.1
rule: live_blocked_form (region=after_last_horizontal_rule priority=980)
```

The horizontal rule that rule keys on is the **dialog's own chrome**, not the
"Auto mode is now Claude Code's default permission mode" notice above it:
re-captured with `hasSeenAutoDefaultNotice` seeded true, so the notice was
absent, and the state, rule and region were identical.

So there is no fallthrough to `default_known_agent_idle_fallback` here — which
is the hole `codex.toml` and `grok.toml` were written for — and the table's
`Rules` field for this row is live rather than a formality. Its residue is
empty by construction, which the instrument shows: `scripts/d3-reader-eval.py`
reads `seen` for this fixture on **all four** perturbation classes, `heading`
included, because `live_blocked_form` is keyed on form chrome and not on the
screen's words.

### 2. Neither consumer of the reader is reached on that pane

`KnownScreensIn` has exactly two (ADR 0066 D2, verbatim: "the failure line's
last row and the `looks_like` field on the D3 record"):

- `dispatch.go:5766`, `awaitDelivered` — inside `default:` on `!det.Seen()`.
- `dispatch.go:6000`, `awaitSettled` — below `case det.State == "blocked":
  return det.State, det, nil` at `dispatch.go:5942`, which returns first.

So on a blocked-with-a-rule pane no D3 record is written at all, nothing in
`scripts/readings-census.py` counts it, and `WhatHerdrSaw` is never called.

The line the bring-up actually got is `awaitAgent`'s `Die` at
`dispatch.go:5828`:

```
agent in <session> never settled idle (status "blocked") — check the session
(posse peek <session>)
```

`awaitSettled` RETURNS that detection and `awaitAgent` binds it as `gate`. The
rule id, its region and the known-screen set are all in hand at that line, and
it prints the status string alone. Filed as **ranger-base-x2grk**, with the
D2 question (is a blocked hand-back a third consumer of the D3 report, or does
it just name herdr's rule?) stated rather than answered — that part is
richard's if it is wanted.

The row still ships: the screen is real, the marker is read off a capture, and
the alternative was a fixture labelled "no known screen here" over a pane that
plainly has one. A claude release that reshapes the dialog past
`after_last_horizontal_rule` is what would make it speak.

### 3. The sibling screen cannot be drawn on this box

`Teach auto mode about your environment?` over `Auto mode works better when it
knows your environment. Takes about a minute.`, same `Yes / Not now / Don't
show again` shape, and posse runs claude seats in auto mode — so it is in the
path and yk5qb flagged it. It has **no capture and therefore no row**.

Read 2026-11-08 off the shipped bundle (`~/.local/share/claude/versions/2.1.319`,
`strings` then the minified source). The dialog's gate is two functions:

```js
function hHe(h){ return $Mr()===!0 && h.toolPermissionContext.mode==="auto"
  && !h.viewingAgentTaskId && !wc() && !xn()
  && !i.CLAUDE_BRIDGE_REATTACH_SESSION && dO() }

var n0t=604800000, r0t=5, _ie=5;
function dO(){ if(!Zge())return!1;
  if((rP()?.environment?.length??0)>0)return!1;
  let h=fe();
  if(h.numStartups<r0t)return!1;                     // >= 5 startups
  if((h.autoModeEnvSetup?.denials??0)<_ie)return!1;  // >= 5 denials
  if(s0t())return!1;
  let M=h.autoModeEnvSetup;
  if(M?.dismissed)return!1;
  if(M?.dismissedAt&&Date.now()-M.dismissedAt<n0t)return!1;
  return!0 }

function Zge(){ return gZe() && yt().skillOverrides?.["auto-mode-setup"]!=="off" }
function gZe(){ return mZe() && Xu()?.envOnboarding===!0 }
function $Mr(){ return lC().envOnboardingEligible }
function s0t(){ if(T7())return Mpn("auto")||nkt(Dpn); return SHo() }
function SHo(){ return !["policySettings","userSettings","flagSettings"]
  .some((h)=>Se(h)?.skipAutoPermissionPrompt===!0) && !fe().hasSeenAutoModeEntryWarning }
```

The **whole local half was satisfied** and the screen did not draw.
MEASURED 2026-11-08, scratch `CLAUDE_CONFIG_DIR`, 120x40 tmux pane:
`numStartups: 40` against a floor of 5, `autoModeEnvSetup: {denials: 40}`
against a floor of 5, no `dismissed`, no `dismissedAt`,
`hasSeenAutoModeEntryWarning: true` — which is what makes `SHo()` false, so
`s0t()` does not bail — and auto mode on, the footer reading
`⏵⏵ auto mode on`.

`$Mr()` is **not a second gate**, which is worth saying because it reads like
one: it is a startup LATCH of the same function, set once from
`if($Mr()===void 0)uUt(h.getState().toolPermissionContext.mode==="auto"&&dO())`.
So `hHe` and `dO` are one condition, not two.

What is left is **one clause**: `Xu()?.envOnboarding === true` inside `Zge()`,
beside `mZe()`, which is the test for the `auto-mode-setup` skill being
available at all (`Brr()` = `!CLAUDE_CODE_REMOTE && Ide()`, and
`yts().value !== ""`).

MEASURED: no environment variable reaches it. The only
`CLAUDE_CODE_*ONBOARD*` name anywhere in the binary — including in the
bundle's own enumeration of its environment variables — is
`CLAUDE_CODE_POWERUP_ONBOARDING`, a different screen; there is no
`FORCE_ENV_ONBOARDING` and no `FORCE_AUTO_MODE`. Unlike the upsell, which has
`CLAUDE_CODE_FORCE_FULLSCREEN_UPSELL` and reads it *before* its own counter
clause.

**ASSUMED, not measured: that `envOnboarding` is an account-side flag.** The
reading above went one step too far on its first write and this is the
correction. `Xu` is a name this bundle reuses across chunks for at least three
unrelated functions — a settings-validation message formatter, a model-alias
resolver and a tool runner — and the one in `gZe`'s chunk scope was not
resolved, so "server-side" is an inference from the `Xu()?.<flag> === true`
shape and not a reading. What the decision here rests on is the measured
half: nothing local turns it on, so this box cannot draw the screen.

A row written off the bundle instead of a capture is exactly the guess
`knownscreen.go`'s header refuses, so there is none. Whoever's account draws
it can take the capture with the recipe in
`etc/herdr/agent-detection/README.md`, "claude — captures with no override".

### 4. `testdata/<agent>/` for an agent with no override: the third class

The bead names the path `etc/herdr/agent-detection/testdata/claude/<name>.txt`,
and the tree could not hold a file there. MEASURED 2026-11-08, with the two
captures planted and nothing else changed:

```
claude   blocked-fullscreen-upsell   blocked  ?  —  <-- FAIL (no manifest — …/claude.toml is missing)
claude   idle-composer               idle     ?  —  <-- FAIL (no manifest — …/claude.toml is missing)
verify-detection: 2/17 fixtures FAILED
```

That is ranger-base-j66o's clause, and it is right for what it was written for:
`testdata/<agent>/` outliving its manifest, an override deleted or moved aside
per the README's retirement recipe, whose fixtures used to be skipped silently.
It also reds `TestQAVerifyDetectionFailsACommittedRuleDeletion`'s **control**
arm, which is in arm 1 of `make test` — so the fixture could not ship at the
path the bead named.

Three options were weighed:

- **Write `claude.toml`.** A fork shadows upstream permanently and silently
  (the whole subject of "Retiring this override"), and §1 says there is no hole
  to close. A manifest written to satisfy a directory layout is the worst
  reason to take on that debt.
- **A second corpus root** beside `testdata/`, invisible to verify-detection
  the way `upstream/` is. Honest, but it splits one corpus and one key
  namespace in two, and `upstream/`'s own reason — herdr has no *kind* for the
  agent — is not true of claude.
- **Declare it per directory**, which is what landed. `testdata/<agent>/NO-OVERRIDE`
  says the absence is deliberate; the fixtures are still replayed and their
  state is still checked, against whatever herdr resolves, so they are a
  tripwire on herdr's own bundled claude manifest. The manifest SOURCE is
  reported in the row instead of required, because nothing of this repo's was
  staged for that agent and requiring otherwise fails every such fixture
  forever — which is the shape `upstream/` exists to keep out of this script.

Both directions are checked, which is what separates the declaration from a
skip: a declaration beside a manifest that exists is a FAIL, not an exemption.
`TestQAVerifyDetectionReplaysADeclaredNoOverrideAgent` pins three arms — break
a declared fixture's state and the run fails (replayed, not skipped); remove
the declaration and j66o's orphan failure comes back; write the manifest and
the stale declaration fails. It finds the declared agent by walking
`testdata/`, so a second one needs no edit there.

`scripts/d3-reader-eval.py` reads the same declaration in
`Herdr.declared_no_override`, for the narrower reason that its
`manifest answered from … not the checkout` warning would otherwise fire on
every run over a corpus it cannot apply to.

### 5. Corpus counts: said in a dozen places, now in rather fewer

The corpus went from 8 screens / 15 fixtures to 9 / 17, and the old numbers
were written into prose in `knownscreen.go`, `readingslog.go`,
`d3knownscreen_qa_test.go`, `scripts/readings-census.py`,
`scripts/d3-reader-eval.py` and ADR 0066. Where the count was not
load-bearing it is now gone rather than reset ("not one of the known screens",
"three of the shipped fixtures"), which is AGENTS.md's own doctrine about a
number said in two places. Where it is load-bearing it is updated: four rows
now have no silence key, not three.

**Dated measurements were left exactly as they are.** ADR 0066's 2026-10-04
amendment and 2026-10-05 ruling records say "the eight known screens" and "the
fifteen labelled fixtures", and so does `knownscreen.go`'s `MEASURED
2026-10-04` paragraph and `docs/notes.d/ranger-base-6uokf.md`. Those are true
statements about those dates and rewriting them would falsify the record. The
one ADR line that changed is in **Verification**, a live checklist a reader
runs: "exact on all fifteen fixtures" became "exact on every fixture the tree
ships", with both counts and the note that the arm enumerates the directory.

ADR 0066's other Verification bullet still reads as it did —
`scripts/d3-reader-eval.py` prints `2/2, 2/2, 7/7` and `0` under `kw prev FP`
after the two claude fixtures joined, because claude adds no residue for the
reason §1 gives.
