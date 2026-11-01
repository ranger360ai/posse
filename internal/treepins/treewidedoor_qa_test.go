package treepins

// QA pins for ranger-base-ik44f — fast doors to the other four tree-wide pins.
//
// THE CLASS, as ranger-base-rulbl named it: "a QA test whose subject is the
// TREE, living inside a package nobody runs whole". internal/posse is ~950s,
// past the 600s a seat can spend in one foreground call, so the standing
// advice is a focused `-run` filter — and `-run` selects by test NAME. A
// tree-wide pin is nobody's subject, so no seat's filter ever names it, and
// the pin is unreachable at exactly the moment it would have mattered.
//
// rulbl shipped the first door (`make fmt-check`) and censused the rest.
// This bead is the other four:
//
//	TestTreeIsGofmtClean                          make fmt-check   ~1.5s
//	TestShippedTreeNamesRolesNotThisCrew          make crew-check  ~2.5s
//	TestShippedStringsNameRolesNotThisCrew        make crew-check
//	TestTestCorpusHidesNoCrewNameBehindAnEscape   make crew-check
//
// and eight more the first census could not see, doored under
// ranger-base-sx2dq (see arm 2):
//
//	TestSeedSurfaceNameCountIsZero                make seed-check    ~0.2s
//	TestSeedConfigLiveKeysAreRead                 make seed-check
//	TestPublicationRootCommitOmitsExcludedPaths   make history-check ~0.5s
//	TestPublicationRootCommitADRsCarryProvenance  make history-check
//	TestPublicationHistoryNeverCarriesTheSeedScript make history-check
//	TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover
//	                                              make doc-check     ~1.2s
//	TestQACageCredDocDoesNotCallTheOnDiskCredentialStale   make doc-check
//
// (seven rows under a count of eight: the eighth was
// TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp, deleted with
// its door entry under ranger-base-xrdb0. The numeral is sx2dq's and stays —
// it says what that bead doored, not what the class holds today, which only
// the derived sentence below may say.)
//
// and five more that took their root from `git rev-parse --show-toplevel`,
// doored under ranger-base-xndgk (FINDING 5 of the ranger-base-xtgvp verify):
//
//	TestQAIdentityLiteralsNeverAppearInATrackedPath make identity-check ~0.5s
//	TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned
//	                                              make identity-check
//	TestQAEveryOpsHitInTrackedMarkdownIsRuled     make ops-check       ~2s
//	TestQAOpsShapeTableCanStillSayNo              make ops-check
//	TestShippedExampleTableCoversEveryVersionInGitHistory
//	                                              make history-check   ~3s
//
// and four more that arrived afterwards, each doored by the bead that wrote
// it and given its own membership row in arm 2 — three rows now, the fourth
// being TestQAADR0026StatusLineDoesNotDeferTheImplementedRung
// (ranger-base-8dnuy), deleted with its door entry under ranger-base-xrdb0:
//
//	TestQAADR0035PaneModeSurfaceClaimIsBuilt      make doc-check
//	                                              (ranger-base-vwgt)
//	TestInstancePathFormNeverAppearsInTrackedContentUndispositioned
//	                                              make ops-check
//	TestQAInstancePathCensusCanStillSayNo         make ops-check
//	                                              (both ranger-base-l9ii)
//
// and one more, given its own door rather than folded into crew-check's
// (crew-check's own comment reserves that door for one question, which this
// pin does not ask):
//
//	TestQATreeGoFilesWriteExecutablesUnderTheForkLock
//	                                              make execwrite-check
//	                                              (ranger-base-rwnbd, widened
//	                                              by ranger-base-6cznr)
//
// and one more, folded into seed-check's own door rather than given a new
// one — it touches examples/config.yaml, which is exactly what that door
// already reads:
//
//	TestQAExampleConfigConstitutionBlockNamesTheWholePromotedSet
//	                                              make seed-check
//	                                              (ranger-base-nn33e)
//
// and four more, folded into seed-check's door for the same reason the row
// above was: their subject is examples/config.yaml. Two pairs, one census.
// The first pin derives the seed's documented duration defaults from the
// call sites that pair a config key with its Default* constant, and its
// partner holds the register that silences it for a key documented at
// something other than its default:
//
//	TestSeedConfigDocumentedDurationDefaultsAreTheConstants
//	                                              make seed-check
//	TestSeedConfigDocumentedDefaultRegisterIsNotStale
//	                                              make seed-check
//	                                              (both ranger-base-vofbl)
//
// and the same pair one value type over, from the widening that gave that
// census a NUMBER half — bare-number lines against the int, float64 and
// uint64 readers the duration rule could not see:
//
//	TestSeedConfigDocumentedNumberDefaultsAreTheConstants
//	                                              make seed-check
//	TestSeedConfigDocumentedNumberRegisterIsNotStale
//	                                              make seed-check
//	                                              (both ranger-base-p9qve)
//
// and one more, folded into doc-check's own door for the same reason — it
// is a prose pin over two shipped documents, which is that door's subject:
//
//	TestQAShippedLaunchLinesParseAsThePersonaLaunch
//	                                              make doc-check
//	                                              (ranger-base-qnn6j)
//
// and one more into the same door, a prose pin whose subject is a PATH: the
// door a detection refusal hands the operator has to name a filing the tree
// holds, and it was rendered from the runtime's NAME, so three of four
// built-ins named a file that is not there:
//
//	TestQADetectionDoorCitesAFilingThatExists
//	                                              make doc-check
//	                                              (ranger-base-ecchw)
//
// and one more into that same door, the SECOND prose pin whose subject is a
// path: a docs/notes.d citation in any markdown file in the tree has to
// resolve to a fragment the tree holds. v0.5.1 was cut over an ADR citing one
// that was stranded on the branch that wrote it, and nothing was red — the
// citation is prose and the fragment's absence is a missing file:
//
//	TestQAEveryNotesFragmentCitationResolves      make doc-check
//	                                              (ranger-base-nnnf1)
//
// and two more into that same door, both of the SAME shape as those two and
// each filed as a documented coupling nothing compared (ranger-base-c768n,
// findings 1 and 2 of the ranger-base-qbbzz verify). One derives the base of
// every URL the detection doors compose — host, org, repo and branch, an
// unheld literal whose pin compared it with itself — from the hrefs
// www/index.html already ships; the other holds the one const in agents.go
// whose whole value is that a classifier reads exactly the text ADR 0070 D2
// cleared, against that blockquote:
//
//	TestQAPublicDocBaseIsTheSpellingTheSiteShips  make doc-check
//	TestQAClaudeAutoModeCarveOutIsADR0070D2Verbatim
//	                                              make doc-check
//	                                              (both ranger-base-c768n)
//
// and then a whole SECOND PACKAGE, which the register could not see at all
// until ranger-base-g6sb1. internal/treepins is 589.965s whole — the same
// wall internal/posse is, one directory over — and every rule above keys on
// a test COMPUTING the repo root, which a test in that package never does:
// its TestMain chdirs the binary to the root before anything runs, so a pin
// there reads the tree through a plain relative path. Twenty-three are
// derived from a second rule (an enumeration rooted at a relative tree path;
// see "The internal/treepins half of the class" below), and one more is
// registered by hand because its enumeration is a line of Python:
//
//	TestNotesFragmentIndexIsCurrent               make notes-check  ~0.3s
//	TestADRCitedGoFilesResolveOrAreDeclared       make adr-check    ~1.5s
//	TestADRCitationCheckCanFail                   make adr-check
//	TestADRCitationDeclarationsExemptOnlyWhatTheyDeclare
//	                                              make adr-check
//	TestADRCitationCorpusReadsTheExecutableSupplements
//	                                              make adr-check
//	TestADR0015NamesTheHookCommitPinAndItDoesWhatItSays
//	                                              make adr-check
//	TestQAFixtureRuntimeExesResolveToNothingOnThisBox
//	                                              make corpus-check   ~4s
//	TestQATheGofmtDoorReachesEveryGoFile          make corpus-check
//	TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot
//	                                              make corpus-check
//	TestQATheTolerantTempDirWrapperCompilesInEveryArm
//	                                              make corpus-check
//	TestQANoMakefilePrereqLineReadAsBytesOutsideMkPrereqs
//	                                              make corpus-check
//	TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared
//	                                              make corpus-check
//	TestNoUnswappedInternalRhqCommentsOutsideFrozenRecords
//	                                              make corpus-check
//	TestRhqLeftoverExemptionsStillNameRealLines   make corpus-check
//	TestQANoShippedPosseCallHandsBdAStoreSelectingFlag
//	                                              make corpus-check
//	                                              (ranger-base-9mjxb)
//	TestQAEveryTreeWidePinHasADoor                make register-check ~20s
//	TestQAOneRepoRootHelperInTheTestPackage       make register-check
//	TestQATheTreeWideDoorsReportRealDrift         make register-check
//	TestQAMakeTestOpensTheTreeWideDoors           make register-check
//	TestQAEveryTrackedProgramATreePinRunsIsDispositioned
//	                                              make register-check
//	TestQATheTreepinsEnumerationRuleMatchesWhatItClaims
//	                                              make register-check
//	TestQAEveryTreepinsDoorFilterNamesItsPins     make register-check
//	TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles
//	                                              make register-check
//	TestQABoxCheckCensusCoversEveryVerifyScript   make scripts-check ~0.9s
//	TestQANoAssertionArmDecidesThroughAForkedMatcher
//	                                              make scripts-check
//	TestQABoxCheckCensusCoversEveryVerifyTarget   make scripts-check
//	                                              (ranger-base-hrf47)
//	TestQAEveryBrewNamingScriptIsRosteredOrExempt make scripts-check
//	                                              (ranger-base-mk6gw)
//	TestShippedPIDsCarryTheNarrowedHookRows       make pid-check      ~10s
//	TestShippedPIDsLetBeadsOwnHooksRun            make pid-check
//
// and two more, folded into corpus-check's own door because its subject is
// exactly theirs — a census over this repo's own .go sources. They are the
// two arms that keep the multiplexer slot a slot: which production files may
// hold the herdr client, and whether every file the roster still names still
// holds it (ADR 0076 §1/§4):
//
//	TestQATheMultiplexerSlotIsTheOnlyRouteToTheHerdrClient
//	                                              make corpus-check
//	TestQAEveryHerdrClientHolderStillHoldsIt      make corpus-check
//	                                              (both ranger-base-csnzn)
//
// (Three of those rows are exempt from the two-way check, listed with their
// reasons in twdDoorHolders, all three for the same cause: they read the
// MAKEFILE by name rather than enumerating the tree, and
// `os.ReadFile("Makefile")` is the enumeration rule's own negative case. Two
// are the last two rows of register-check — the register's own arms. The
// third is the scripts-check row ranger-base-hrf47 added, and it is the one
// worth reading, because it is not this register inspecting itself: it
// censuses the Makefile's verify-* targets against the roster and EXCLUDED
// table of scripts/verify-box.sh. Nothing about reading two files by name
// makes a pin reachable — an unrelated bead adding a verify-* target reds it
// just as a tree walk would, and for as long as it sat behind no door the
// only thing that said so was an unfiltered 589.965s run of this package.
// Its derived sibling three rows up globs scripts/, so scripts-check was
// green over it the whole time. A pin whose file set is spelled is not the
// same as a pin only its own bead can red, and the door is what closes that
// gap — not the rule.)
//
// and one more into corpus-check's door for that same reason — a census over
// this repo's own shipped .go, holding every bounded child to naming its
// WaitDelay. os/exec's Wait does not return while a grandchild holds the
// killed child's pipes, so a context deadline alone bounds the child's life
// and not the call; the rule was written out in three doc comments and
// honoured at five of fifteen sites (ranger-base-2cttm). Its two-way control
// is deliberately NOT doored: it reads planted fixtures rather than the tree,
// so no unrelated bead can red it and nothing derives it:
//
//	TestQAEveryBoundedChildNamesItsWaitDelay      make corpus-check
//	                                              (ranger-base-2cttm)
//
// and one more into corpus-check's door for that same reason, and the one
// whose absence is the best evidence this arm is not decoration — a census
// over this repo's own TEST corpus, the real-bd daemon-leak guard
// (ranger-base-42mv). It walks the repo root and reads every _test.go in the
// tree, and for the two months it existed it had no door at all: its scan
// took the walk root as a PARAMETER and the one literal was spelled at the
// call site in the test body, so the enumeration the derivation looks for was
// one hop from where it looks, the pin was never derived, and arm 2 never
// asked. The root is spelled at the walk now, which is the whole of what
// makes it derivable. Its five control arms are deliberately not doored,
// for the reason the row above gives — they drive the rule over planted
// fixtures, so nothing derives them and no unrelated bead can red them:
//
//	TestQANoTestShellsRealBdWithoutStoppingItsDaemon
//	                                              make corpus-check
//	                                              (ranger-base-0ckgm)
//
// and one more into register-check's door, because what it holds is THIS
// FILE's derivation rather than the tree. Its rule is the pair — a walk
// rooted at one of its own function's parameters, and a call site handing
// that parameter a tree path — and it was a PROHIBITION with a live answer of
// zero and a two-row waiver naming the hole. The derivation reads that pair
// now (ranger-base-pyl22, twdParamHops), so the waiver is gone and the pin is
// the coupling that keeps the hop as wide as the rule which finds the pairs.
// Its fixture arm is undoored for the same reason the three above are:
//
//	TestQAEveryTreePathHandedThroughAParameterIsDerived
//	                                              make register-check
//	                                              (ranger-base-0ckgm, turned
//	                                              into a coupling by
//	                                              ranger-base-pyl22 — see
//	                                              the five rows below)
//
// and TWELVE more, which arrived in one landing because the DERIVATION was
// widened rather than a spelling pinned (ranger-base-8wxbn). Of the 39
// walker calls in internal/treepins' tests, MEASURED 2026-10-31, 23 rooted
// at a relative literal the rule read and 16 did not — and twelve tree-wide
// pins were sitting behind four of those spellings, undoored, reachable only
// by the 589.965s package run. Three of the four are a root that never
// leaves the function body (a local, a package const, a range variable over
// a literal list) and twdRelEnv now reads all three; the fourth takes the
// root as an ABSOLUTE path from repoRoot() and hands it to a helper that
// walks a subtree of it, which is a second rule and a second fence
// (twdTreepinsRootHelper). No new door: every one of the twelve went into
// the door whose subject it already shared.
//
//	TestADRRecordsNameTheLivePackageDirectory     make adr-check
//	TestADRSweptRecordsStillCiteTheLiveDirectory  make adr-check
//	                                              (both read every record in
//	                                              docs/adr through a LOCAL
//	                                              assigned filepath.Join of
//	                                              literals)
//	TestKeychainReadIsExecdFromExactlyOneFile     make corpus-check
//	                                              (a RANGE VARIABLE over
//	                                              []string{"internal","cmd"},
//	                                              in the test body)
//	TestQAEveryPosseTestFileIsSharedOrInANamedArm make corpus-check
//	TestQAEveryMakefileDoorPinIsReachableInItsDoorsArm
//	                                              make corpus-check
//	TestQAEverySuiteArmTypeChecks                 make corpus-check
//	                                              (all three read every test
//	                                              file in internal/posse
//	                                              through the package CONST
//	                                              armPkgDir)
//	TestQANoEntryPointRunsGoTestOnTheDefaultTimeout
//	                                              make scripts-check
//	                                              (a range variable over a
//	                                              literal list of Joins —
//	                                              scripts/*.sh and
//	                                              .github/workflows/*.yml)
//	TestWallRenderSourcesNameEveryRenderedDoNotEditSite
//	                                              make corpus-check
//	TestEveryRenderedDoNotEditSiteNamesItsRenderer
//	                                              make corpus-check
//	TestLaunchCheckSourcesNameEveryParityDeciderAndRuntimeDeclarer
//	                                              make corpus-check
//	TestLaunchCheckSourcesHoldNothingTheDerivationCannotSee
//	                                              make corpus-check
//	                                              (all four take the root
//	                                              from repoRoot() and walk
//	                                              internal/posse's sources
//	                                              through it)
//	TestTreePinsRunFromModuleRoot                 make register-check
//	                                              (the marginal one, and the
//	                                              register's own floor: its
//	                                              subject is the cwd
//	                                              TestMain installs, which
//	                                              is what makes every
//	                                              relative literal in this
//	                                              package a TREE path — and
//	                                              a _test.go file added at
//	                                              the module root reds it,
//	                                              which is nobody's bead)
//
// and FIVE more, which is the sixth spelling and the last of the six
// (ranger-base-pyl22). A walk root that arrives as a PARAMETER was refused
// on purpose through all of the above, and the refusal was read too wide: the
// thing twdTestFiles turns down is interprocedural constant PROPAGATION, and
// one argument resolved at one call site is not that. twdParamHops does that
// one step, against the walker table rootparam_qa_test.go already builds, so
// no new mechanism is coined and no second copy of "what is a param-rooted
// walk" exists to drift. The alternative was a third hand register beside
// twdIndirect and twdDoorHolders, and neither of their reasons reaches these
// — twdIndirect is for an enumeration no rule over Go source can derive, and
// one derives these today, one file over, under a fixture arm.
//
//	TestOneCredentialAcquirerAndNoOther           make corpus-check
//	TestNoNewShadowPredicate                      make corpus-check
//	TestNoBdCloseVerbReachableFromDispatch        make corpus-check
//	TestPaneModeReadingDecidesNothing             make corpus-check
//	                                              (all four hand arParse `.`,
//	                                              which walks every non-test
//	                                              .go under internal/ and
//	                                              cmd/)
//	TestNoShippedScriptAcquiresACredential        make corpus-check
//	                                              (hands cgScan `.`, which
//	                                              walks every runnable file
//	                                              under scripts/, plugin/,
//	                                              etc/ and docs/)
//	                                              (all five ranger-base-pyl22)
//
// Both helpers are genuinely polymorphic — eight more tests hand arParse a
// fixture root — so neither root could be respelled at the walk the way
// scanRealBdCallSites' was (ranger-base-0ckgm). 0.43s between the five.
//
// and TWO more into that same door, which landed on main while the five
// above were being measured on their own branch, so this comment is the
// merge of the two (ranger-base-bm6j8 and ranger-base-pyl22, both
// 2026-10-31). Their subject is this repo's own sources too: the census
// that every binary posse execs from a LAUNCHER process
// is resolved and run outside every gates dir (ADR 0077 D3), and the
// derivation of the env binders its second arm reads. Both walk the tree
// through ewdGoFiles, a relative literal, which is the first rule above and
// not the widened one. (That walk was rooted at cmd/ and internal/ when this
// paragraph was written; ranger-base-tpy6q rooted it at the repository root,
// which changes neither the spelling rule nor the door.) The census's own fixture arm is undoored
// for the reason the rows above give — it drives the rule over planted
// sources, so nothing derives it:
//
//	TestQAEveryProtocolChildRunsOutsideTheGates
//	                                              make corpus-check
//	TestQATheEnvBinderDerivationFindsThisTreesBinders
//	                                              make corpus-check
//	                                              (both ranger-base-bm6j8)
//
// and THREE more, which are the first members of this class that computed no
// root AT ALL (ranger-base-nt0pi). Every rule for the internal/posse half
// keys on a body that reaches the repo root — qibRepoRoot, a hand-rolled
// climb, git — because `go test` runs that package's binary with the PACKAGE
// directory as cwd, so reading the tree costs a climb. It costs a climb; it
// does not cost a ROOT. `filepath.Glob("../../docs/notes.d/notes-*.md")`
// reads the tree with no helper, no runtime.Caller and no subprocess, so
// there was nothing for either rule or for the fence to see, and three pins
// whose subject is the tree on any reading sat behind the ~950s package run.
// Their climbs take the root from qibRepoRoot now, which is the whole of
// what makes them derived — the register's own lesson read a third time:
// the class is not bounded by how a test spells its root, it is bounded by
// there being ONE:
//
//	TestQAPrescribedGitDiffChecksStateTheirFormat make doc-check
//	TestQAPrescribedGitDiffChecksRunUnderADiffDriver
//	                                              make doc-check
//	                                              (both read AGENTS.md and
//	                                              NOTES.md by name and glob
//	                                              docs/notes.d/notes-*.md,
//	                                              which is this door's own
//	                                              subject: prose over
//	                                              shipped code and docs)
//	TestShelfPIDsLintCleanAsASet                  make seed-check
//	                                              (globbed
//	                                              examples/skills/*, and
//	                                              reads the shelf through
//	                                              shelfPIDs — the seed
//	                                              surface, which is that
//	                                              door's subject)
//	                                              (all three
//	                                              ranger-base-nt0pi)
//
// 1.26s between the three (0.00s, 1.21s and 0.05s, at a one-minute load
// average of 97.58). The shelf pin also MOVED ARM to get its door: it was in
// posse_arm2, a door's `-run` took no `-tags`, and a filter that matches
// nothing exits 0 — so it is in internal/posse/shelfpidlint_qa_test.go, arm
// 1, rather than teaching one door to pass a tag. That tag question was open
// for seven other pins and was ranger-base-m94g2's to answer; it is answered
// in the paragraph below, the other way, and the file stays where nt0pi put
// it. Both readings are right at their own size: seven pins across two arms
// is a mechanism, one pin is a `git mv`.
//
// and EIGHT more, which are the same defect ONE HOP OUT (ranger-base-8yv7z).
// nt0pi fixed the climb spelled AT the walker and fenced that spelling, and
// said its own residual out loud: a climb that reaches the walker through an
// IDENTIFIER, or through a PARAMETER, is unread. Both were live, and not
// singly — the fence, once it reads an identifier, finds six pins spelled
// that way and a seventh population behind a parameter chain:
//
//	TestQAThePolicyDropInsDoNotSetTheSameKeyTwice  make seed-check
//	                                              (`const dir =
//	                                              "../../etc/claude/managed-settings.d"`
//	                                              in a local const, ReadDir'd
//	                                              — the one spelling nt0pi
//	                                              named)
//	TestQAKnownScreenReaderNamesEveryFixtureExactly
//	                                              make seed-check
//	                                              (every detection fixture
//	                                              under
//	                                              etc/herdr/agent-detection/testdata,
//	                                              through a local)
//	TestEmbeddedSeedMatchesExamplesDir            make seed-check
//	                                              (walks EVERY file under
//	                                              examples/ against the
//	                                              binary's embed)
//	TestShippedPIDsDenyPromote                    make seed-check
//	TestShippedPIDsDenyRefresh                    make seed-check
//	                                              (both enumerate the shelf
//	                                              through a local holding
//	                                              `../../examples/agents`)
//	TestExampleAgentsArePIDs                      make seed-check
//	TestExampleAgentsHandoffsAreShapes            make seed-check
//	TestShelfPIDsGuardFailsRatherThanSkipsWhenShort
//	                                              make seed-check
//	                                              (all three read the shelf
//	                                              through shelfPIDs, which
//	                                              handed a climbing literal
//	                                              to a helper that handed its
//	                                              own parameter to a helper
//	                                              that globbed at that
//	                                              parameter — TWO call sites
//	                                              from the walk)
//	                                              (all eight
//	                                              ranger-base-8yv7z)
//
// No new door: every one of them reads the surface a publication carries —
// examples/, and the configuration the shipped tree carries beside it under
// etc/ — which is what TestSeedSurfaceNameCountIsZero has always walked
// whole, so seed-check's subject is theirs and the Makefile says so once
// rather than eight times. Two MOVED ARM to get one, into files of their own
// (policydropins_qa_test.go from posse_arm3,
// shippedpidsdenyrefresh_qa_test.go from posse_arm2), for the reason
// TestShelfPIDsLintCleanAsASet did and with the same disclaimer: a door takes
// no `-tags`, a filter that matches nothing exits 0, and two tests changing
// files is not a mechanism. ranger-base-m94g2 still owns the tag question.
//
// THE FIXES ARE THE ROOT AGAIN, not a wider derivation. Six take the root
// from qibRepoRoot where they held a climb; the shelf's helper takes it as a
// PARAMETER its callers spell `qibRepoRoot(t)`, which is what puts those
// three tests in the class by the rule half of it is already derived from.
// Nothing in the derivation widened, and the chain is still not followed
// there.
//
// WHAT WIDENED IS THE FENCE, and the two halves of it are read from the two
// spellings above: twdClimbSet reads a climb held in a local, a local const
// or a package const, and twdClimbHops closes the param-rooted-walker
// relation over calls so a climb handed any number of hops from the walk is
// refused. (A third spelling joined the first half later and is the section
// below: a climb reaching the walker as a RANGE VARIABLE over a list of
// literals, ranger-base-g31e0.) THE FENCE MAY FOLLOW THE CHAIN WHERE THE DERIVATION MAY NOT, and
// that asymmetry is the load-bearing decision here: a wrong resolution in the
// derivation is a door demanded for a fixture walk or a pin quietly leaving
// the class, while a wrong resolution in a prohibition only tells a seat to
// take the root from qibRepoRoot — which is the right instruction for every
// body it can match. So twdRelEnv's refusal stands untouched and the fence
// goes as wide as the spellings that actually carried pins.
//
// AND IT HAS A JURISDICTION LINE, measured rather than assumed. The first
// fence that read identifiers also fenced detectionRig
// (verifydetection_qa_test.go), which holds `filepath.Join("..", "..")` in a
// local and ReadDirs a subdirectory of it — a shape the posse half scores as
// NOT tree-wide on purpose (twdRootExprs' head comment), and whose nine
// callers would have joined the class the moment the fence's own remediation
// was applied. So an identifier binds here only when the climb it holds NAMES
// a path below the root: the climb arm owns a climb that names a path, the
// twin/hidden arms own the climb that names the ROOT, and the two do not
// overlap. That is the only live answer this bead changed by argument rather
// than by measurement, and the fixture table below types both sides of it.
//
// AND THE SPELLING IS FENCED, not merely fixed: a climbing literal at a
// WalkDir/Walk/ReadDir/Glob is the fourth arm of
// TestQAOneRepoRootHelperInTheTestPackage (twdClimbRoot), with its own
// fixture control, because this arm's live answer is zero by construction
// and a prohibition nothing violates cannot be told from a broken detector.
// Its own residual is said out loud below.
//
// (The spellings that are still unread, said out loud so the next reader
// does not have to census them again. A root parsed out of the Makefile is
// unread (gofmtdoor_qa_test.go) and costs nothing: that test also walks ".",
// so it is derived and doored on that. A CHAIN — a helper taking a root as
// its own parameter and handing it on to a param-rooted walker, with the
// literal two call sites away — is unread by the hop and by the rule that
// finds the pairs, identically and on purpose; that one would be the
// propagation, and nothing in the package is spelled that way. And two
// fixture roots are correctly unseen. In internal/posse, a climb that
// arrives at the walker through an IDENTIFIER was unread by the fence —
// `const dir = "../../etc/claude/managed-settings.d"; os.ReadDir(dir)`,
// which policyhooks_qa_test.go spelled on the day nt0pi landed — and that
// one was filed rather than fenced there, because closing it adds a class
// member and that bead's three were a defect, not a decision. It is fenced
// now, with the parameter hop beside it and with eight more members
// (ranger-base-8yv7z, below); what is still unread in internal/posse is a
// named path JOINED onto a bare-root identifier, which is the hidden arm's
// shape and is scored by its dataflow.)
//
// and FIFTEEN more, which are the ONE-PACKAGE CENSUSES — and the first
// members admitted by a DECISION rather than by a defect
// (ranger-base-m94g2 decided, ranger-base-na4rm doored). A pin in
// internal/posse that enumerates its OWN package directory —
// `os.ReadDir(".")`, `filepath.Glob("*.go")` — computes no root and climbs
// nowhere, so it is outside every rule above for a reason none of them
// states: not how it spells the root, but that it does not need one. The
// class's NAME argues them out ("a pin whose subject is the TREE": a package
// directory is one directory) and the class's operative KEY argues them in,
// which is the key this file already adopted in as many words for its
// internal/treepins half — whether the file set is SPELLED in the test or
// DISCOVERED. Discovered: a bead that adds a file to internal/posse, having
// never read the census, reds it, and the ~950s arm run is its only route.
// Both sides of that argument, and why the narrower blast radius is not a
// reason to decide the other way, are in docs/notes.d/ranger-base-m94g2.md.
//
// THE RULE IS THE TREEPINS RULE WITH A SECOND CALLER, and no new code:
// twdAnalyze has computed relEnum for both packages since
// ranger-base-g6sb1, and this half carried it unused — the only thing that
// differs between the packages is what a relative literal MEANS, the repo
// root over there (that TestMain chdirs) and the package directory here.
// Which is also why these compose with ranger-base-nt0pi's three climbs
// rather than subsuming them: twdRelTreePath refuses a `../` prefix on
// purpose, those three derive from f.ascent, and the seed reaches none of
// them (MEASURED 2026-10-31, over all three).
//
//	TestNoSecondReadOfARuntimeHomeVariable        make pkgcensus-check
//	TestQAEverySandboxProbeFileIsGated            make pkgcensus-check
//	TestQANoEventSubscriptionRemainsInInternalPosse
//	                                              make pkgcensus-check
//	TestQANoInferredIntentMechanismRemainsInInternalPosse
//	                                              make pkgcensus-check
//	TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself
//	                                              make pkgcensus-check
//	TestQANoProductGitDiffReaderIsBare            make pkgcensus-check
//	TestQAOneThrottleOnlyWatchArmsTheRefill       make pkgcensus-check
//	TestQAProcessNoticesResolveThroughTheDeclaredSeams
//	                                              make pkgcensus-check
//	TestQAEveryCountedSkipReasonHasAReportingSite make pkgcensus-arm2-check
//	TestQAEveryTestBinaryReExecKeepsTheChildATestBinary
//	                                              make pkgcensus-arm2-check
//	TestQAOnlyWorktreeGoWritesAMergeRefusal       make pkgcensus-arm2-check
//	TestBackupHasNoOverride                       make pkgcensus-arm3-check
//	TestQAOneDoorForTheReportedReadingAndItNamesNoRuntime
//	                                              make pkgcensus-arm3-check
//	TestQAPosseNeverWritesGroksTrustTable         make pkgcensus-arm3-check
//	TestQAWatchStreamWritesGoThroughTheDispatcher make pkgcensus-arm3-check
//	                                              (all fifteen
//	                                              ranger-base-na4rm)
//
// THREE DOORS FOR ONE SUBJECT, split by BUILD TAG and by nothing else — and
// they are the first doors in this register to carry `-tags`. Seven of the
// fifteen are compiled out of arm 1, a door's `-run` ran with no tag, and a
// `go test -run` over a name that is not in the build exits 0: an untagged
// door would have stood green in front of seven pins it could not select,
// which is rot class 4 of armtags_qa_test.go doored INTO existence. That arm
// asks each door pin for the arm its own door names now, from this file's
// twdDoors rows, and arm 1 above holds each recipe's `-tags` to that same
// number — two halves, because this one alone would pass a tagged door that
// quietly lost its tag. The precedent for the line is test-arm2/test-arm3,
// which carry a `-run` door before their tagged internal/posse line
// (ranger-base-p1r17).
//
// AND IT IS A DERIVED MEMBERSHIP, not fifteen rows in twdDoorHolders. That
// map is the one way into a door variable that does not go through the
// derivation, its own comment calls it the parking spot this file keeps
// warning about, and all three of its rows are there for one cause — a pin
// that reads named files and enumerates nothing. A census that enumerates is
// the opposite case, and fifteen rows there would have been the register
// conceding the class it exists to derive.
//
// THE SEED ALSO FOUND ONE THE DECISION DID NOT: fifteen, against the
// fourteen the record lists. The fifteenth is
// TestNoSecondReadOfARuntimeHomeVariable (ranger-base-uz4cs, landed the same
// day), whose own head comment had just written the OLD answer down — "this
// one reads its own directory and stays out of it" — which is the fourth
// carrier of that scoping and the reason the decision needed a derivation
// rather than a list. It says the new answer now. And the thing the decision
// told this bead to look for and not waive — a census globbing package-local
// FIXTURES rather than sources, a member by the key and perhaps not by
// intent — is not there: MEASURED 2026-10-31, none of the fifteen chdirs, so
// every `.`, `*.go` and `*_test.go` among them is the package's own sources
// read from the cwd `go test` installs. Zero to disposition.
//
// 39.1s cold and 9.1s warm over the three doors (MEASURED 2026-10-31, this
// bead, `make` itself included: 14.6s/11.0s/13.4s cold and 2.9s/3.3s/2.8s
// warm, at a one-minute load average of 35.5 rising to 43.0). Almost all of
// it is linking each arm's test binary — the fifteen pins report 2.7s
// between them — which agrees with ranger-base-m94g2's own ~40s/~5.7s over
// fourteen of them, and says the thing that decides how to read these three
// numbers: a seat that runs `make test` pays the linking once either way,
// and a seat that ran only arm 1 pays about 26s more for the two tagged
// doors because their binaries are cold.
//
// MUTATION-CHECKED, five of them, because what this bead added is two RULES
// and a FLOOR and all three are assertions about absence (MEASURED
// 2026-10-31, on an isolated copy of the tree so a sibling suite was not
// reddened by the planting):
//
//  1. a posse_arm2 pin moved into the UNTAGGED door variable reds arm 4
//     alone — the door now names a pin its own build cannot select.
//  2. pkgcensus-arm2-check losing its `-tags` reds arm 1 alone, and arm 4
//     stays green: arm 4 reads the door's `arm` and the pin is still in
//     arm 2, so it has nothing to object to. That pair is why the tag is
//     checked in two places and not one.
//  3. the arm-3 door relabelled `arm: 1` in twdDoors reds arm 1 AND arm 4 —
//     the register and the recipe disagreeing, read from both ends.
//  4. the union's relEnum seed going quiet reds arm 2, on fifteen doors it
//     can no longer justify. The mutation is the seed's PREDICATE and not
//     the line, because deleting the line is a compile error rather than a
//     red — and a predicate that stops matching is what the two earlier
//     widenings of this half actually looked like.
//  5. twdPosseEnumOnlyTests going quiet, with the union untouched, reds arm
//     2 through the new floor alone and with the floor's own message. That
//     is the case the floor exists for and the one no other arm can see.
//
// and THREE more, which are the shape the two halves of the derivation miss
// BECAUSE it is both of them at once (ranger-base-g31e0, from
// ranger-base-tw3t7's verify of the two beads above). A list of relative
// literals that mixes the package's OWN directory with a climb OUT of it,
// ranged over and walked — `for _, d := range []string{".",
// "../../cmd/posse"}` — was read by NEITHER rule. The FENCE binds at an
// AssignStmt or a ValueSpec, so a CompositeLit bound the slice and the range
// VARIABLE was never bound at all, which made 8yv7z's "once the fence reads
// an identifier it finds six" six of nine. The SEED voided the whole list on
// one unreadable element — right where it was written, because in
// internal/treepins a climb LEAVES the tree — and wrong here, where a climb
// is how a test REACHES it: the unreadable element makes the reading more
// tree-wide, and refusing the list threw away the readable `"."` that
// ranger-base-m94g2 had just decided is a member on its own. So the pins
// were outside the class for doing both, and the ~950s arm run was their
// only route:
//
//	TestQAKeychainStoreIsWiredToTheAbsoluteBinary make pkgcensus-check
//	                                              (`.` and `../../cmd`,
//	                                              walked — no file anywhere
//	                                              execs the bare word a
//	                                              persona's shim answers to,
//	                                              ranger-base-ypf5/r64)
//	TestExactModelHasNoDeclarationSurface         make pkgcensus-check
//	                                              (`.` and `../../cmd/posse`,
//	                                              ReadDir'd — ADR 0053 D5's
//	                                              declaration surface)
//	TestVisibilityOverrideIsNeverDispatched       make pkgcensus-check
//	                                              (`..` and `../../cmd`,
//	                                              walked — guardrail 4's
//	                                              rule half, the override out
//	                                              of every dispatched
//	                                              environment, and the one of
//	                                              the three whose silent
//	                                              unreachability mattered
//	                                              most)
//	                                              (all three
//	                                              ranger-base-g31e0)
//
// NO NEW DOOR, and the radius is why it needed saying: QA_PKGCENSUS_PINS is
// named for a class that was one package when na4rm doored it, and these
// three ask the same never-question one directory further out. The subject
// is a reading of this repo's own Go sources whose file SET is discovered —
// a bead that adds a .go file in that radius reds them — which is this
// door's subject exactly and the same key the fifteen are in on. 0.23s
// between the three (0.02s, 0.02s and 0.19s, over three warm runs that
// agreed to the hundredth, at a one-minute load average of 6.00).
//
// TWO FIXES, ONE TO EACH HALF, and which half takes a site is decided by
// MEASUREMENT and not by preference. The fence reads a range over a list now
// (twdClimbSet.list, climbingList) and the seed skips a CLIMBING element
// rather than voiding the list (twdRelTreeList) — and with both in, the two
// mixed lists are DERIVED members through their readable `"."`, while
// visibility_test.go's `[]string{"..", "../../cmd"}` is not, because every
// element of it climbs and the seed can read none of them. So that one took
// the root from qibRepoRoot, the house fix a third time, and the other two
// are doored where they stand: a census of internal/posse's own sources is a
// member AS a relative read (ranger-base-m94g2), and demanding a root of the
// mixed spelling alone would have treated it more harshly than the
// `"."`-only sibling the house blessed, for the same door.
//
// WHICH IS ALSO WHY THE FENCE ARM NOW READS relEnum (see its own comment
// below). Its sentence is "no rule in this file can see it, the pin gets no
// door" — true of visibility_test.go, false of the two the seed derives, and
// a prohibition that prints a false reason is worse than one that stays
// quiet. `!f.root` was already that condition for the direct rule; this is
// the same condition for the third seed.
//
// and one more, folded into seed-check's door because that door's own
// comment already says to type it when you add a detection fixture
// (ranger-base-zm6yw, FINDING 1 of the ranger-base-wq349 verify). The shape
// was in NEITHER half of the class: a climb whose root is a bare root with a
// SUBDIRECTORY joined onto it. The fence reads a Join's FIRST element, which
// here was `os.Getwd()`; the hidden-walker arm wants the two `..` at the END
// of the list, so it read the path as a walk of one subdirectory. MEASURED
// 2026-10-31 at main 5620d3b9 over five probes that each read every file
// under internal/ and differ only in how the root arrives — three scored, two
// scored by NOTHING — and ranger-base-56eln's residual paragraph said the
// shape "is the hidden arm's and is scored by its dataflow", which is the
// sentence a later reader trusts instead of re-censusing. It is corrected at
// twdClimbSet.bind, and twdJoinClimbsToASubpath is the arm:
//
//	TestQABobFixturesAreStillUnknownToHerdr       make seed-check
//	                                              (ADR 0060 D2's tripwire on
//	                                              etc/herdr/agent-detection/
//	                                              upstream/bob — a herdr
//	                                              release changes what it
//	                                              measures, and until this
//	                                              bead no door named it and
//	                                              no census could see it)
//	                                              (ranger-base-zm6yw)
//
// and one more beside it, in this file and in this register's own door: the
// live-defect pin that holds the half of the same finding which is a
// JURISDICTION decision rather than a rule gap — a subpath joined onto a
// bare-root IDENTIFIER, which detectionRig spells three times and which
// ranger-base-sx2dq ruled out of the class by name while the knownscreen pin
// one door over reads the sibling directory and was ruled in. It ships GREEN
// asserting the hole, with the live census beside it so the hole cannot be
// confused with a broken detector, and ranger-base-er0gn decides it:
//
//	TestQATheClimbFenceStillHasItsBareRootJoinHole
//	                                              make register-check
//	                                              (ranger-base-zm6yw)
//
// and `make tree-check` is all of them — one hundred and fourteen pins and
// seventeen doors, 55-56s over three warm runs at one hundred and nine of
// them (MEASURED 2026-10-31, ranger-base-h1q5o: 56s, 55s and 55s in that
// order, at a one-minute load average of 4.56, 3.70 and 3.25 before the
// three and no suite slot held by anyone; 72s on the run before them, with
// the two tagged binaries cold. This is the merge-back that landed
// ranger-base-na4rm's fifteen onto a main that had meanwhile landed
// ranger-base-8yv7z's eight, so it is the first reading of the class as it
// LANDS rather than as either side of the merge had it. The largest class
// this command has ever run and the FASTEST range ever read over it: a 1s
// spread, on the quietest box it has been read on. Which is the "ordered by
// nothing in it" finding at the end of its argument — three doors and
// twenty-six pins have joined the class since the 81-89s below and the
// command got twenty-five seconds FASTER, so nothing in the range is the
// class, and a seat pricing this command reads the BOX and not this
// sentence's number), 60-62s over three warm runs at one hundred and one of
// those pins — na4rm's own branch, which is this class without 8yv7z's eight
// and never was main (MEASURED 2026-10-31, ranger-base-na4rm: 61.8s, 60.2s
// and 62.0s in that order, at a one-minute
// load average of 15.28, 9.49 and 6.90 before the three and no suite slot
// held by anyone. The first reading over a class with a tagged door in it,
// and at the QUIET END of every range below, two seconds apart across a load
// average that halved. The class grew by fifteen pins and three `go test`
// starts and the command came in twenty seconds FASTER than the 81-89s read
// at eighty-three of them, which is the "ordered by nothing in it" finding
// arriving once more and from the quiet end: what the spread tracks is how
// much the box MOVES, and this box was steady. The three new doors cost 9.1s
// warm between them, measured alone, and the two tagged ones are 26s of that
// cold), 68-106s over three warm runs at ninety-four of them (MEASURED
// 2026-10-31, ranger-base-8yv7z: 81s, 68s and 106s in that order, with a
// one-minute load average of 11.15, 9.45 and 10.76 before the three and BOTH
// suite slots held by sibling seats throughout. A 38s spread read at a load
// that never left 9-11 and barely moved across the three — so that reading
// says the spread is not the load's LEVEL and not its MOVEMENT either, which
// is as far as the pair of readings two entries below could take it; what it
// is remains unmeasured, and the honest advice is still the quiet end of the
// range plus room for half as much again. The eight pins that bead adds
// report 0.464-0.514s measured alone, over three runs, so none of a 38s
// spread is theirs), 59s warm at eighty-six of
// those pins (MEASURED 2026-10-31, ranger-base-nt0pi:
// one warm run at a one-minute load average of 5.62, the quietest box this
// command had been read on until the entry above, and the first reading taken
// over that whole class), and 81-89s over three warm runs taken at
// eighty-three of those pins (MEASURED 2026-10-31, ranger-base-3e68k: 87s,
// 81s and 89s in that
// order, with a one-minute load average of 25.12, 27.90 and 26.08 before the
// three, both suite slots held by sibling seats for the first two and one
// for the third. This is the
// merge-back that landed ranger-base-pyl22's five onto a main that had
// meanwhile landed ranger-base-bm6j8's two, so it is the first reading of
// the class at its real size.
//
// AND IT WAS READ TWICE, an hour apart, over the same code — the second time
// because an AGENTS.md line written between the two reds arm 4, and a
// reading taken on a tree whose suite is red is not a reading of this tree.
// That accident is the most useful thing in this entry. The earlier three
// were 62s, 127s and 146s at a one-minute load of 23.45, 13.98 and 91.60: an
// 84s spread. These three are 8s apart at a load that never left 25-28. Same
// class, same doors, same box, one hour. So what this command's spread
// tracks is the MOVEMENT of the load and not its level — which is what every
// "ordered by nothing in it" reading below has been saying with only one
// reading's worth of evidence to say it with, and this is the pair that says
// it directly. Price the command at ninety seconds when the box is steady,
// whatever level it is steady AT, and expect two and a half minutes when it
// is not. What the seven new pins cost is known from their own beads and is
// in neither spread: 0.43s between ranger-base-pyl22's five, 0.24s and 0.21s
// for ranger-base-bm6j8's two)
// — which is the command a seat types after a filtered run. (It was
// 74.5-172.1s at eighty-one pins (ranger-base-pyl22: 172.1s, 164.7s and
// 74.5s in that order, with a
// one-minute load average of 57.34, 57.89 and 94.79 before the three and one
// suite slot held by a sibling seat throughout for the first two and BOTH
// held for the third. The widest reading this command has produced, on the
// busiest box it has been read on, and ordered by nothing in that load: the
// FASTEST run is the one
// at the highest load average and with both suite slots gone, which is
// ranger-base-9mjxb's finding for the third time and the fourth wide reading
// to say it. The five pins this bead adds report 0.43s between them,
// measured alone, so none of a 98s spread is theirs — price the command from
// the quiet end of these ranges and read the high end as what it costs
// beside other people's suites);
// 60-82s at seventy-eight pins
// (ranger-base-bm6j8: 60.3s, 71.9s and 82.3s in that order, with a one-minute
// load average of 8.05, 11.10 and 8.97 before the three and NO suite slot
// held by anyone — the first reading of this command ever taken here on an
// idle box, which is why its 22s spread is worth saying at all: there was no
// sibling suite to blame it on. The load average moved by three across the
// three runs and the wall moved by twenty-two, monotonically upward, in the
// order they were run and in no order of the load: that is the "ordered by
// nothing in it" finding of ranger-base-0ckgm and ranger-base-c768n
// arriving from the quiet end, where there is no sibling suite to blame it
// on. Price this command at a minute and expect ninety. The two pins this
// bead added report 0.24s and 0.21s, so none of the spread is theirs);
// 57.0-57.7s at seventy-six pins
// (ranger-base-v3tt4: 57.6s, 57.0s and 57.7s in that order, with a
// one-minute load average of 31.41, 13.93 and 10.10 before the three and one
// suite slot held by a sibling seat throughout. That was the merge-back that
// landed ranger-base-8wxbn's twelve onto a main that had meanwhile landed the
// two before them, so it was the first reading of the class at its real size
// — and a 0.7s spread read across a threefold move in the load average,
// which is the tightest any reading of this command has produced and says
// from the other side what the three wide readings below say: the box moved
// and the wall did not, so a spread, when there is one, is not that number.
// ranger-base-8wxbn measured its twelve on its own branch, at seventy-four,
// and read 56-112s there; what that reading is still good for is their COST,
// because the pins are the same twelve. Eleven of them report 0.70s between
// them, and the twelfth, TestQAEverySuiteArmTypeChecks, reports 0.14s on its
// own line because its three parallel subtests build one suite arm each —
// measured alone it is 2.44s, 2.50s and 2.72s of parallel subtest, so about
// 2.7s of wall. Three seconds of that command was those twelve pins and the
// rest was the box);
// 73-132s at sixty-four pins
// (ranger-base-0ckgm: 73.4s, 105.8s and 131.9s in that order, with a
// one-minute load average of 45.88, 19.41 and 31.54 before the three and
// BOTH suite slots held by sibling seats throughout — a 58s spread read
// with the load average never below 19, and ordered by nothing in it: the
// fastest run is the one at the HIGHEST load and the slowest the one in the
// middle, which is the same conclusion the two other wide readings below
// reached from the other side. The two pins it added report 0.03s and 0.27s,
// so none of that spread is theirs);
// 76-82s at sixty-two pins
// (ranger-base-2cttm: 82s, 81s and 76s in that order, with a one-minute load
// average of 15.86, 10.32 and 11.91 before the three and no sibling suite
// slot held — the tightest spread any reading of this command has produced,
// and the run at the top of that load was the slowest by six seconds, which
// is as close as these readings come to being ordered by the box at all. The
// pin it added reports 0.17s, so none of that was its cost);
// 66-139s at sixty-one pins
// (ranger-base-c768n: 115.3s, 139.0s and 66.3s in that order, with a
// one-minute load average of 9.3, 54.2 and 63.1 before the three and one
// suite slot held by a sibling seat throughout — so the FASTEST run was the
// one at the highest load, which is ranger-base-9mjxb's finding again and
// NOT the "ran beside somebody" reading further down: that spread is not
// ordered by the load average, and the two readings that say so are the two
// widest ones. The two pins it added each report 0.00s, so what they cost is
// below `go test`'s own per-test resolution and none of that spread is
// theirs; the brew-caller census ranger-base-mk6gw added reads every file
// under scripts/ rather than three named ones, and reports 0.08-0.19s over
// four runs — the one pin here whose cost is readable at all — and a whole
// `make tree-check` with it in measured 62.9s on 2026-10-31 at a one-minute
// load average of 19.7, inside that spread);
// 49-93s at fifty-six pins, at a
// one-minute load average of 128.0, 75.2 and 36.9 before the three runs with
// BOTH suite slots held by sibling seats throughout — 80s, 49s and 93s in
// that order, so the slowest run was the one at the LOWEST load and that
// spread is not a function of the load average either;
// 48.4-51.5s at fifty-five pins,
// at a load average of 9.6 to 36.5; 46-51s at fifty-three pins;
// 14.9-16.5s over twenty-three pins
// behind eight doors, before ranger-base-g6sb1 found a second package;
// 15-43s at twenty-two, 12-27s under
// ranger-base-8dnuy, and 40-46s at a smaller class before that. Re-measured
// whenever the class changes — under ranger-base-xrdb0, and again under
// ranger-base-ecchw, and again under ranger-base-vofbl, and again under
// ranger-base-nnnf1, and again under ranger-base-yrag8 — because the
// sentence a seat prices the command from
// should not quote a run of a class it did not run; and stated without a
// second numeral, deliberately: a historical count in this comment is
// invisible to arm 4's one-claim rule, which is ranger-base-erqvh row 2. The
// seconds are NOT pinned — an elapsed-seconds red belongs to the box, per
// the `test` target's own note, and a warm build cache is most of this
// spread — but they are measured, not carried. ALL FOUR 2026-10-04 readings
// say the spread is the box and not the cache. ranger-base-g6sb1 read 46.5-77.2s
// over three warm runs at forty-nine pins, the one-minute load average going
// 8.9 -> 23.8 across them with a sibling seat holding a suite slot
// throughout; ranger-base-vofbl re-read the same command at fifty-one, three
// warm runs later the same day with no sibling suite and a load of 5.4-7.1,
// and got 44-46s. The two pins it added cost 0.06s between them, so what
// moved between the two readings was the box. ranger-base-nnnf1 read 53-75s
// at fifty-two, three warm runs with the one-minute load average falling
// 24.2 -> 8.6 across them and the slowest run the one at the top of that
// fall; the pin it added costs 0.07s. ranger-base-hrf47 read 55.34s, 99.78s
// and 69.69s at the same count on the same day — it and nnnf1 each added one
// row to this enumeration in parallel and neither saw the other's — at a
// one-minute load average of 9.11, 8.36 and 25.06, with a sibling seat
// holding one of the two suite slots throughout; its one added pin costs
// 0.03s. The live seconds are ranger-base-yrag8's, which merged those two
// rows back together and re-read the command over the class they make: 51s,
// 46s and 48s, three warm runs with BOTH suite slots free throughout and a
// one-minute load average of 4.4 before the first and 9.1, 7.5 and 6.6 after
// each. That is the quiet end of every range above, read at the largest class
// yet, which is the same conclusion from the other side: the readings that
// ran wide ran beside somebody. So a seat pricing the command from the low end
// of any of these is reading a quiet box, and the high end is what it costs
// beside someone else's suite. Cold, with internal/treepins' test binary to
// compile as well as internal/posse's, the same command read 92.6s.)
//
// THAT SENTENCE IS THE ONLY LIVE COUNT IN THIS FILE, and arm 4 holds it to
// the Makefile, both numerals and the enumeration above it. It read seventeen
// and six until ranger-base-4jogv, and arm 4 is why it is not quoted here:
// the rule matches only a sentence shaped exactly `<word> pins and <word>
// doors`; a second sentence in that shape reds it, but a historical count
// spelled some other way — like the sentence just above, on purpose — is
// invisible to it. The DOOR number entered wrong at d189b623, which wrote
// "seven doors" over an enumeration of eight and was then faithfully
// decremented by one when the selector door went; the PIN number drifted on
// its own, because the three tests listed directly above were added to a
// door variable by beads that had no reason to read this comment. A seat
// prices `make tree-check` from this sentence, in the one file whose whole
// subject is "the doors are wide enough", so nothing but a derivation may
// say how many there are.
//
// WHY THE DOOR RUNS THE PIN. fmt-check re-runs the TOOL, because gofmt is a
// tool and `gofmt -l` cannot disagree with `go/format`. These four are Go:
// their reading is an ast parse, an unquote and a case-boundary scan, and a
// shell rewrite of that would be a second implementation to keep in sync by
// hand — a door that goes NARROWER than the pin while both look green, which
// is worse than no door at all. So each door is a `-run` filter naming the
// pin, and the only thing left to hold is that the filters name ALL of them.
//
// Four arms — rulbl's three, and a fourth that holds the sentence above:
//
//  1. the doors are wired — `make test` depends on tree-check, tree-check
//     reaches every door, and each door reads only (no `-w`, no `./...`,
//     and `-count=1`, because a door that answers from cache can lie).
//  2. the doors are WIDE ENOUGH, two-way and mechanically: the class is
//     derived by parsing internal/posse/*_test.go, and every member must be
//     named by a Makefile door variable, and every name in those variables
//     must be a member. A tree-wide pin added tomorrow reds this until it is
//     given a door, which is the whole deliverable.
//
//     THAT PROMISE WAS FALSE AS FIRST SHIPPED (ranger-base-sx2dq). The
//     derivation keyed on one identifier, qibRepoRoot, and the tree carried
//     a byte-identical twin of it (qspRepoRoot) plus a hand-rolled
//     `filepath.Abs("../..")`: three tree-wide pins were spelled outside the
//     class, got no door, and this arm said nothing. The twin is folded, the
//     hand-rolled climb goes through the helper, the derivation also follows
//     a test into a helper that WALKS from the root, and
//     TestQAOneRepoRootHelperInTheTestPackage below is what keeps a third
//     copy from re-opening the same hole. Five members became thirteen.
//
//     AND WAS STILL FALSE (ranger-base-xndgk FINDING 5). Both rules key on
//     GO's filesystem calls, and five pins took their root from `git
//     rev-parse --show-toplevel` — stdout from a subprocess, which neither
//     rule can see. One of them censuses every tracked PATH in the
//     repository, one `git grep`s every tracked FILE, one scans every
//     tracked markdown file, one reads the history of every shipped
//     example. All five now take the root from the one helper, that
//     spelling is fenced in TestQAOneRepoRootHelperInTheTestPackage, and
//     thirteen members became eighteen. It has moved both ways since — ADR
//     0016's socket hints and their selector pin went, three later pins
//     arrived — and this line deliberately stops counting there: the live
//     number is the one in the `make tree-check` sentence above, which arm 4
//     derives from the Makefile. The lesson the third rule is NOT:
//     the class was never bounded by how a test spells its root, so the
//     thing that bounds it is the single helper, not another rule.
//  3. the doors can FAIL: `make -n`'s own expansion of each, run for real
//     against a scratch copy of the tree carrying the real drift — a crew
//     name in a shipped file. Clean arm first, so a door that always fails is
//     not mistaken for one that detects.
//  4. the head comment's COUNT is the Makefile's — both numerals derived,
//     and every name in a door variable named up there, so the enumeration
//     the count rests on cannot go short while the count stays green
//     (ranger-base-4jogv) — AND the package README's door table is the same
//     list in the same order (ranger-base-r5546). That table is a second
//     copy of `tree-check`'s prerequisites in a page people outside this
//     file read; it was correct the day it shipped and nothing said so
//     after. The Makefile declines to write the door COUNT twice on
//     purpose, which is the same refusal one directory over.
//
//     AND AGENTS.md's DOOR BLOCK, one page further out again
//     (ranger-base-xed72) — the same list in the same order, in the page
//     every seat reads at session start. Standing orders carried a second
//     copy of the PIN count as well, held by nobody: it said four fewer than
//     the Makefile had, through two beads that doored a pin, corrected the
//     sentence above, and had no reason to read a crew-wide page. That page
//     states no count of this class at all now and points here instead, and
//     this arm holds both halves of that — the block against `tree-check`'s
//     prerequisites, and the absence of a numeral against the same spellings
//     twdSpelled writes.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The two packages in the class. internal/posse is ~950s whole and
// internal/treepins 589.965s, so a seat runs a `-run` filter in both and a
// tree-wide pin in either is nobody's subject.
const (
	twdPossePkg    = "internal/posse"
	twdTreepinsPkg = "internal/treepins"
)

