## Two tests that were really measuring the box (ranger-base-rstk)

`ci.yml`'s first run on main was red on `ubuntu-latest` and green on
`macos-latest` (33218587437, 2026-08-28), and `make test-linux` was red at the
same HEAD. Neither was a product defect and neither was distro variance: both
were pins reading something off the machine they ran on. The fixes are in the
fixtures, and each mechanism now has an arm that fails on **every** platform —
which is the part worth keeping, because the original defects were invisible on
the box every persona develops on.

**1. An identity borrowed from the hostname.** The queue-cutover pins drive
`scripts/queue-cutover.sh`, whose last step commits in the queue repo. That repo
is a `git clone`, and a clone carries no local config, so the commit fell
through to git's identity auto-detection — which succeeds wherever the hostname
has a domain part and fails where it does not:

```
fatal: unable to auto-detect email address (got "root@d9b2c1bf2bfd.(none)")
```

Hostname-shaped, not OS-shaped: latent on any box, including a future macOS
runner. `qcEnv` now hands the script an identity of the fixture's own in a
config of our own, with the box's global and system config — and
`GIT_AUTHOR_*`/`GIT_COMMITTER_*`/`EMAIL` — cut out of the environment.

The instrument that makes this measurable anywhere is `user.useConfigOnly`:

```
git -c user.useConfigOnly=true commit …
# fatal: no email was given and auto-detection is disabled
```

It switches the guessing OFF, so a fixture that loses its identity fails on a
laptop exactly as it fails on a domainless runner. That one line turns "red only
on ubuntu-latest" into a local, one-second reproduction — and it is how the same
class can be swept anywhere else it turns up.

**2. A prescription pasted under the wrong HOME.** `posse gates install-hooks`
prints its chain prescription with `AbbrevHome` applied, so a repo under `$HOME`
is prescribed as `cd ~/…` — right for the operator, who pastes it into the shell
that printed it. `gateschain_qa_test.go` printed the block under one HOME and
pasted it under another. On darwin nothing was ever abbreviated (`t.TempDir()`
lives under `/var/folders`, `$HOME` does not) so the mismatch was invisible; on
`ubuntu-latest`, where `HOME=/tmp` holds the temp dirs, the `cd` resolved to a
doubled path that does not exist.

**And `sh` does not stop for a failed `cd`.** The rest of the block is relative
to it, so the hook files were written into whatever directory the test binary
was standing in — this package's own source directory. A writable checkout
absorbs that in silence; `make test-linux`, whose whole guarantee is that
`/repo` is mounted read-only, is where it finally spoke, as
`cannot create pre-push: Read-only file system`.

Two arms, both platform-independent: the fixture repo now lives **under** the
HOME the run shares, so the abbreviated `~` form is what every platform
exercises; and `qaSh` runs the block in a scratch directory of its own that must
be **empty** afterwards. Asserting on the leak rather than on the mount is what
makes a darwin run red for the same bug — the general shape being that a script
whose first line is a `cd` must be pinned by where it *wrote*, never only by its
exit status.

Both were found by costing CI rows for platforms the gate does not reach
(`docs/runbooks/ci-platform-coverage.md` §4, ranger-base-4fxz); the third
finding recorded there — `TestBackfillDoesNotFailTheListing` red only as root,
where the 0444 its fixture depends on stops meaning anything — is untouched and
is not a defect.
