## A nesting refusal wears the setuid refusal's face, and one seat can see both after all (ranger-base-26y03)

`TestQASelfCheckAnswersInsideASeatbelt` (internal/posse/proctablecage_qa_test.go,
landed under ranger-base-yxmwx at aa9190ca) FAILED — not skipped — on exactly
the seat its subject exists to serve: a seatbelt-caged one. The arm runs
`go run`'s moral equivalent inside `sandbox-exec` to prove `SysSelfOrphans`
answers from in there, and macOS will not nest a seatbelt, so from a caged seat
the child launch dies at `sandbox_apply: Operation not permitted`, exit 71,
before it execs anything.

The arm anticipated this in prose and still could not catch it, which is the
part worth keeping. Its control is:

```go
if out, err := exec.Command(sandboxExec, "-p", qaSeatbeltAllowAll, "/bin/ps", ...).Output(); err == nil && len(out) > 0 {
        t.Skip("this box's sandbox does not refuse the setuid exec of /bin/ps")
}
```

That reads the nested `/bin/ps` **succeeding** as "the refusal this arm pins is
gone, nothing to measure". A nesting refusal fails the same command for an
unrelated reason, and the control reads a failure as "the refusal is live, keep
going" — so it waves the caged seat through to die one step later, on the error
it was already holding in its hand. **A control that distinguishes two outcomes
of one command cannot also distinguish two causes of one of them.** The
applicability question has to be asked before the probe, not inferred from it.

The fix is one line: `sbSkipUnlessSandboxable(t)` at the head of the arm
(internal/posse/seatbeltapply_qa_test.go, ranger-base-xjw9 — a probe that
cannot apply a sandbox must SKIP). That file carries no build tag, so it was
already linked into arm 1 beside this one; nothing else moved.

### Why the close could not see it, and why that was not inevitable

ranger-base-26y03 reasons that "a seat that can run `sandbox-exec` by hand, as
gilfoyle's close did, is by construction a seat that is NOT itself caged, so
the close's verification route and the failing route are mutually exclusive and
one seat cannot see both." The first half is right and the conclusion is not —
**an uncaged seat can be the caged seat, one level down**, which is the same
move the arm under repair makes:

```
$ go test -c -o $S/posse.test ./internal/posse
$ printf '(version 1)\n(allow default)\n(deny file-write* (subpath "/private/var/empty/nothing"))\n' > $S/caged.sb
$ sandbox-exec -f $S/caged.sb $S/posse.test -test.run TestQASelfCheck -test.v
```

MEASURED 2026-10-01, gilfoyle seat, `RHQ_CAGE=shims` (so uncaged by seatbelt),
darwin 25.4.0, worktree at aa9190ca. Before the fix that recipe reproduces the
reported failure verbatim — `proctablecage_qa_test.go:79`, `sandbox_apply:
Operation not permitted`, exit status 71 — while the same binary run directly
passes. After it, the caged run skips with xjw9's NOTHING MEASURED line and
**the direct run still PASSES rather than skipping**, which is the half of the
verification that matters: the door must not cost the uncaged seats the arm.
The same nested vantage shows the contrast the bead reports from a natively
caged seat, same cause, same moment:

```
TestQAWorktreeGrantRefusesSharedGitStateUnderSandboxExec  --- SKIP (sbSkipUnlessSandboxable)
TestQAWorktreeCommitStaysGreenUnderTheNarrowedProfile     --- SKIP (sbSkipUnlessSandboxable)
TestQASelfCheckAnswersInsideASeatbelt                     --- FAIL (sandbox_apply)   [before]
                                                          --- SKIP (sbSkipUnlessSandboxable)  [after]
```

So the lesson is not "one seat cannot see both". It is that **a sandbox-exec
arm needs verifying from under a sandbox, and the cage is three lines of
profile away on any seat that can apply one.** A seat that is natively caged
genuinely cannot — it is the one that reports the bug.

### The over-skip this accepts, on purpose

`sbSkipUnlessSandboxable` probes with `sandboxApplyProbeProfile`, which carries
a deny, because xjw9's grid measured that a deny anywhere refuses: under a
lenient allow-default outer wrapper a bead's one-line allow-default probe
reports "sandboxable" while every profile the shop actually renders is refused.
This arm's own apply is allow-default, so in that one corner — row two of the
grid, a lenient outer wrapper, which is not the shape of a real cage — the door
skips an arm whose apply would have gone through. That is a lost measurement,
not a false green, and it is the cheaper error: a second applicability reader
of our own, shaped to this arm, is how you get a suite that skips where the
code under test is still wrong (seatbeltapply_qa_test.go's own header makes
that argument, and ranger-base-heur moved the question into production for it).
One reader of "may this process apply a seatbelt", used by everything whose
measurement is a sandbox-exec.

### The one-line fix cost a second line, in another file

`make test-arm1` red on `verify-parallel`, not on a test:

```
testparallel: 1 test(s) carry t.Parallel that this tool would not give:
  proctablecage_qa_test.go	TestQASelfCheckAnswersInsideASeatbelt
```

`sbSkipUnlessSandboxable` reads `sandboxApplyRefusal`, a package-level func var
with a writer in the test tree, so adding the call made a parallel test reach a
written package var for the first time — cmd/testparallel's whole subject. The
clearance that covers the other three readers (ranger-base-btdvw's map, keyed
per test and never per file) argues that the seam's one writer,
`rchFakeApplyRefusal`, is serial, and Go resumes paused parallel tests only
after the sequential pass finishes. Re-read 2026-10-01 rather than inherited:
all three callers — `TestQARecordReachAbstainsWhenThisProcessMayNotApplyAProfile`,
`…AbstainsBeforeItRendersTheProfile`, `…StillReportsAFailureThatIsNotAnApplyRefusal`
— carry no `t.Parallel`, and reachability_qa_test.go sits under the same
`!posse_arm2 && !posse_arm3` tag, so within this arm the reader has no
concurrent writer at all. So the fourth line says `calls sandboxApplyRefusal`
for the same reason the first three do.

Worth knowing before you reach for this door elsewhere: **`sbSkipUnlessSandboxable`
is not free of gate consequences.** Any parallel test that grows the call grows
a `cmd/testparallel` clearance with it, and that gate runs ahead of the suite —
so it reds in seconds, before the ~950s package, which is the pleasant version
of this surprise.