// twdDoor is one Makefile door: the variable that holds its `-run` filter,
// the target that reads it, and the package its pins live in. The package is
// not decoration — arm 2 is two-way, and "this name is not a tree-wide test"
// is only answerable against the package the door actually runs.
type twdDoor struct {
	variable string
	target   string
	pkg      string
	// tool: the door re-runs the TOOL the pin runs rather than running the
	// pin under a `-run` filter. Allowed only where the pin's whole body IS
	// that tool invocation, so the two cannot disagree — gofmt for
	// fmt-check, scripts/notes-index.py for notes-check. Anywhere else it
	// would be a second implementation that can go narrower than the pin
	// while both look green.
	tool bool
	// arm: the internal/posse build arm this door's `-tags` selects, and 1
	// for a door that passes no tag at all — which every door did until
	// ranger-base-na4rm. It is here rather than in armtags_qa_test.go
	// because it is one fact about a door and this is the door table: arm 4
	// over there derives its (variable, arm) pairs from these rows, so the
	// hand-kept list that went a door short until ranger-base-6cznr cannot
	// drift again, and arm 1 below holds the Makefile recipe to the same
	// number. internal/treepins is one binary with no arm tags, so every
	// door into it is 1 by construction.
	arm int
}

// The Makefile variables that hold the class. One per door, plus the pins
// whose door is a tool rather than a filter — the union is what arm 2
// measures against the two packages.
var twdDoors = []twdDoor{
	{"QA_CREW_PINS", "crew-check", twdPossePkg, false, 1},
	{"QA_TOOL_PINS", "fmt-check", twdPossePkg, true, 1},
	{"QA_SEED_PINS", "seed-check", twdPossePkg, false, 1},
	{"QA_HISTORY_PINS", "history-check", twdPossePkg, false, 1},
	{"QA_DOC_PINS", "doc-check", twdPossePkg, false, 1},
	{"QA_IDENTITY_PINS", "identity-check", twdPossePkg, false, 1},
	{"QA_OPS_PINS", "ops-check", twdPossePkg, false, 1},
	{"QA_EXECWRITE_PINS", "execwrite-check", twdPossePkg, false, 1},
	// The one-package censuses (ranger-base-na4rm). One subject, three
	// doors, split by BUILD TAG and by nothing else: seven of the fifteen
	// are compiled out of the default build, and a `go test -run` over a
	// name that is not in the build exits 0.
	{"QA_PKGCENSUS_PINS", "pkgcensus-check", twdPossePkg, false, 1},
	{"QA_PKGCENSUS_ARM2_PINS", "pkgcensus-arm2-check", twdPossePkg, false, 2},
	{"QA_PKGCENSUS_ARM3_PINS", "pkgcensus-arm3-check", twdPossePkg, false, 3},
	// internal/treepins (ranger-base-g6sb1).
	{"QA_NOTES_PINS", "notes-check", twdTreepinsPkg, true, 1},
	{"QA_ADR_PINS", "adr-check", twdTreepinsPkg, false, 1},
	{"QA_CORPUS_PINS", "corpus-check", twdTreepinsPkg, false, 1},
	{"QA_REGISTER_PINS", "register-check", twdTreepinsPkg, false, 1},
	{"QA_SCRIPTS_PINS", "scripts-check", twdTreepinsPkg, false, 1},
	{"QA_PID_PINS", "pid-check", twdTreepinsPkg, false, 1},
}

