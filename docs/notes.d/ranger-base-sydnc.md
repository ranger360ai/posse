# `shasum` was not on the cutover host's PATH, and two guards read clean because of it (ranger-base-sydnc)

The bead was filed on a SUSPECT — the host's git 2.56 — over ten
commit-message/identity wall pins that failed there while CI's Linux and the
laptop were green. The suspect was wrong, the reds had two causes, and only the
second one is a hole in shipped code. Dated, because both halves moved under
the bead in one day:

| reds | cause | fixed by |
|---|---|---|
| the ten check-3 message/identity pins | the host's OS account name reaching check 3 | ranger-base-c7i8t |
| nine `TestQAHookFreshness*` + the test wrapper's self-test | **`shasum` is not on that host's PATH** | this bead |

## The hole

`shasum` is not on every box's PATH. The host's distribution ships it with
perl, at `/usr/bin/core_perl/shasum`, which is off the default PATH, and
carries coreutils' `sha256sum` instead. ubuntu and macOS carry both, which is
why CI and the laptop were green over this for as long as either script has
existed.

Two shipped scripts hashed by the one spelling the host does not have:

- **`scripts/verify-hook-freshness.sh`** — the L3 commit wall's detective
  control (ranger-base-8zki). `shasum: command not found`, so both sides of
  every compare hashed to the EMPTY STRING, the empty strings compared EQUAL,
  and the script printed `verify-hook-freshness: fresh` and exited 0 over a
  hook planted stale. On that box it passed every stale and every foreign
  hook, silently — the one outcome the file exists to prevent. Nine arms of
  `internal/treepins/hookfreshness_qa_test.go` said so on the host with "a
  stale body must be a finding (exit 1), got 0", which is how it was found.
- **`scripts/gotest.sh`** — the test-binary reuse wrapper (ranger-base-nw9zg).
  Its cache key IS a sha, so every build hashed to the empty string, every
  build named the same cache file (`<slug>-.test`), and the reuse branch handed
  back the FIRST binary ever cached for that package, forever after. A changed
  package's new tests never ran and a red test greened the wrapper. The
  poisoned entry is unprunable too: `<slug>-.test` does not match the
  `-[0-9a-f]{16}.test` that `--prune` groups by.

**Neither `set -e` nor `set -o pipefail` catches this class.** A command
substitution whose command is not found yields the empty string, and the status
of the assignment it feeds is discarded. MEASURED 2026-11-09 on this box, bash
3.2.57, in the exact shape `reuse()` had:

```
reuse() { local sha; sha=$(nosuchcmd -a 256 /etc/hosts | cut -c1-16); printf 'sha=[%s]' "$sha"; }
main()  { local bin; bin=$(reuse); echo "AFTER bin=[$bin]"; }
→ nosuchcmd: command not found / AFTER bin=[sha=[]] / script end / rc=0
```

`gotest.sh` carries `set -euo pipefail` and got past it; so did the host's own
run.

## The fix, and why it is shaped this way

Each script picks its hasher ONCE by probe (`sha256sum`, else `shasum -a 256`,
else a loud exit), PROVES it against a known digest before measuring anything,
and treats a digest that is not 64 hex characters as an error rather than as a
value. The three rules are one rule: **an empty hash must never compare equal
to anything, least of all to another empty hash.** `scripts/release-artifacts.sh`
already had the probe; it is the only other script here that hashes portably.

Where a loud exit cannot reach, the direction of failure is chosen:

- in `verify-hook-freshness.sh` the REFERENCE hashes are asserted 64-hex in the
  main body, so a box that cannot hash exits 2 ("nothing measured", never a
  pass), while a CANDIDATE that cannot be hashed reads empty against a 64-hex
  reference and is reported STALE — loud, and conservative. This is the other
  side of ranger-base-s8b4g's note, which warned against the matcher reporting
  the whole fleet stale; the answer to both is that neither side may be empty.
- in `gotest.sh`, `reuse()`'s own `die` only ends the command substitution it
  runs in, so the CALLER checks what came back and refuses to report a result
  for a binary the wrapper cannot name.

MEASURED 2026-11-09 on this box, with `shasum` hidden from PATH and `sha256sum`
left on it — the host's shape, built as a directory of symlinks to everything
the PATH holds except `shasum`, because on darwin both live in the same
directory and dropping it takes `git` and `mktemp` with it:

| arm | before | after |
|---|---|---|
| `gotest.sh --self-test` | 3 arms red, the same 3 the host reported | all 8 ok |
| planted stale hook, freshness | `fresh`, exit 0 | `prepare-commit-msg is STALE`, exit 1 |
| no hasher on PATH at all | `fresh`, exit 0 | exit 2, "nothing measured" / exit 2, names both spellings |

## The pins

`internal/treepins/hashportability_qa_test.go`, four live arms and one control.
Measured both ways: against the pre-fix scripts the four red (the control, a
fresh box, stays green — it is a control), and against the fixed scripts all
five pass in 9.7s. A census of `shasum` spellings under `scripts/` would have
caught this one too, but it would be green over the script that already probes
and red over the darwin-only instruments that need not — a rule about text in
place of the behaviour. The census is ranger-base-uxadf, a cheap second door —
and it is a tree-wide pin by internal/treepins' own rule, so it costs a
Makefile door and a register line, which is why it is not folded in here.

## What this bead also eliminated, and should not be re-asked

Both were measured out on 2026-11-09 before the host reading landed, and both
cost a session:

1. **It is not git 2.56.** All 142 pins of the commit-wall cluster pass under a
   Homebrew git 2.56.0 bottle poured into a scratch keg here, relocated with
   `install_name_tool` and ad-hoc re-signed — arm 1 77/77, arm 2 45/45, arm 3
   20/20, 0 fail, 0 skip. The same pins are green under 2.50.1 at the same
   base, and all ten premises the message arm rests on read identically under
   the two gits (`docs/adr/0083-commit-wall-host-premises.probe.sh`, section
   C). One real 2.56 fact, kept because it has a date on it: **`core.commentChar=auto`
   is deprecated as of git 2.52.0** and documented for removal in Git 3.0,
   with "incompatible with adding comments to the commit message in the
   `prepare-commit-msg` hook" among the stated reasons. It still works on 2.56,
   so the arms reading it are not wrong today, but `commentcharauto_qa_test.go`
   and `commentcharautodetect_qa_test.go` have an expiry now.
2. **It was not a tree-colliding identity literal** in the shape first
   suspected. `go test -overlay`, one line changed in `DeriveIdentityLiterals`:
   the repository's own name as the username reds 29 of that cluster, not ten,
   including every one of arm 3's identity pins that the host reported green.
   The literal was a live hazard on its own account all the same — 862 of
   1,618 tracked paths and 1,522 of 1,618 tracked files carry that name — and
   ranger-base-87jw1 landed the exclusion for it. The host still reported the
   same eleven after that landing, and they went green at ranger-base-c7i8t,
   which reached the same account name through the visibility check instead.
   So the account name WAS the cause of those ten; the shape it arrived in was
   not the one this bead measured. That is the whole value of the overlay
   reading: it is why nobody spent the session after c7i8t re-asking it.
