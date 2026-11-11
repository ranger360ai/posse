## The disk explainer knew one errno, and this box writes the other (ranger-base-akk9f)

Filed from ranger-base-r5tnw, which lost a full-suite reading to it.
ranger-base-krra built `scripts/test-times.sh`'s DISK machinery for exactly one
failure: a box red that reads like a broken diff. It caught ONE spelling, and
this box produces another.

MEASURED 2026-11-11, jian-yang worktree for ranger-base-r5tnw, linux 7.2.15
x86_64, go1.27.3, loadavg 5, both suite slots held by sibling seats. `make
test` came back exit 2 with arm 1 fully green and arm 2 red on four tests,
every red an EDQUOT write and not one of them related:

    TestListingAndSweepAgreeOnWhatOneTreeHolds   worktree_test.go:2390
      write .../003/README.md: disk quota exceeded
    TestCLIStillKnowsTheMode/claude              unattended_live_test.go:95
      mise ... failed copy: Disk quota exceeded (os error 122)
    TestQAParityAccountRefusalIsNamedOnEveryRuntime/grok
      turnoutcome_qa_test.go:120 (the write underneath was EDQUOT)
    TestTwoFragmentBranchesLandBackToBack/neither_ran_the_generator
      landgeneratedindex_test.go:186  git add: unable to write loose object
      file: Disk quota exceeded
    plus: testing: can't write /tmp/go-build.../testlog.txt: disk quota exceeded

That is krra's shape precisely — unrelated names through the house filter,
reading as a broken change — and BOTH halves of krra's answer were silent:

1. `explain_disk` grepped the literal `no space left on device`. EDQUOT prints
   `disk quota exceeded`, so the block did not fire. The log went straight from
   FAIL to the package-times table with no cause named anywhere.
2. The pre-run DISK line printed `5915 MB free on /tmp`, above
   `DISK_FLOOR_MB=5120`, so it did not even warn. It cannot: free space is not
   the binding constraint when a quota is.

### It is one box-wide budget, not "your" quota

MEASURED 2026-11-11, `/proc/mounts`:

    tmpfs /tmp tmpfs rw,nosuid,nodev,size=14247372k,nr_inodes=1048576,
          inode64,huge=advise,usrquota 0 0

`usrquota` is per UID, and every seat on this box runs as the same user
(`posse`). So the "per-user" quota is ONE BOX-WIDE budget that all concurrent
suites spend together — which is why a sibling seat can red your arm with
writes you never made, and why `df` is not merely unhelpful but actively
misleading: it reports the DEVICE (14G, 2.8-3.2G free throughout) while the
pool a test may write into is already gone. ~5G of it was sibling seats' live
`posse-testhome-*` trees (2.3G + 1.9G + 915M, three concurrent suites).

Re-measured with both slots held by live siblings (dinesh/ranger-base-xpxo6,
gwart/ranger-base-4kars): `make test-arm2` queued 553s for a slot, then red at
63.2s on `hookwallsweep_qa_test.go:293` (`git commit: unable to write loose
object file: Disk quota exceeded`); `make test-arm3` red at 36.6s on five more
(`planguardpark_test.go:170` and `:198`, `plan429backoff_qa_test.go:434`,
`rollingrun_qa_test.go:112`, cagerender). The explainer stayed silent both
times. The trees holding the space were 1-2 minutes old, so they were LIVE
siblings and not leaked residue — nothing to clean, and test-times.sh is right
that clearing is not a seat decision.

### The workaround, and what it costs

MEASURED 2026-11-11, same worktree, `TMPDIR` on /home (930G ext4, no quota):

    make test-arm2  -> ok, internal/posse 71.1s. Green, where the same arm on
                       /tmp had red four times. The quota was the whole of
                       those reds.
    make test-arm3  -> FAIL, and exactly TWO tests, both artifacts of the
                       workaround and not of anything in the tree:
                         TestQAGrantStillReachesTheGitConfigInEveryShape/deniesFiles
                           seatbeltgitidentity_qa_test.go:281
                         TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch
                           seatbelttelemetry_qa_test.go:76
                       Run those two alone on the house default and both pass
                       (0.022s).

`TMPDIR` and never `GOTMPDIR` — krra's trap: `cmd/go` hands the test binary
`GOTMPDIR` as its own `TMPDIR`, so setting `TMPDIR` alone moves the build dir
and every `t.TempDir` together, while setting `GOTMPDIR` greens a full-disk
repro.

Why those two pins, and it is NOT a product bug: the writable-set builder
honours the variable (`internal/posse/seatbelt.go:709` reads
`os.Getenv("TMPDIR")` and adds it when non-empty, with an unconditional add of
`/tmp` at `:723`). Both pins build their grant set in a hermetic environment
while `t.TempDir()` still lands under the OUTER `TMPDIR`, so with `TMPDIR`
moved the fixture sits outside every granted root and the premise the pin needs
is gone. Their failure messages say so plainly — the set they print ends in a
bare `/tmp` while the file they want covered is under /home. On every normal
run the two agree and the assumption is invisible. One level up: those two pins
cannot be read at all on a box whose `/tmp` is too full to run them.

### What was built

- `explain_disk` matches BOTH spellings, case-insensitively, and says which one
  fired. Case-insensitively because the two writers disagree: go's syscall text
  is lower case and the Rust tool on the same run printed a capital D. Both
  were in the one arm-2 log, so the `--self-test` fixture carries both and the
  arm that counts 2 is the arm that proves the case folding.
- The advice is now per-errno, because it differs. ENOSPC sends the reader to
  `df` and `du` on GOCACHE; EDQUOT sends them to `du` on the sibling
  `posse-testhome-*` trees, says the budget is shared so the writes may not be
  theirs, says `df` will look fine, and prints the `TMPDIR` workaround with the
  two-pin cost above. A quota red sent to `df` is worse than silence: `df`
  answers that the box has room, and what the reader learns is that the
  explainer is wrong about its own box. So the two arms that carry this bead
  are the DISCRIMINATIONS in both directions — an EDQUOT log must not print the
  ENOSPC header, and krra's own fixture must not grow a quota one.
- The DISK preflight says its number is the DEVICE. Where `/proc/mounts` names
  a quota option for that filesystem it says which (`/tmp is mounted
  usrquota`); where that file does not exist — darwin, where this script also
  runs — it says the generic sentence. It probes no quota: there is no `quota`
  binary on this box, and a usage/limit reading was out of scope for this bead.
- `internal/treepins/suitedisk_qa_test.go` requires the new arm labels by name,
  so deleting the guard and its arms together still reds the suite.

Deliberately NOT done: no floor on a quota, and no red on one. A floor is a
claim about a box, which is the class of red this whole script exists to
explain rather than to throw — and under a shared `usrquota` the number a floor
would read is not even this run's to spend.

One known false-positive path, pre-existing and widened by one pattern: a log
that QUOTES these strings fires the block. The only producer in this tree is
this script's own `--self-test`, whose output `go test` prints only when
`internal/treepins` arm 2 fails. The block is advisory and never touches the
exit status, and it ends by telling the reader to believe none of the reds
until the suite has re-run, so the cost is a paragraph and not a wrong verdict.