// twdPinVars is the variable names alone, derived so the two lists cannot
// drift apart.
var twdPinVars = func() []string {
	var out []string
	for _, d := range twdDoors {
		out = append(out, d.variable)
	}
	return out
}()

// twdRootHelper is the ONE repo-root helper internal/posse's tests may use.
// It is a single identifier on purpose — the class below is derived from it,
// so a second spelling is a member of the class that the derivation cannot
// see. That is not hypothetical: the tree carried qspRepoRoot, byte-for-byte
// identical, and three tree-wide pins hid behind it (ranger-base-sx2dq).
const twdRootHelper = "qibRepoRoot"

// twdTreepinsRootHelper is the ONE repo-root helper internal/treepins' tests
// may use. Same fence as twdRootHelper, one package over, and it is here for
// a reason the posse half does not have: this package's TestMain chdirs the
// binary to the repo root, so a pin that needs the root as an ABSOLUTE path
// — to hand to a helper that walks a subtree of it — takes it from
// repoRoot(), and the walk is then rooted at that helper's PARAMETER, which
// the relative-literal rule cannot read. Four pins are spelled exactly that
// way (launchchecksources_qa_test.go, wallrendersources_qa_test.go) and sat
// outside the class for as long as the rule was literals only
// (ranger-base-8wxbn). So the derivation reads the HELPER CALL rather than
// the root's value — which is bounded only as long as there is one helper,
// the same bargain ranger-base-sx2dq struck next door.
const twdTreepinsRootHelper = "repoRoot"

// twdVar returns the `|`-separated test names a Makefile variable holds.
func twdVar(t *testing.T, makefile, name string) []string {
	t.Helper()
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, name))
		if !strings.HasPrefix(rest, ":=") {
			continue
		}
		var out []string
		for _, f := range strings.Split(strings.TrimSpace(strings.TrimPrefix(rest, ":=")), "|") {
			if f = strings.TrimSpace(f); f != "" {
				out = append(out, f)
			}
		}
		return out
	}
	t.Fatalf("the Makefile defines no %s — a door variable the pin below reads is gone, so the class has no recorded membership", name)
	return nil
}

// twdSameSet reports whether two name lists hold the same names, order aside.
func twdSameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

// Arm 1: the doors exist, `make test` opens them, and they only read.
func TestQAMakeTestOpensTheTreeWideDoors(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The umbrella reaches every door. It carries no recipe of its own, so
	// `make -n tree-check` prints exactly what a seat would have to type.
	//
	// Every membership below is TOKENS, not the line's bytes: see mkPrereqs
	// (ranger-base-hna69). This arm read the bytes until ranger-base-7kwb8,
	// which is worse here than at the three sites hna69 fixed, because ONE
	// `#` in front of the tail of `tree-check:` takes six of these seven
	// doors off the umbrella at once — make stops at the comment, the old
	// reader did not, and every `strings.Contains` below still found its
	// door's name sitting on the line.
	treeLine, tree := mkPrereqs(t, src, "tree-check")
	// Derived from twdDoors, not spelled: a door added to the register and
	// forgotten here would be a door this arm never checks the wiring of.
	var doors []string
	for _, d := range twdDoors {
		doors = append(doors, d.target)
	}
	for _, door := range doors {
		if !mkRuns(tree, door) {
			t.Errorf("`make tree-check` no longer reaches `%s`, so one tree-wide pin is back to being ~950s away: %q", door, treeLine)
		}
	}

	// And `make test` reaches the umbrella, so a full run fails on the class
	// in seconds instead of at ~950 (rulbl's reason for fmt-check).
	if testLine, deps := mkPrereqs(t, src, "test"); !mkRuns(deps, "tree-check") {
		t.Errorf("`make test` no longer depends on tree-check: %q", testLine)
	}

	// All seven doors, not four of them (ranger-base-x5bgs finding 2): a file
	// of any one of their names in the tree silences it the same way, and
	// fmt-check/seed-check/history-check/doc-check were checked at wiring
	// (arm above) but not here.
	phonyLine, phony := mkPrereqs(t, src, ".PHONY")
	if !mkRuns(phony, "tree-check") {
		t.Errorf(".PHONY does not name tree-check — a file of that name in the tree would silence it: %q", phonyLine)
	}
	for _, door := range doors {
		if !mkRuns(phony, door) {
			t.Errorf(".PHONY does not name %s — a file of that name in the tree would silence it: %q", door, phonyLine)
		}
	}

	for _, door := range twdDoors {
		if door.tool {
			// A tool door runs no `go test`, so none of the shape below
			// applies to it. fmt-check is held open by
			// gofmtdoor_qa_test.go; notes-check is held open just under
			// this loop, against the pin's own argv.
			continue
		}
		rawRecipe := makeRecipe(src, door.target)
		if len(rawRecipe) == 0 {
			t.Errorf("the Makefile has no `%s` target", door.target)
			continue
		}
		// mkRecipeCode, not the lines' bytes (ranger-base-x5bgs finding 1): a
		// recipe line is handed to the SHELL verbatim, so it is the shell's
		// comment rule that governs it, not make's — a `#` in front of the
		// tail silences the line for sh while every `strings.Contains` below
		// still found its bytes sitting after the `#`.
		recipe := strings.Join(mkRecipeCode(rawRecipe), "\n")
		if recipe == "" {
			t.Errorf("`make %s`'s recipe is all comments — make hands the shell a no-op and the door is dark: %q", door.target, rawRecipe)
			continue
		}
		// The door reads its filter from the one variable arm 2 measures.
		// Spelled here, so a door quietly narrowed to a literal subset of
		// the class fails rather than passing on a shorter list.
		if !strings.Contains(recipe, "'^($("+door.variable+"))$$'") {
			t.Errorf("`make %s` no longer runs exactly $(%s), anchored — the door and the pin it stands in for can now name different tests:\n%s", door.target, door.variable, recipe)
		}
		if !strings.Contains(recipe, "./"+door.pkg) {
			t.Errorf("`make %s` no longer names ./%s — a package-tree run of it is the wall this door exists to avoid:\n%s", door.target, door.pkg, recipe)
		}
		// And the door's `-tags`, both ways (ranger-base-na4rm). A door
		// that passes no tag runs the DEFAULT build, so a pin of its own
		// that is compiled out of it is selected by nothing and the door
		// exits 0 — which is what armtags_qa_test.go's arm 4 holds, against
		// this same `arm` number. Here is the other half of that pair: the
		// recipe has to ASK for the arm the register says it runs, or arm 4
		// is checking its pins against a build the Makefile never selects.
		if door.arm == 1 {
			if strings.Contains(recipe, "-tags ") {
				t.Errorf("`make %s` passes `-tags` and twdDoors says it runs arm 1, the default build — so arm 4 is checking this door's pins against a build this recipe does not run:\n%s", door.target, recipe)
			}
		} else if tag := armTagFor(door.arm); !strings.Contains(recipe, "-tags "+tag+" ") {
			t.Errorf("`make %s` does not pass `-tags %s`, and twdDoors says its pins live in arm %d — a `go test -run` over a name that is not in the build exits 0, so this door stands green in front of every pin it names:\n%s", door.target, tag, door.arm, recipe)
		}
		if strings.Contains(recipe, "./...") {
			t.Errorf("`make %s` runs the package tree, which is the wall it is a door through:\n%s", door.target, recipe)
		}
		// A door that can answer from cache is a door that can lie: the
		// drift these pins are about arrives as a new FILE in a walked
		// directory, and nothing promises go's test cache key notices one.
		if !strings.Contains(recipe, "-count=1") {
			t.Errorf("`make %s` may answer from go's test cache, so it can report a tree it never read:\n%s", door.target, recipe)
		}
		if strings.Contains(recipe, " -w") || strings.Contains(recipe, "rm ") {
			t.Errorf("`make %s` writes — the point of a door is that a seat can ask the question without changing the tree:\n%s", door.target, recipe)
		}
	}

	// notes-check, the second TOOL door (ranger-base-g6sb1). It is allowed
	// to be a tool door for fmt-check's reason and no other: the pin's whole
	// body is one exec.Command, so re-running that command is not a second
	// implementation of anything. What has to hold is that the recipe runs
	// the SAME command — so the argv is read out of the pin rather than
	// spelled here, and a flag added to either side without the other reds.
	argv := twdExecArgv(t, twdTreepinsPkg, "TestNotesFragmentIndexIsCurrent")
	if len(argv) < 2 {
		t.Errorf("TestNotesFragmentIndexIsCurrent no longer runs a single literal command (%v) — notes-check is a TOOL door, which is only honest while the pin's whole body is the command the door re-runs. If the pin has grown a reading of its own, the door has to become a `-run` filter.", argv)
	} else {
		rawNotes := makeRecipe(src, "notes-check")
		notes := strings.Join(mkRecipeCode(rawNotes), "\n")
		if notes == "" {
			t.Errorf("`make notes-check`'s recipe is all comments — make hands the shell a no-op and the door is dark: %q", rawNotes)
		}
		for _, word := range argv {
			if !strings.Contains(notes, word) {
				t.Errorf("`make notes-check` does not run %q, which TestNotesFragmentIndexIsCurrent does — the door and the pin it stands in for are now asking different questions:\n  pin:  %v\n  door: %s", word, argv, notes)
			}
		}
		if !strings.Contains(notes, "--check") {
			t.Errorf("`make notes-check` no longer passes --check, so it REGENERATES the index instead of reporting a stale one — a door that fixes the drift silently is a door that never reports it:\n%s", notes)
		}
	}
}

// twdExecArgv returns the literal argv of the one exec.Command in a named
// test, or nil if the test makes none or makes more than one. Used to read a
// tool door's command out of the pin rather than spelling it twice.
func twdExecArgv(t *testing.T, pkg, test string) []string {
	t.Helper()
	paths := twdTestFiles(t, pkg)
	fset := token.NewFileSet()
	var found [][]string
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || fn.Name.Name != test {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
					return true
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "exec" {
					return true
				}
				var argv []string
				for _, a := range call.Args {
					lit, ok := a.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true // not a literal argv; not this shape
					}
					v, err := strconv.Unquote(lit.Value)
					if err != nil {
						return true
					}
					argv = append(argv, v)
				}
				found = append(found, argv)
				return true
			})
		}
	}
	if len(found) != 1 {
		return nil
	}
	return found[0]
}

// mkRecipeCode returns a recipe's lines (as makeRecipe reads them) with any
// shell comment stripped, and drops lines left empty by that. makeRecipe
// hands a RECIPE line back whole, tab stripped, because that line is make's
// own text — but make then hands it to the SHELL verbatim, so what governs a
// `#` inside it is the shell's comment rule, not make's: a `#` starts a
// comment only where it begins a word (nothing, or whitespace, before it) and
// it is not inside a quote. A byte check against the raw line, uncorrected,
// still finds its bytes sitting after a `#` a seat put there to silence the
// line — or after one that only commented out the line's tail
// (ranger-base-x5bgs finding 1).
func mkRecipeCode(lines []string) []string {
	var out []string
	for _, line := range lines {
		var single, double bool
		cut := len(line)
		for i := 0; i < len(line); i++ {
			switch line[i] {
			case '\'':
				if !double {
					single = !single
				}
			case '"':
				if !single {
					double = !double
				}
			case '#':
				if !single && !double && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
					cut = i
				}
			}
			if cut != len(line) {
				break
			}
		}
		if c := strings.TrimSpace(line[:cut]); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// The reader itself, on the lines a whole-line byte check got wrong. Without
// this the only thing measuring the `#` stop is a Makefile mutant, and the
// suite never runs one.
func TestQARecipeLinesStopAtAnUnquotedShellComment(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   []string
		want []string
	}{
		{"plain line", []string{"$(GOBIN) test ./internal/posse -count=1"}, []string{"$(GOBIN) test ./internal/posse -count=1"}},
		{"whole line commented", []string{"# $(GOBIN) test ./internal/posse -count=1"}, nil},
		{"commented tail", []string{"$(GOBIN) test ./internal/posse -count=1 # -run '^($(QA_CREW_PINS))$$'"}, []string{"$(GOBIN) test ./internal/posse -count=1"}},
		{"# inside single quotes is not a comment", []string{"echo '#not-a-comment' -count=1"}, []string{"echo '#not-a-comment' -count=1"}},
		{"# inside double quotes is not a comment", []string{`echo "#not-a-comment" -count=1`}, []string{`echo "#not-a-comment" -count=1`}},
		{"# glued to a word is not a comment", []string{"echo a#b"}, []string{"echo a#b"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := mkRecipeCode(c.in)
			if !slices.Equal(got, c.want) {
				t.Errorf("mkRecipeCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// twdJoinsTwoDotDots reports whether an argument list ENDS in two ".."
// literals — `filepath.Join(dir, "..", "..")`, the climb from internal/posse
// to the repo root written out by hand.
//
// It must be the end of the list, and that is the whole distinction between
// this class and its neighbours: `filepath.Join("..", "..", "examples",
// "agents")` climbs to the root and then goes back DOWN into one directory,
// so a walk from it reads examples/agents and reds only when that directory
// changes. Nine tests do that, none of them tree-wide.
func twdJoinsTwoDotDots(args []ast.Expr) bool {
	if len(args) < 2 {
		return false
	}
	for _, a := range args[len(args)-2:] {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		if v, err := strconv.Unquote(lit.Value); err != nil || v != ".." {
			return false
		}
	}
	return true
}

// twdJoinClimbsToASubpath reports whether a filepath.Join's argument list
// spells the climb from internal/posse to the repo root PAST its first
// element and then goes back DOWN — `filepath.Join(wd, "..", "..", "etc",
// "herdr")`, the climb written beside a base this file cannot read.
//
// WHY IT IS A SECOND ARM AND NOT A WIDER twdClimbRoot (ranger-base-zm6yw,
// FINDING 1 of the ranger-base-wq349 verify). twdClimbRoot reads only the
// FIRST element, for the reason its own comment gives: whatever follows, the
// path starts outside the package. When the base is `os.Getwd()` the climb is
// not the first element, and NOTHING in this file scored the reading — not
// the fence, whose first-element read answers false, and not the `hidden`
// arm, whose twdJoinsTwoDotDots requires the two `..` at the END of the list
// and therefore reads this as a walk of a SUBDIRECTORY rather than of the
// root. MEASURED 2026-10-31 at main 5620d3b9 over five probes that all read
// every file under internal/ and differ ONLY in how the root arrives: three
// scored (climbEnum or handRolledTreeWalk) and two scored by nothing, and
// this was one of the two. It was live that day —
// internal/posse/bobtripwire_qa_test.go, in no door variable, its subject
// what an unrelated bead puts in a fixture directory.
//
// THE JURISDICTION LINE IS bind's, which is why something must FOLLOW the
// pair: `filepath.Join(wd, "..", "..")` evaluates to the repo root itself,
// and a hand-rolled climb to the ROOT is twdRootExprs' and the `hidden`
// arm's, which do score it (MEASURED on the same five probes). Only a Join
// that climbs and then NAMES something is the fence's, and the two still do
// not overlap.
func twdJoinClimbsToASubpath(args []ast.Expr) bool {
	lit := func(e ast.Expr) (string, bool) {
		b, ok := e.(*ast.BasicLit)
		if !ok || b.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(b.Value)
		if err != nil {
			return "", false
		}
		return filepath.ToSlash(v), true
	}
	dotdot := func(e ast.Expr) bool {
		v, ok := lit(e)
		return ok && v == ".."
	}
	// A segment that is not part of the climb — the thing the path NAMES —
	// asked of twdClimbNamesASubpath against an EMPTY set, so this arm and
	// bind's jurisdiction line are one matcher and cannot drift. The empty
	// set is what makes it literal-only: an identifier resolves through the
	// val half, which nothing has put anything in here.
	empty := twdNewClimbSet()
	names := func(e ast.Expr) bool {
		return twdClimbNamesASubpath(e, empty)
	}
	for i := 0; i+2 < len(args); i++ {
		if !dotdot(args[i]) || !dotdot(args[i+1]) {
			continue
		}
		for _, a := range args[i+2:] {
			if names(a) {
				return true
			}
		}
	}
	return false
}

// twdClimbRoot reports whether an expression spells a relative path that
// climbs OUT of the package it is written in: a `..` literal, a literal
// beginning `../`, or a filepath.Join whose first element is one.
//
// It is deliberately not twdRelTreePath's negative case. That rule refuses a
// climb because in internal/treepins a climb leaves the tree; here a climb
// is how a test in internal/posse reaches the tree, and the point is that
// doing so computes no root, so neither rule in this file can see the
// reading. Only the FIRST element is read: whatever follows, the path starts
// outside the package.
//
// What it does not read is a climb that arrives through an identifier —
// `const dir = "../../etc/claude/managed-settings.d"; os.ReadDir(dir)`,
// which is live in internal/posse today (policyhooks_qa_test.go) and filed
// rather than fenced here (ranger-base-nt0pi). The fence is as wide as the
// three spellings that carried a pin, and no wider, for the same reason
// twdAsksGitForTheRoot states: nothing here can enumerate every way to name
// a directory, and what bounds the class is there being one helper. So this
// is as wide as the spelling that actually carried pins, and no wider.
func twdClimbRoot(e ast.Expr) bool {
	climbs := func(x ast.Expr) bool {
		lit, ok := x.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return false
		}
		v = filepath.ToSlash(v)
		return v == ".." || strings.HasPrefix(v, "../")
	}
	switch v := e.(type) {
	case *ast.BasicLit:
		return climbs(v)
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || len(v.Args) == 0 {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || x.Name != "filepath" {
			return false
		}
		switch sel.Sel.Name {
		case "Join":
			// The first element, and — since ranger-base-zm6yw — the climb
			// spelled past it beside a base this file cannot read.
			return climbs(v.Args[0]) || twdJoinClimbsToASubpath(v.Args)
		case "Clean", "ToSlash":
			return twdClimbRoot(v.Args[0])
		}
	}
	return false
}

// twdClimbSet is what an identifier naming a climb OUT of the package can be
// read through: the names, at one package's file scope and in one function
// body, bound to an expression twdClimbRoot calls a climb — and, since
// ranger-base-g31e0, the names bound to a LIST of them, from which a range
// variable inherits the climb.
//
// WHY IT EXISTS (ranger-base-8yv7z). ranger-base-nt0pi fenced the climb
// spelled AT the walker and said its residual out loud: a climb that arrives
// through an IDENTIFIER is unread. It was live in the tree that same day —
// `const dir = "../../etc/claude/managed-settings.d"; os.ReadDir(dir)` in
// internal/posse/policyhooks_qa_test.go — carrying a pin whose population is
// every drop-in in that directory, outside the class and undoored. So the
// fence reads the identifier now.
//
// IT IS NOT twdRelEnv, and the difference is which side of the file it serves.
// twdRelEnv feeds a DERIVATION: a name it resolves wrongly is a member the
// register invents or drops, which is a door asked for or not asked for, so it
// is kept narrow and every refusal in its head comment is load-bearing. This
// set feeds a PROHIBITION. Its only consequence is a seat being told to take
// the root from qibRepoRoot and join onto it, which is the right advice for
// any body this could match — so where the derivation must be exact, the
// fence may be generous, and that is the whole reason the two are separate
// maps rather than one.
type twdClimbSet struct {
	// val: a name bound to a climb that NAMES a path below the repo root.
	val map[string]bool
	// list: a name bound to a []string holding at least one such climb —
	// `roots := []string{"..", "../../cmd"}`, which binds the SLICE and not
	// its elements, so until ranger-base-g31e0 nothing about it entered this
	// set and `for _, r := range roots { filepath.Walk(r, …) }` was unread.
	//
	// SEPARATE FROM val FOR twdRelEnv's REASON, one rule over: ranging a
	// STRING yields its runes, so a range variable may inherit a climb only
	// from something this rule knows is a slice of paths. One map would bind
	// `r` in `dir := "../../etc"; for _, r := range dir`, where r is a rune.
	list map[string]bool
}

// twdNewClimbSet is the empty set, by one function so the two maps cannot be
// half-built anywhere.
func twdNewClimbSet() twdClimbSet {
	return twdClimbSet{val: map[string]bool{}, list: map[string]bool{}}
}

// climbs reports whether an expression spells a climb out of the package,
// reading identifiers through the set: a literal exactly as twdClimbRoot
// reads one, an identifier the set names, or either of those under
// filepath.Join (first element only, as twdClimbRoot does) or
// Clean/ToSlash/FromSlash/Abs.
func (c twdClimbSet) climbs(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return c.val[v.Name]
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || len(v.Args) == 0 {
			break
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			break
		}
		switch sel.Sel.Name {
		case "Join":
			if c.climbs(v.Args[0]) {
				return true
			}
			// Not a return: a Join whose FIRST element is not a climb may
			// still spell one past it, and twdClimbRoot below reads that
			// (ranger-base-zm6yw).
		case "Clean", "ToSlash", "FromSlash", "Abs":
			return c.climbs(v.Args[0])
		}
	}
	return twdClimbRoot(e)
}

// bind adds name if its right-hand side climbs AND the path it spells names
// something BELOW the repo root — or if it spells a `[]string` with such an
// element, in which case the name goes in the LIST half (ranger-base-g31e0),
// because what a range over it yields is the element and not the slice. `_`
// is never a name.
//
// THE SECOND CONDITION IS THE JURISDICTION LINE, and it is what keeps this
// arm off nine tests it would be wrong about. A climb made of nothing but
// `..` segments evaluates to the repo root itself, and a hand-rolled climb to
// the root is already the business of twdRootExprs and the `hidden` arm
// below — which carries the dataflow precision the posse half needs:
// detectionRig (verifydetection_qa_test.go) holds `filepath.Join("..", "..")`
// in a local and then ReadDirs `filepath.Join(repo, "etc", "herdr",
// "agent-detection")`, and the posse half scores that as a SUBDIRECTORY walk
// on purpose — it reds when that one directory changes, which that rule says
// is not this class (twdRootExprs' head comment, ranger-base-sx2dq). Binding
// the bare root here would have fenced it, and the fence's remediation —
// take the root from qibRepoRoot — would then have made `root && walk` true
// in that body and put its nine callers in the class.
//
// So: the climb arm owns a climb that NAMES a path, the root arms own the
// climb that names the ROOT, and the two do not overlap. A literal bare
// `../..` written AT the walker is the one place both can fire, because
// twdClimbRoot below reads it directly and ranger-base-nt0pi's fixture says
// it must; nothing is lost either way, since the `hidden` arm reads that
// spelling too.
//
// RESIDUAL, said out loud and MEASURED this time: a named path built by
// JOINING onto a bare-root identifier — `repo := filepath.Join("..", "..");
// os.ReadDir(filepath.Join(repo, "etc"))` — is unread here, because the
// bare-root identifier never enters this set and nothing built from it does.
//
// WHAT THIS PARAGRAPH USED TO SAY, AND WHY IT IS GONE (ranger-base-zm6yw,
// FINDING 1 of the ranger-base-wq349 verify). It said that shape "is the
// hidden arm's and is scored by its dataflow". It is not, and that was the
// sentence a later reader trusted instead of re-censusing. MEASURED
// 2026-11-01 at main 5620d3b9 through twdAnalyze itself: the hidden arm fires
// only when the walk's root IS the bare root — twdRootExprs scores
// `filepath.Join(repo, "etc")` false because twdJoinsTwoDotDots wants the two
// `..` at the END of the list — so for that body climbEnum, handRolledTreeWalk,
// root and relEnum are ALL false and the reading is scored by nothing.
// TestQATheClimbFenceStillHasItsBareRootJoinHole holds the hole open, with the
// live census beside it, so it cannot go quiet a second time.
//
// It is left open DELIBERATELY and not fenced, because fencing it is a
// jurisdiction decision and not a rule gap: detectionRig
// (verifydetection_qa_test.go) is exactly this shape, twdRootExprs' head
// comment rules it out of the class BY NAME (ranger-base-sx2dq), and the
// knownscreen pin one door over reads the sibling directory under
// etc/herdr/agent-detection and was ruled IN (ranger-base-8yv7z, doored in
// $(QA_SEED_PINS)). Those two rulings cannot both stand, and whichever wins
// moves detectionRig's callers into or out of the class — so it is filed as
// ranger-base-er0gn (-l architecture) rather than decided here.
//
// What IS closed is the same reading spelled in ONE Join —
// `filepath.Join(os.Getwd(), "..", "..", "etc", "herdr")`, where the climb
// and the subpath are in the same argument list and no identifier holds the
// root. That needs no jurisdiction call, because the value NAMES a path and
// the root arms never claimed it: twdJoinClimbsToASubpath above.
func (c twdClimbSet) bind(name string, rhs ast.Expr) {
	if name == "_" {
		return
	}
	// A LIST of paths, one of which climbs: the name holds the slice, and a
	// range over it binds its value variable in extend below.
	if c.climbingList(rhs) {
		c.list[name] = true
	}
	if !c.climbs(rhs) {
		return
	}
	if twdClimbNamesASubpath(rhs, c) {
		c.val[name] = true
	}
}

// climbingList reports whether an expression spells a `[]string` with at
// least one element bind would take on its own — a climb that NAMES a path
// below the root. An inline composite literal is read element by element; an
// identifier resolves through the list half of the set.
//
// WHY THE FENCE READS A LIST (ranger-base-g31e0, ranger-base-tw3t7's FINDING
// 1). The fence binds at an AssignStmt or a ValueSpec, so a climb that
// reaches the walker as a RANGE VARIABLE over a list of literals was unread:
// the right-hand side is a CompositeLit, climbs() answers false for one, and
// the range value was never bound at all. That spelling was live three times
// in internal/posse — `roots := []string{"..", "../../cmd"}` walked in
// visibility_test.go, `[]string{".", "../../cmd"}` in gatedkeychain_test.go,
// `[]string{".", "../../cmd/posse"}` in exactmodel_qa_test.go — and it is
// the spelling twdRelEnv has read since ranger-base-g6sb1, one rule over, so
// the fence was narrower than the derivation it serves on a shape its sibling
// already resolved.
//
// THE JURISDICTION LINE IS THE ELEMENT'S, not the list's: an element is
// tested by exactly the pair of conditions bind applies to a scalar, so a
// range over bare roots only — `[]string{"..", "../.."}` — stays
// twdRootExprs' and the hidden arm's, and widening the fence to lists cannot
// take detectionRig's nine callers with it (ranger-base-sx2dq). The fixture
// table below types both sides of that.
func (c twdClimbSet) climbingList(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return c.list[v.Name]
	case *ast.CompositeLit:
		at, ok := v.Type.(*ast.ArrayType)
		if !ok || at.Len != nil {
			return false
		}
		if id, ok := at.Elt.(*ast.Ident); !ok || id.Name != "string" {
			return false
		}
		for _, el := range v.Elts {
			if c.climbs(el) && twdClimbNamesASubpath(el, c) {
				return true
			}
		}
	}
	return false
}

// twdClimbNamesASubpath reports whether a climbing expression spells a path
// with at least one element that is not `..` — `../../etc/claude` does,
// `../..` and `filepath.Join("..", "..")` do not. Identifiers resolve through
// the VAL half of the set, which holds only subpath-naming climbs, so an
// identifier in it names one by construction. A list element is read
// directly rather than through either half — climbingList tests it with this
// function and climbs() both, which is how the jurisdiction line stays per
// element.
func twdClimbNamesASubpath(e ast.Expr, c twdClimbSet) bool {
	named := func(s string) bool {
		for _, el := range strings.Split(filepath.ToSlash(s), "/") {
			if el != ".." && el != "." && el != "" {
				return true
			}
		}
		return false
	}
	switch v := e.(type) {
	case *ast.Ident:
		return c.val[v.Name]
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return false
		}
		str, err := strconv.Unquote(v.Value)
		return err == nil && named(str)
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok || len(v.Args) == 0 {
			return false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return false
		}
		switch sel.Sel.Name {
		case "Clean", "ToSlash", "FromSlash", "Abs":
			return twdClimbNamesASubpath(v.Args[0], c)
		case "Join":
			for _, a := range v.Args {
				if twdClimbNamesASubpath(a, c) {
					return true
				}
			}
		}
	}
	return false
}

// bindDecls reads one `const`/`var` spec list for climbing identifiers.
func (c twdClimbSet) bindDecls(specs []ast.Spec) {
	for _, spec := range specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || len(vs.Names) != len(vs.Values) {
			continue
		}
		for i, name := range vs.Names {
			c.bind(name.Name, vs.Values[i])
		}
	}
}

// twdPkgClimbSet reads one package's FILE-SCOPE consts and vars for climbing
// paths, over every file at once because a const declared in one file roots a
// walk in another — the reason twdPkgRelEnv is built that way too.
func twdPkgClimbSet(files []*ast.File) twdClimbSet {
	c := twdNewClimbSet()
	// Two passes, so a declaration that reads one written below it binds.
	for i := 0; i < 2; i++ {
		for _, file := range files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
					continue
				}
				c.bindDecls(gd.Specs)
			}
		}
	}
	return c
}

