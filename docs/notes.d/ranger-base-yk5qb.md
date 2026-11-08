## A fresh account's first claude seat meets a notice nobody seeded (ranger-base-yk5qb)

MEASURED 2026-11-08, the ADR 0080 bring-up (ranger-base-offid leg 4): the
first seat the watch launched on the new host went straight to `blocked`.
Dispatch logged `never settled idle (status "blocked")` and `did not take the
launch`; the pane held Claude Code's `Try the new fullscreen renderer? 1. Yes
/ 2. Not now` for a service user who had run `claude` only once, to sign in.
The operator answered that one screen by hand and a fresh launch prompted and
worked.

Directory trust was not the problem and the seeder was not at fault. This is
a DIFFERENT class from the two screens posse answers: per **account** (per
config dir) rather than per repo, one marker key per notice, and the key is
written by the CLI when the screen is answered.

### 1. The key family, and why posse will not enumerate it

MEASURED 2026-11-08, claude 2.1.319 darwin-arm64, over one box's
`~/.claude.json`: **19** top-level keys are notice markers, by the rule
`^hasSeen | ^hasCompletedOnboarding$ | Dismissed$ | SeenCount$ |
AnsweredAt$ | Notice$ | Nudge$` (names only; no values were read):

```
additionalModelOptionsAnsweredAt  effortCalloutDismissed
effortCalloutV2Dismissed          fullscreenUpsellSeenCount
hasCompletedOnboarding            hasSeenAutoDefaultNotice
hasSeenAutoModeEntryWarning       hasSeenAutoModeOutsideReadPrompt
hasSeenTasksHint                  hasShownOpus46Notice
opus46FeedSeenCount               overageCreditUpsellSeenCount
passesUpsellSeenCount             promoStartupSeenCount
pushNotifUpsellSeenCount          rcLongTurnNudgeSeenCount
remoteControlUpsellSeenCount      subscriptionNoticeCount
transcriptShareDismissed
```

That is what one box has *met*, not the set that exists. The bundle's own
list of config keys carries more that this box has not drawn yet —
`subscriptionUpsellShownCount`, `fullscreenDownsellSeenCount`,
`voiceLangHintShownCount`, `voiceFooterHintSeenCount`,
`experimentNoticesSeenCount`, `seenNotifications`, `announcementImpressions`,
`seenSecurityWarnings`, `savedModelSuccessorShown` — and a release adds
notices under keys nobody has seen.

So the interstitial registry gets no row: `internal/posse/interstitial.go`'s
rows carry `Where`, `Key`, `Silence` and a `Probe`, and a row here could
honestly hold none of the last three. A permanently-unknown row in
`posse runtime check`'s grid is the sentence `bobSignInSilence` exists to
avoid writing, and `knownscreen.go`'s header already rules the same way for
the three codex screens with no silence key. The remedy is a bring-up step,
which is where it went (INSTALL.md §1 and §10).

### 2. `Not now` is not an answer: the gate is a counter

Read 2026-11-08 off the shipped bundle
(`~/.local/share/claude/versions/2.1.319`, `strings` then the minified
source). The upsell dialog's gate, last clause first:

```js
var qXt=3;
function bHe(){ …
  if((fe().fullscreenUpsellSeenCount??0)>=qXt)return!1;
  … return!0 }
function XLo(e){ let r=Bhr();
  if(r.upsellImpression!==void 0)return r.upsellImpression;
  let n=Math.min((fe().fullscreenUpsellSeenCount??0)+1,qXt);
  return r.upsellImpression=n,b(n,e),n }                 // persists n
```

`XLo` runs from the dialog's impression effect, so every *showing* increments
the count and writes it. The screen therefore draws up to **three** times:
answering `Not now` leaves it armed for two more launches, and on a
dispatched seat each of those is another blocked session. `Don't show again`
and `Yes, try it` are the answers that retire it. `CLAUDE_CODE_FORCE_FULLSCREEN_UPSELL`
is read in `bHe` BEFORE the counter clause and returns true outright, so it
is the lever a capture could be taken with on an account that has already
answered — three earlier bails in the same function still apply, so it is a
lever and not a guarantee.

A second screen of the same shape sits beside it in the bundle —
`Teach auto mode about your environment?` over `Auto mode works better when
it knows your environment. Takes about a minute.`, with the same
`Yes / Not now / Don't show again` — and posse runs claude seats in auto
mode, so it is in the path.

### 3. Scope left open

- A `KnownScreens` row would put this screen's NAME in the unrecognized-pane
  failure line instead of a bare `blocked` (ADR 0066 D3). It needs a capture
  under `etc/herdr/agent-detection/testdata/claude/`, which the tree has none
  of for claude, and every marker in that table is read off a shipped
  fixture. Filed rather than guessed: see the bead's handoff.
- The per-config-dir caveat: an interactive run answers the notices for the
  config dir *that run resolved*. A seat whose `CLAUDE_CONFIG_DIR` or `HOME`
  is not the one the operator answered in meets them again (ADR 0077 D1 pins
  which file a seat reads; ranger-base-c33j6 is the same split one screen
  over).
