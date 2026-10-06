# The one register kind that named no key, so the row's own truth went unread (ranger-base-orq4e)

2026-10-06. `TestAutostartShellDefaultRegisterIsTotalAndNotStale`
(`internal/posse/autostart_test.go`, arm 3) holds the half of the
documented-default census that shell reaches: every key
`plugin/autostart.sh` reads is either COMPARED against the seed line or
REGISTERED with a reason the seed line is not a claim about the script's
fallback. Its header says "Each kind is checkable, so the register cannot be
prose: a row that stops being true reds the pin that holds it."

That was true of two of the three kinds. The `seedShellNoFallback` arm's whole
body was `runStandDown(t, "")` — one global fact about an EMPTY config,
re-asserted once per registered key and mentioning no key. So the row's own
truth was never read, and any key at all could be registered there and escape
the comparison: the arm proved the arm switch disarms, which is a fact about
`autostart_interval` whichever row you are standing in.

MEASURED 2026-10-06, this box, at b8e3a336, before the fix:

    mis-register                in kind                       result
    autostart_max_beads         seedShellNotTheFallback       FAILS
    autostart_resume            seedShellNoDocumentedValue    FAILS
    autostart_resume            seedShellNoFallback           GREEN   <- the hole

And the whole defect ranger-base-m9mwc was filed to prevent, reintroduced
through it — `plugin/autostart.sh` raising its absent-key cap from 3 to 5, the
behaviour pins moved with the script as a developer legitimately would,
`autostart_max_beads` moved out of `seedShellDefaultCompared` and registered
as `seedShellNoFallback`, and `examples/config.yaml` left documenting 3:
`go test -tags posse_arm3 -run TestAutostart ./internal/posse` ok, `make
seed-check` ok, `make tree-check` all 14 doors green. Both halves of the
census green over a seed that documented a cap the script did not apply.

## The fix

The arm names the key now. `configWithout(armed, key)`
(`internal/posse/autostart_helpers_test.go`) takes the baseline config that is
already known to arm — `absent`, the control the other arms compare against —
and removes THIS key's line; the hook over it must arm nothing, quietly and
exit 0. For `autostart_interval`, today's only member, that is the empty
config and the hook stands down. For every other key the hook reads, removing
the line removes nothing — the baseline never named it — so the run arms the
loop with the fallback sitting in its argv, which is what a fallback IS, and
the row reds naming the key and the argv.

Not the alternative reading, "an absent key leaves the key's FLAG out of the
argv": that needs a per-key flag name, which is a second table and a second
parser of the script's argv builder. Removing a line and watching the whole
argv go away needs neither.

Checked per key, the arm proves the first half of the kind's sentence (absent
is a disarm) — the half that distinguishes it from every other key the script
reads. The second half (a value it cannot read is a broken arm, not a
defaulted one) stays where it already is, in
`TestAutostartBareIntervalIsRefusedNotDisarmed` and
`TestAutostartNullIntervalIsTheSameBrokenArmAsABareKey`, which name
`autostart_interval` outright. Holding it in the register too would mean a
third field in the row carrying a per-key unreadable value, and it would
discriminate nothing the removal above does not: every key the script falls
back for arms when its line is gone. A SECOND member of this kind is when to
revisit that — the same "at two members it would be a new mechanism"
threshold ranger-base-m9mwc left its own unbuilt option on.

MEASURED after the fix, same box: both mis-registrations above now red naming
the row, the full reintroduction reds on `autostart_max_beads`, and the pin is
green on the tree as it stands (1.5s).