// extend returns the package set plus ONE function body's own locals and
// consts — `const dir = "../../etc/claude/managed-settings.d"` is a local
// const, which is the spelling this bead was filed for. Two passes, for the
// reason twdRootExprs makes two.
func (c twdClimbSet) extend(body *ast.BlockStmt) twdClimbSet {
	out := twdNewClimbSet()
	for k := range c.val {
		out.val[k] = true
	}
	for k := range c.list {
		out.list[k] = true
	}
	for i := 0; i < 2; i++ {
		ast.Inspect(body, func(n ast.Node) bool {
			switch st := n.(type) {
			case *ast.AssignStmt:
				// `x := expr` and `x, err := expr` both bind x, the shape
				// twdRootExprs and twdBodyRelEnv both read.
				if len(st.Rhs) == 1 && len(st.Lhs) >= 1 {
					if id, ok := st.Lhs[0].(*ast.Ident); ok {
						out.bind(id.Name, st.Rhs[0])
					}
					return true
				}
				if len(st.Lhs) != len(st.Rhs) {
					return true
				}
				for j, l := range st.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						out.bind(id.Name, st.Rhs[j])
					}
				}
			case *ast.ValueSpec:
				if len(st.Names) != len(st.Values) {
					return true
				}
				for j, name := range st.Names {
					out.bind(name.Name, st.Values[j])
				}
			case *ast.RangeStmt:
				// The VALUE of a range over a list holding a climb that
				// names a path (ranger-base-g31e0): the body walks EVERY
				// element, so the variable can hold that climb and the
				// reading is the tree. The list may be written at the range
				// or held in a local or a package var, which is why bind
				// above keeps the slice's name. The KEY is an index and is
				// never bound, and the list half is never read from a
				// string — see twdClimbSet's own head comment.
				id, ok := st.Value.(*ast.Ident)
				if !ok || id.Name == "_" {
					return true
				}
				if out.climbingList(st.X) {
					out.val[id.Name] = true
				}
			}
			return true
		})
	}
	return out
}

// twdClimbHops is the fence's other half (ranger-base-8yv7z): for each
// function, the parameter positions at which a path handed in reaches a
// walker — directly, or through any number of further calls that pass that
// same parameter on.
//
// THE SHAPE IT IS FOR. internal/posse's shelf helper was
// `shelfPIDs()` -> `shelfPIDsIn(t, filepath.Join("..", "..", "examples", "agents"))`
// -> `statShelf(dir)` -> `filepath.Glob(filepath.Join(dir, "*.md"))`: the
// whole shelf read through a climb spelled TWO call sites from the walk.
// Three tree-wide pins sat behind it and the register saw none of them.
//
// AND WHY FOLLOWING THE CHAIN IS ALLOWED HERE AND NOWHERE ELSE. twdRelEnv and
// twdTestFiles both refuse to follow a value across a function boundary, and
// twdParamHops resolves exactly ONE argument at ONE call site because a wrong
// resolution there is a wrong class membership — a door demanded for a
// fixture walk, or a pin quietly leaving the class. This is a prohibition, so
// it has no such cost: it decides nothing about membership and its only
// output is an instruction to take the root from qibRepoRoot, which is the
// right instruction for every body it can match. So the fence closes the
// relation over calls and the derivation does not, and neither is a copy of
// the other.
//
// It propagates a PARAMETER POSITION and not a path: the climbing literal is
// still resolved at exactly one place, the outermost call site, by
// twdClimbSet above.
func twdClimbHops(fset *token.FileSet, files map[string]*ast.File) map[string][]int {
	at := map[string]map[int]bool{}
	add := func(fn string, idx int) bool {
		if at[fn] == nil {
			at[fn] = map[int]bool{}
		}
		if at[fn][idx] {
			return false
		}
		at[fn][idx] = true
		return true
	}
	// The base case is the walker table the derivation reads, so the fence
	// and the hop cannot disagree about what a param-rooted walk is.
	for fn, ws := range twdParamHops(fset, files) {
		for _, w := range ws {
			add(fn, w.idx)
		}
	}
	decls := rpDecls(fset, files)
	for changed := true; changed; {
		changed = false
		for _, d := range decls {
			params := rpParamIndex(d.fd)
			if len(params) == 0 {
				continue
			}
			carries := rpCarriers(d.fd.Body, params)
			ast.Inspect(d.fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				for idx := range at[id.Name] {
					if len(call.Args) <= idx {
						continue
					}
					param, ok := rpRootParam(call.Args[idx], params, carries)
					if !ok {
						continue
					}
					if add(d.fd.Name.Name, params[param]) {
						changed = true
					}
				}
				return true
			})
		}
	}
	out := map[string][]int{}
	for fn, idxs := range at {
		for idx := range idxs {
			out[fn] = append(out[fn], idx)
		}
		sort.Ints(out[fn])
	}
	return out
}

// twdPkgEnv is everything twdAnalyze needs that is a property of the PACKAGE
// rather than of the one body it reads: the relative-path env, the climbing
// env, the derivation's one-step walker table and the fence's closed one.
// Built once per parse, and by one function, so the two fixture drivers below
// cannot assemble a friendlier version of it than twdParseDir does.
type twdPkgEnv struct {
	rel       *twdRelEnv
	climb     twdClimbSet
	hops      map[string][]rpRootWalker
	climbHops map[string][]int
}

// twdNewPkgEnv builds that env from one package's parsed test files.
func twdNewPkgEnv(fset *token.FileSet, byPath map[string]*ast.File) *twdPkgEnv {
	var files []*ast.File
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		files = append(files, byPath[path])
	}
	return &twdPkgEnv{
		rel:       twdPkgRelEnv(files),
		climb:     twdPkgClimbSet(files),
		hops:      twdParamHops(fset, byPath),
		climbHops: twdClimbHops(fset, byPath),
	}
}

// twdWalkers are the calls that turn a repo root into a reading of the whole
// tree. A function that computes the root AND makes one of these is walking
// from the root, and every file in the tree is then its subject.
var twdWalkers = map[string]bool{"WalkDir": true, "Walk": true, "ReadDir": true, "Glob": true}

// twdFunc is one function declared in internal/posse/*_test.go: who it calls,
// and whether its own body reaches outside the package.
type twdFunc struct {
	name  string
	where string
	test  bool
	calls map[string]bool
	// root: the body calls qibRepoRoot, the package's ONE repo-root helper.
	root bool
	// rootedWalk: the body computes the root AND walks from it, so anything
	// added anywhere in the tree can red whoever calls it.
	rootedWalk bool
	// walk: the body calls one of twdWalkers.
	walk bool
	// ascent: the body spells the climb from internal/posse to the repo
	// root by hand — `"..", ".."` joined, or a `"../.."` literal.
	ascent bool
	// caller: the body asks runtime.Caller where its own source lives,
	// which is how qibRepoRoot finds the root without depending on cwd.
	caller bool
	// handRolledTreeWalk: the body walks a directory whose path IS the repo
	// root reached by a hand-rolled climb — not one reached through
	// qibRepoRoot, and not a subdirectory of it.
	handRolledTreeWalk bool
	// relEnum: the body ENUMERATES a directory at a relative tree path —
	// a WalkDir/Walk/ReadDir/Glob whose root is a literal relative path, or
	// a filepath.Join of literals starting at one. This is the
	// internal/treepins half of the class (ranger-base-g6sb1): that
	// package's TestMain chdirs the whole binary to the repo root, so a
	// relative path IS a tree path and no root helper is ever named.
	//
	// It also carries the PARAMETER HOP (ranger-base-pyl22): a call handing
	// a relative tree path to a helper that roots a walk at that very
	// parameter enumerates the tree just as plainly, and the entry says so
	// — `Walk(.)@arParse` rather than `Walk(docs/adr)`. One step, at the
	// call site; see twdParamHops.
	relEnum []string
	// execTree: relative literals naming a tracked regular file that this
	// body hands to exec.Command — this repo's own programs, run on the
	// tree. What they read is outside Go, so they are dispositioned rather
	// than derived (twdIndirect, twdExecDispositions).
	execTree []string
	// treeRoot: the body calls repoRoot, internal/treepins' ONE repo-root
	// helper. Over there the root is an absolute path handed to a walking
	// helper, so the CALL is the evidence and not the value — see
	// twdTreepinsRootHelper.
	treeRoot bool
	// getwdClimb: the body climbs from os.Getwd() looking for go.mod. That
	// is the spelling repoRoot itself is written in, and therefore the
	// spelling a twin of it would be written in — internal/treepins'
	// equivalent of `caller && ascent` next door.
	getwdClimb bool
	// climbEnum: the body ENUMERATES through a relative literal that climbs
	// OUT of its package — `filepath.Glob("../../docs/notes.d/notes-*.md")`,
	// `filepath.Glob(filepath.Join("..", "..", "examples", "skills", "*"))`.
	// In internal/posse that is a reading of the tree which computes no
	// root at all, so every rule in this file is blind to it: three
	// tree-wide pins were spelled that way and had no door
	// (ranger-base-nt0pi). It is the FOURTH spelling the fence below
	// refuses, beside a twin helper, a hand-rolled climb and git.
	climbEnum bool
	// shellRoot: the body asks GIT for this repo's root —
	// `exec.Command("git", "rev-parse", "--show-toplevel").Output()` — and
	// keeps the answer. A third spelling of the root, invisible to every
	// rule above, which is how five tree-wide pins sat undoored
	// (ranger-base-xndgk FINDING 5).
	shellRoot bool
}

// twdAsksGitForTheRoot reports whether a call is
// `exec.Command("git", ..., "--show-toplevel", ...).Output()` (or
// CombinedOutput) — git asked for this repo's root with the answer KEPT.
//
// The value being kept is the whole distinction. Four calls in
// silentrevert_qa_test.go ask git the same question and throw the answer
// away, as a probe for "is there a checkout here at all" — as does
// qibSkipUnlessCheckout, which asks --git-dir. A probe is not a second way
// to compute the root, and it is not fenced.
//
// RESIDUAL, said out loud: this matches the exec.Command spelling, which is
// what all four offending call sites wrote and what the next one would
// write. A root taken from the package's own git(dir, ...) helper with dir
// spelled "." is not matched. Nothing here can enumerate every way to name
// this repo — what bounds the class is that there is ONE helper, and the pin
// below fences the three spellings that have actually appeared: a twin
// helper, a hand-rolled climb, and this.
func twdAsksGitForTheRoot(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Output" && sel.Sel.Name != "CombinedOutput") {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || (fn.Sel.Name != "Command" && fn.Sel.Name != "CommandContext") {
		return false
	}
	if x, ok := fn.X.(*ast.Ident); !ok || x.Name != "exec" {
		return false
	}
	for _, a := range inner.Args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if v, err := strconv.Unquote(lit.Value); err == nil && v == "--show-toplevel" {
			return true
		}
	}
	return false
}

// twdRootExprs finds, inside one function body, every expression that
// evaluates to the repo root reached by hand: a bare `"../.."`, a
// `filepath.Join(..., "..", "..")`, those wrapped in Abs/Clean/EvalSymlinks,
// and the identifiers assigned from any of them. It returns a predicate over
// expressions.
//
// This is a small dataflow rather than "the body mentions `..` and walks
// something" on purpose, and the difference is nine tests: detectionRig
// climbs to the root and then ReadDirs `<root>/etc/herdr/agent-detection`,
// which reds when that one directory changes. Only a walk rooted AT the root
// is tree-wide.
func twdRootExprs(body *ast.BlockStmt) func(ast.Expr) bool {
	roots := map[string]bool{}
	var isRoot func(ast.Expr) bool
	isRoot = func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.Ident:
			return roots[v.Name]
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return false
			}
			return filepath.ToSlash(s) == "../.." || filepath.ToSlash(s) == "../../"
		case *ast.CallExpr:
			sel, ok := v.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || x.Name != "filepath" {
				return false
			}
			switch sel.Sel.Name {
			case "Join":
				return twdJoinsTwoDotDots(v.Args)
			case "Abs", "Clean", "EvalSymlinks", "ToSlash":
				return len(v.Args) == 1 && isRoot(v.Args[0])
			}
		}
		return false
	}
	// Two passes, so `root := ...` before the walk and after both bind.
	for i := 0; i < 2; i++ {
		ast.Inspect(body, func(n ast.Node) bool {
			var lhs, rhs []ast.Expr
			switch st := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = st.Lhs, st.Rhs
			case *ast.ValueSpec:
				for _, id := range st.Names {
					lhs = append(lhs, id)
				}
				rhs = st.Values
			default:
				return true
			}
			// `x := expr` and `x, err := expr` both bind x.
			if len(rhs) != 1 || len(lhs) == 0 {
				if len(lhs) != len(rhs) {
					return true
				}
				for i, l := range lhs {
					if id, ok := l.(*ast.Ident); ok && isRoot(rhs[i]) {
						roots[id.Name] = true
					}
				}
				return true
			}
			if id, ok := lhs[0].(*ast.Ident); ok && isRoot(rhs[0]) {
				roots[id.Name] = true
			}
			return true
		})
	}
	return isRoot
}

// twdRelEnv is what an identifier naming a relative tree path can be read
// THROUGH: the names, in one package and one function body, that hold such a
// path or a []string of them.
//
// WHY THIS EXISTS, AND WHAT IT DELIBERATELY IS NOT. twdRelTreePath below
// reads a literal and nothing else, and twdTestFiles' head comment says why:
// following a root into another function is interprocedural constant
// propagation, a second thing to be wrong about. That refusal stands and
// nothing HERE crosses a function boundary: a walk rooted at a PARAMETER is
// invisible to this rule, whatever its callers pass.
//
// The class is not, since ranger-base-pyl22 — but by a separate rule and not
// by a wider env. twdParamHops resolves one argument at one call site, so the
// hop is written where it can be read as one step rather than hidden inside
// this map, where it would have to be a value flowing across a boundary.
//
// What it buys is the three spellings that never leave the body. MEASURED
// 2026-10-31 (ranger-base-8wxbn): of the 39 walker calls in
// internal/treepins/*_test.go, 23 rooted at a literal the old rule read and
// 16 did not —
//
//	dir := filepath.Join("docs", "adr"); os.ReadDir(dir)   a local
//	const armPkgDir = "internal/posse"; os.ReadDir(...)    a package const
//	for _, root := range []string{"cmd", "internal"}       a range variable
//	for _, d := range arRoots                              over a literal list
//
// — eight of those sixteen, carrying eight tree-wide pins that had no door.
// The rest are a parameter (read one step later, by twdParamHops rather than
// by this map — ranger-base-pyl22), a root parsed out of the Makefile
// (gofmtdoor_qa_test.go, whose test also walks "." and is derived and doored
// on that), the repo-root helper below, and two fixture roots that are
// correctly unseen.
type twdRelEnv struct {
	// val: an identifier -> every relative tree path it can hold.
	val map[string][]string
	// list: an identifier -> the element paths of the []string it holds.
	// Separate from val on purpose: `for _, c := range "docs"` ranges over
	// a string's RUNES, so a range variable may only be bound from
	// something this rule knows is a slice of paths.
	list map[string][]string
}

func twdNewRelEnv() *twdRelEnv {
	return &twdRelEnv{val: map[string][]string{}, list: map[string][]string{}}
}

func (e *twdRelEnv) clone() *twdRelEnv {
	out := twdNewRelEnv()
	for k, v := range e.val {
		out.val[k] = v
	}
	for k, v := range e.list {
		out.list[k] = v
	}
	return out
}

// twdRelTreePaths returns every relative tree path an expression can
// evaluate to, reading identifiers through env. A literal resolves exactly as
// twdRelTreePath resolves it; anything env cannot name resolves to nothing,
// which is what keeps a parameter, a t.TempDir() and a `..` climb out.
func twdRelTreePaths(e ast.Expr, env *twdRelEnv) []string {
	switch v := e.(type) {
	case *ast.Ident:
		return env.val[v.Name]
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return nil
		}
		switch sel.Sel.Name {
		case "Clean", "ToSlash", "FromSlash":
			if len(v.Args) == 1 {
				return twdRelTreePaths(v.Args[0], env)
			}
		case "Join":
			if len(v.Args) == 0 {
				return nil
			}
			firsts := twdRelTreePaths(v.Args[0], env)
			if len(firsts) == 0 {
				return nil
			}
			// The trailing elements are read exactly as twdRelTreePath reads
			// them — a string literal, taken as written — plus an identifier
			// env knows holds exactly one path. A variable it does not know
			// is the whole of this rule's precision: the root is then no
			// longer a tree path it can claim to know, which is what keeps
			// filepath.Join(root, d) out.
			var tail []string
			for _, a := range v.Args[1:] {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					str, err := strconv.Unquote(lit.Value)
					if err != nil {
						return nil
					}
					tail = append(tail, filepath.ToSlash(str))
					continue
				}
				id, ok := a.(*ast.Ident)
				if !ok || len(env.val[id.Name]) != 1 {
					return nil
				}
				tail = append(tail, env.val[id.Name][0])
			}
			var out []string
			for _, f := range firsts {
				out = append(out, strings.Join(append([]string{f}, tail...), "/"))
			}
			return out
		}
		return nil
	}
	if rel, ok := twdRelTreePath(e); ok {
		return []string{rel}
	}
	return nil
}

// twdRelTreeList returns the element paths of a []string of relative tree
// paths: a `[]string{...}` composite literal, or an identifier env knows
// holds one.
//
// One unreadable element makes the whole list unknown, rather than the
// readable ones standing for it — a range over that list walks the element
// the rule could not read too, so claiming the list would be claiming a root
// this file does not know. A variable, a scratch directory, an absolute path
// and a helper's return are all that element.
//
// EXCEPT A CLIMB, which is skipped rather than voiding the list
// (ranger-base-g31e0, ranger-base-tw3t7's FINDING 2). The refusal above is
// about not CLAIMING a root, and a `../` element is the one unreadable
// element whose own reading is never narrower than the list's: in
// internal/posse it is how a test reaches the tree, so it makes the reading
// MORE tree-wide, and in internal/treepins it leaves the tree and this rule
// claims nothing of it either way. What voided the list was the reason to be
// careful, not the climb — and voiding it threw away the readable `"."`
// beside it, which is the one-package census ranger-base-m94g2 decided is in
// the class on its own. `for _, d := range []string{".", "../../cmd/posse"}`
// was derived by NEITHER half of the register until this bead, precisely
// because it did both: the fence could not see the climb and the seed
// discarded the `"."`.
//
// A list of nothing BUT climbs still returns nothing, so visibility_test.go's
// `[]string{"..", "../../cmd"}` is the fence's and not this rule's — which is
// what keeps the two findings two.
func twdRelTreeList(e ast.Expr, env *twdRelEnv) []string {
	switch v := e.(type) {
	case *ast.Ident:
		return env.list[v.Name]
	case *ast.CompositeLit:
		at, ok := v.Type.(*ast.ArrayType)
		if !ok || at.Len != nil {
			return nil
		}
		if id, ok := at.Elt.(*ast.Ident); !ok || id.Name != "string" {
			return nil
		}
		var out []string
		for _, el := range v.Elts {
			paths := twdRelTreePaths(el, env)
			if len(paths) == 0 {
				if twdClimbRoot(el) {
					continue
				}
				return nil
			}
			out = append(out, paths...)
		}
		return out
	}
	return nil
}

// twdBindRel binds one identifier to whatever relative tree paths an
// expression spells — as a value, and, if it spells a []string of them, as a
// list.
func twdBindRel(env *twdRelEnv, name string, rhs ast.Expr) {
	if name == "_" {
		return
	}
	if paths := twdRelTreePaths(rhs, env); len(paths) > 0 {
		env.val[name] = paths
	}
	if elems := twdRelTreeList(rhs, env); len(elems) > 0 {
		env.list[name] = elems
	}
}

// twdPkgRelEnv reads one package's FILE-SCOPE consts and vars for relative
// tree paths: `const armPkgDir = "internal/posse"`, `var arRoots =
// []string{"internal", "cmd"}`. Both spellings root a walk in this package
// today, and neither is a function boundary — the declaration and its value
// are in a file the rule is already parsing. It is built over every file at
// once because a const in one file roots a walk in another, which armPkgDir
// does.
func twdPkgRelEnv(files []*ast.File) *twdRelEnv {
	env := twdNewRelEnv()
	// Two passes, so a declaration that reads one written below it binds.
	for i := 0; i < 2; i++ {
		for _, file := range files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Names) != len(vs.Values) {
						continue
					}
					for j, name := range vs.Names {
						twdBindRel(env, name.Name, vs.Values[j])
					}
				}
			}
		}
	}
	return env
}

// twdBodyRelEnv extends the package-scope env with ONE function body's own
// locals and range variables. Two passes, for the same reason twdRootExprs
// makes two: `dir := ...` before the walk and after it both bind.
//
// It stays inside the body. A parameter is never bound — that is the one
// spelling this rule refuses to read, so arParse(t, root, dirs) enumerates
// nothing as far as THIS map is concerned, whatever its callers pass. What
// puts its callers in the class is twdParamHops, which reads the call site's
// argument rather than the helper's body (ranger-base-pyl22). The two are
// kept apart on purpose: one hop at a named place is reviewable, and a
// parameter quietly bound here would be a value crossing a boundary with
// nothing to say so.
func twdBodyRelEnv(body *ast.BlockStmt, pkg *twdRelEnv) *twdRelEnv {
	env := pkg.clone()
	for i := 0; i < 2; i++ {
		ast.Inspect(body, func(n ast.Node) bool {
			switch st := n.(type) {
			case *ast.AssignStmt:
				// `x := expr` and `x, err := expr` both bind x, the same
				// shape twdRootExprs reads.
				if len(st.Rhs) == 1 && len(st.Lhs) >= 1 {
					if id, ok := st.Lhs[0].(*ast.Ident); ok {
						twdBindRel(env, id.Name, st.Rhs[0])
					}
					return true
				}
				if len(st.Lhs) != len(st.Rhs) {
					return true
				}
				for j, l := range st.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						twdBindRel(env, id.Name, st.Rhs[j])
					}
				}
			case *ast.ValueSpec:
				if len(st.Names) != len(st.Values) {
					return true
				}
				for j, name := range st.Names {
					twdBindRel(env, name.Name, st.Values[j])
				}
			case *ast.RangeStmt:
				// The VALUE of a range over a []string of tree paths — not
				// the Key, which is an index.
				id, ok := st.Value.(*ast.Ident)
				if !ok {
					return true
				}
				if elems := twdRelTreeList(st.X, env); len(elems) > 0 {
					env.val[id.Name] = elems
				}
			}
			return true
		})
	}
	return env
}

