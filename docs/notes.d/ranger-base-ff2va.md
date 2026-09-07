## The per-site census: zero of the 103 (now 116) sites are actually blind (ranger-base-ff2va)

ranger-base-eku36 fixed the WHOLE-BINARY case: internal/posse's TestMain now
`filepath.EvalSymlinks`'s the temp `$HOME` before `os.Setenv`, so every
AbbrevHome-d subject the package prints during a normal test run sees the
operator's spelling. It explicitly did not touch the 103 `t.Setenv("HOME",
...)` sites that override that already-resolved HOME back to a fresh,
unresolved `t.TempDir()` for the duration of one test — and asked for a
per-site census of which of those actually put an AbbrevHome-dependent
subject at risk, "not a blanket EvalSymlinks."

As of 2026-09-07 there are 116 such sites in 47 files (`grep -rc
't.Setenv("HOME"' --include='*_test.go' .` — up from the 103 the bead was
filed against two days earlier; nothing here depends on the exact count).

**The question that matters isn't "does AbbrevHome get called under a
per-test HOME" — it's "does any test build its EXPECTED tilde string
independently of AbbrevHome, while the subject it feeds to the real
AbbrevHome call could resolve differently than the raw `t.Setenv` value."**
That is the shape ranger-base-eku36's own pin
(`TestTheTestHomeIsResolvedSoAbbrevHomeCanSeeUnderIt`, app_test.go:197) has:
`want` is hand-built from `filepath.Base(base)`, never from calling
`AbbrevHome` itself, while `got` is `AbbrevHome(real)` with `real` explicitly
re-resolved via `filepath.EvalSymlinks`. If the two diverge, the test catches
it. That is the only shape that CAN catch this bug.

### Method

1. `grep -rc 't.Setenv("HOME"' --include='*_test.go' .` → 116 sites, 47
   files.
2. Of those 47 files, which ever look at an AbbrevHome-derived value at all?
   Cross-referenced against every `AbbrevHome(` call site in the whole test
   tree (`grep -rln 'AbbrevHome(' --include='*_test.go' .` → 25 files) and
   every literal `"~/"` / tilde-form occurrence. Intersection with the 47:
   **app_test.go, credcomposite_test.go, visibility_test.go**, plus two
   pairs reached only through a shared helper —
   **hookcluster_qa_helpers_test.go → hookcluster_qa_test.go** (`newVisWall`
   → `DeriveIdentityLiterals` → `AbbrevHome`) and **worktree_qa_helpers_test.go
   → worktree_qa_test.go** (`wtqaHome` → dispatcher's "own tree "+AbbrevHome
   message). The other 41 files' 103-ish sites set HOME and never read
   anything AbbrevHome touches in that test — not a risk regardless of
   symlink resolution, because nothing in the test observes the abbreviated
   form.

   (The dozen files gwart's own probe named — app_test, credentialdenymove_qa,
   hookcluster_qa, pathscoped, runtimecheck, runtimepreflight,
   seatbeltcredentialread_qa, seatbeltstatedirnarrow_qa,
   skillsrules_parity_qa, verify_d3fn1_qa, visibility, worktree_qa — were
   flagged on a coarser signal, "reads a tilde form somewhere in the file."
   Reading each one: seven of those twelve (credentialdenymove_qa,
   pathscoped, runtimecheck, runtimepreflight, seatbeltcredentialread_qa,
   seatbeltstatedirnarrow_qa, skillsrules_parity_qa) only compare against
   LITERAL config-format tilde strings like `"~/.claude"` or feed
   `ExpandTilde` — the reverse direction, string-only, no filesystem
   resolution involved at all. Only the five named above actually call
   `AbbrevHome` on a filesystem subject under a per-test HOME.)

3. For each of those five, traced where the "subject" (the path fed to
   `AbbrevHome`) and the "want" (what the assertion compares it against) each
   come from:

   - **app_test.go** `TestAbbrevHome` (line 167): `home := "/tmp/somehome"`
     — a fixed literal, never a real directory on disk, so there is nothing
     for a filesystem symlink to resolve differently. Its five
     `t.Setenv("HOME", ...)` sites at lines 63/112/135 test `newApp`'s home
     *selection*, comparing `a.Home` against paths built by the SAME
     `filepath.Join(root, ...)` the test used for `root` — never through
     `AbbrevHome` at all.
   - **credcomposite_test.go:795** (`TestDarwinCompositeReadsTheFileOnExit...`,
     S2 case, line 728): `AbbrevHome(fallbackPath)` in the assertion is
     the exact same call refresh.go's `meterRow` makes over the exact same
     `fallbackPath` string (from `fallbackDir`, itself `filepath.Join` of
     `t.TempDir()` with the fallback file's fixed basename — not even under
     the per-test `HOME`, so the `home+"/"` prefix test in `AbbrevHome`
     never matches, resolved or not).
   - **visibility_test.go** `TestDeriveIdentityLiterals` (line 865, HOME at
     874) and `TestIdentityLiteralGuardHook` (line 1284, HOME at 1294): both
     build `instance`/`repo` by `filepath.Join(home, ...)` (pure string
     arithmetic on the literal `t.TempDir()` value) and then compare against
     `AbbrevHome(instance)` — called by the test itself, not hand-built. Same
     shape as hookcluster_qa below.
   - **hookcluster_qa_helpers_test.go:53** (`newVisWallCfg`) →
     `TestQAIdentityPathArmSeesTheTildeForm`: `abbrev := w.literal(t,
     "instance-path")` comes from `DeriveIdentityLiterals(hookRepo(w.pub))`.
     `w.pub`/`w.instance` are plain `filepath.Join(home, ...)` strings; the
     `.beads/redirect` content the test plants is itself a literal string
     built the same way. `hookRepo` only resolves anything (via `git
     rev-parse --git-common-dir`) for a LINKED git worktree — `w.pub` is a
     plain `git init`, so `hookRepo` returns it unchanged. No step between
     "what the test wrote" and "what `AbbrevHome` sees" touches the
     filesystem for resolution; the assertion `strings.HasPrefix(abbrev,
     "~/")` passes or fails identically whether the per-test HOME is
     resolved or not, because both the numerator and the denominator of the
     prefix test are the same unresolved string.
   - **worktree_qa_helpers_test.go:38** (`wtqaHome`) →
     `TestDispatchGivesEachPersonaItsOwnTree` (assertion at line 113):
     `strings.Contains(out, "own tree "+AbbrevHome(m.Dir))` — `out` already
     contains the dispatcher's own `"own tree "+AbbrevHome(m.Dir)` line,
     printed with the identical `m.Dir` the test later reads back out of
     the persisted run record. This is circular by construction: it can
     only ever measure "the dispatcher's message contains the dispatcher's
     own computation," never whether that computation is correct. `m.Dir`
     itself is the literal path the dispatcher chose for `git worktree add`
     (worktree.go:803), not something re-derived through git afterward.

4. The one place production code resolves a path independently of the raw
   `t.Setenv` HOME string — `hookRepo`'s linked-worktree branch
   (`git rev-parse --git-common-dir`, gates.go:5283) — is never exercised by
   any of the 47 per-test-HOME files: none of them dispatch into or read
   identity literals from a LINKED worktree. The files that DO exercise real
   `git worktree add` trees (worktree_test.go, gates_test.go, and a dozen
   more under the seatbelt/cage/launcher names) do not override `HOME`
   per-test at all — they run under TestMain's package-wide HOME, which
   ranger-base-eku36 already resolved.

### Verdict

**No helper is needed.** Every one of the 103 (116) per-test HOME sites
either (a) never looks at an AbbrevHome output at all, or (b) looks at one
whose "expected" half is derived by calling AbbrevHome (or the production
function that wraps it) on the identical subject production used — which
makes the assertion immune to the resolution bug by construction, not
because the underlying behavior is actually pinned. That immunity is exactly
gwart's "tolerates both... has nothing to measure" finding, confirmed at the
level of individual call sites rather than whole files. A blanket
`EvalSymlinks`-before-`Setenv` change across 103 sites would touch ~40 files
for zero behavioral gain, since none of their assertions could observe the
difference either way.

This is a DIFFERENT finding from "these sites are well-tested" — none of the
five sites above actually PIN that AbbrevHome resolves correctly under a
per-test HOME; they'd pass over a broken AbbrevHome too, same as before. That
gap (self-referential tests that can't catch an AbbrevHome regression) is a
real one, but it is a test-quality question distinct from this bead's
"macOS symlink blindness" scope — ranger-base-eku36's own pin
(`TestTheTestHomeIsResolvedSoAbbrevHomeCanSeeUnderIt`) already covers the
one place a hand-built (non-circular) expectation exists, and that pin runs
under the already-fixed package-wide HOME, not a per-test override.
