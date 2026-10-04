# Is a tree pin anyone else's practice? A survey of the field, and the public wording (ranger-base-jyuid)

Spike, 2026-10-04, one seat, tree `c202af14`. Operator ask: *"concerned if we
publish this no one else will know what a treepin is … are any other AI
harnesses converging on the same idea?"*

Every claim below is **MEASURED** (the URL was fetched and read on 2026-10-04,
or the number came from a command run at `c202af14`) or **UNKNOWN** (looked
for, not found). Nothing here is a recommendation stronger than the evidence;
the two judgement calls — the public name and whether the package moves — are
the operator's, filed as a question bead at the end.

## 1. What posse actually has (the thing being named)

`internal/treepins` holds tests whose fixture is the repository tree. MEASURED
at `c202af14`:

| fact | value |
|---|---|
| test files in `internal/treepins` | 68, and no non-test file — the package has no doc comment an outsider could read |
| files citing a bead id or an ADR number in their source | 65 of 68 (the three without: `root_test.go`, `execwrite_test.go`, `notesindex_qa_test.go`) |
| files that enumerate the tree (`WalkDir`/`Walk`/`ReadDir`/`Glob`) | 23 |
| files that exec a command (gofmt, python, git, the built binary) | 50 |
| Makefile doors under `make tree-check` | 14 (`fmt-check` … `pid-check`); `make treepins` is the unfiltered run |
| how the crew spells it in prose | `treepins` 221 (nearly all the package path), "tree-wide pin(s)" 28, `TreePin(s)` 10, "tree pin(s)" 5, **"treepin" as a standalone word: 0** |

So the bead's title word, `treepin`, is a package name; the crew's prose term
is *tree-wide pin*. The practice has four properties, and the survey is a
search for each:

1. **The fixture is the tree.** `TestMain` chdirs to the module root and a
   pin reads docs, Makefile, ADRs or source through a plain relative path.
2. **The test cites the decision it guards.** Not as a courtesy: ADR 0051
   ("Naming a source file") sets the citation convention and
   `adrtestcitation_qa_test.go` fails when an ADR names a Go file that is not
   there.
3. **A pin must name the process it protects** (ADR 0006 §7, operator ruling
   2026-09-10): a new prose-reading pin is filed only with the answer to
   "what process behaves differently if this pin is deleted?" The ruling
   records the failure it ends — "one pin that encoded a live bug as its
   fixture so the fix broke the test".
4. **Every pin has a door.** `treewidedoor_qa_test.go` reds when a tree-wide
   pin has no `make <x>-check` target, because a `-run` filter never names a
   test that is nobody's subject (ranger-base-rulbl).

## 2. The field outside AI