// twdParamHops is the walker table the PARAMETER HOP resolves against: the
// (function, parameter, walker) triples that root a walk at one of the
// function's own parameters, read by rootparam_qa_test.go's rpScan and not
// by a second copy of that loop.
//
// WHY THIS EXISTS, AND WHY IT IS NOT THE REFUSAL ABOVE (ranger-base-pyl22).
// twdRelEnv's head comment refuses to follow a root into another function,
// and twdTestFiles' refuses interprocedural constant propagation. Both
// refusals stand and this is neither of them: it resolves ONE argument at
// ONE call site, with twdRelTreePaths, and propagates no value anywhere. A
// chain — a helper handing its own parameter on to a param-rooted walker —
// is still unread, by both this and rpScan, and is said out loud as the
// residual in the head comment.
//
// WHAT IT BUYS, and why the alternative was worse. Five tree-wide pins sat
// behind two param-rooted helpers — arParse (absencerules_qa_test.go) and
// cgScan (credentialgate_qa_test.go), both `filepath.Join(root, d)` over a
// package-level list of literals — and both are genuinely polymorphic: eight
// more tests hand arParse a fixture root, so the parameter cannot be
// replaced with a literal the way scanRealBdCallSites' was
// (ranger-base-0ckgm). The alternative was a third hand register beside
// twdIndirect and twdDoorHolders, and neither of their reasons covers these:
// twdIndirect is for an enumeration no rule over GO source can derive, and
// rpScan derives this one today, one file over, under a fixture arm. A
// hand-written row for a pin a shipped rule already finds is the parking
// spot this file keeps warning about — a second list to keep in sync by
// hand, which is how the spelling got here in the first place.
//
// It is built for BOTH packages, because twdParseDir is one function; only
// the internal/treepins rule reads relEnum. A relative literal in
// internal/posse is that package's own directory and not a tree path at all
// (rootparam_qa_test.go's head comment says why), so the two entries it
// produces there — assertNoHintVocabulary(t, ".") and scanReExecSites(".")
// — are carried and unused, and the question of whether a census of one
// package's own sources is in the class stays where ranger-base-0ckgm filed
// it.
func twdParamHops(fset *token.FileSet, byPath map[string]*ast.File) map[string][]rpRootWalker {
	_, byName := rpParamRootedWalkers(fset, byPath)
	return byName
}

// twdHopEntry renders one resolved hop as relEnum evidence. Spelled once, so
// that the pin which holds the derivation and rpScan together compares the
// same bytes the derivation wrote.
func twdHopEntry(w rpRootWalker, rel string) string {
	return w.walk + "(" + rel + ")@" + w.fn
}

// twdTestFiles lists one package's test files.
//
// THE GLOBS ARE SPELLED OUT, one literal per package, rather than composed
// from `pkg` — deliberately, and this file is the evidence. Composing the
// root from the parameter made the rule blind to its own parser:
// twdRelTreePath cannot read a variable, so every test that reaches
// twdParseDir stopped being derived as tree-wide, and arm 2 immediately red
// with two doors it could no longer justify (MEASURED 2026-10-04, on this
// file, by that red — the register caught the refactor that broke it, which
// is the only reason this comment exists). A tree reading in this package is
// written in a spelling the rule can see, the same way internal/posse's
// tests are held to ONE repo-root helper. The alternative — teaching
// twdRelTreePath to follow constants and parameters — is interprocedural
// constant propagation, a second thing to be wrong about, to buy back a
// spelling nobody needs.
//
// READ PRECISELY, because this paragraph was read too wide once
// (ranger-base-pyl22). What it refuses is a VALUE FOLLOWED across boundaries:
// twdRelTreePath staying literal-only, and this glob staying spelled. It does
// not refuse resolving one argument at one call site, which is what
// twdParamHops does for a walk root that arrives as a parameter, and what
// rootparam_qa_test.go's rule had already been doing for two months when the
// refusal was quoted against it. Five tree-wide pins sat undoored in that
// gap.
func twdTestFiles(t *testing.T, pkg string) []string {
	t.Helper()
	var paths []string
	var err error
	switch pkg {
	case twdPossePkg:
		paths, err = filepath.Glob(filepath.Join("internal", "posse", "*_test.go"))
	case twdTreepinsPkg:
		paths, err = filepath.Glob(filepath.Join("internal", "treepins", "*_test.go"))
	default:
		t.Fatalf("no test-file glob for package %q — the class cannot be derived for a package this file does not name", pkg)
	}
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test files found under %s: %v", pkg, err)
	}
	return paths
}

// twdParse reads every function declared in internal/posse/*_test.go.
func twdParse(t *testing.T) (map[string]*twdFunc, int) {
	t.Helper()
	return twdParseDir(t, twdPossePkg)
}

// twdParseTreepins reads every function declared in
// internal/treepins/*_test.go — this file among them, which is the point:
// the register's own arms are in the class they derive.
func twdParseTreepins(t *testing.T) (map[string]*twdFunc, int) {
	t.Helper()
	return twdParseDir(t, twdTreepinsPkg)
}

// twdParseDir reads every function declared in one package's *_test.go.
func twdParseDir(t *testing.T, pkg string) (map[string]*twdFunc, int) {
	t.Helper()
	paths := twdTestFiles(t, pkg)
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	byPath := map[string]*ast.File{}
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
		byPath[path] = file
	}
	// Every env over EVERY file before any body is read: a const declared in
	// one file roots a walk in another (armPkgDir does), and the param-rooted
	// helper and its call site are routinely in different files (cgScan is
	// called from its own, arParse from four tests in its own). So none of
	// this can be built one file at a time.
	env := twdNewPkgEnv(fset, byPath)
	out := map[string]*twdFunc{}
	tests := 0
	for i, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			f := twdAnalyze(fn, fmt.Sprintf("%s:%d", paths[i], fset.Position(fn.Pos()).Line), env)
			if f.test {
				tests++
			}
			out[f.name] = f
		}
	}
	return out, tests
}

// twdAnalyze reads ONE function declaration: who it calls, and every way its
// body reaches outside its own package. Every rule in this file is in here,
// so the fixture-driven mutation check below drives the same code the tree is
// derived with — a second copy of the rule, written to be easy to test, is a
// copy that can agree with the fixture and disagree with the tree.
func twdAnalyze(fn *ast.FuncDecl, where string, pkg *twdPkgEnv) *twdFunc {
	f := &twdFunc{
		name:  fn.Name.Name,
		where: where,
		test:  strings.HasPrefix(fn.Name.Name, "Test"),
		calls: map[string]bool{},
	}
	env := twdBodyRelEnv(fn.Body, pkg.rel)
	climb := pkg.climb.extend(fn.Body)
	var getwd, dirCall, goMod bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				switch filepath.ToSlash(v) {
				case "../..", "../../":
					f.ascent = true
				case "go.mod":
					goMod = true
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if twdJoinsTwoDotDots(call.Args) {
			f.ascent = true
		}
		if twdAsksGitForTheRoot(call) {
			f.shellRoot = true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			f.calls[fun.Name] = true
			if fun.Name == twdRootHelper {
				f.root = true
			}
			if fun.Name == twdTreepinsRootHelper {
				f.treeRoot = true
			}
			// The parameter hop: this call hands a helper that walks one of
			// its own parameters a relative tree path, at that parameter's
			// own position. The reading is the tree, so the enumeration is
			// this body's (ranger-base-pyl22, twdParamHops).
			for _, w := range pkg.hops[fun.Name] {
				if len(call.Args) <= w.idx {
					continue
				}
				for _, rel := range twdRelTreePaths(call.Args[w.idx], env) {
					f.relEnum = append(f.relEnum, twdHopEntry(w, rel))
				}
			}
			// And the FENCE's own hop: this call hands a CLIMB out of the
			// package to a function that reaches a walker at that
			// parameter, however many calls further on. The reading is the
			// tree and no root is computed, which is the whole defect
			// (ranger-base-8yv7z, twdClimbHops).
			for _, idx := range pkg.climbHops[fun.Name] {
				if len(call.Args) > idx && climb.climbs(call.Args[idx]) {
					f.climbEnum = true
				}
			}
		case *ast.SelectorExpr:
			if twdWalkers[fun.Sel.Name] {
				f.walk = true
				if len(call.Args) > 0 {
					for _, rel := range twdRelTreePaths(call.Args[0], env) {
						f.relEnum = append(f.relEnum, fun.Sel.Name+"("+rel+")")
					}
					if climb.climbs(call.Args[0]) {
						f.climbEnum = true
					}
				}
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "os" && fun.Sel.Name == "Getwd" {
				getwd = true
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "filepath" && fun.Sel.Name == "Dir" {
				dirCall = true
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "runtime" && fun.Sel.Name == "Caller" {
				f.caller = true
			}
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "exec" && (fun.Sel.Name == "Command" || fun.Sel.Name == "CommandContext") {
				for _, a := range call.Args {
					if prog, ok := twdTrackedProgram(a); ok {
						f.execTree = append(f.execTree, prog)
					}
				}
			}
		}
		return true
	})
	f.rootedWalk = f.root && f.walk
	f.getwdClimb = getwd && dirCall && goMod
	if !f.root {
		isRoot := twdRootExprs(fn.Body)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !twdWalkers[sel.Sel.Name] {
				return true
			}
			if isRoot(call.Args[0]) {
				f.handRolledTreeWalk = true
			}
			return true
		})
	}
	return f
}

// twdTreeWideTests returns the tests whose subject is the TREE. Three rules,
// unioned, and the union is deliberate — ranger-base-sx2dq is what a single
// rule cost:
//
//   - DIRECT: the test's own body calls qibRepoRoot, so it reads outside its
//     package. This was the whole rule, and it is kept because it is what
//     the five original members are doored on.
//   - WALK-REACHING: the test reaches, through this package's own test
//     helpers, a function that computes the repo root and WALKS from it.
//     TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover names no
//     root of its own — it calls coxnGoSources, which walks every shipped
//     .go file — and any file added anywhere can red it. The direct rule
//     never saw it.
//   - ENUMERATING (ranger-base-na4rm): the test reaches a body that
//     enumerates through a relative tree path — relEnum, which is the
//     internal/treepins rule below with a second caller and no new code.
//     This is the one rule here with no root in it, and it had to be,
//     because the members it finds compute none: a one-package census reads
//     `os.ReadDir(".")` and climbs nowhere. Both root rules were blind to
//     fifteen pins by construction, and the thing that makes a census a
//     member is the same thing that makes any of these one — its file set is
//     DISCOVERED, so a bead that adds a file to the package reds it without
//     having read it. ranger-base-m94g2 decided that; the floor on this half
//     alone is in arm 2 (twdPosseEnumOnlyTests), because three rules fail
//     three ways and a union floor hides any one of them.
//
// The reach is only propagated from a ROOTED WALK, not from any function
// that touches the root: qspSeedScript looks up one path at the root and its
// callers red only when that script changes, which is not this class.
//
// What keeps a third spelling from slipping past both rules is not a rule
// here at all — it is TestQAOneRepoRootHelperInTheTestPackage below, which
// holds internal/posse to ONE repo-root helper. That is the pin
// ranger-base-sx2dq was filed for: the tree carried a byte-identical twin of
// qibRepoRoot (qspRepoRoot) and a hand-rolled `..`/`..` ascent, and pins
// spelled either way were outside the class and got no door, silently.
func twdTreeWideTests(t *testing.T) (names []string, funcs int) {
	t.Helper()
	all, funcs := twdParse(t)

	// Seed the reach with the rooted walkers, then close it over callers.
	reaches := twdReach(all, func(f *twdFunc) bool { return f.rootedWalk })
	// And the third seed, which is the SECOND package's rule with a second
	// caller and no new code (ranger-base-na4rm): a body that enumerates
	// through a relative tree path. twdAnalyze has computed relEnum for both
	// packages since ranger-base-g6sb1 and this half carried it unused.
	enum := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })

	for name, f := range all {
		if f.test && (f.root || reaches[name] || enum[name]) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, funcs
}

// twdPosseEnumOnlyTests is the third seed's members alone — the tests in
// internal/posse that are in the class ONLY because they enumerate, naming no
// root and reaching no rooted walk.
//
// It exists for the reason twdWalkReachingTests and twdTreepinsRootOnlyTests
// do: the rules fail apart. A relEnum seed that silently stopped matching
// would leave arm 2's union floor green on the two root rules alone, and
// fifteen pins would go quiet with no numeral moving anywhere a seat reads.
func twdPosseEnumOnlyTests(t *testing.T) []string {
	t.Helper()
	all, _ := twdParse(t)
	reaches := twdReach(all, func(f *twdFunc) bool { return f.rootedWalk })
	enum := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	var out []string
	for name, f := range all {
		if f.test && enum[name] && !f.root && !reaches[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// The internal/treepins half of the class (ranger-base-g6sb1).
//
// WHY A SECOND RULE AT ALL. Everything above keys, one way or another, on a
// test COMPUTING the repo root: it calls qibRepoRoot, or it reaches a helper
// that walks from a root, or it asks git for one. That key exists because
// `go test` runs internal/posse's binary with the PACKAGE directory as its
// working directory, so reading the tree costs an explicit climb. It does not
// exist here. internal/treepins' TestMain (configdirfence_test.go) chdirs the
// whole binary to the repo root before any test runs, so a pin in this
// package reads the tree through a plain relative path — "scripts/notes-
// index.py", "docs/adr" — and never names a root helper at all. Every rule
// above is blind to this package by construction, and this package is
// 589.965s whole: the same wall, one directory over.
//
// TestNotesFragmentIndexIsCurrent is what got out. Adding one
// docs/notes.d/<bead>.md fragment landed a commit that `make fmt-check`,
// `make tree-check` (all eight doors as it then stood) and a full
// `go test -tags posse_arm2 ./internal/posse` all called clean; the 589.965s
// arm-1 run said `notesindex_qa_test.go:13: docs/notes.d/README.md: stale
// index`. The pin was right, its remediation line was exact, and nothing a
// seat could type in under ten minutes would ever have asked it.
//
// THE KEY, and why it is not the obvious one. "A test in a package whose
// TestMain chdirs to the repo root, that reads a tree path" is the honest
// description of the blind spot, but it is not a usable key: MEASURED
// 2026-10-04, 291 of internal/treepins' 362 tests name a path the tree holds.
// Four fifths of a package is not a class. What actually separates a pin that
// an unrelated bead can red from one it cannot is whether the reading
// ENUMERATES — whether the set of files read is spelled in the test, or
// discovered. A test that reads "Makefile" reds when somebody edits the
// Makefile, which is the bead that edited it. A test that globs
// "docs/notes.d/*.md" reds when ANY bead writes its fragment there. So the
// key is a WalkDir/Walk/ReadDir/Glob rooted at a relative tree path. It found
// twenty tests in the package as it stood before this bead — and that number
// is frozen prose, like every other count in this comment: the live one is
// in the `make tree-check` sentence above, which arm 4 derives.
//
// THAT IS WIDER THAN THE POSSE RULE, deliberately. Over there a walk rooted
// at a SUBDIRECTORY is excluded — detectionRig ReadDirs
// `<root>/etc/herdr/agent-detection` and reds only when that one directory
// changes, and nine tests do that shape. The exclusion is defensible there
// and would be the bug here: docs/notes.d is a subdirectory, and it is the
// subdirectory every bead on this box writes into. Drawing the line at the
// repo root would have reproduced exactly the blind spot this bead was filed
// for — it finds nine of those twenty, and not the one that escaped.
//
// AND IT STILL DOES NOT FIND THE ONE THAT ESCAPED. TestNotesFragmentIndexIsCurrent
// enumerates nothing in Go: it runs `python3 scripts/notes-index.py --check`
// and the glob is a line of Python. No rule over Go source can see it, and a
// shell/Python source scanner would be a second implementation of "does this
// program read a directory" — the narrower-than-the-pin door this file's head
// comment refuses. So that half is REGISTERED by hand (twdIndirect), held
// honest by a pin that re-reads the program and the directory each entry
// claims, and fenced by a dispositioned census of every tracked program a
// test in this package hands to exec.Command (twdExecDispositions).

// twdRelTreePath returns the relative tree path an expression spells, if it
// spells one: a string literal, or a filepath.Join of string literals whose
// first element is one. An absolute path, a `..` climb, or anything carrying
// a variable is NOT one — a Join with a t.TempDir() in it is a scratch
// directory, and that distinction is most of this rule's precision.
func twdRelTreePath(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		str, err := strconv.Unquote(v.Value)
		if err != nil || str == "" {
			return "", false
		}
		slash := filepath.ToSlash(str)
		if filepath.IsAbs(str) || slash == ".." || strings.HasPrefix(slash, "../") {
			return "", false
		}
		return slash, true
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
			return "", false
		}
		switch sel.Sel.Name {
		case "Clean", "ToSlash", "FromSlash":
			if len(v.Args) == 1 {
				return twdRelTreePath(v.Args[0])
			}
		case "Join":
			if len(v.Args) == 0 {
				return "", false
			}
			first, ok := twdRelTreePath(v.Args[0])
			if !ok {
				return "", false
			}
			parts := []string{first}
			for _, a := range v.Args[1:] {
				lit, ok := a.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					// A variable element: the root is no longer a tree path
					// this rule can claim to know.
					return "", false
				}
				str, err := strconv.Unquote(lit.Value)
				if err != nil {
					return "", false
				}
				parts = append(parts, filepath.ToSlash(str))
			}
			return strings.Join(parts, "/"), true
		}
	}
	return "", false
}

// twdTrackedProgram reports the relative path of a tracked REGULAR file an
// expression names — one of this repo's own programs, handed to exec.Command.
//
// Regular file, not a directory, on purpose: `go run ./cmd/testparallel` and
// `go test ./internal/treepins` name package paths, not readings, and the Go
// in them is source the rules above read directly. The residual is said out
// loud in twdExecDispositions.
func twdTrackedProgram(e ast.Expr) (string, bool) {
	rel, ok := twdRelTreePath(e)
	if !ok {
		return "", false
	}
	rel = strings.TrimPrefix(rel, "./")
	if !strings.Contains(rel, "/") {
		// A bare word: `sh`, `python3`, `make`. Even if the tree happens to
		// hold a file of that name, exec resolves it on PATH.
		return "", false
	}
	info, err := os.Lstat(filepath.FromSlash(rel))
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return rel, true
}

