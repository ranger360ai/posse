## The release carried the refusal and not the way out of it (ranger-base-mhv7j)

ADR 0060 D2 is one sentence: posse ships the filing, the operator sends it.
Every detection door said so, naming
`etc/herdr/agent-detection/upstream-bob.md`, and the sentence was true of the
repository and false of every artifact the repository publishes. Github
issue #4 is the operator meeting that on the work box: a dispatched bob seat
is `agent_not_found`, `posse prompt` refuses the unlabelled pane, and the
file the refusal told him to send was not on his machine.

### What the release actually carries

MEASURED 2026-10-09, `scripts/release-artifacts.sh --rev cddad3ad --version
v0.5.1`, darwin 25.4.0 / arm64. The Homebrew keg, unpacked out of
`posse-0.5.1.arm64_big_sur.bottle.tar.gz`:

```
posse/0.5.1/bin/posse
posse/0.5.1/share/doc/posse/INSTALL.md
posse/0.5.1/share/doc/posse/README.md
```

Three files, and that is not an oversight to correct: the keg has to agree
with the formula's own `def install` (`bin.install "posse"`,
`doc.install "README.md", "INSTALL.md"`), which `tapformula_qa_test.go`
pins, or a poured install and a source one differ silently. The tarball
beside it adds LICENSE and nothing else. So `etc/` was never on that box and
making it so would have moved the path out from under the door's sentence
anyway — a bottle would put it at `$(brew --prefix)/share/posse/…` and the
door would have to render environment-dependent prose.

### Why the pin was green through all of it

`detectiondoor_qa_test.go` has held, since ranger-base-ecchw, that a filing
a door names is a file in the tree. It was true the whole time. The reader
of that door has no tree: that is the entire defect, and it is the same
shape as ecchw one layer out — a path in a sentence, checked against
something that is not what the reader has.

So the pin reads both copies now: the checkout, because a persona standing
in one follows the path, and `posse.Filings`, because that is the copy every
release has. The second reading fails on a declaration that is in the tree
and outside the embed directives — verified by pointing bob's
`DetectionFiling` at `README.md`, which is in that directory and matches no
directive: both the carried check and `FilingFor` red, and the tree check
stays green, which is the combination the defect produced.

### The cost of embedding

MEASURED 2026-10-09, `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"`,
darwin/arm64, same box, HEAD before and after:

| binary | bytes |
|---|---|
| before (03fba176) | 9,682,338 |
| after (cddad3ad) | 9,781,570 |
| delta | +99,232 (+1.02%) |

The keg's own binary is byte-for-byte that 9,781,570 — the release's extra
`-X …Build=cddad3ad` costs nothing measurable. 12 files are carried:
five `upstream-*.md` covering notes and the seven-file `upstream/bob/`
package. The local overrides (`codex.toml`, `grok.toml`) and `testdata/` in
the same directory are deliberately NOT embedded — `make install-detection`
and `make verify-detection` are checkout-only by nature and a release binary
can do nothing with either.

### Verified by pouring it

The keg's own `posse`, run from `/tmp` with `RHQ_HOME` pointed at a
directory that does not exist — no checkout, no instance:

```
$ .../keg/posse/0.5.1/bin/posse runtime filing bob --out .../poured
wrote the bob upstream detection filing to .../poured (8 files, ADR 0060 D2)
…
send upstream-bob.md with the rest attached; nothing here has been published
```

All eight files `diff` clean against the tree. The same binary at HEAD~1
answers `usage: posse runtime check|probe <name>`, which is what the
operator had.

### What this does not close

Three repo-relative doc paths posse prints are still repo-relative, and the
same reasoning applies to them on a poured install:
`etc/herdr/agent-detection/README.md` (the no-filing branch's authoring
page) and `docs/runbooks/agent-detection-manifest.md` (the declared
runtime's door). They are pages to READ, not material to send, so a URL
would serve where a verb is needed here, and a URL pins a branch name and
answers to this repo's visibility rules — a decision, not a sweep. Filed as
**ranger-base-x8bv0** (`-l code`, discovered-from this bead) rather than
folded in. The carried-copy reading in `detectiondoor_qa_test.go` excludes
`README.md` by name for exactly as long as that bead is open.