| name used | who | what it checks | cites the decision? | tooling | source (fetched 2026-10-04) |
|---|---|---|---|---|---|
| **architectural fitness function** | Ford, Parsons, Kua, *Building Evolutionary Architectures* (2017; 2nd ed. 2022) — "any mechanism that provides an objective integrity assessment of some architectural characteristic(s)" | layering, dependency direction, naming, coupling, run in CI | optional — ArchUnit's `.because(...)` carries a reason string; the ADR link is community advice, not a rule | ArchUnit (Java; Thoughtworks Radar *Assess* May 2018 → *Trial* Nov 2018), ArchUnitTS, NetArchTest, dependency-cruiser, arch-go, go-arch-lint, Spring Modulith, Protean `protean check` | [thoughtworks.com/radar/tools/archunit](https://www.thoughtworks.com/radar/tools/archunit); [docs.proteanhq.com](https://docs.proteanhq.com/guides/architecture-fitness-functions/) |
| **ADR + fitness function** ("decisions as code") | the `architecture-decision-record` GitHub repo; the `software-architecture-guide` repo | "A decision record documents the decision, while a fitness function assures the decision"; "Write an ADR to record *why* a constraint exists; write a fitness function to *enforce* it" | the pairing is stated; **neither says the test should reference the ADR** | ArchUnit, ArchUnitTS; "ADR Guard" action (fails a PR when watched paths change without an ADR) | [README](https://raw.githubusercontent.com/architecture-decision-record/architecture-decision-record/main/README.md); [fitness-functions.md](https://raw.githubusercontent.com/janmarkuslanger/software-architecture-guide/main/architecture-metrics/fitness-functions.md) |
| **Architectural Decision Enforcement (ADE)** | Raphael Schellander, OST MSE thesis, 2026 | a tool-agnostic DSL; "rule files, located alongside ADRs, express enforceable constraints"; 17 ADRs → 9 enforceable → 44 generated checks | **yes, by construction** — the rule file lives beside the ADR | CLI with Go, .NET and *filesystem* plugins | [eprints.ost.ch/1381](https://eprints.ost.ch/id/eprint/1381/) |
| **architecture conformance checking** / **architecture erosion** | the research field (surveys 2012, mapping study arXiv 2112.10934) | implemented architecture against intended architecture; strategies named: conformance, design enforcement, "architecture to implementation linkage" | no — compares models, not records | Lattix, ArCh, ConQAT, ARCADE, Dclcheck | [arxiv.org/pdf/2112.10934](https://arxiv.org/pdf/2112.10934) |
| **convention tests** | TestStack.ConventionTests (.NET) | reflection over assemblies: "all classes with a certain naming convention are always defined in the correct namespace" | no | NuGet library | [teststackconventiontests.readthedocs.io](https://teststackconventiontests.readthedocs.io/) |
| **dependency policy as a test** | the Go repository, `src/go/build/deps_test.go` | "depsRules defines the expected dependencies between packages in the Go source tree. **It is a statement of policy.** DO NOT CHANGE THIS DATA TO FIX BUILDS. … Negative assertions should almost never be removed." Also `cmd/api`: once `api/go1.N.txt` exists "silent API additions stop being permitted"; and `test/fixedbugs/issueNNNNN.go`, tests named for the issue they guard | the policy header cites no issue or design doc; the *fixedbugs* naming is the universal regression-test citation | plain `go test`, in the ordinary suite | [deps_test.go](https://raw.githubusercontent.com/golang/go/master/src/go/build/deps_test.go) |
| **tidy** / **verify** / **presubmit** | rust-lang (`src/tools/tidy`, "the Rust project's custom internal linter … infrastructure, policy, and documentation", runs on every `./x test` and every PR); Kubernetes `hack/verify-*.sh` (`make verify`); Chromium `PRESUBMIT.py` | no binaries checked in, licences, platform-specific code policy, error codes documented; generated files current | per-check comments; UNKNOWN whether they cite a decision record | bespoke, one per project; `repolinter` (archived early 2026) → `alint`; RepoProof | [tidy README](https://raw.githubusercontent.com/rust-lang/rust/master/src/tools/tidy/Readme.md); [k8s hack/README](https://raw.githubusercontent.com/kubernetes/kubernetes/master/hack/README.md) |
| **policy as code** | OPA/conftest (Rego over structured config); Semgrep custom rules; ast-grep; Danger (Dangerfile over PR metadata) | config and AST patterns; PR hygiene | Semgrep's `metadata.references` can carry a link and `message` is to say "why … and how to remediate"; conftest has no such field | conftest, semgrep, ast-grep, danger | [semgrep rule syntax](https://docs.semgrep.dev/writing-rules/rule-syntax) |
| **characterization / golden-master / pinning tests** | Feathers, *Working Effectively with Legacy Code* (2004) | **freezes the behaviour the code has today**, warts included, so a refactor cannot change it unnoticed | no — by definition there is no decision, only behaviour | approvals/snapshot libraries; `pytest-pinned` | [Wikipedia: Characterization test](https://en.wikipedia.org/wiki/Characterization_test) |
| **Docs as Tests** | Manny Silva, Doc Detective (FOSDEM 2025) | every procedure and snippet in the docs is run against the product | no | Doc Detective | [fosdem.org 2025](https://fosdem.org/schedule/event/fosdem-2025-4928-no-more-broken-docs-keep-docs-accurate-with-doc-detective) |

## 3. AI coding harnesses and agent conventions

| harness | what it offers against drift | a repo-level test pattern? | cites the decision? | source (fetched 2026-10-04) |
|---|---|---|---|---|
| Claude Code | `CLAUDE.md` ("advisory"); "Give Claude a way to verify its work" — tests, build exit code, linter, "a script that diffs output against a fixture"; hooks are "deterministic"; a Stop hook "blocks the turn from ending until it passes" | no named pattern; the advice is *have a check* | no | [best-practices](https://code.claude.com/docs/en/best-practices.md) |
| Cursor | rules in the prompt (forum: "models still may choose to ignore them"); **Enforcement Hooks** "block commands before execution … regardless of what the model decides" | no | no | [cursor.com/docs/hooks](https://cursor.com/docs/hooks) |
| OpenAI Codex / `AGENTS.md` (with Amp, Jules, Cursor, Factory, Roo) | an instruction file; Codex hooks (`PreToolUse`/`PostToolUse`) | no | no | [ampcode.com/news/AGENTS.md](https://ampcode.com/news/AGENTS.md) |
| OpenAI "Harness engineering" (Feb 2026; the primary URL returned 403 on 2026-10-04 — quoted via Fowler and Guo) | "architectural constraints … monitored by LLM-based agents and deterministic custom linters and structural tests"; lint messages "double as remediation instructions"; `AGENTS.md` "a table of contents, not an encyclopedia" over `docs/`; "garbage collection" agents that "find inconsistencies in documentation or violations of architectural constraints" | **yes — "structural tests" and custom linters over the repo**, generated by the agent | the message carries the *fix*; UNKNOWN whether it carries a decision id | [martinfowler.com memo](https://www.martinfowler.com/articles/exploring-gen-ai/harness-engineering-memo.html); [ignorance.ai, 2026-02-22](https://www.ignorance.ai/p/the-emerging-harness-engineering) |
| GitHub Copilot coding agent | `copilot-instructions.md`, `copilot-setup-steps.yml` | no | no | [github.blog, 2025-07-31](https://github.blog/2025-07-31-onboarding-your-ai-peer-programmer-setting-up-github-copilot-coding-agent-for-success/) |
| Devin · Aider · OpenHands · Jules · SWE-agent | Knowledge base · `CONVENTIONS.md` · `.openhands/microagents/repo.md` · `AGENTS.md` · UNKNOWN | no | no | [docs.devin.ai](https://docs.devin.ai/onboard-devin/knowledge-onboarding); [docs.all-hands.dev](https://docs.all-hands.dev/usage/prompting/microagents-repo) |
| GitHub spec-kit `constitution.md` | "project-wide constraints document … every AI agent in the workflow must respect them" | no — prose | n/a | [github.com/github/spec-kit](https://github.com/github/spec-kit) |
| **ContextCov** (Reshabh K Sharma, arXiv 2603.00822, 2026-02-28) | "compiles documented constraints into three complementary checks: static AST queries for code patterns, runtime shell shims that intercept prohibited commands, and architectural validators"; 88.3% adherence vs 67.0% prompt-only on SWE-bench Lite | **yes — AGENTS.md constraints become executable checks** | traceable to the instruction line, not to a decision record | [arxiv.org/abs/2603.00822](https://arxiv.org/abs/2603.00822) |
| Thoughtworks Radar v34 (Apr 2026) | *Architecture drift reduction with LLMs* (Assess): "drift compounds as agents and humans replicate existing patterns, including degraded ones"; Spectral, ArchUnit, Spring Modulith. *Feedback sensors for coding agents* (Trial): compilers, linters, "structural tests", test suites, run "before a commit is made" | yes — "structural tests" as a sensor | no | [radar: drift](https://www.thoughtworks.com/radar/techniques/architecture-drift-reduction-with-llms); [radar: sensors](https://www.thoughtworks.com/radar/techniques/feedback-sensors-for-coding-agents) |

Practitioner writing, 2026, in date order — this is where the convergence is
visible:

- **Alexandre Castro, 2026-04-06** — "every ADR that isn't explicitly
  deprecated or superseded should map to at least one fitness function";
  the agent reads the ADRs to review a PR. No test cites an ADR id.
  ([platformtoolsmith.com](https://platformtoolsmith.com/blog/operationalizing-adrs-fitness-functions/))
- **Stephan Schwab, 2026-04-17** — "Your test suite is the one instruction
  file the AI can't misinterpret"; lists architectural tests (ArchUnit)
  among ordinary TDD. ([caimito.net](https://www.caimito.net/en/blog/2026/04/17/tests-beat-instructions-for-ai-coding-agents.html))
- **Daniel Vaughan, 2026-04-28 / 05-17 / 08-14** — ADRs enforced by
  `AGENTS.md`, `PostToolUse` hooks and CI fitness functions; coins
  "agentic entropy": invariants "exist nowhere in [the agent's] window".
  Nothing cites an ADR id and nothing checks that citations resolve.
  ([codex.danielvaughan.com](https://codex.danielvaughan.com/2026/08/14/agentic-entropy-architectural-drift-coding-agents-codex-cli-conformity-seeding-spec-growth-engine-defence/))
- **Eleftheria Drosopoulou, 2026-05-22** — the one published source where
  **the test cites the decision**: ArchUnit `because("Payments and Orders are
  separate bounded contexts. See ADR-0031 for coupling constraints.")`,
  `DECISION:` comments in code with "revisit if" clauses, and expiry tests
  that fail with "ADR-0047 review overdue". ([javacodegeeks.com](https://www.javacodegeeks.com/2026/05/the-reason-most-architecture-decision-records-get-written-and-never-read-is-architectural-not-cultural.html))
- **Tian Pan, 2026-06-01** — "idiomatic drift"; argues the opposite: "A
  linter cannot tell you 'we don't throw in this codebase, we return a
  Result'", proposes `IDIOMS.md` and audits instead of checks.
  ([tianpan.co](https://tianpan.co/blog/2026/06/01/the-idiom-your-coding-agent-wrote-around))
- **Scott Spence, 2026-06-21** — custom oxlint rules ("project taste
  executable"), `tools/check-boundaries.ts` scanning tracked imports, demo-data
  regexes with an `@allow-demo-data` escape, and advisories that **reference
  a docs path** (`docs/specs/sveltekit-entrypoint-rules.md`). The nearest
  practitioner shape to ours; no test suite, no decision ids.
  ([scottspence.com](https://scottspence.com/posts/how-i-stop-llms-drifting-in-production-codebases))

Research that measures the problem rather than the fix: *AI-Generated Smells*
(Zhu, Tsantalis, Rigby, arXiv 2605.02741, 2026-05-04) — "neither functional
correctness nor detailed prompting mitigates this decay"; the architecture
erosion mapping study above. Neither proposes tests.

## 4. Verdict: a variant of the architectural fitness function, not a renaming and not new

**The genus has a name and we should use it.** A test with the repository as
its fixture, run in the ordinary suite, failing when a structural rule is
broken, is an *architectural fitness function* (2017), and in older academic
clothes an *architecture conformance check*. Go's `deps_test.go` has been one
since before either name existed: a unit test whose data "is a statement of
policy". rust-lang's `tidy` and Kubernetes' `verify-*` are the same idea
spelled as a linter. The AI-harness field is converging on exactly this in
2026 — OpenAI's "structural tests", Thoughtworks' "feedback sensors",
ContextCov's compiled constraints, five practitioner posts in four months —
but under the *existing* names, not a new one. Nobody uses "pin" for it.

**What is a variant, with the evidence that it is unpublished:**

| property of ours | nearest published analog | gap |
|---|---|---|
| every pin cites the ADR or bead that made the rule (65/68) and a pin checks the citations resolve | ADE thesis (rule file *beside* the ADR, 2026); Drosopoulou's `because("See ADR-0031")` (2026-05) | the citation as a *filing rule*, and a check that cited files exist — UNKNOWN; none found |
| prose and docs are subjects (notes index current, runbook names a real file, ADR status line true) | Docs as Tests (procedures run against the product); OpenAI's doc "garbage collection" agents | ours assert *decisions recorded in prose*, theirs that *procedures work* |
| ADR 0006 §7: a pin must name the process it protects | — | UNKNOWN; none found. It exists because our pins once froze a live bug — the characterization-test failure mode |
| a door register: every tree-wide pin has a fast `make` target, pinned by a test | rust `./x test tidy` is one door for all checks | a *test that every pin has a door* — UNKNOWN; none found |

So: **variant**. Adopt the known name alongside ours.

**Two collisions the word "pin" carries, both MEASURED:**

1. In this repo's own public docs, *pin* already means a **version pin**:
   INSTALL.md says "two of them are pinned on purpose", `make
   verify-grok-pin`, `make verify-bd-pin`, `etc/*/version-pin.toml`
   (INSTALL.md lines 34–51, 101). README's "pins and reasons included" is
   that sense. An outsider meets *pin* = version first.
2. In the field, a **pinning test** is a characterization test — Feathers'
   term for freezing *current behaviour*. A tree pin is the opposite
   intent: it asserts a rule we *chose*, and §7 exists because a pin that
   merely froze behaviour froze a bug. A reader who knows the field will
   read "pin" as "snapshot of whatever the code does", which is the one
   thing we do not mean.

Web search for `"treepin" OR "tree pin" OR "tree-wide pin"` with software
terms on 2026-10-04 returned no software use at all.

## 5. Proposed public wording (the operator decides; nothing renamed here)

**The term outsiders would search for:** *architecture fitness function*
(first), *architecture test*, *conformance check*; secondarily *repo lint*,
*tidy*, *convention test*. Our docs should contain the first phrase so the
search lands.

**Definition paragraph** — for CONTRIBUTING.md "The loop while you work"
(replacing the current "Some of this repo's checks are pins whose subject is
the *tree*" sentence) and, shortened to its first three sentences, for the
README's testing paragraph and the NOTES.md entry-points table:

> **Tree pins.** Some tests in this repository have the repository itself as
> their fixture. They read the tree — source, docs, Makefile, the ADRs — and
> fail when a decision we wrote down has stopped being true: the notes index
> lists every fragment, every ADR that names a source file names one that
> exists, no shipped file names a person where it should name a role.
> Elsewhere these are called *architecture fitness functions* or
> *architecture tests* (ArchUnit is the usual example; Go's own
> `deps_test.go` is an older one). Ours differ in two ways. Each pin cites
> the ADR or issue that made the rule, so a red test tells you which decision
> you are about to reverse, not just which line. And each has a fast door —
> `make fmt-check`, `make notes-check`, `make adr-check` … — because the
> package they live in takes minutes and a `-run` filter never names a test
> that is nobody's subject; `make tree-check` runs every door in about a
> minute. They live in `internal/treepins`. A tree pin is not a *pinning
> test* in the characterization-test sense: it asserts a rule we chose, not
> the behaviour we happen to have.

**Should the word or the package change?** Three options, priced:

| option | what changes | cost (MEASURED at `c202af14`) | residual risk |
|---|---|---|---|
| **A. Keep "tree pin", gloss it** (recommended) | the paragraph above lands in CONTRIBUTING, README, NOTES; `internal/treepins/README.md` carries it so the package is self-describing (it has no doc file today) | one docs bead, 4 files | the two "pin" collisions, answered by the paragraph's last sentence and the fitness-function gloss |
| B. Rename the prose term ("tree checks" / "fitness functions"), keep the path | NOTES, CONTRIBUTING, Makefile comments; 13 notes fragments stay as history | same bead plus ~20 prose edits; the repo already lives with path ≠ prose (221 vs 28) | two names for one thing, now deliberate |
| C. Rename the package too (`internal/fitness`, `internal/treechecks`) | Makefile (12+ refs), `ci.yml`, `armtags`/`treewidedoor`/`adrtestcitation` pins, every ADR and fragment citing the path — `adrtestcitation` reds on each stale one, which is its job | a code bead, a full `make test`, and ~250 citations | a published repo's history and bead comments keep the old name forever |
| do nothing | — | 0 | the operator's concern as stated: a Makefile target, a NOTES table row and a CONTRIBUTING sentence use a word with zero hits outside this repo |

**Recommendation:** A. The word is fine once it is glossed — it is short, it
is already the path, and the thing it names is a *variant* whose genus name
("architecture fitness function") can sit beside it in one sentence. Renaming
buys nothing the gloss does not, and C spends a suite run and a citation
sweep on a published name. The decisive sentence is the last one in the
paragraph: without it, "pin" tells a reader who knows Feathers the opposite of
what we mean.

## 6. Beads

- question bead for the operator: the A/B/C choice above, one decision.
- `-l code` bead for dinesh, dep-blocked on it: land the paragraph (option A
  shape, or as amended) in CONTRIBUTING.md, README.md, NOTES.md and
  `internal/treepins/README.md`; run `make notes-check`, `make doc-check`,
  `make tree-check`.

No code changed under this bead; the deliverable is this page.