// twdReach closes a seed set over the call graph: every function that calls a
// seeded function, transitively. Both halves of the class propagate the same
// way, so there is one closure and three seeds, not three closures.
func twdReach(all map[string]*twdFunc, seeded func(*twdFunc) bool) map[string]bool {
	reaches := map[string]bool{}
	for name, f := range all {
		if seeded(f) {
			reaches[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, f := range all {
			if reaches[name] {
				continue
			}
			for callee := range f.calls {
				if reaches[callee] {
					reaches[name] = true
					changed = true
					break
				}
			}
		}
	}
	return reaches
}

// twdIndirect is the half of the internal/treepins class that no rule over Go
// source can derive: a pin whose enumeration happens inside one of this
// repo's own programs. Hand-registered, and held honest by
// TestQAEveryTrackedProgramATreePinRunsIsDispositioned below, which re-reads
// both the test and the program rather than trusting the row.
//
// One entry. It is the entry this bead exists for.
var twdIndirect = []struct {
	test string // the pin, in internal/treepins
	prog string // the tracked program it runs
	dir  string // the tree directory that program enumerates
	why  string // what the Go body shows, and what it does not
}{
	{
		test: "TestNotesFragmentIndexIsCurrent",
		prog: "scripts/notes-index.py",
		dir:  "docs/notes.d",
		why: "the pin's whole body is `python3 scripts/notes-index.py --check`; " +
			"the enumeration is that script's own `directory.glob(\"*.md\")` over " +
			"docs/notes.d, so every bead that writes a fragment there is in this " +
			"pin's reach and no rule over Go source can see it",
	},
}

// twdExecDispositions is the fence around the register above: every tracked
// program a test in internal/treepins hands to exec.Command must be here or
// in twdIndirect. The value says what the program reads INSTEAD of a tracked
// directory — checked by hand when the row was written, and re-asked of every
// new row by the pin that reds on an undispositioned program.
//
// RESIDUAL, said out loud. This matches a relative literal naming a tracked
// regular file in an exec.Command argv — what every call site below writes,
// and what the next one would. A program path assembled from a variable, or
// carried into exec.Command inside a []string, is not matched: the second
// arm of notesindex_qa_test.go builds its argv that way, which is correct
// here (it points the script at a t.TempDir()) and would not be correct for
// a future pin spelled the same way. Nothing in Go can bound what a
// subprocess reads. What bounds this is that the census defaults to
// INCLUSION — an enumerating program makes its callers members — and that
// the list of programs is short enough to have been read.
var twdExecDispositions = map[string]string{
	"scripts/bd-argv-gate.sh":        "parses one command line out of its argv and stdin; it opens no directory at all",
	"scripts/gotest.sh":              "`find`s only $CACHE and its own scratch tree, never a tracked path",
	"scripts/macos-install-probe.sh": "probes a scratch install root under $D; the only tracked file it reads is the formula it is handed",
	"scripts/path-warning.sh":        "reads the PATH string it is given",
	"scripts/tap-formula.sh":         "renders a formula from the --checksums file named on its command line",
	"scripts/test-linux.sh":          "assembles a docker invocation; it lists nothing",
	"scripts/test-times.sh":          "runs `go test` and parses its output; the one `ls` in it is inside an advice string it prints",
}

// twdDoorHolders are tests a door variable may name although no class rule
// derives them: the arms that hold the register itself open. They read the
// Makefile and run `make -n`, which is not a reading of the tree, so nothing
// in this file can find them — and they live in the same 589.965s package as
// the pins they guard, so left out of a door the register's own floor is as
// unreachable as the pins were.
//
// This is the ONE way into a door variable that does not go through the
// derivation, which is why it is a list with reasons rather than a flag: it
// is exactly the shape of parking spot this file keeps warning about. Arm 2
// holds each row to being a real test in the package its door runs, and two
// rows is what it should stay near. A pin put here to dodge the two-way
// check would be a pin whose membership nothing derived and whose door
// nothing justified.
var twdDoorHolders = map[string]string{
	"TestQAEveryTreepinsDoorFilterNamesItsPins":            "asks what make's expansion of each internal/treepins door filter selects; it reads the Makefile and runs `go test -list`",
	"TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles": "arm 4 — holds this file's head comment and the treepins README's door table to the Makefile's own pin and door counts; it reads the Makefile, this file and that README",
	// ranger-base-hrf47. The third exemption, and the first that is not one of
	// this register's own arms: it censuses every verify-* target in the
	// Makefile against the roster and EXCLUDED table of scripts/verify-box.sh,
	// reading those two files BY NAME. So the enumeration rule cannot see it
	// (`os.ReadFile("Makefile")` is the rule's own negative case — see
	// TestQATheTreepinsEnumerationRuleMatchesWhatItClaims), while the thing
	// that makes it tree-wide in the sense the class means is unaffected: an
	// unrelated bead adding a verify-* target reds it, and until this row
	// existed nothing but an unfiltered 589.965s package run would say so. Its
	// sibling in the same file IS derived, because that one globs scripts/ —
	// so scripts-check went green over this one for as long as both existed
	// (the escape ranger-base-hrf47 was filed for).
	"TestQABoxCheckCensusCoversEveryVerifyTarget": "censuses the Makefile's verify-* targets against the roster and EXCLUDED table of scripts/verify-box.sh; it reads those two files by name and enumerates nothing",
}

// twdTreepinsTreeWideTests returns the tests in internal/treepins whose
// subject is the TREE: the derived ones, plus the registered indirect ones.
//
// TWO RULES HERE TOO, and the second one arrived the same way the posse
// half's second did (ranger-base-8wxbn). ENUMERATION is the first: a
// walker rooted at a relative tree path, now read through twdRelEnv so a
// local, a package const and a range variable over a literal list count as
// spelling one — and, since ranger-base-pyl22, through twdParamHops, so a
// tree path HANDED to a helper that walks that very parameter counts too.
// (Both feed the same relEnum and the same reach, because both are the one
// question "does this body enumerate the tree": a second seed would be a
// second closure saying the same thing.) The second rule is the ROOT HELPER:
// a test that reaches repoRoot() AND reaches a walker is reading the tree
// through an absolute root, which the first cannot see at all.
//
// Two reaches INTERSECTED rather than one flag, because here the root and
// the walk are in different functions: the test takes the root, and the
// helper it hands it to does the walking (lcsSources, wrsRenderSites). That
// is also the residual, said out loud — a test that took the root for one
// purpose and separately called something that walks a FIXTURE would be a
// false member. MEASURED 2026-10-31: no test in the package is in that
// shape, the two that call repoRoot() without reaching a walker
// (TestLaunchCheckSourcesAllExist, TestWallRenderSourcesAllExist) stay out,
// and what bounds the rule is the fence — one repo-root helper, held by
// TestQAOneRepoRootHelperInTheTestPackage.
func twdTreepinsTreeWideTests(t *testing.T) (names []string, derived []string, funcs int) {
	t.Helper()
	all, funcs := twdParseTreepins(t)
	reaches := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	rootReach := twdReach(all, func(f *twdFunc) bool { return f.treeRoot })
	walkReach := twdReach(all, func(f *twdFunc) bool { return f.walk })
	for name, f := range all {
		if f.test && (reaches[name] || (rootReach[name] && walkReach[name])) {
			derived = append(derived, name)
		}
	}
	sort.Strings(derived)
	names = append(names, derived...)
	for _, row := range twdIndirect {
		if !slices.Contains(names, row.test) {
			names = append(names, row.test)
		}
	}
	sort.Strings(names)
	return names, derived, funcs
}

// twdClassOf returns one package's tree-wide tests, for the two-way check
// below. Two packages, two rules, because the two packages reach the tree in
// ways that have no spelling in common.
func twdClassOf(t *testing.T, pkg string) []string {
	t.Helper()
	switch pkg {
	case twdPossePkg:
		names, _ := twdTreeWideTests(t)
		return names
	case twdTreepinsPkg:
		names, _, _ := twdTreepinsTreeWideTests(t)
		return names
	}
	t.Fatalf("no class rule for package %q — a door variable was given a package this file cannot derive, so its membership is checked against nothing", pkg)
	return nil
}

// The fence on the indirect register: every row still names a real pin
// running a real program over the directory it claims, and every tracked
// program a test in this package execs is dispositioned.
//
// The directory check is not decoration. If scripts/notes-index.py is pointed
// at another directory tomorrow, the row above becomes a claim about a
// reading that no longer happens — and the door would keep passing while the
// pin's subject moved out from under it.
func TestQAEveryTrackedProgramATreePinRunsIsDispositioned(t *testing.T) {
	t.Parallel()
	all, funcs := twdParseTreepins(t)
	if funcs < 200 {
		t.Fatalf("only %d test functions parsed under %s — the walk this pin reads is finding nothing", funcs, twdTreepinsPkg)
	}

	registered := map[string]bool{}
	for _, row := range twdIndirect {
		registered[row.prog] = true
		f, ok := all[row.test]
		if !ok || !f.test {
			t.Errorf("twdIndirect registers %s, which is not a test in %s — the indirect half of the class names a pin that is gone, and arm 2 is holding a door open for nothing", row.test, twdTreepinsPkg)
			continue
		}
		if !slices.Contains(f.execTree, row.prog) {
			t.Errorf("twdIndirect says %s runs %s, and its body does not (%s runs %v) — the row is a claim about a reading that no longer happens, so the pin's subject has moved out from under its door. The row's reason was: %s", row.test, row.prog, f.where, f.execTree, row.why)
			continue
		}
		body, err := os.ReadFile(filepath.FromSlash(row.prog))
		if err != nil {
			t.Errorf("twdIndirect registers %s and the tree does not hold it: %v", row.prog, err)
			continue
		}
		if !strings.Contains(string(body), row.dir) {
			t.Errorf("twdIndirect says %s enumerates %s, and that path does not appear in the program at all — either the program was pointed somewhere else, in which case this pin's reach and its door have silently diverged, or the row was wrong when it was written", row.prog, row.dir)
		}
	}

	var undispositioned []string
	for name, f := range all {
		if !f.test {
			continue
		}
		for _, prog := range f.execTree {
			if registered[prog] {
				continue
			}
			if _, ok := twdExecDispositions[prog]; ok {
				continue
			}
			undispositioned = append(undispositioned, f.where+" "+name+" runs "+prog)
		}
	}
	sort.Strings(undispositioned)
	for _, u := range undispositioned {
		t.Errorf("%s — a tracked program of this repo's own, run by a test in %s, and nothing here says what it reads. If it enumerates a tracked directory, its callers are tree-wide pins and belong in twdIndirect with a door; if it reads only what it is handed, say so in twdExecDispositions. Undispositioned is the one answer this census does not take, because a program's reading is outside Go and the default has to be inclusion.", u, twdTreepinsPkg)
	}

	// Two-way, so the disposition list cannot keep rows for programs nothing
	// runs any more — a dead row is a reader's reason to believe a question
	// was asked that nobody asks.
	execed := map[string]bool{}
	for _, f := range all {
		for _, prog := range f.execTree {
			execed[prog] = true
		}
	}
	var dead []string
	for prog := range twdExecDispositions {
		if !execed[prog] {
			dead = append(dead, prog)
		}
	}
	sort.Strings(dead)
	for _, d := range dead {
		t.Errorf("twdExecDispositions carries %s (%q), which no test in %s hands to exec.Command any more — drop the row rather than leave a disposition for a reading that does not happen", d, twdExecDispositions[d], twdTreepinsPkg)
	}
}

// The mutation check the derivation needs, because the thing being derived is
// a RULE and a rule that stops matching leaves a green register over a class
// it no longer finds. The floors in arm 2 catch a collapse; this catches the
// rule being quietly wrong at the edges, which is how both earlier widenings
// of the posse half were needed.
//
// Driven over source text rather than over the tree: a fixture says what the
// rule must and must not match, so a change to the rule reds here with the
// case it broke rather than reddening arm 2 with a number.
func TestQATheTreepinsEnumerationRuleMatchesWhatItClaims(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		want bool
	}{
		{"glob of a tree subdirectory", `func TestX(t *testing.T) { filepath.Glob("docs/notes.d/*.md") }`, true},
		{"walk of the repo root", `func TestX(t *testing.T) { filepath.WalkDir(".", nil) }`, true},
		{"ReadDir of a tree directory", `func TestX(t *testing.T) { os.ReadDir("docs/adr") }`, true},
		{"Join of literals", `func TestX(t *testing.T) { filepath.Glob(filepath.Join("internal", "posse", "*_test.go")) }`, true},
		{"reading one named file is not enumeration", `func TestX(t *testing.T) { os.ReadFile("Makefile") }`, false},
		{"a scratch directory is not a tree path", `func TestX(t *testing.T) { dir := t.TempDir(); os.ReadDir(dir) }`, false},
		{"a Join carrying a variable is not a tree path", `func TestX(t *testing.T) { dir := t.TempDir(); filepath.Glob(filepath.Join(dir, "*.md")) }`, false},
		{"an absolute path is not a tree path", `func TestX(t *testing.T) { os.ReadDir("/etc") }`, false},
		{"a climb out of the tree is not a tree path", `func TestX(t *testing.T) { os.ReadDir("../..") }`, false},
		{"through a helper", `func helper() { filepath.WalkDir("scripts", nil) }
func TestX(t *testing.T) { helper() }`, true},
		{"through two helpers", `func inner() { os.ReadDir("examples/agents") }
func outer() { inner() }
func TestX(t *testing.T) { outer() }`, true},
		{"a helper that enumerates nothing", `func helper() { os.ReadFile("go.mod") }
func TestX(t *testing.T) { helper() }`, false},
		// The three spellings twdRelEnv reads (ranger-base-8wxbn), and the
		// ones it must still refuse beside them.
		{"a local assigned a Join of literals", `func TestX(t *testing.T) { dir := filepath.Join("docs", "adr"); os.ReadDir(dir) }`, true},
		{"a local assigned a literal, walked before the assignment is read", `func TestX(t *testing.T) { os.ReadDir(dir); dir := "scripts" }`, true},
		{"a package const", `const pkgDir = "internal/posse"
func TestX(t *testing.T) { os.ReadDir(pkgDir) }`, true},
		{"a Join through a const element", `const adr = "adr"
func TestX(t *testing.T) { os.ReadDir(filepath.Join("docs", adr)) }`, true},
		{"a range variable over an inline literal list", `func TestX(t *testing.T) { for _, root := range []string{"cmd", "internal"} { filepath.WalkDir(root, nil) } }`, true},
		{"a range variable over a package-level literal list", `var roots = []string{"internal", "cmd"}
func TestX(t *testing.T) { for _, root := range roots { filepath.Walk(root, nil) } }`, true},
		{"a range variable over a list of Joins", `func TestX(t *testing.T) { for _, g := range []string{filepath.Join("scripts", "*.sh")} { filepath.Glob(g) } }`, true},
		// The PARAMETER HOP (ranger-base-pyl22). A walk root that arrives
		// as a parameter is read through the ONE argument at the call site —
		// which is the case directly below, and it read false until this
		// bead. What is still refused is anything that needs the value
		// followed further than that: see the chain case at the end.
		{"a walk root that arrives as a parameter, handed a tree path", `func helper(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { helper(".") }`, true},
		{"a parameter root reached through a local join, handed a tree path", `func helper(root string, dirs []string) {
	for _, d := range dirs {
		filepath.Walk(filepath.Join(root, d), nil)
	}
}
func TestX(t *testing.T) { helper(".", nil) }`, true},
		{"a parameter root handed a tree path through a local the env knows", `func helper(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { dir := filepath.Join("docs", "adr"); helper(dir) }`, true},
		{"a parameter root handed a scratch directory", `func helper(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { helper(t.TempDir()) }`, false},
		{"a parameter root handed a climb out of the tree", `func helper(root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { helper("../..") }`, false},
		{"a tree path at a position that is not the walked parameter", `func helper(name string, root string) { filepath.Walk(root, nil) }
func TestX(t *testing.T) { helper("docs", t.TempDir()) }`, false},
		{"a parameter that roots no walk", `func helper(root string) { os.ReadFile(root) }
func TestX(t *testing.T) { helper("Makefile") }`, false},
		{"a chain of two parameters is still unread", `func inner(root string) { filepath.Walk(root, nil) }
func outer(root string) { inner(root) }
func TestX(t *testing.T) { outer(".") }`, false},
		{"a range variable over a list carrying a variable", `func TestX(t *testing.T) { d := t.TempDir(); for _, root := range []string{"cmd", d} { filepath.WalkDir(root, nil) } }`, false},
		// Why val and list are two maps: ranging a STRING yields its runes,
		// so a range variable may only be bound from a slice of paths.
		{"a range variable over a string literal is not a tree path", `func TestX(t *testing.T) { for _, root := range "docs" { filepath.WalkDir(root, nil) } }`, false},
		{"a local assigned a scratch directory", `func TestX(t *testing.T) { dir := t.TempDir(); os.ReadDir(dir) }`, false},
		{"a local assigned a climb out of the tree", `func TestX(t *testing.T) { dir := filepath.Join("..", ".."); os.ReadDir(dir) }`, false},
		{"a local assigned from a helper's return", `func root() string { return "." }
func TestX(t *testing.T) { dir := root(); os.ReadDir(dir) }`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := twdFixtureEnumerates(t, c.src); got != c.want {
				t.Errorf("the enumeration rule says %v for this source, want %v — the key the internal/treepins half of the class is derived from no longer means what this file says it means:\n%s", got, c.want, c.src)
			}
		})
	}

	// And the rule is load-bearing over the real tree: with the enumerator
	// set emptied, nothing matches. A rule that would answer the same with
	// its key removed is not deriving anything.
	all, _ := twdParseTreepins(t)
	live := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	n := 0
	for name, f := range all {
		if f.test && live[name] {
			n++
		}
	}
	if n < 25 {
		t.Fatalf("the enumeration rule finds %d tree-wide tests in %s (42 on 2026-10-31 with the parameter hop, 35 without it, 23 on 2026-10-04) — the key has stopped matching and the register is deriving a class that is missing members", n, twdTreepinsPkg)
	}
	dead := twdReach(all, func(f *twdFunc) bool { return false })
	if len(dead) != 0 {
		t.Fatalf("the reach closure returns %d functions from an empty seed — it is matching something other than its key, so the count above says nothing", len(dead))
	}
}

// twdFixtureEnumerates runs the internal/treepins rule over one snippet of
// source and reports whether its TestX is a member. It goes through
// twdAnalyze — the same function twdParseDir reads the tree with — so a
// fixture that passes here cannot be passing against a second, friendlier
// copy of the rule.
func twdFixtureEnumerates(t *testing.T, src string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", "package treepins\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	env := twdNewPkgEnv(fset, map[string]*ast.File{"fixture_test.go": file})
	all := map[string]*twdFunc{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		f := twdAnalyze(fn, "fixture", env)
		all[f.name] = f
	}
	return twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })["TestX"]
}

// twdFixtureClimbs runs the climbing-enumeration rule over one snippet of
// source and reports whether its TestX trips it. Like twdFixtureEnumerates
// it goes through twdAnalyze, so a fixture cannot pass against a second,
// friendlier copy of the rule.
func twdFixtureClimbs(t *testing.T, src string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", "package posse\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	env := twdNewPkgEnv(fset, map[string]*ast.File{"fixture_test.go": file})
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != "TestX" {
			continue
		}
		return twdAnalyze(fn, "fixture", env).climbEnum
	}
	t.Fatalf("fixture declares no TestX:\n%s", src)
	return false
}

// Arm 2: every tree-wide pin has a door, and every door names a real one.
// Two-way, because both directions are the bug: a pin with no door is
// ranger-base-ik44f arriving again, and a door naming a test that no longer
// exists is a green `-run` filter that runs nothing.
func TestQAEveryTreeWidePinHasADoor(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	doored := map[string]twdDoor{}
	for _, d := range twdDoors {
		for _, name := range twdVar(t, src, d.variable) {
			if prev, dup := doored[name]; dup {
				t.Errorf("%s is named by both $(%s) and $(%s) — one pin, two doors, and the two can drift apart", name, prev.variable, d.variable)
			}
			doored[name] = d
		}
	}

	// QA_TOOL_PINS is the one membership this arm cannot check by wiring: a
	// tool door (`gofmt -l`) names no test, so a pin listed there is a CLAIM
	// that some tool reads it. Exactly one pin makes that claim today, and
	// its door is held open by gofmtdoor_qa_test.go. A second name arriving
	// here would be a pin declaring itself doored with nothing to show —
	// the cheapest way to silence the check below — so it fails until this
	// arm is taught what the new tool door is and who pins it.
	if tool := twdVar(t, src, "QA_TOOL_PINS"); len(tool) != 1 || tool[0] != "TestTreeIsGofmtClean" {
		t.Errorf("$(QA_TOOL_PINS) = %v, want exactly [TestTreeIsGofmtClean] — that variable records the pins whose door is a TOOL rather than a `-run` filter, which is a claim nothing here can verify. A new entry needs its own arm; a pin moved here to quiet this test has no door at all.", tool)
	}

	// QA_HISTORY_PINS is the second membership this arm cannot check by
	// wiring, for a different reason than QA_TOOL_PINS: its three pins read
	// `git log` in THIS repo, so arm 3 cannot plant drift for them in a
	// copied tree (a `git archive` scratch tree has no .git, and reds them
	// in every arm including the control). They are doored and run clean;
	// what is NOT proven for them is that their door can fail. So the
	// membership is named here, and a fourth pin moved into this variable to
	// dodge arm 3's drift plant fails until this arm is taught why.
	wantHistory := []string{
		"TestPublicationRootCommitOmitsExcludedPaths",
		"TestPublicationRootCommitADRsCarryProvenance",
		"TestPublicationHistoryNeverCarriesTheSeedScript",
		// The fourth, taught here rather than moved in quietly
		// (ranger-base-xndgk FINDING 5): it reads THIS repo's `git rev-list`
		// and `git show` for every version of every shipped example, which
		// is the same reason as the three above — a copied tree has no
		// history to plant drift in. It was undoored because its root came
		// from `git rev-parse --show-toplevel`, a spelling neither class
		// rule could see.
		"TestShippedExampleTableCoversEveryVersionInGitHistory",
	}
	if got := twdVar(t, src, "QA_HISTORY_PINS"); !twdSameSet(got, wantHistory) {
		t.Errorf("$(QA_HISTORY_PINS) = %v, want exactly %v — that variable records the tree-wide pins whose subject is this repo's git HISTORY, which is the one thing a copied tree does not have, so arm 3 runs them clean and plants no drift. A new entry needs its own arm; a pin moved here is a pin whose door nothing has shown can fail.", got, wantHistory)
	}

	// The same membership-by-name treatment for the two doors
	// ranger-base-xndgk FINDING 5 added, and for the same reason as
	// QA_HISTORY_PINS rather than a new one: all four pins READ THE TREE
	// THROUGH GIT — `git ls-files`, `git grep` — so in a copied tree they
	// find no checkout and skip, and arm 3 cannot plant drift for them
	// there. Their membership is therefore named here, so neither variable
	// can become the quiet place to park a pin whose door nothing has shown
	// can fail.
	//
	// These four were the finding: every one of them took its root from
	// `git rev-parse --show-toplevel`, which is stdout from a subprocess and
	// invisible to both class rules, so none of them was in any door
	// variable at all. TestQAIdentityLiteralsNeverAppearInATrackedPath
	// censuses EVERY TRACKED PATH in the repository.
	wantIdentity := []string{
		"TestQAIdentityLiteralsNeverAppearInATrackedPath",
		"TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned",
	}
	if got := twdVar(t, src, "QA_IDENTITY_PINS"); !twdSameSet(got, wantIdentity) {
		t.Errorf("$(QA_IDENTITY_PINS) = %v, want exactly %v — this box's identity literals asked of the tree twice, once of tracked PATHS and once of tracked CONTENT. Both read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantIdentity)
	}
	wantOps := []string{
		"TestQAEveryOpsHitInTrackedMarkdownIsRuled",
		"TestQAOpsShapeTableCanStillSayNo",
		// The instance path-form census and its control, taught here rather
		// than moved in quietly (ranger-base-l9ii). Same door because it is
		// the same question — ADR 0024 D1 ops residue in the public tree —
		// asked of every tracked FILE rather than of tracked markdown: this
		// deployment's constitution checkout written as a live path
		// (`~/src/…`, `$HOME/src/…`), dispositioned per file. And it belongs
		// in a named membership for the same reason the four above do: it
		// reads the tree through `git ls-files`, so in the `git archive`
		// scratch tree arm 3 works in it finds no checkout and skips, and
		// arm 3 cannot plant drift for it there. Its door is shown able to
		// fail by its own control instead, which drives the matcher on
		// planted lines.
		"TestInstancePathFormNeverAppearsInTrackedContentUndispositioned",
		"TestQAInstancePathCensusCanStillSayNo",
	}
	if got := twdVar(t, src, "QA_OPS_PINS"); !twdSameSet(got, wantOps) {
		t.Errorf("$(QA_OPS_PINS) = %v, want exactly %v — the ops-residue census over every tracked markdown file, the instance path-form census over every tracked file, and the control beside each that says it can still say no. Both censuses read the tree with git, so arm 3 cannot plant drift for them in a copied tree; a new entry needs its own arm.", got, wantOps)
	}

	// The second package's named membership, for the same reason the four
	// above are named: $(QA_NOTES_PINS) is the only door variable in the
	// register whose pin is NOT derived from source — the enumeration it
	// stands on happens inside scripts/notes-index.py, which no rule over
	// Go can read. It is carried by twdIndirect and fenced by
	// TestQAEveryTrackedProgramATreePinRunsIsDispositioned; naming it here
	// as well keeps it from becoming the quiet place to park a pin whose
	// membership nothing derived.
	wantNotes := []string{"TestNotesFragmentIndexIsCurrent"}
	if got := twdVar(t, src, "QA_NOTES_PINS"); !twdSameSet(got, wantNotes) {
		t.Errorf("$(QA_NOTES_PINS) = %v, want exactly %v — that variable holds the one pin in the class whose enumeration is a line of Python rather than a line of Go, so nothing here derives its membership. A second entry needs a row in twdIndirect and a reason; a pin moved here to quiet the check below has no door at all.", got, wantNotes)
	}

	names, funcs := twdTreeWideTests(t)
	// A pin over a derived set is satisfied by deriving nothing: say how
	// many test functions were actually parsed, and fail a walk that found
	// far fewer than internal/posse holds (1000+ on 2026-09-04).
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this arm derives the class from found nothing", funcs)
	}
	// Two floors, because the two rules fail apart. The union's floor
	// catches a derivation that collapsed; it is NOT what catches a member
	// going invisible one at a time — the old floor here was len < 5, which
	// fired only when qibRepoRoot was renamed and never when a pin was
	// simply spelled with the twin helper (ranger-base-sx2dq). That is
	// TestQAOneRepoRootHelperInTheTestPackage's job, below.
	if len(names) < 14 {
		t.Fatalf("only %d tree-wide tests found (18 on 2026-09-04) — %s has been renamed or wrapped, and this arm is now deriving a class that is missing members: %v", len(names), twdRootHelper, names)
	}
	// And the walk-reaching rule specifically: it is the half that catches a
	// pin reading the tree through a helper, and a rule that silently stops
	// matching leaves the union looking healthy on the direct half alone.
	if reach := twdWalkReachingTests(t); len(reach) < 1 {
		t.Errorf("no test reaches a rooted walk through a helper (at least TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover did on 2026-09-04) — half the class rule is matching nothing")
	}
	// And the enumeration rule, which is the THIRD seed on this half and the
	// one with no root in it at all (ranger-base-na4rm): fifteen pins are in
	// the class only by enumerating their own package directory, so a seed
	// that stopped matching takes a seventh of the class out while the two
	// root rules keep the union floor green. Ten rather than fifteen,
	// because a bead that deletes a census should not have to come here —
	// what this catches is the SEED going quiet, not the population moving.
	if enumOnly := twdPosseEnumOnlyTests(t); len(enumOnly) < 10 {
		t.Errorf("only %d tests in %s are members by enumeration alone (15 on 2026-10-31) — the third seed has stopped matching, and the one-package censuses are back behind the ~950s arm run: %v", len(enumOnly), twdPossePkg, enumOnly)
	}
	t.Logf("parsed %d test functions under internal/posse, %d of them tree-wide", funcs, len(names))

	// The second package (ranger-base-g6sb1). Its own floors, because the
	// two rules fail apart and a collapse on one side must not be covered
	// by the other side's health.
	tpNames, tpDerived, tpFuncs := twdTreepinsTreeWideTests(t)
	if tpFuncs < 200 {
		t.Fatalf("only %d test functions parsed under %s — the walk this arm derives the second half of the class from found nothing", tpFuncs, twdTreepinsPkg)
	}
	if len(tpDerived) < 30 {
		t.Fatalf("only %d tree-wide tests derived in %s (40 on 2026-10-31, 23 on 2026-10-04 of which 20 predate ranger-base-g6sb1's own arms) — the enumeration rule has stopped matching, and this arm is now deriving a class that is missing members: %v", len(tpDerived), twdTreepinsPkg, tpDerived)
	}
	// And the root-helper half on its own, for the reason the posse half has
	// the same check one screen up: the two rules fail apart, and a union
	// floor stays green on the enumeration half alone while every pin that
	// takes its root from repoRoot() goes quiet (ranger-base-8wxbn).
	if rootOnly := twdTreepinsRootOnlyTests(t); len(rootOnly) < 1 {
		t.Errorf("no test in %s is a member only by reaching %s and a walker (five were on 2026-10-31: root_test.go's own arm, and the two pairs over internal/posse's sources) — half the second package's class rule is matching nothing", twdTreepinsPkg, twdTreepinsRootHelper)
	}
	t.Logf("parsed %d test functions under %s, %d of them tree-wide (%d derived, %d registered indirect)", tpFuncs, twdTreepinsPkg, len(tpNames), len(tpDerived), len(twdIndirect))

	// Both ways, per package. Both directions are the bug: a pin with no
	// door is ranger-base-ik44f arriving again, and a door naming a test
	// that is not in its package's class is a `-run` filter matching
	// nothing, which passes in silence.
	for _, class := range []struct {
		pkg   string
		names []string
	}{
		{twdPossePkg, names},
		{twdTreepinsPkg, tpNames},
	} {
		found := map[string]bool{}
		for _, name := range class.names {
			found[name] = true
			d, ok := doored[name]
			if !ok {
				t.Errorf("%s (%s) reads the tree and no Makefile door names it — it is reachable only by a `-run` filter that happens to spell it, which is ranger-base-ik44f arriving again. Give it a door, or add it to one whose subject it shares.", name, class.pkg)
				continue
			}
			if d.pkg != class.pkg {
				t.Errorf("%s is a tree-wide test in %s and $(%s) doors it with `go test ./%s` — the door runs a package the pin does not live in, so its filter matches nothing and it passes in silence", name, class.pkg, d.variable, d.pkg)
			}
		}
		members := twdTestNames(t, class.pkg)
		for name, d := range doored {
			if d.pkg != class.pkg || found[name] {
				continue
			}
			if _, holder := twdDoorHolders[name]; holder {
				if !members[name] {
					t.Errorf("twdDoorHolders registers %s and %s declares no such test — the one exemption from the derivation is holding a door open for a name that is not there", name, class.pkg)
				}
				continue
			}
			if members[name] {
				// The test is there; the DERIVATION stopped calling it
				// tree-wide. That is the more interesting of the two
				// failures and it has a usual cause, so name it: the rule
				// reads a spelling, and a reading rewritten into a
				// spelling it cannot see goes quiet without the pin
				// changing at all. This arm found exactly that on
				// 2026-10-04, when twdTestFiles' glob was briefly composed
				// from a parameter.
				t.Errorf("$(%s) names %s, which IS a test in %s and is no longer derived as tree-wide. Either it stopped reading the tree, in which case drop it from the door — or its reading was rewritten into a spelling the rule cannot see, which is the usual cause: an enumeration root composed from a variable instead of spelled as a literal. The door still passes; the pin is back to being a package-tree run away.", d.variable, name, class.pkg)
				continue
			}
			t.Errorf("$(%s) names %s, which is not a test in %s at all — the door's `-run` filter matches nothing there and passes in silence", d.variable, name, class.pkg)
		}
	}

	// And the exemption list cannot keep rows no door uses: a door-holder
	// nothing doors is a reason recorded for a decision nobody took.
	for name := range twdDoorHolders {
		if _, ok := doored[name]; !ok {
			t.Errorf("twdDoorHolders registers %s and no Makefile door names it — drop the row, or give it the door the row says it needs", name)
		}
	}
}

// twdTestNames is the set of test functions one package declares, used to
// tell "this door names a test that is no longer derived" from "this door
// names nothing at all".
func twdTestNames(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	all, _ := twdParseDir(t, pkg)
	out := map[string]bool{}
	for name, f := range all {
		if f.test {
			out[name] = true
		}
	}
	return out
}

// twdTreepinsRootOnlyTests is the root-helper half of the internal/treepins
// class on its own: tests that are members ONLY because they reach
// repoRoot() and a walker, with no relative-literal enumeration anywhere in
// their reach.
func twdTreepinsRootOnlyTests(t *testing.T) []string {
	t.Helper()
	all, _ := twdParseTreepins(t)
	enum := twdReach(all, func(f *twdFunc) bool { return len(f.relEnum) > 0 })
	rootReach := twdReach(all, func(f *twdFunc) bool { return f.treeRoot })
	walkReach := twdReach(all, func(f *twdFunc) bool { return f.walk })
	var out []string
	for name, f := range all {
		if f.test && !enum[name] && rootReach[name] && walkReach[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// twdWalkReachingTests is the walk-reaching half of the class on its own:
// tests that are members ONLY because they reach a rooted walk through a
// helper, with no qibRepoRoot call of their own.
func twdWalkReachingTests(t *testing.T) []string {
	t.Helper()
	all, _ := twdParse(t)
	reaches := twdReach(all, func(f *twdFunc) bool { return f.rootedWalk })
	var out []string
	for name, f := range all {
		if f.test && reaches[name] && !f.root {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// The pin ranger-base-sx2dq was filed for. The class above is DERIVED, and a
// derivation is only as wide as the thing it keys on: internal/posse carried
// two byte-identical repo-root helpers, qibRepoRoot and qspRepoRoot, and a
// pin spelled with the second one was outside the class, got no door, and
// nothing said so. Folding the twin made the rule true again; this is what
// keeps a third copy from arriving and quietly undoing it.
//
// Two shapes, both measured against the tree rather than imagined:
//
//   - a TWIN HELPER: a function that asks runtime.Caller where it lives and
//     climbs two levels, which is qibRepoRoot's body written again under
//     another name. Whether or not it walks, its callers are invisible.
//   - a HIDDEN WALKER: a function that walks a tree AND spells the climb to
//     the repo root by hand (`filepath.Abs("../..")`,
//     `filepath.Join(dir, "..", "..")`). TestSeedConfigLiveKeysAreRead was
//     exactly this: a WalkDir over every non-test .go file in the repo,
//     reached by no helper at all, so no identifier match could ever find
//     it.
//   - a SHELLED ROOT: a body that asks git — `exec.Command("git",
//     "rev-parse", "--show-toplevel").Output()` — and keeps the answer.
//     ranger-base-xndgk FINDING 5: FOUR tree-wide pins were spelled this
//     way, one of them a census of EVERY TRACKED PATH in the repository, and
//     none of them was in any door variable. Both rules above key on Go's
//     own filesystem calls, so neither could ever have seen a root that
//     arrives as the stdout of a subprocess. A probe that throws the answer
//     away is not this shape and is not fenced.
//
// Reading ONE file at the repo root is not either shape and is not fenced —
// ~48 test files do it, they red only when that one file changes, and that
// is not this class.
func TestQAOneRepoRootHelperInTheTestPackage(t *testing.T) {
	t.Parallel()
	all, funcs := twdParse(t)
	if funcs < 300 {
		t.Fatalf("only %d test functions parsed under internal/posse — the walk this pin reads is finding nothing", funcs)
	}
	if _, ok := all[twdRootHelper]; !ok {
		t.Fatalf("internal/posse's tests no longer declare %s — the class in this file is derived from it, so every derived arm above is now deriving nothing", twdRootHelper)
	}

	var twins, hidden []string
	for name, f := range all {
		if name == twdRootHelper || f.root {
			continue
		}
		if f.caller && f.ascent {
			twins = append(twins, f.where+" "+name)
		}
		if f.handRolledTreeWalk {
			hidden = append(hidden, f.where+" "+name)
		}
	}
	sort.Strings(twins)
	sort.Strings(hidden)
	for _, w := range twins {
		t.Errorf("%s asks runtime.Caller and climbs two levels — that is a second %s under another name, and every pin spelled with it is outside the tree-wide class this file derives, so it gets no door and nothing says so. Call %s instead. (ranger-base-sx2dq: the tree carried exactly this, byte for byte.)", w, twdRootHelper, twdRootHelper)
	}
	for _, h := range hidden {
		t.Errorf("%s walks a tree from a hand-rolled climb to the repo root — a tree-wide pin no identifier match can reach, which is how TestSeedConfigLiveKeysAreRead went undoored. Take the root from %s.", h, twdRootHelper)
	}

	// THE FOURTH SPELLING (ranger-base-nt0pi): no root at all. A relative
	// literal that climbs out of the package reaches the tree directly,
	// because `go test` runs internal/posse's binary with the package
	// directory as cwd — so there is nothing for the twin, climb or git
	// arms above to catch, and the enumeration is as wide as any walk from
	// the root. Three pins were spelled that way and had no door:
	// TestQAPrescribedGitDiffChecksStateTheirFormat and its partner globbed
	// `../../docs/notes.d/notes-*.md`, and TestShelfPIDsLintCleanAsASet
	// globbed `../../examples/skills/*`.
	//
	// THE CONTROL FIRST, because this arm's live answer is zero by
	// construction once those three are fixed, and a prohibition nothing
	// violates is a prohibition nobody can tell from a broken detector. The
	// fixtures go through twdAnalyze, the same function that reads the
	// tree.
	for _, c := range []struct {
		name string
		src  string
		want bool
	}{
		{"a glob through a climbing literal", `func TestX(t *testing.T) { filepath.Glob("../../docs/notes.d/notes-*.md") }`, true},
		{"a ReadDir of the climb itself", `func TestX(t *testing.T) { os.ReadDir("../..") }`, true},
		{"a Join whose first element climbs", `func TestX(t *testing.T) { filepath.Glob(filepath.Join("..", "..", "examples", "skills", "*")) }`, true},
		{"a Clean around the same Join", `func TestX(t *testing.T) { os.ReadDir(filepath.Clean(filepath.Join("..", "..", "examples"))) }`, true},
		{"reading one named file through a climb is not an enumeration", `func TestX(t *testing.T) { os.ReadFile("../../AGENTS.md") }`, false},
		{"an enumeration that does not climb is the package's own directory", `func TestX(t *testing.T) { os.ReadDir(".") }`, false},
		// THE IDENTIFIER (ranger-base-8yv7z). This row read false until this
		// bead, and said so out loud as nt0pi's residual: six live pins read
		// the tree through a climb held in a local or a package const, and
		// every one of them was outside the class with no door.
		{"a climb through a local", `func TestX(t *testing.T) { dir := "../../etc"; os.ReadDir(dir) }`, true},
		{"a climb through a local const", `func TestX(t *testing.T) {
	const dir = "../../etc/claude/managed-settings.d"
	os.ReadDir(dir)
}`, true},
		{"a climb through a package const", `const dropIns = "../../etc/claude/managed-settings.d"
func TestX(t *testing.T) { os.ReadDir(dropIns) }`, true},
		{"a climb through a local assigned a Join", `func TestX(t *testing.T) { dir := filepath.Join("..", "..", "examples"); filepath.WalkDir(dir, nil) }`, true},
		{"a climb through a local, joined at the walker", `func TestX(t *testing.T) { dir := "../../examples"; os.ReadDir(filepath.Join(dir, "agents")) }`, true},
		// AND THE JURISDICTION LINE. A climb made only of `..` evaluates to
		// the repo root, and a hand-rolled climb to the ROOT is the `hidden`
		// arm's — which scores a walk of a SUBDIRECTORY of it as not
		// tree-wide. detectionRig is that shape and nine tests reach it, so
		// binding the bare root here would have fenced it and the fence's own
		// remediation would then have put all nine in the class.
		{"a bare-root local walked directly is the hidden arm's", `func TestX(t *testing.T) { repo := filepath.Join("..", ".."); os.ReadDir(repo) }`, false},
		// AND THE CLIMB SPELLED PAST A JOIN'S FIRST ELEMENT
		// (ranger-base-zm6yw). These three rows read false until this bead,
		// and the row above them used to claim the shape was "the hidden
		// arm's", which MEASUREMENT said it was not: the fence reads a Join's
		// first element, which here is the cwd, and the hidden arm wants the
		// two `..` at the END of the list, so a reading of every file under
		// internal/ was scored by nothing at all. It was live —
		// bobtripwire_qa_test.go, in no door variable.
		{"a climb spelled past the Join's first element", `func TestX(t *testing.T) { wd, _ := os.Getwd(); filepath.WalkDir(filepath.Join(wd, "..", "..", "internal"), nil) }`, true},
		{"the same climb held in a local first", `func TestX(t *testing.T) { wd, _ := os.Getwd(); dir := filepath.Join(wd, "..", "..", "etc", "herdr"); os.ReadDir(dir) }`, true},
		{"and through a Clean around it", `func TestX(t *testing.T) { wd, _ := os.Getwd(); os.ReadDir(filepath.Clean(filepath.Join(wd, "..", "..", "etc"))) }`, true},
		// The jurisdiction line holds on this arm too: something must FOLLOW
		// the pair, because a Join that ends in it evaluates to the root and
		// the root arms already score it (both MEASURED on the five probes).
		{"the same Join ending at the root is the hidden arm's, not the fence's", `func TestX(t *testing.T) { wd, _ := os.Getwd(); filepath.WalkDir(filepath.Join(wd, "..", ".."), nil) }`, false},
		{"one level is not the climb out of internal/posse", `func TestX(t *testing.T) { wd, _ := os.Getwd(); os.ReadDir(filepath.Join(wd, "..", "posse")) }`, false},
		// AND THE RESIDUAL, which is a jurisdiction decision and not a gap:
		// a subpath joined onto a bare-root IDENTIFIER. detectionRig is this
		// shape and ranger-base-sx2dq ruled it out of the class by name, so
		// widening to it would reverse that ruling — ranger-base-er0gn.
		// These rows record what the rule ANSWERS, and
		// TestQATheClimbFenceStillHasItsBareRootJoinHole says out loud that
		// nothing else answers either.
		{"a subpath joined onto a bare-root local is scored by NOTHING (ranger-base-er0gn)", `func TestX(t *testing.T) { repo := filepath.Join("..", ".."); os.ReadDir(filepath.Join(repo, "etc", "herdr")) }`, false},
		{"an identifier holding a scratch directory is not a climb", `func TestX(t *testing.T) { dir := t.TempDir(); os.ReadDir(dir) }`, false},
		{"an identifier holding the package's own directory is not a climb", `func TestX(t *testing.T) { dir := "testdata"; os.ReadDir(dir) }`, false},
		// THE PARAMETER HOP, and the CHAIN (ranger-base-8yv7z). The shelf was
		// read through `filepath.Join("..", "..", "examples", "agents")`
		// handed to a helper that handed its own parameter to a helper that
		// globbed at that parameter — two call sites from the walk. The fence
		// closes the relation over calls; the DERIVATION still does not (see
		// twdClimbHops).
		{"a climb handed to a helper that walks that parameter", `func scan(dir string) { filepath.Glob(filepath.Join(dir, "*.md")) }
func TestX(t *testing.T) { scan(filepath.Join("..", "..", "examples", "agents")) }`, true},
		{"a climb handed two call sites from the walk", `func inner(dir string) { filepath.Glob(filepath.Join(dir, "*.md")) }
func mid(t *testing.T, dir string) { inner(dir) }
func TestX(t *testing.T) { mid(t, "../../examples/agents") }`, true},
		{"a climb through an identifier handed down a chain", `func inner(dir string) { os.ReadDir(dir) }
func mid(dir string) { inner(dir) }
func TestX(t *testing.T) { dir := "../../examples"; mid(dir) }`, true},
		{"a hop handed a scratch directory is not a climb", `func scan(dir string) { filepath.Glob(filepath.Join(dir, "*.md")) }
func TestX(t *testing.T) { scan(t.TempDir()) }`, false},
		{"a hop handed the package's own directory is not a climb", `func scan(dir string) { filepath.Glob(filepath.Join(dir, "*.md")) }
func TestX(t *testing.T) { scan("testdata") }`, false},
		{"a climb at a position that is not the walked parameter", `func scan(name string, dir string) { os.ReadDir(dir) }
func TestX(t *testing.T) { scan("../../examples", t.TempDir()) }`, false},
		{"a parameter that roots no walk is not a hop", `func read(p string) { os.ReadFile(p) }
func TestX(t *testing.T) { read("../../AGENTS.md") }`, false},
		// THE RANGE VARIABLE (ranger-base-g31e0). These three rows read
		// false until this bead: the set bound at an AssignStmt or a
		// ValueSpec, `roots := []string{...}` binds the SLICE, and climbs()
		// answers false for a CompositeLit — so the range value was never
		// bound and three live pins read the tree with nothing watching.
		// The list may be written at the range or held in a name, and
		// nothing about the hole depended on which.
		{"a climb reaching the walker as a range variable over an inline list", `func TestX(t *testing.T) { for _, r := range []string{"..", "../../cmd"} { filepath.Walk(r, nil) } }`, true},
		{"a climb reaching the walker as a range variable over a named list", `func TestX(t *testing.T) { roots := []string{"..", "../../cmd"}
	for _, r := range roots { filepath.Walk(r, nil) } }`, true},
		{"a climb beside the package's own directory in ONE list", `func TestX(t *testing.T) { for _, d := range []string{".", "../../cmd/posse"} { os.ReadDir(d) } }`, true},
		// AND THE JURISDICTION LINE AGAIN, held per ELEMENT: a list of bare
		// roots is twdRootExprs' and the hidden arm's, so widening to lists
		// cannot take detectionRig's nine callers with it.
		{"a range over bare roots only is the hidden arm's shape", `func TestX(t *testing.T) { for _, r := range []string{"..", "../.."} { os.ReadDir(r) } }`, false},
		{"a range over scratch directories is not a climb", `func TestX(t *testing.T) { for _, r := range []string{t.TempDir()} { os.ReadDir(r) } }`, false},
		{"a range variable that roots no walker is not a climb", `func TestX(t *testing.T) { for _, p := range []string{"../../AGENTS.md"} { os.ReadFile(p) } }`, false},
	} {
		if got := twdFixtureClimbs(t, c.src); got != c.want {
			t.Errorf("the climbing-enumeration rule says %v for this source, want %v (%s) — the fence below is then measuring something other than what this comment says, and its zero is not evidence:\n%s", got, c.want, c.name, c.src)
		}
	}

	// THE ARM FIRES WHERE NOTHING SEES THE READING, which is why it reads
	// both of the register's own answers before it speaks. `!f.root` has
	// always been here: a body that takes the root from the one helper is
	// derived by the direct rule, so its climb is not what makes anything
	// invisible. `len(f.relEnum) == 0` is the same condition for the seed
	// (ranger-base-g31e0): since ranger-base-na4rm a body that enumerates a
	// readable relative path IS a derived member and IS doored, and a
	// mixed list — `for _, d := range []string{".", "../../cmd/posse"}` —
	// is exactly that body with a climb beside the `"."`.
	//
	// WITHOUT THIS THE ARM'S OWN SENTENCE WOULD BE FALSE for such a body:
	// it says "no rule in this file can see it, the pin gets no door", and
	// the seed sees it and the door is in the Makefile. Worse, the
	// remediation would ask a census of internal/posse's own sources to
	// take a root, which is the one thing ranger-base-m94g2 decided it need
	// not do — the fifteen one-package censuses read `.` relatively and are
	// members BECAUSE of it. Demanding a root of the mixed spelling alone
	// would treat it more harshly than the `"."`-only sibling the house
	// blessed, for nothing: the door is the same door.
	//
	// What stays fenced is a climb NOTHING resolves. visibility_test.go's
	// `[]string{"..", "../../cmd"}` is that: every element climbs, so the
	// seed reads no element and the list is not a reading this file can
	// name — the fence is its only route, which is what keeps
	// ranger-base-tw3t7's two findings two.
	var climbed []string
	for name, f := range all {
		if f.climbEnum && !f.root && len(f.relEnum) == 0 {
			climbed = append(climbed, f.where+" "+name)
		}
	}
	sort.Strings(climbed)
	for _, c := range climbed {
		t.Errorf("%s enumerates a directory through a relative literal that climbs out of the package — a reading of the TREE that computes no root, so no rule in this file can see it, the pin gets no door and nothing says so. Take the root from %s and join onto it. (ranger-base-nt0pi: three pins were spelled this way, one of them a census of every `git diff` prescription this repo ships.)", c, twdRootHelper)
	}

	var shelled []string
	for name, f := range all {
		if f.shellRoot {
			shelled = append(shelled, f.where+" "+name)
		}
	}
	sort.Strings(shelled)
	for _, sh := range shelled {
		t.Errorf("%s asks git for this repo's root and keeps the answer — `exec.Command(\"git\", \"rev-parse\", \"--show-toplevel\").Output()` is a root no rule in this file can see, so a pin spelled with it is outside the tree-wide class and gets no door. Take the root from %s and, if the reading needs a checkout, skip with qibSkipUnlessCheckout. (ranger-base-xndgk FINDING 5: four pins were spelled this way, one of them a census of every tracked path in the repository.)", sh, twdRootHelper)
	}

	// AND THE SAME FENCE ONE PACKAGE OVER (ranger-base-8wxbn). Most pins in
	// internal/treepins read a relative literal and name no helper at all,
	// because that package's TestMain chdirs the binary to the repo root.
	// Five do not: they take the root as an ABSOLUTE path from repoRoot()
	// and hand it to a helper that walks a subtree of it, and the
	// derivation finds them by reading that CALL. So a second repo-root
	// helper there is a class member nothing can see — exactly what
	// qspRepoRoot was next door, and the reason this pin exists at all.
	tp, tpFuncs := twdParseTreepins(t)
	if tpFuncs < 200 {
		t.Fatalf("only %d test functions parsed under %s — the walk the second half of this pin reads is finding nothing", tpFuncs, twdTreepinsPkg)
	}
	if _, ok := tp[twdTreepinsRootHelper]; !ok {
		t.Fatalf("%s's tests no longer declare %s — half that package's class is derived from the call to it, so every pin spelled with it is now outside the class, gets no door, and nothing says so", twdTreepinsPkg, twdTreepinsRootHelper)
	}
	var tpTwins, tpHidden, tpShelled []string
	for name, f := range tp {
		if name == twdTreepinsRootHelper {
			continue
		}
		if f.getwdClimb {
			tpTwins = append(tpTwins, f.where+" "+name)
		}
		if f.handRolledTreeWalk {
			tpHidden = append(tpHidden, f.where+" "+name)
		}
		if f.shellRoot {
			tpShelled = append(tpShelled, f.where+" "+name)
		}
	}
	sort.Strings(tpTwins)
	sort.Strings(tpHidden)
	sort.Strings(tpShelled)
	for _, w := range tpTwins {
		t.Errorf("%s climbs from os.Getwd() looking for go.mod — that is a second %s under another name, and every pin in %s spelled with it is outside the tree-wide class, so it gets no door and nothing says so. Call %s instead.", w, twdTreepinsRootHelper, twdTreepinsPkg, twdTreepinsRootHelper)
	}
	for _, h := range tpHidden {
		t.Errorf("%s walks a tree from a hand-rolled climb to the repo root. In %s that is never needed — TestMain chdirs the binary to the root, so a relative literal IS a tree path — and a pin spelled that way is outside both rules of that package's class. Walk the relative path, or take the root from %s.", h, twdTreepinsPkg, twdTreepinsRootHelper)
	}
	for _, sh := range tpShelled {
		t.Errorf("%s asks git for this repo's root and keeps the answer, in %s — stdout from a subprocess, which neither rule of that package's class can see, so the pin gets no door. Take the root from %s, or walk the relative path TestMain already put you at.", sh, twdTreepinsPkg, twdTreepinsRootHelper)
	}
}

// twdBareRootJoinReadings is the detector for the one shape FINDING 1 of
// ranger-base-wq349 leaves open: a walker rooted at a path built by JOINING a
// subpath onto an identifier that holds the bare repo root.
//
// It lives in the pin below rather than in the fence on purpose. Promoting it
// is the whole of the fix, and the fix is a jurisdiction decision filed as
// ranger-base-er0gn — so the measurement is written once, here, where it says
// what the hole IS and what the tree carries, and the bead that decides moves
// these twenty lines into twdClimbSet rather than inventing them.
//
// It reads the bare root through twdRootExprs, which is the dataflow the
// corrected residual paragraph names, and the subpath through
// twdClimbNamesASubpath, which is the fence's own jurisdiction test — so the
// detector cannot be wider or narrower than the two rules it sits between.
func twdBareRootJoinReadings(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	empty := twdNewClimbSet()
	var out []string
	for _, path := range twdTestFiles(t, twdPossePkg) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			isRoot := twdRootExprs(fn.Body)
			held := map[string]bool{}
			// below: a Join whose base is the bare root (or something
			// already below it) and which NAMES at least one further
			// segment. Clean/Abs around it do not change the reading.
			var below func(ast.Expr) bool
			below = func(e ast.Expr) bool {
				call, ok := e.(*ast.CallExpr)
				if !ok {
					return false
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || len(call.Args) == 0 {
					return false
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "filepath" {
					return false
				}
				switch sel.Sel.Name {
				case "Clean", "Abs", "ToSlash", "FromSlash", "EvalSymlinks":
					return below(call.Args[0])
				case "Join":
					if len(call.Args) < 2 {
						return false
					}
					base := call.Args[0]
					id, isIdent := base.(*ast.Ident)
					if !isRoot(base) && !below(base) && !(isIdent && held[id.Name]) {
						return false
					}
					for _, a := range call.Args[1:] {
						if twdClimbNamesASubpath(a, empty) {
							return true
						}
					}
				}
				return false
			}
			// Two passes, so a name bound before the walk and after both
			// bind — twdRootExprs' own reason.
			for i := 0; i < 2; i++ {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					var lhs, rhs []ast.Expr
					switch st := n.(type) {
					case *ast.AssignStmt:
						lhs, rhs = st.Lhs, st.Rhs
					case *ast.ValueSpec:
						for _, id := range st.Names {
							lhs = append(lhs, id)
						}
						rhs = st.Values
					default:
						return true
					}
					if len(rhs) != 1 || len(lhs) == 0 {
						return true
					}
					if id, ok := lhs[0].(*ast.Ident); ok && id.Name != "_" && below(rhs[0]) {
						held[id.Name] = true
					}
					return true
				})
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !twdWalkers[sel.Sel.Name] {
					return true
				}
				arg := call.Args[0]
				id, isIdent := arg.(*ast.Ident)
				if below(arg) || (isIdent && held[id.Name]) {
					pos := fset.Position(call.Lparen)
					out = append(out, fmt.Sprintf("%s:%d %s %s", path, pos.Line, fn.Name.Name, sel.Sel.Name))
				}
				return true
			})
		}
	}
	sort.Strings(out)
	return out
}

// A LIVE-DEFECT PIN (ranger-base-zm6yw, FINDING 1 of the ranger-base-wq349
// verify). It ships GREEN asserting a hole, and the FIX makes it RED.
//
// THE HOLE. A walker rooted at a subpath JOINED onto an identifier holding the
// bare repo root is in neither half of the tree-wide class:
//
//	repo := filepath.Join("..", "..")
//	os.ReadDir(filepath.Join(repo, "etc", "herdr", "agent-detection"))
//
// MEASURED 2026-11-01 at main 5620d3b9 (darwin 25.6.0, go 1.26.10), through
// twdAnalyze itself: climbEnum=false, handRolledTreeWalk=false, root=false,
// relEnum=false. The fence scores it false by its declared jurisdiction line —
// a climb must NAME a path, and `../..` names the root — and the hidden arm
// fires only when the walk's root IS the bare root. So the reading gets no
// door, no census names it, and until this bead nothing said so: twdClimbSet's
// residual paragraph asserted the shape "is the hidden arm's and is scored by
// its dataflow", which is the sentence a later reader trusts instead of
// re-censusing, and it was wrong.
//
// WHY IT IS A PIN AND NOT A FENCE ARM. Closing it is a jurisdiction decision,
// not a rule gap. detectionRig (verifydetection_qa_test.go) is exactly this
// shape and twdRootExprs' head comment rules it OUT of the class by name
// (ranger-base-sx2dq, "only a walk rooted AT the root is tree-wide"), while
// TestQAKnownScreenReaderNamesEveryFixtureExactly reads the SIBLING directory
// under etc/herdr/agent-detection and was ruled IN (ranger-base-8yv7z, doored
// in $(QA_SEED_PINS)) — and ranger-base-m94g2/na4rm then made a DISCOVERED
// file set the criterion outright, which makes detectionRig's callers members.
// Those rulings cannot all stand, and whichever wins moves five call sites'
// worth of tests into or out of the class. Filed as ranger-base-er0gn
// (-l architecture).
//
// WHEN THE FIX LANDS THIS TEST GOES RED, and that red is the instruction:
// twdBareRootJoinReadings above is the arm — move it into twdClimbSet beside
// twdJoinClimbsToASubpath, remediate what it names, and replace this body with
// the fence's own census (the live list must then be EMPTY).
//
// THE LIVE CENSUS IS NOT DECORATION: a live-defect pin with no instance is a
// pin that cannot tell "the hole is open" from "the detector broke", which is
// the negative-control trap this file names everywhere else. It is asserted
// non-empty, and it prints what it found so the bead that decides has the
// list without re-deriving it.
func TestQATheClimbFenceStillHasItsBareRootJoinHole(t *testing.T) {
	t.Parallel()

	const inversion = "the bare-root-join hole is CLOSED — some rule in this file now scores a walker rooted at " +
		"filepath.Join(<bare root>, \"sub\"). Promote twdBareRootJoinReadings into twdClimbSet beside " +
		"twdJoinClimbsToASubpath, and rewrite this pin as the fence's census, which must then be empty " +
		"(ranger-base-er0gn decides which bodies that takes with it)."

	// The shape, scored by every rule this file has. All four false IS the
	// defect; any one of them flipping is the fix arriving.
	for _, c := range []struct {
		name string
		src  string
	}{
		{"walked through a local", `func TestX(t *testing.T) { repo := filepath.Join("..", ".."); dir := filepath.Join(repo, "etc", "herdr"); os.ReadDir(dir) }`},
		{"walked at the call", `func TestX(t *testing.T) { repo := filepath.Join("..", ".."); os.ReadDir(filepath.Join(repo, "etc", "herdr")) }`},
		{"the whole of internal/, which is nobody's one directory", `func TestX(t *testing.T) { repo := filepath.Join("..", ".."); filepath.WalkDir(filepath.Join(repo, "internal"), nil) }`},
	} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "fixture_test.go", "package posse\n"+c.src+"\n", 0)
		if err != nil {
			t.Fatalf("parse fixture: %v\n%s", err, c.src)
		}
		env := twdNewPkgEnv(fset, map[string]*ast.File{"fixture_test.go": file})
		var f *twdFunc
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name != "TestX" {
				continue
			}
			f = twdAnalyze(fn, "fixture", env)
		}
		if f == nil {
			t.Fatalf("fixture declares no TestX:\n%s", c.src)
		}
		if f.climbEnum || f.handRolledTreeWalk || f.root || len(f.relEnum) > 0 {
			t.Errorf("%s: climbEnum=%v handRolledTreeWalk=%v root=%v relEnum=%v — %s\n%s",
				c.name, f.climbEnum, f.handRolledTreeWalk, f.root, len(f.relEnum) > 0, inversion, c.src)
		}
	}

	// And the tree, so the hole is not asserted over nothing. detectionRig
	// and its ReadDirs are the live instances; the floor is 1 rather than
	// their exact count, because a bead that rewrites one of them should not
	// have to come here — what this catches is the DETECTOR going quiet.
	live := twdBareRootJoinReadings(t)
	if len(live) == 0 {
		t.Errorf("no body in %s reads a subpath joined onto a bare-root identifier — either %s",
			twdPossePkg, inversion)
	}
	t.Logf("%d live reading(s) of a subpath joined onto a bare-root identifier, scored by no rule in this file:\n  %s",
		len(live), strings.Join(live, "\n  "))
}

// twdSourceFiles lists what to copy: git's idea of the working tree, which
// leaves out bin/ and dist/ and every other build output.
//
// It falls back to a walk when git cannot answer, and the fallback is not
// decoration: a `git archive | tar -x` scratch tree is the house rig for
// mutation runs on this repo, it has no .git, and four root/internal pins
// already red on that alone in every arm including the control. A fifth
// would be a tax on every future rig, paid to save ten lines here.
func twdSourceFiles(t *testing.T) []string {
	t.Helper()
	if out, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output(); err == nil {
		var files []string
		for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if rel != "" {
				files = append(files, rel)
			}
		}
		return files
	}
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// twdSeedTree copies the working tree into a scratch directory: the door's
// recipe compiles internal/posse from there, so qibRepoRoot — which resolves
// off the compiled source's own path — answers the scratch root and the pins
// walk the copy. The WORKING tree and not HEAD, so a pin renamed in one
// commit with its door does not fail this arm against a stale HEAD.
func twdSeedTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copied := 0
	for _, rel := range twdSourceFiles(t) {
		info, err := os.Lstat(rel)
		if err != nil || !info.Mode().IsRegular() {
			// A symlink (plugin/bin/posse points at the installed binary)
			// or a path git knows and the tree does not. Neither compiles
			// and neither is read by the pins.
			continue
		}
		body, err := os.ReadFile(rel)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied < 500 {
		t.Fatalf("only %d files copied into the scratch tree — the pins below have file-count floors and would fail on the rig rather than on the drift", copied)
	}
	return dir
}

// twdExpand returns `make -n`'s own expansion of a door's recipe, so nothing
// here is a second copy of the command to drift from it.
func twdExpand(t *testing.T, target string) string {
	t.Helper()
	out, err := exec.Command("make", makeExpandFlag, "-n", target).Output()
	if err != nil {
		t.Fatalf("make -n %s: %v", target, err)
	}
	recipe := strings.TrimSpace(string(out))
	if recipe == "" {
		t.Fatalf("`make -n %s` expands to nothing", target)
	}
	if strings.Contains(recipe, "$(") {
		t.Fatalf("the expanded recipe still holds a make variable — this arm would be measuring the wrong text:\n%s", recipe)
	}
	assertRecipeIsOnlyRecipe(t, target, recipe)
	return recipe
}

// Arm 3: the doors can fail, on the drift they exist for.
func TestQATheTreeWideDoorsReportRealDrift(t *testing.T) {
	t.Parallel()
	crew := twdExpand(t, "crew-check")
	seed := twdExpand(t, "seed-check")
	doc := twdExpand(t, "doc-check")
	notes := twdExpand(t, "notes-check")
	dir := twdSeedTree(t)

	run := func(recipe string) (string, error) {
		cmd := exec.Command("sh", "-c", recipe)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		return string(b), err
	}

	// The clean arm first, both doors: a door that always fails detects
	// nothing, and a filter that matches nothing passes in silence.
	for _, door := range []struct{ name, recipe string }{{"crew-check", crew}, {"seed-check", seed}, {"doc-check", doc}, {"notes-check", notes}} {
		got, err := run(door.recipe)
		if err != nil {
			t.Fatalf("`make %s` failed on a clean copy of this tree — it reports drift that is not there:\n%s", door.name, got)
		}
		if strings.Contains(got, "no tests to run") {
			t.Fatalf("`make %s`'s filter matched no test at all, so its green says nothing:\n%s", door.name, got)
		}
	}

	// history-check is the one door this arm cannot copy: its three pins read
	// `git log` in THIS repo, and the scratch tree has no .git — all three
	// red there in every arm including the control, so a copied run would
	// measure the copy and not the door. Run it where it lives instead. It
	// only reads (`git log`, `git rev-list`), and this proves the two halves
	// a filter can get wrong: the recipe runs, and it names real tests.
	{
		out, err := exec.Command("sh", "-c", twdExpand(t, "history-check")).CombinedOutput()
		if err != nil {
			t.Errorf("`make history-check` failed in this repo — the door reports drift that is not there:\n%s", out)
		}
		if strings.Contains(string(out), "no tests to run") {
			t.Errorf("`make history-check`'s filter matched no test at all, so its green says nothing:\n%s", out)
		}
	}

	// The crew door's drift: a crew name in a file the shipped tree walk
	// reads. Assembled, never spelled — this file is inside that walk's
	// repo-root pass, the same reason instancebound_qa_test.go assembles.
	name := "gw" + "art"
	probe := filepath.Join("internal", "twd_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, probe), []byte("seat: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(crew)
	if err == nil {
		t.Fatalf("`make crew-check` passed a tree naming the originating instance's crew — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(probe), name, "TestShippedTreeNamesRolesNotThisCrew"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make crew-check` failed without naming %q, so it says a seat is wrong and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, probe)); err != nil {
		t.Fatal(err)
	}

	// The seed door's drift: the retired harness name on the published
	// surface. Assembled for the same reason the crew probe is — this file
	// is inside the walk that reads it.
	stale := "ranger" + "hq"
	seedProbe := filepath.Join("internal", "twd_seed_drift_probe.txt")
	if err := os.WriteFile(filepath.Join(dir, seedProbe), []byte("harness: "+stale+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(seed)
	if err == nil {
		t.Fatalf("`make seed-check` passed a tree carrying the retired harness name on the seed surface — the door reads nothing, or its members all SKIPPED, which is the same green:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(seedProbe), "TestSeedSurfaceNameCountIsZero"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make seed-check` failed without naming %q, so it says the surface is dirty and not where:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, seedProbe)); err != nil {
		t.Fatal(err)
	}

	// The doc door's drift: the ADR 0019 framing that was retired, back in a
	// shipped .go file — the exact way it walked in before, as a comment
	// nobody's test looks at. Assembled, so this file is not itself a hit
	// when a wider scanner than coxnGoSources is pointed at the tree.
	dead := "stale " + "leftover"
	docProbe := filepath.Join("internal", "twd_doc_drift_probe.go")
	body := "package internal\n\n// the on-disk credential file is a " + dead + " of a keychain login\n"
	if err := os.WriteFile(filepath.Join(dir, docProbe), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(doc)
	if err == nil {
		t.Fatalf("`make doc-check` passed a shipped source carrying the framing ADR 0019's amendment retired — the door reads nothing:\n%s", got)
	}
	for _, want := range []string{filepath.ToSlash(docProbe), dead, "TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make doc-check` failed without naming %q:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, docProbe)); err != nil {
		t.Fatal(err)
	}

	// The notes door's drift, and the one this bead was filed for
	// (ranger-base-g6sb1): a docs/notes.d fragment with no entry in that
	// directory's generated index. Every bead on this box writes one of
	// these, which is why it is the drift that got out — `make fmt-check`,
	// all eight doors as they then stood and a whole
	// `-tags posse_arm2 ./internal/posse` called the commit clean, and only
	// the 589.965s arm-1 run of internal/treepins said `stale index`.
	//
	// Not assembled, unlike the three probes above: a notes fragment is
	// data in a directory nothing in this package scans for content, so
	// there is no walk for this file's own bytes to turn up in.
	notesProbe := filepath.Join("docs", "notes.d", "twd-drift-probe.md")
	if err := os.WriteFile(filepath.Join(dir, notesProbe), []byte("# A fragment no index knows about\n2026-10-04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = run(notes)
	if err == nil {
		t.Fatalf("`make notes-check` passed a tree holding a notes fragment the index does not list — the door reads nothing, and the drift that filed ranger-base-g6sb1 is undetected again:\n%s", got)
	}
	for _, want := range []string{"README.md", "stale index", "scripts/notes-index.py"} {
		if !strings.Contains(got, want) {
			t.Errorf("`make notes-check` failed without naming %q, so it says the index is wrong and not what to run:\n%s", want, got)
		}
	}
	if err := os.Remove(filepath.Join(dir, notesProbe)); err != nil {
		t.Fatal(err)
	}
}

// The other half of arm 3 for the internal/treepins doors: that make's own
// expansion of each `-run` filter NAMES the pins its variable holds.
//
// WHY NOT THE DRIFT PLANT. The four doors above are exercised against a
// scratch copy of the tree, which is the strongest thing this file does. It
// is not available here, for three reasons that each hold on their own: the
// copy has no .git, and several of these pins read the tree through git; a
// `go test ./internal/treepins` in the copy is a cold build of the module
// rather than a door's worth of seconds; and register-check's own pins
// include THIS test, so arm 3 running it would be arm 3 running arm 3.
//
// What is left is the half a filter actually gets wrong. `-run` is a regex,
// it is assembled by make out of a variable and a `$$`, and a filter that
// matches nothing exits 0 and prints `ok`. Arm 2 proves the NAMES are real
// tree-wide tests in the right package; this proves that make's expansion of
// them, run by go, selects exactly those tests. It uses `-list` in place of
// `-run` — one substitution on make's own text, which is why the regex under
// test is still make's and not a copy of it — so it selects without running:
// ~0.3s per door against the ~31s of running them.
func TestQAEveryTreepinsDoorFilterNamesItsPins(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	for _, door := range twdDoors {
		if door.pkg != twdTreepinsPkg || door.tool {
			continue
		}
		recipe := twdExpand(t, door.target)
		listing := strings.Replace(recipe, "-run ", "-list ", 1)
		if listing == recipe {
			t.Errorf("`make %s` has no `-run ` to read as a `-list `, so this arm cannot ask what its filter selects:\n%s", door.target, recipe)
			continue
		}
		cmd := exec.Command("sh", "-c", listing)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("listing `make %s`'s filter failed: %v\n%s", door.target, err, out)
			continue
		}
		var listed []string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Test") {
				listed = append(listed, strings.TrimSpace(line))
			}
		}
		want := twdVar(t, src, door.variable)
		if !twdSameSet(listed, want) {
			t.Errorf("`make %s`'s filter selects %v, and $(%s) names %v — the regex make assembles and the pins the variable holds are not the same set, so the door runs fewer tests than its own membership says and still prints `ok`:\n%s", door.target, listed, door.variable, want, listing)
		}
	}
}

// twdSpelled spells a non-negative integer the way this file's head comment
// does. The comment is prose and says "twenty pins", not "20 pins", so the
// pin below has to compare against a WORD; the alternative — rewriting the
// sentence in digits so a test can read it — would let the test choose how
// the file reads, which is backwards.
func twdSpelled(n int) string {
	ones := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	switch {
	case n < 0 || n > 999:
		return strconv.Itoa(n) // out of the prose range; the pin will name the mismatch
	case n < 20:
		return ones[n]
	case n < 100 && n%10 == 0:
		return tens[n/10]
	case n < 100:
		return tens[n/10] + "-" + ones[n%10]
	// A HUNDRED AND UP (ranger-base-na4rm, which took the class past
	// ninety-nine). This used to answer with the digits, and the digits are
	// not a spelling: twdCountClaim's numeral group matches words, so the
	// one live count sentence would have stopped matching ALTOGETHER and
	// arm 4 would have failed on "the head comment carries 0 claims" — a
	// red that names the rule and not the class. The spelling is house
	// prose: `one hundred and one`, `two hundred`.
	case n%100 == 0:
		return ones[n/100] + " hundred"
	default:
		return ones[n/100] + " hundred and " + twdSpelled(n%100)
	}
}

// twdHead returns this file's own head comment as flowed prose — `//` markers
// stripped, lines joined with a space. Flowed, not per-line, because the
// claim it holds WRAPS (today "... over three runs" then "at twenty pins and
// seven doors", and the break moves whenever the sentence is rewrapped), and
// a per-line scan is blind to exactly that.
func twdHead(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("internal/treepins/treewidedoor_qa_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "package treepins" || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break // the head comment is over; `import (` is the first line here
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "//")))
	}
	if len(out) == 0 {
		t.Fatal("this file has no head comment — the sentence arm 4 holds is gone, and with it the number a seat prices `make tree-check` from")
	}
	return strings.Join(out, " ")
}

// twdCountClaim matches the ONE sentence in the head comment that says how
// big the class is. Deliberately narrow: the comment is full of correctly
// frozen history ("five pins took their root from ...", "five members became
// thirteen"), and a rule that read those as live claims would red on prose
// that is true.
//
// THE NUMERAL IS A PHRASE NOW, not a word (ranger-base-na4rm): past
// ninety-nine the spelling is `one hundred and one`, three words and an
// `and`, and the one-word rule this replaced would have matched nothing at
// all — a count that stopped being held, reported as a comment with no claim
// in it. What keeps the widening honest is the VOCABULARY: the numeral group
// is built from twdSpelled's own words rather than from `[a-z]+`, so no run
// of ordinary prose can be read as a count, and the two cannot disagree
// about what a numeral looks like. The `pins and ... doors` skeleton is
// unchanged and is what keeps the frozen history out: "those twelve pins and
// the rest was the box" has no `doors` after it, and never did.
var twdCountClaim = regexp.MustCompile(twdNumPhrase + ` pins and ` + twdNumPhrase + ` doors`)

// twdNumPhrase is one spelled numeral, captured: a number word, or several
// joined by a hyphen or by ` and ` the way twdSpelled joins them. Built from
// the same words twdSpelled writes, descending so that `one hundred and one`
// is preferred over the `one` inside it.
var twdNumPhrase = func() string {
	var words []string
	for n := 0; n <= 999; n++ {
		w := twdSpelled(n)
		if strings.ContainsAny(w, "0123456789") {
			continue
		}
		for _, part := range strings.FieldsFunc(w, func(r rune) bool { return r == ' ' || r == '-' }) {
			// `and` is a JOINER in `one hundred and one`, not a number
			// word: left in the vocabulary it would let the group match a
			// bare conjunction, and the joiner below spells it anyway.
			if part == "and" || slices.Contains(words, part) {
				continue
			}
			words = append(words, part)
		}
	}
	// Longest first. Go's regexp picks the match a backtracking search would
	// have found, so an alternation offering `nine` before `nineteen` reads
	// the first four letters of `nineteen` and then fails the phrase.
	sort.Slice(words, func(i, j int) bool {
		if len(words[i]) != len(words[j]) {
			return len(words[i]) > len(words[j])
		}
		return words[i] < words[j]
	})
	word := `(?:` + strings.Join(words, "|") + `)`
	return `(` + word + `(?:(?:[- ]|\s+and\s+)` + word + `)*)`
}()

// Arm 4: the head comment's count is the Makefile's count (ranger-base-4jogv).
//
// WHY THIS IS A PIN AND NOT A ONE-TIME CORRECTION. Both numerals were wrong
// at main, and they went wrong by two different routes, neither of which any
// existing arm can see:
//
//   - the DOORS number entered wrong (d189b623 wrote "seven doors" over an
//     enumeration of eight) and was then faithfully decremented to six when
//     the selector door and its pin were removed — a correct edit applied to
//     a wrong number stays wrong;
//   - the PINS number drifted with no edit at all, because a bead that adds
//     a name to $(QA_DOC_PINS) has no reason to read this file's prose.
//
// Arm 2 is two-way and mechanical, so no pin is undoored and no door is
// empty — the MECHANISM was never wrong. What was wrong is the sentence a
// seat reads to decide whether `make tree-check` is worth typing, in the one
// file whose entire subject is "the doors are wide enough". So the count is
// derived here, and the enumeration it rests on is held one-way: every name
// in a door variable must be named up there. One-way on purpose — the head
// also names tests that are NOT members (TestQAOneRepoRootHelperInTheTestPackage
// is the fence, not a pin), and a two-way rule would red on those.
//
// AND THE README's DOOR TABLE, for the same reason one page further out
// (ranger-base-r5546). internal/treepins/README.md carries the doors as a
// table, one row per door with its subject — more useful than a
// bare list, and the part that can drift. It was 14/14 correct the day it
// shipped and nothing would have said so when it stopped being: a door
// renamed, added or removed edits the Makefile and this file's enumeration,
// both of which are pinned, and leaves the public page saying what used to
// be true. Order too, not just membership — the table reads as the order
// `make tree-check` runs them, and a reader pricing a partial run from it
// is owed that.
//
// AND AGENTS.md's DOOR BLOCK AND ITS ABSENT COUNT (ranger-base-xed72). The
// crew-wide page carried the door list a third time and the PIN count a
// second time, and the count is the one that had already gone wrong: four
// short of the sentence above, because a bead that doors a pin corrects the
// file it edits and standing orders are nobody's subject. The sentence's own
// failure text says why that cannot be fixed by typing the right number —
// "two places drift apart and the reader cannot tell which is stale" — so
// the page states no count at all now, names this file instead, and this arm
// holds all three of those: the block against `tree-check`'s prerequisites
// (membership and order, like the README's), the pointer, and the absence of
// a numeral.
//
// AND ADR 0067's TWO (ranger-base-x0wc5). The record that writes this shop's
// dictionary down spelled the door count twice more — D1's "`check` is a verb
// and the name of fourteen Makefile doors" and the appendix row for `check` —
// both live, both held by nothing, and both now say it without the numeral.
// The record is worth holding for the same reason AGENTS.md is and one step
// stronger: it is the page that defines what `door` and `pin` MEAN, so a
// count of the class written there reads as the definition of the class.
//
// It also found the hole in the rule xed72 shipped. twdProseCount matched a
// numeral IMMEDIATELY before `pins`/`doors`, which is how AGENTS.md happened
// to spell it; "fourteen Makefile doors" has a word in between and went
// straight through. So the numeral may now be up to two plain words from its
// noun, and the noun must be PLURAL — singular prose is description, not a
// count ("a Makefile target that runs one tree-wide pin" is the `door` row,
// and it is true). MEASURED 2026-10-05: over this record and that bullet the
// widened rule matches nothing, and over D1's old sentence it matches.
func TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// The pins, deduped across door variables: arm 2 already fails a name
	// carried by two doors, so counting the union rather than the sum keeps
	// this arm from reporting a second, derived symptom of that one bug.
	seen := map[string]bool{}
	var pins []string
	for _, v := range twdPinVars {
		for _, name := range twdVar(t, src, v) {
			if seen[name] {
				continue
			}
			seen[name] = true
			pins = append(pins, name)
		}
	}
	// Tokens, and stopping at the `#`: a commented-out door is not a door.
	// The byte reader this replaced counted the comment marker AND the names
	// behind it, so this arm answered by accident either way — `# crew-check
	// ...` counted the `#` as an eighth door and red for a reason that has
	// nothing to do with the six it had just silenced, and `#ops-check`, no
	// space, still counted seven while `make tree-check` ran six
	// (ranger-base-7kwb8).
	_, doors := mkPrereqs(t, src, "tree-check")

	head := twdHead(t)

	// One live claim, and it is the derived one. Two matches means a second
	// sentence started counting and the two can now disagree; zero means the
	// claim moved out from under the pin that guards it.
	claims := twdCountClaim.FindAllStringSubmatch(head, -1)
	if len(claims) != 1 {
		t.Fatalf("the head comment carries %d `<n> pins and <n> doors` claims, want exactly 1 — the count of this class is said in one place on purpose, because two places drift apart and the reader cannot tell which is stale: %v", len(claims), claims)
	}
	gotPins, gotDoors := claims[0][1], claims[0][2]
	wantPins, wantDoors := twdSpelled(len(pins)), twdSpelled(len(doors))
	if gotPins != wantPins || gotDoors != wantDoors {
		t.Errorf("the head comment says %q pins and %q doors; the Makefile has %s (%d) and %s (%d).\n"+
			"  door variables: %v\n"+
			"  tree-check prerequisites: %v\n"+
			"A seat prices `make tree-check` from that sentence. Fix the sentence, not this test — and if a pin or a door really did come or go, the enumeration above the sentence needs the same edit.",
			gotPins, gotDoors, wantPins, len(pins), wantDoors, len(doors), twdPinVars, doors)
	}

	// And the enumeration the count rests on. Without this, the count stays
	// green while the list under it goes short — which is how the pins
	// number drifted in the first place: three tests were doored by beads
	// that never touched this comment.
	for _, name := range pins {
		// Word-bounded: a plain strings.Contains is a SUBSTRING test, so a
		// pin renamed to a strict prefix of a name already sitting in this
		// comment (e.g. a since-removed long name in the inventory above)
		// would satisfy it without being named here (ranger-base-erqvh row 1).
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(head) {
			t.Errorf("$(%s) door variable names %s, and this file's head comment does not — the enumeration the count sentence rests on is short by at least one, so the next reader counts a smaller class than `make tree-check` runs", twdVarOf(t, src, name), name)
		}
	}

	// And the README's table of the same doors, membership AND order.
	readme := filepath.Join(twdTreepinsPkg, "README.md")
	table := twdReadmeDoors(t, readme)
	if !slices.Equal(table, doors) {
		t.Errorf("%s lists the doors as\n  %v\nand `make tree-check`'s prerequisites are\n  %v\n"+
			"That table is a second copy of this list in a page read outside this package, and nothing but this arm would ever say it had gone stale. Fix the table, not this test.", readme, table, doors)
	}

	// And AGENTS.md's door block, held the same way — plus the count that
	// page is no longer allowed to write (ranger-base-xed72).
	agents := twdAgentsSection(t)
	block := twdAgentsDoors(t, agents)
	if !slices.Equal(block, doors) {
		t.Errorf("%s's tree-wide-pin bullet lists the doors as\n  %v\nand `make tree-check`'s prerequisites are\n  %v\n"+
			"That block is the door list every seat reads at session start, and nothing but this arm would ever say it had gone stale. Fix the block, not this test.", twdAgentsPath, block, doors)
	}
	flowed := strings.Join(agents, " ")
	if counts := twdProseCount().FindAllString(flowed, -1); len(counts) > 0 {
		t.Errorf("%s's tree-wide-pin bullet counts this class in prose (%q), and nothing holds that numeral — which is how it came to say four fewer pins than the Makefile had. Say it without the count (`all of them`, `two of them`): the one place a count of this class is written is this file's head comment, derived from the Makefile above.", twdAgentsPath, counts)
	}
	if !strings.Contains(flowed, "treewidedoor_qa_test.go") {
		t.Errorf("%s's tree-wide-pin bullet no longer names treewidedoor_qa_test.go — that page states no count of this class on purpose and points at the file that does, so a reader who loses the pointer has nowhere left to get the number.", twdAgentsPath)
	}

	// And the dictionary record, which is allowed to say what a door IS and
	// not how many there are (ranger-base-x0wc5).
	count := twdProseCount()
	for _, para := range twdADRParagraphs(t) {
		if counts := count.FindAllString(para, -1); len(counts) > 0 {
			t.Errorf("%s counts this class in prose (%q), and nothing holds that numeral:\n  %s\n"+
				"That record defines `pin` and `door`, so a count written there reads as part of the definition — and it is the copy that stayed wrong while AGENTS.md was being fixed. Say it without the count (`the Makefile doors`, `runs them all`): the one place a count of this class is written is this file's head comment, derived from the Makefile above.", twdADRPath, counts, para)
		}
	}
}

// twdReadmeDoors reads the door names out of the README's door table: the
// rows of the one table whose first column is a `make <door>` in backticks,
// in the order they appear.
//
// Shaped rather than grepped, and deliberately narrow about what a row is —
// the file names `make tree-check`, `make crew-check` and `make notes-check`
// in its PROSE as well, and a reader that took those would be reading the
// page's advice as if it were the list.
func twdReadmeDoors(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v — the package page carries the door table this arm holds to the Makefile", path, err)
	}
	row := regexp.MustCompile("^\\|\\s*`make ([a-z0-9-]+)`\\s*\\|")
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s has no door table — a row is a table line whose first cell is a backticked `make <door>`, and a page that lost them is a page that stopped saying what `make tree-check` runs", path)
	}
	return out
}

// twdAgentsPath is the crew-wide page that carries the third copy of the door
// list. Read by NAME, like the Makefile and the README: that is the
// enumeration rule's own negative case, so this arm stays a door-holder
// rather than becoming a derived member of the class (twdDoorHolders).
const twdAgentsPath = "AGENTS.md"

// twdAgentsSection returns the lines of AGENTS.md's tree-wide-pin bullet,
// trimmed: from the bullet that opens "A `-run` filter cannot reach a
// tree-wide pin" to the next top-level bullet or the end of the file.
//
// Scoped to the one bullet and not the page, because the rest of AGENTS.md
// counts its own things honestly — "121 pattern kills from 63 seats", "three
// binaries now" — and a page-wide count census would red on prose that is
// true. The bullet is found by its own sentence rather than a line number,
// and a page that has lost that sentence fails here rather than passing over
// nothing.
func twdAgentsSection(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(twdAgentsPath)
	if err != nil {
		t.Fatalf("%s: %v — the crew-wide page carries the door block this arm holds to the Makefile", twdAgentsPath, err)
	}
	lines := strings.Split(string(b), "\n")
	start := -1
	for i, l := range lines {
		if strings.Contains(l, "filter cannot reach a tree-wide pin") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s has no `-run filter cannot reach a tree-wide pin` bullet — that bullet is where a seat learns these doors exist at all, and this arm cannot hold a block it cannot find", twdAgentsPath)
	}
	out := []string{strings.TrimSpace(lines[start])}
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "- ") { // the next top-level bullet
			break
		}
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// twdAgentsDoors reads the door names out of the fenced block in that bullet:
// a line whose first token is `make <door>`, inside the fence.
//
// Fenced, and shaped rather than grepped, for the reason twdReadmeDoors is:
// the bullet names `make fmt-check`, `make tree-check`, `make crew-check` and
// `make notes-check` in its PROSE as advice, and a reader that took those
// would be reading the advice as if it were the list.
func twdAgentsDoors(t *testing.T, section []string) []string {
	t.Helper()
	row := regexp.MustCompile(`^make ([a-z0-9-]+)\s`)
	var out []string
	fenced := false
	for _, l := range section {
		if strings.HasPrefix(l, "```") {
			fenced = !fenced
			continue
		}
		if !fenced {
			continue
		}
		if m := row.FindStringSubmatch(l); m != nil {
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s's tree-wide-pin bullet has no door block — a row is a fenced line whose first token is `make <door>`, and a page that lost them is a page that stopped saying what `make tree-check` runs", twdAgentsPath)
	}
	return out
}

// twdADRPath is the record that writes this shop's dictionary down, and so
// the fourth page to carry a count of this class. Read by NAME, like the
// Makefile, the README and AGENTS.md, and for the same reason: a
// `filepath.Glob("docs/adr/0067-*.md")` would be a rooted enumeration, which
// is exactly the key twdTreeWideTests derives membership of this class from —
// this arm would become a member of the class it doors.
const twdADRPath = "docs/adr/0067-house-words-are-a-posse-dictionary-in-the-shape-of-ste.md"

// twdADRParagraphs returns that record's blank-line-separated paragraphs,
// each flowed to one line.
//
// Flowed because the claim this holds WRAPS — D1 said "the name of fourteen
// Makefile" and "doors)" on two lines, so a per-line scan was blind to the
// very sentence this bead was filed for. Per PARAGRAPH and not per file
// because flowing the whole record lets a numeral ending one paragraph reach
// a `pins` opening the next, and a red for a sentence pair nobody wrote is a
// red the next reader cannot act on. A table row is one line, so the
// appendix is covered either way.
func twdADRParagraphs(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(twdADRPath)
	if err != nil {
		t.Fatalf("%s: %v — that record is the dictionary this shop's door and pin words are defined in, and this arm holds it to stating no count of the class", twdADRPath, err)
	}
	var out []string
	for _, para := range strings.Split(string(b), "\n\n") {
		var fields []string
		for _, l := range strings.Split(para, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				fields = append(fields, l)
			}
		}
		if len(fields) > 0 {
			out = append(out, strings.Join(fields, " "))
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s is empty — the dictionary record cannot have lost its prose", twdADRPath)
	}
	return out
}

// twdProseCount matches a count of this class written in prose: one of this
// shop's spelled numerals (or digits) before a PLURAL `pins` or `doors`, and
// the `all <n>` form the door count was also written in. The numerals come
// from twdSpelled rather than being retyped, so this rule and the derivation
// above cannot disagree about what a count looks like.
//
// Narrow on purpose, the same way twdCountClaim is: it catches the spellings
// these pages actually carried — "forty-nine pins across the two, behind
// fourteen doors" and "`make tree-check` is all fourteen" in AGENTS.md,
// "the name of fourteen Makefile doors" in ADR 0067 — and leaves prose that
// counts something else alone.
//
// TWO BOUNDS, each paid for by a real sentence (ranger-base-x0wc5). The
// numeral may sit up to two plain words from its noun, because the ADR's
// spelling put one there and the adjacent-only rule read straight past it —
// the rule shipped to hold three pages matched the spelling of exactly one.
// The noun must be plural, which is what keeps the widened gap honest: the
// same record's `door` row says "a Makefile target that runs one tree-wide
// pin", and every nearby "D5 pin", "no prose pin" and "by the pin" would
// have reddened a gap rule that took the singular. A count of a class with
// one member is not a sentence this class has ever written.
func twdProseCount() *regexp.Regexp {
	// Up past a hundred since ranger-base-na4rm: the class is over
	// ninety-nine, so `one hundred and one pins` is a count these pages can
	// now write and a ceiling of 99 would have let it through the one rule
	// that exists to keep a second copy of the number off them.
	alts := make([]string, 0, 201)
	for n := 199; n >= 0; n-- { // descending, so `forty-nine` is tried before `forty`
		alts = append(alts, regexp.QuoteMeta(twdSpelled(n)))
	}
	alts = append(alts, `\d+`)
	num := `(?:` + strings.Join(alts, "|") + `)`
	// Plain words only in the gap: a token carrying `.` or `|` would let the
	// match cross a sentence end or a table cell wall.
	gap := `(?:\s+[A-Za-z][A-Za-z'-]*){0,2}`
	return regexp.MustCompile(`(?i)\b(?:` + num + gap + `\s+(?:pins|doors)|all\s+` + num + `)\b`)
}

// twdVarOf names the door variable that carries a test, for the message above.
func twdVarOf(t *testing.T, makefile, name string) string {
	t.Helper()
	for _, v := range twdPinVars {
		for _, n := range twdVar(t, makefile, v) {
			if n == name {
				return v
			}
		}
	}
	return "unknown"
}
