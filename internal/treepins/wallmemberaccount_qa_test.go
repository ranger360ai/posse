package treepins

// QA census for ranger-base-s9h52 — EVERY wall-check member spends its one
// line on a cause, read as a census over the member set rather than one
// member at a time.
//
// THE CLASS, THREE TIMES BY HAND BEFORE THIS. `scripts/posse-wall-check`
// reports a failed member by `evidence()`: the first `file.go:NN:` line of
// that test's output which is not the venue marker, truncated to 300
// characters, and nothing after it (ADR 0097 D3). The launcher puts that one
// line on a P1 bead. So a member's budget is ONE line, and the two ways to
// spend it on nothing are:
//
//	ranger-base-a688i  a format string ending `:\n%s` whose `%s` is the only
//	                   account of WHY. The bead read `the seat ended before it
//	                   ran (exit 1):` and stopped there.
//	ranger-base-axt1f  a runner that reduces its child's error to `err == nil`
//	                   before any message is formatted. The exit status then
//	                   exists as text nowhere, so `exit status 128` and
//	                   `signal: killed` report identically and NO line budget
//	                   brings the difference back.
//
// Each of those three beads pinned ONE NAMED FILE, which is exactly why
// nothing read the other members: a named read is nobody's census. This pin
// reads the whole set.
//
// WHY THE RULE IS NARROW, AND WHAT IT IS NOT (the decision ranger-base-s9h52
// was filed to make: a priced exemption table, or a narrower rule). It is a
// NARROWER RULE, and there is no exemption table anywhere in this file.
//
// The wide rule — ranger-base-axt1f's own, which that bead applied to its one
// file — is "no format string in a member file carries a newline". MEASURED
// 2026-11-21 over the member set: 785 testing format calls in 24 member
// files, 204 of them carrying a newline. Narrowing it by PROSE SHAPE does not
// converge either: "the newline defers and the first line carries no format
// verb at all" reads 44, and those 44 still mix the class with messages whose
// first line is a complete claim about an artifact the reader can go and read
// (a render's argv, a verdict file, a bead's text). Pricing 44 rows by hand is
// a register that drifts, and every row of it would be a judgement about
// someone else's prose.
//
// So this census narrows by SUBJECT instead: it reads the functions that
// launch a bwrap SEAT, and the messages that carry a seat's account. That is
// the one place where the deferred value is the ONLY account of why — a
// child's status and words, unreproducible after the fact — and it is what
// both of the beads above actually hit. A message whose tail is an artifact
// the reader can re-read is out of scope and needs no row saying so, because
// the rule never reaches it.
//
// WHAT A LAUNCH IS SPELLED LIKE, which this census got wrong once
// (ranger-base-hhps5, found by ranger-base-t0ha1's verify). The key read the
// functions that hand `exec.Command`/`CommandContext` a LITERAL `"bwrap"`
// ARGUMENT, and the prose above states no such limit — so a seat launched
// through a shell or a pty wrapper, with the word inside the command STRING,
// was invisible, and the `launchers` floor could not say so: an invisible
// launcher does not lower the count of the visible ones. MEASURED 2026-11-22
// at 2866fdf4 by a planted member file and its control, the same probe twice
// and the only difference the spelling of the argv —
// `exec.Command("bwrap", "--dev-bind", "/", "/", "/bin/true")` RED,
// `exec.Command("sh", "-c", "bwrap --dev-bind / / /bin/true")` GREEN. That
// control is what made the green a reading: arm 1 did see a planted launcher.
// The key is now the bwrap command WORD wherever it reaches the call's
// arguments, one hop included (wmaSeatLaunches), and there were already two
// such launches in the member set: grokleader_qa_test.go's pty arm, which
// joins the render's argv into a line for `script -qfec`, and
// wallunixlive_test.go's `sh -c 'bwrap "$@"; :'` arm. Both render their
// seat's error, so the widening reds nothing — it closes the route the next
// one would have escaped by. Arm 3 carries the planted pair and its negative
// cases (`bwraptest`, `bwrap-wrapper`, `bwrap.sh` and a skip message that
// says the word are none of them launches); arm 1 carries a floor on the
// wrapper route.
//
// THE OVER-READ DIRECTION IS ARM 3'S, AND ONLY ARM 3'S (ranger-base-8752f,
// from ranger-base-udagn's verify of the widening). A ceiling on the wrapper
// count — `len(wrapped) > 8` — used to sit beside that floor here, on the
// reasoning that widening a key is how a census starts grading functions that
// launch nothing. It could not perform the detection its own message claimed.
// MEASURED 2026-11-22 at 330d6940, one mutant at a time: the whole ceiling
// block deleted reds nothing, and `wmaSeatWord` rewritten as a
// `strings.Contains` — the canonical over-read, and exactly the shape the
// ceiling's message described — still reads 2 of 32 through the wrapper route,
// the same count as the unmutated key. It cannot: the neighbour names that an
// over-reading key would newly match (`bwraptest`, `bwrap-wrapper`,
// `bwrap.sh`) live in arm 3's fixtures and in no member file, so no value of
// that bound distinguishes the two keys over this tree. The SAME mutant reds
// arm 3's neighbour row, and nowhere else — so the claim is held hermetically,
// by the arm this comment already called "the precise half of this", and the
// census logs its own counts instead of asserting a bound nothing witnesses.
//
// MEASURED 2026-11-21 (linux 7.2.15, go1.27.3, this tree): 24 member files,
// 30 bwrap-launching functions in them, 2 of which rendered no error at all —
// `bsLive.in` (bwrapseatstorelive_test.go) and `srLive.in`
// (storerepogitwalllive_test.go), byte-identical copies of the two runners
// ranger-base-axt1f fixed — and 19 messages reading their accounts after a
// newline. All of those are fixed in the landing that adds this file.
// RE-MEASURED 2026-11-22 (same box, go1.27.3) with the widened key: 24 member
// files, 32 launching functions, 31 of them handing a literal `bwrap` argv
// entry and 2 taking the wrapper route (one does both), 33 child errors
// graded. The +1 over the argv-entry reading is grokleader_qa_test.go's
// function, which this census could not see at all before.
//
// THE LINE RULE IS POSITIONAL, AND THE ACCOUNT KEY READS THE FOLD
// (ranger-base-momix, from ranger-base-0b6zo's verify of ranger-base-mklu0).
// Arm 2 used to say NO NEWLINE AT ALL in a message carrying a seat's account,
// and its report said of a message that had one that "the account is dropped
// in transit". Both were wrong, and the only reason the tree was green is that
// the key read almost nothing: wmaAccountLocals recognised an account solely
// as a local assigned from a multi-return seat-LAUNCHING FuncDecl whose last
// result is a string, which is 3 of the 24 member files — and NOT
// internal/posse/wallclaudeprojectsattack_qa_test.go, the file where this
// class has now been found twice (ranger-base-eheo3's close, then
// ranger-base-mklu0), because its runner is a closure with one result and its
// account comes from wwOneLine and from an inline bgSeatAccount(...) argument.
// Neither spelling is an Ident assigned from a FuncDecl, so arm 2 read
// neither.
//
// What `scripts/posse-wall-check`'s evidence() carries is the FIRST
// `file.go:NN:` line and nothing after it (ADR 0097 D3) — so a newline costs
// the account nothing when the account is in FRONT of it, and D3 and
// ranger-base-mklu0's own fix shape both prescribe exactly that: the account
// first, the cwd and the reasoning on line 2, "where D3 says supporting prose
// costs nothing" (docs/notes.d/ranger-base-mklu0.md).
//
// MEASURED 2026-11-22 at e5221de6, which is what decided this and not the
// prose: with the key widened to read the fold, 8 of 24 member files carry an
// account and 65 messages read one — against 3 files and 32 messages for the
// old key on that same tip, so the widening roughly doubles what arm 2 grades.
// FOUR of those 65 carry a newline, and every one of them puts the account in
// front of it — wallclaudeprojectsattack_qa_test.go:386, the line mklu0
// landed on purpose, and wallunixlive_test.go:237, :526 and :1130, whose
// `the seat exited badly: %s\nargv: bwrap %s` is three more of the same shape
// from a different lineage. It is SIX at 814776ed (MEASURED 2026-11-22,
// ranger-base-yem73): ranger-base-72z36 (a25855a0) moved two more accounts in
// front of their own newline — wallclaudeprojects_qa_test.go:445 and
// wallclaudeprojectsattack_qa_test.go:403 — which is the same shape a third
// time. Arm 2's own Logf names the current set, and that is the reading to
// take; this paragraph is e5221de6's, the tip that decided the rule. NOTHING
// in the member set puts an account after a newline, at either tip. So the old
// rule was never a strictness the tree was paying for: it was a rule that
// could only red the shape the house prescribes, held off by a key too narrow
// to reach any of the four — and the three files it did reach folded
// everything onto one line for a rule the house does not hold. The two
// readings could not both stand, and that is why deciding was the same edit.
//
// AND THE BOTH-WAYS READING, nine mutants one at a time (MEASURED 2026-11-22,
// docs/notes.d/ranger-base-momix.md carries the battery and its output): the
// unmutated arms green; a graded account moved behind a newline reds with that
// file and line; the rule re-tightened to "no newline at all" reds all four
// deferred-prose messages AND the floor that witnesses them, which is the
// decision above taken twice; the key narrowed back to the runner route alone
// reds ten rows, the two route floors among them; the fold dropped from the
// vocabulary, renamed, given a second result, or no longer called by
// bgSeatAccount each reds the arm that owns it; and the four deferred messages
// folded onto one line red the witness floor and nothing else.
//
// THE FOURTH ROUTE IS A PARAMETER (ranger-base-x1ofd, from ranger-base-dxaxt's
// reading of the key while it added arm 6). The key read three routes and the
// member set used four: an account that arrives as a PARAMETER of the function
// that renders it is bound by no `:=` anywhere, and both local maps record only
// locals ASSIGNED from something, so neither could ever hold a parameter name.
// The site is internal/posse/bwrapgitrefslive_test.go's bgSeatAbstainAccount —
// the one spelling of ADR 0097 D3's exit 3, whose `t.Skipf("NOTHING MEASURED:
// %s — %s. …", account, what)` renders its own second parameter — and it was
// graded by nothing at all, by arm 2 or by arm 6. That is ranger-base-momix's
// shape one route further on, and D3 reads an abstention's line with the same
// rule as a failure's (evidence()'s own docstring says so), so an abstention's
// account is spent out of the same 300.
//
// MEASURED 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3) at 814776ed with the
// route added: 24 member files, 8 carrying an account, 66 account messages —
// 32 runner local, 8 folded local, 25 inline fold, and ONE parameter, which is
// bgSeatAbstainAccount's line and nothing else. Arm 2's per-route floors gain
// a row for it and arm 6 grades it as the 66th of 66. The route is read at the
// CALL SITE (wmaParamAccounts): a call of a function declared in the same
// file, whose argument at position i is an account by one of the three routes
// above, makes that function's i-th parameter an account. Nothing keys on the
// name `account`, and the parameter's declared type must be `string`, which is
// what an account is by construction — without that the route propagated one
// existing over-read a hop further, binding wallwatchlive_test.go's int `code`
// because the same file folds an account into a local named `got` and that name
// is also its watcher's exit code.
//
// AND ITS PREFIX IS ALREADY CHARGED HONESTLY, which is why this route lands
// with no arithmetic of its own. That message IS rendered inside a t.Helper()
// — bgSeatAbstainAccount's whole chain is, with 52 call sites at this tip
// (MEASURED 2026-11-22 at 814776ed) — so it is the NINTH of the nine messages
// the paragraph below charges `wallwatchbackstopwiring_linux_test.go:NNNN: `
// (44) for the caller `go test` prints. ranger-base-x1ofd decided that reading
// for this one message and ranger-base-q41e8 landed it for the other eight
// before the route arrived, so the 14-character under-read x1ofd priced
// against it — 30 charged where 44 is honest — exists in this tree nowhere.
// MEASURED at this tip: 44 of prefix plus 18 of literal preamble is a head of
// 62, the NARROWEST of the nine (the rest run 91 to 124) against the 267 a
// 33-character account leaves, and the widest head over the whole set is 209
// at internal/posse/bwrapgitrefslive_test.go:624 — the same figure and the
// same line as before this route landed. The route moves no headroom.
//
// AND THE BOTH-WAYS READING, seven mutants one at a time (MEASURED 2026-11-22
// at 814776ed, docs/notes.d/ranger-base-x1ofd.md carries the battery and its
// output): the unmutated arms green; the route dropped from wmaAccountArg reds
// the parameter floor and arm 3's two positive rows, and arm 2 then reads the
// 65 this tree read without it; the string-type reading dropped reds arm 3's
// non-string row AND its variadic row, and NOTHING over the tree, which is why
// the narrowing is held hermetically and why a variadic bail of its own is not
// in wmaParamAt; the blank reading dropped reds the blank row; one hop widened
// to two reds the second-hop row — in ONE of its two source orders, which is
// why there are two of them; the graded line's own account moved behind a
// newline reds arm 2 at bwrapgitrefslive_test.go:485, while the SAME mutation
// with the route dropped again reds that line nowhere, which is what the route
// is for — the line could have lost its account to a newline at any time and
// the census would have said nothing. The seventh is this landing's own:
// wmaRenderedInHelper answering "not a helper" everywhere reds arm 7's five
// readings with arm 6 GREEN, so the 66th message does not make this tree
// witness the prefix term either, and the term stays where
// ranger-base-q41e8's mutant 8 put it.
//
// AND THE OTHER HALF OF ranger-base-mklu0 IS ARM 6: the 300 characters.
// evidence() truncates that first line with the `file.go:NN: ` prefix inside
// the 300, so an account that is on line 1 but behind a long preamble is cut
// anyway — mklu0's finding 2 exactly, 261 of the 300 spent before the account.
// Arm 2 does not read it, and for a while nothing did: a bound on the LITERAL
// preamble looked useless, because 46 of mklu0's 261 bytes were an
// interpolated cwd whose width is the box's and a bound over literals would
// have passed the one message this class had cost.
//
// WHAT CHANGED IS THE OTHER TERM (ranger-base-uu303's reading,
// ranger-base-dxaxt's landing). The rule is not a bound on the preamble at
// all: it is `prefix + preamble + the account's own fixed width <= the budget`,
// every term read out of the tree, and the account's width was 199 characters
// of REASONING that one edit to one function gave back. At the old tail the
// derived budget was 95 and arm 6 red 28 of 65; at today's 27-character tail
// it is 267 and arm 6 reds nothing, with 58 characters of headroom on the
// widest HEAD it charges: 209 at internal/posse/bwrapgitrefslive_test.go:624,
// which is 30 of go test's own prefix plus a 179-character literal preamble
// (MEASURED 2026-11-22 at 814776ed — the same 209 at :598 at 7dc3313e, where
// ranger-base-dxaxt read it, and ranger-base-72z36's 26 added lines are what
// moved it; and the figure the arm's own Logf prints — read that, not this
// line). Until ranger-base-q41e8 this sentence read "34 … on the widest
// literal preamble in the set (233, bwrapseatstorelive_test.go:751)" and was
// wrong twice over: 233 was ranger-base-uu303's and ranger-base-dxaxt's
// reading of a row that ranger-base-bj1hh then shortened out of the
// account-carrying set altogether — a figure carried across the landing that
// falsified it — and it named the preamble alone while the headroom beside it
// was computed from prefix PLUS preamble, which is the sum the rule actually
// bounds.
// So ORDER WITHIN THE FIRST LINE IS PINNED, as a NECESSARY condition and not a
// sufficient one: three member messages blow the 300 on interpolated fixture
// paths no static census can see (ranger-base-bj1hh), and sufficiency needs
// each member's messages rendered under its fixture and sliced as evidence()
// does — a venue ranger-base-uu303 records rather than files, so the next attempt does not re-try a static rule.
//
// AND THE PREFIX TERM IS CHARGED TO THE FILE go test WILL PRINT, which is not
// always the file the message is written in (ranger-base-x1ofd's decision,
// ranger-base-q41e8's landing). testing's decorate() writes the first
// NON-helper frame, so a message rendered inside a t.Helper() is decorated
// with the CALLING file and its line, and a term charging the helper's own
// basename charges a file that never appears on the line. The member set's
// sharpest case is bgSeatAbstainAccount's `NOTHING MEASURED` line, ADR 0097
// D3's exit 3, whose whole chain calls t.Helper() (bgSeatAbstain,
// bgSeatAbstainIfOutside, wwAbstainIfOutside) with 52 call sites between the
// two doors at 7dc3313e — so there is no one caller to read, and the decision
// is to charge the WIDEST basename in the member set, derived from the set the
// way wmaMemberPattern derives the set itself (wmaRenderedInHelper picks the
// case, wmaHelperPrefix charges it). It is the strictest honest reading, and
// it keeps the rule a FLOOR either way: a line that fits at its own caller and
// not at the widest is still one caller away from being cut, which is the
// direction every other floor in this file points.
//
// AND IT WAS NEVER ONE MESSAGE. The decision was taken for the one line
// ranger-base-x1ofd's parameter route adds, on the reading that nothing arm 6
// graded was in such a helper — which is what wmaLinePrefix's docstring said
// too. RE-READ 2026-11-22 at 814776ed (ranger-base-yem73), the tip this
// paragraph's figures are now taken at: EIGHT of the 65 messages arm 6 graded
// BEFORE that route are rendered inside a t.Helper(), so the term was charged
// to the wrong file eight times over before the route that raised the question
// existed — NINE of 66 with the route in the tree, the ninth being its own and
// priced in the paragraph above, which is the count arm 6's Logf prints and
// the number to read. Four of the eight are named helpers (wallwatchlive_test.go's wwWaitKilled at
// :300 and :303 and launchOn at :139 and :143), one is a fixture helper
// (bwrapgitrefslive_test.go's packUnderALiveSeat at :867), and three are
// closures that call t.Helper() themselves
// (wallclaudeprojectsattack_qa_test.go:386 and :403,
// wallwatchlive_test.go:604). It reds nothing: the widest of the eight is
// wwWaitKilled's line at wallwatchlive_test.go:303, which the widest basename
// moves from 110 to 124 of the same 267 — and the widest head in the set is
// not one of the eight at all (209 at bwrapgitrefslive_test.go:624, charged
// its own 30), so this term moves no headroom anywhere.
//
// THOSE LINE NUMBERS WERE READ ONCE AT 7dc3313e AND LANDED AFTER THE LANDING
// THAT MOVED THEM, which is this file's own warning one paragraph up taken a
// second time (ranger-base-x1ofd read them, ranger-base-q41e8 landed them,
// ranger-base-yem73 found them). ranger-base-72z36 (a25855a0) landed BEFORE
// q41e8 (6706f7c0): its 26 added lines in bwrapgitrefslive_test.go moved
// packUnderALiveSeat's message from :841 to :867, and its third argument swap
// put attack's Logf account in front of that message's newline, which moved
// :389 to :403 AND collapsed the head this paragraph called the widest from
// 206 to 82. So every figure in the sentence above was false the day it
// landed, including the one — attack:389's 206 to 209 — that was a true
// reading of the tree it was taken on. A figure read at one tip and typed at
// another is this file's recurring cost, and what it buys is nothing: arms 2
// and 6 now LIST these messages by file and line in their own Logf lines — the
// deferred set and the helper-rendered set, the two figures this bead was filed
// about — so the reading is a `-v` run away and no paragraph here has to be
// the record of one.
//
// AND THE TERM IS STILL WITNESSED HERMETICALLY AND NOT BY THE TREE, for
// ranger-base-dxaxt's own reason one reading on (its mutant 8: the whole
// prefix replaced by a constant 0 and arm 6 stays green). A mutant answering
// "not a helper" everywhere charges those eight 27 to 41 where the decision
// charges 44, and arm 6 is green either way — so the reading and the
// arithmetic are both held in arm 7 over planted source, including the
// distinction that decided the reading: a closure inside a helper is its own
// frame unless it calls t.Helper() itself, and the sloppier "a Helper() call
// anywhere under the enclosing FuncDecl" reads nine of the 65 rather than
// eight, the ninth (wallwatchlive_test.go:614) written in a test body beside
// a closure that is a helper.
//
// IT ENUMERATES A DISCOVERED FILE SET (the Glob below), so it is in this
// package's one-package-census class and is doored by `make treepins-check` —
// the complement door, which is this package minus the priced table in
// treewidedoor_qa_test.go's `twdTreepinsExcluded` (ADR 0098 D1). It is in no
// such table, so it needs no hand-written Makefile entry and cannot go
// undoored (ADR 0098 D4).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// wmaCheck is the wall check, whose `member_pattern` is the only definition
// of the member set. Read, never retyped: a census whose subject set is a
// second spelling of that pattern is a census that can disagree with the
// thing it exists to serve.
const wmaCheck = "scripts/posse-wall-check"

// wmaPkgGlob is the discovered file set. internal/posse is the one package
// the check runs (`-run <member_pattern> ./internal/posse`).
const wmaPkgGlob = "internal/posse/*_test.go"

// wmaAccountFunc is the one spelling of a seat's one-line account
// (bwrapgitrefslive_test.go, ranger-base-axt1f): the exit status, folded,
// then the seat's words folded onto the same line by wwOneLine.
const wmaAccountFunc = "bgSeatAccount"

// wmaFoldFunc is the fold both accounts share, and the one a caller uses
// alone when there is no status to carry — a seat still running at its
// deadline, whose words are the whole account (ranger-base-mklu0).
const wmaFoldFunc = "wwOneLine"

// wmaAccountFolds is arm 2's account vocabulary, and it is two names because
// the package says it is two: wwOneLine (wallwatchlive_test.go) is the fold,
// bgSeatAccount (bwrapgitrefslive_test.go) is the account built on it, and
// that file's own head comment is why there is no third — "THE FOLD IS
// wwOneLine's AND NOT A SECOND SPELLING OF IT … two foldings of 'put this on
// one line' are two answers waiting to disagree about what a member's line may
// contain". A typed pair of names is the register this file refuses
// everywhere else, so arm 5 reads the pair BACK out of the member set rather
// than trusting this map.
var wmaAccountFolds = map[string]bool{wmaAccountFunc: true, wmaFoldFunc: true}

// wmaWaiters are the Cmd methods that produce the child's error. The error a
// bwrap-launching function must render is the one THESE return, and not
// merely some error somewhere in the body — a function that reports a
// fixture's failure and swallows its seat's is the shape this pin is for.
var wmaWaiters = map[string]bool{
	"CombinedOutput": true, "Output": true, "Run": true, "Start": true, "Wait": true,
}

// wmaSayers are the calls that put a value where a reader can see it. A
// testing method is matched by NAME on any receiver, because a fixture's
// helper takes `t` under whatever name it likes.
var wmaSayers = map[string]bool{
	"Fatal": true, "Fatalf": true, "Error": true, "Errorf": true,
	"Log": true, "Logf": true, "Skip": true, "Skipf": true,
	"Sprintf": true, "Sprint": true, "Errorf_": true,
	wmaAccountFunc: true,
}

// wmaFormatVerbs are the format-taking testing methods: the calls whose first
// argument is the format string a reader meets through evidence().
var wmaFormatVerbs = map[string]bool{"Fatalf": true, "Errorf": true, "Logf": true, "Skipf": true}

var wmaVerbRe = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z]`)

// wmaUnreadableVerbRe matches the two format shapes whose argument POSITIONS
// cannot be read off the text: an explicit argument index (`%[2]s`) reorders
// them, and a star width (`%*d`) consumes one more argument than it looks
// like. Neither occurs in a member message today, so arm 2 holds such a
// message to the stricter rule and says so rather than guessing a position.
var wmaUnreadableVerbRe = regexp.MustCompile(`%[-+# 0-9.]*(\[[0-9]+\]|\*)`)

// wmaVerbs counts the argument-consuming verbs in a stretch of format string.
// `%%` is a literal percent and consumes nothing, and it is removed first
// because wmaVerbRe's flag class contains a space — `100%% done` would
// otherwise read as one verb.
func wmaVerbs(s string) int {
	return len(wmaVerbRe.FindAllString(strings.ReplaceAll(s, "%%", ""), -1))
}

// wmaAccountBeforeNewline answers, for a format string and the 0-based index
// of the account among the arguments that follow it, whether the account is
// rendered BEFORE the first newline — which is the whole of what reaches a
// bead (ADR 0097 D3, and see the head comment for why this is the rule and
// "no newline at all" was not). `readable` is false when the text uses a verb
// whose positions cannot be read; the caller then holds the message to the
// stricter rule. firstLine is what evidence() would carry.
func wmaAccountBeforeNewline(text string, argIdx int) (before, readable bool, firstLine string) {
	i := strings.IndexAny(text, "\n\r")
	if i < 0 {
		return true, true, text
	}
	if wmaUnreadableVerbRe.MatchString(text) {
		return false, false, text[:i]
	}
	return argIdx < wmaVerbs(text[:i]), true, text[:i]
}

// wmaMemberPattern reads the member pattern out of the wall check. One line,
// one literal, and the check's own comment says it is written once there.
func wmaMemberPattern(t *testing.T) *regexp.Regexp {
	t.Helper()
	b, err := os.ReadFile(wmaCheck)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^member_pattern='([^']+)'$`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s no longer carries a `member_pattern='…'` line, so this census cannot read the set it is a census of. That literal is the check's own definition of a member (ADR 0097 D4) and this pin reads it rather than retyping it.", wmaCheck)
	}
	re, cerr := regexp.Compile(string(m[1]))
	if cerr != nil {
		t.Fatalf("%s's member_pattern %q does not compile in Go: %v", wmaCheck, m[1], cerr)
	}
	return re
}

// wmaFile is one parsed member file.
type wmaFile struct {
	path string
	fset *token.FileSet
	ast  *ast.File
}

// wmaMemberFiles is the census's subject: every file in the package the check
// runs that declares at least one member. A Glob and not a list, which is
// what puts this pin in the one-package-census class.
func wmaMemberFiles(t *testing.T) []wmaFile {
	t.Helper()
	re := wmaMemberPattern(t)
	paths, err := filepath.Glob(wmaPkgGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("%s matched no files — this census has nothing to read, so every arm below would pass for the wrong reason", wmaPkgGlob)
	}
	var out []wmaFile
	for _, p := range paths {
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", p, perr)
		}
		member := false
		for _, fn := range wmaDecls(f) {
			if fn.Recv == nil && re.MatchString(fn.Name.Name) {
				member = true
			}
		}
		if member {
			out = append(out, wmaFile{path: p, fset: fset, ast: f})
		}
	}
	return out
}

func wmaDecls(f *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			out = append(out, fn)
		}
	}
	return out
}

// wmaSeatRe is the one spelling of a bwrap launch, read as a command WORD so
// that a path to the binary counts and a neighbour whose name merely carries
// the letters does not: `bwrap` and `/usr/bin/bwrap` match, `bwrap-wrapper`,
// `bwrap.sh` and `wrcNeedBwrap` do not.
var wmaSeatRe = regexp.MustCompile(`(?:^|[^\w.-])bwrap(?:$|[^\w.-])`)

// wmaSeatWord reports whether s names the bwrap binary as a command word.
func wmaSeatWord(s string) bool { return wmaSeatRe.MatchString(s) }

// wmaStringLit unquotes a string literal expression.
func wmaStringLit(x ast.Node) (string, bool) {
	lit, ok := x.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// wmaExprSeatWord reports whether any string literal anywhere under x names
// bwrap as a command word — so a line spelled across a `+` is read too.
func wmaExprSeatWord(x ast.Expr) bool {
	found := false
	ast.Inspect(x, func(n ast.Node) bool {
		if s, ok := wmaStringLit(n); ok && wmaSeatWord(s) {
			found = true
		}
		return true
	})
	return found
}

// wmaExprMentions reports whether x reads any of these names.
func wmaExprMentions(x ast.Expr, names map[string]bool) bool {
	found := false
	ast.Inspect(x, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return true
	})
	return found
}

// wmaSeatWordNames are the names this function builds a bwrap command word
// INTO: `line := "bwrap …"`, `q = append(q, "bwrap")`, `argv := []string{"bwrap", …}`.
// ONE HOP, the same depth arm 2's account read uses, and it is what lets a
// seat reach the call in a variable — grokleader_qa_test.go's pty arm joins
// its argv into a line before handing it to `script -qfec`.
func wmaSeatWordNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	record := func(lhs, rhs []ast.Expr) {
		carries := false
		for _, r := range rhs {
			if wmaExprSeatWord(r) {
				carries = true
			}
		}
		if !carries {
			return
		}
		for _, l := range lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
				names[id.Name] = true
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return names
}

// wmaSeatLaunches reports HOW this function starts a bwrap seat, and the two
// answers are kept apart because one of them is this census's own blind spot
// made visible (ranger-base-hhps5):
//
//	direct   an exec.Command/CommandContext argument is the literal "bwrap"
//	         — the argv entry, which is all this key read before.
//	wrapped  the word reaches the call some OTHER way: inside a command LINE
//	         for a shell or a pty wrapper, or one hop away in a name the
//	         function built the line in. A launch arm 1 could not see at all.
//
// Either is a launch. `wrapped` is counted so the floor below can say the
// tree still has one, because a floor on the visible launchers cannot: an
// invisible launcher does not lower the count of the visible ones.
func wmaSeatLaunches(fn *ast.FuncDecl) (direct, wrapped bool) {
	names := wmaSeatWordNames(fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
			return true
		}
		for _, a := range c.Args {
			if s, ok := wmaStringLit(a); ok {
				switch {
				case s == "bwrap":
					direct = true
				case wmaSeatWord(s):
					wrapped = true
				}
				continue
			}
			if wmaExprSeatWord(a) || wmaExprMentions(a, names) {
				wrapped = true
			}
		}
		return true
	})
	return direct, wrapped
}

// wmaLaunchesSeat reports whether this function starts a bwrap seat, by
// either route.
func wmaLaunchesSeat(fn *ast.FuncDecl) bool {
	direct, wrapped := wmaSeatLaunches(fn)
	return direct || wrapped
}

// wmaChildErrs are the names this function binds the SEAT's error to: the
// error result of a Cmd waiter. Those are the names that have to reach a
// reader.
func wmaChildErrs(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		c, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !wmaWaiters[sel.Sel.Name] {
			return true
		}
		// The error is the LAST result of every waiter in wmaWaiters.
		if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok && id.Name != "_" {
			names[id.Name] = true
		}
		return true
	})
	return names
}

// wmaSaid reports whether name is handed to something that shows it.
func wmaSaid(fn *ast.FuncDecl, name string) bool {
	said := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := ""
		switch f := c.Fun.(type) {
		case *ast.Ident:
			callee = f.Name
		case *ast.SelectorExpr:
			callee = f.Sel.Name
			// `err.Error()` is a rendering of err all by itself.
			if f.Sel.Name == "Error" {
				if id, ok := f.X.(*ast.Ident); ok && id.Name == name {
					said = true
				}
			}
		}
		if !wmaSayers[callee] {
			return true
		}
		for _, a := range c.Args {
			if id, ok := a.(*ast.Ident); ok && id.Name == name {
				said = true
			}
		}
		return true
	})
	return said
}

// wmaAccountLocals are the locals a call site binds to a seat runner's
// account — the LAST string result of a function that launches a seat. One
// hop, which is every spelling the member set uses and the same depth
// treewidedoor_qa_test.go's parameter hop reads.
func wmaAccountLocals(f wmaFile) map[string]bool {
	runners := map[string]bool{}
	for _, fn := range wmaDecls(f.ast) {
		if !wmaLaunchesSeat(fn) || fn.Type.Results == nil {
			continue
		}
		var res []string
		for _, r := range fn.Type.Results.List {
			n := 1
			if len(r.Names) > 1 {
				n = len(r.Names)
			}
			name := "?"
			if id, ok := r.Type.(*ast.Ident); ok {
				name = id.Name
			}
			for i := 0; i < n; i++ {
				res = append(res, name)
			}
		}
		if len(res) >= 2 && res[len(res)-1] == "string" {
			runners[fn.Name.Name] = true
		}
	}
	locals := map[string]bool{}
	if len(runners) == 0 {
		return locals
	}
	ast.Inspect(f.ast, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 || len(as.Lhs) < 2 {
			return true
		}
		c, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := ""
		switch fun := c.Fun.(type) {
		case *ast.Ident:
			callee = fun.Name
		case *ast.SelectorExpr:
			callee = fun.Sel.Name
		}
		if !runners[callee] {
			return true
		}
		if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok && id.Name != "_" {
			locals[id.Name] = true
		}
		return true
	})
	return locals
}

// wmaCallee names the function a call expression calls, by its last
// identifier: a package-level func, a method, a field holding a closure.
func wmaCallee(c *ast.CallExpr) string {
	switch fun := c.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// wmaIsFoldCall reports whether x is a call of one of the account folds.
func wmaIsFoldCall(x ast.Expr) bool {
	c, ok := x.(*ast.CallExpr)
	return ok && wmaAccountFolds[wmaCallee(c)]
}

// wmaFoldLocals are the locals a call site binds to a FOLDED account, which is
// the route wmaAccountLocals cannot see and the one the twice-found file uses
// (ranger-base-momix): the account is produced by wwOneLine or bgSeatAccount
// rather than returned by a seat runner this census can recognise as one.
// Reading the fold instead of the runner is what makes the key independent of
// how the seat was launched — a closure, a one-result helper, a bare inline
// call — and the runner route stays beside it because a runner that returns
// its account unfolded is still an account.
//
// ONE HOP, the same depth as every other reading in this file. An account the
// fold produces three assignments away is not read, and the floors in arm 2
// are what notice a route going quiet.
func wmaFoldLocals(f wmaFile) map[string]bool {
	locals := map[string]bool{}
	record := func(lhs, rhs []ast.Expr) {
		for i, r := range rhs {
			if i >= len(lhs) || !wmaIsFoldCall(r) {
				continue
			}
			if id, ok := lhs[i].(*ast.Ident); ok && id.Name != "_" {
				locals[id.Name] = true
			}
		}
	}
	ast.Inspect(f.ast, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return locals
}

// wmaParamAt names the parameter at the 0-based ARGUMENT position i, with a
// grouped list flattened the way it is written: `what, account string` is two
// parameters and not one, which is the spelling the site this route exists
// for actually uses. An unnamed or blank parameter answers "", because it
// binds nothing a message could read.
//
// IT ANSWERS "" FOR A PARAMETER THAT IS NOT A `string`, which is a
// narrowing and was MEASURED to be the one the tree needed (2026-11-22, linux
// 7.2.15-1-omarchy, go1.27.3, at 79b7bf11). An account is a string by
// construction — bgSeatAccount and wwOneLine both return one — so a parameter
// of any other type cannot be holding one, and the type is read off the
// declaration rather than guessed from the name. Without it this route bound
// wallwatchlive_test.go's `code`, an int: the three routes above are file-wide
// NAME sets, that file folds an account into a local named `got`, and the same
// name is its watcher's exit code at `wwSeatAccount(got, said)` — so a
// name-only reading propagates that one over-read a hop further, into a
// parameter no account can reach. The route reads the one site either way; the
// type is what keeps it from reading more than it can defend.
//
// THE SAME READING ANSWERS A VARIADIC, and there is no second branch for one:
// a `...string` parameter's type is an *ast.Ellipsis and not an Ident, so the
// line below already refuses it — what is bound there is a slice and not the
// account. A bail of its own stood here and was taken out when the mutant for
// it reddened nothing (ranger-base-x1ofd), which is this file's own rule about
// a guard nothing witnesses (see the head comment on the wrapper ceiling,
// ranger-base-8752f). Arm 3 carries the variadic row, and it reds under the
// mutant that drops this one line.
func wmaParamAt(fn *ast.FuncDecl, i int) string {
	if fn.Type.Params == nil {
		return ""
	}
	at := 0
	for _, p := range fn.Type.Params.List {
		n := len(p.Names)
		if n == 0 {
			n = 1
		}
		for k := 0; k < n; k++ {
			if at == i {
				if id, ok := p.Type.(*ast.Ident); !ok || id.Name != "string" {
					return ""
				}
				if len(p.Names) == 0 || p.Names[k].Name == "_" {
					return ""
				}
				return p.Names[k].Name
			}
			at++
		}
	}
	return ""
}

// wmaParamAccounts are the PARAMETER names this file hands an account to —
// the fourth route, and the one the key read not at all (ranger-base-x1ofd).
// The two local maps record only locals ASSIGNED from something, so neither
// ever holds a parameter name, and the route the member set reaches
// bgSeatAbstainAccount's NOTHING MEASURED line by was graded by nothing.
//
// It is read at the CALL SITE and not off the parameter's name: a call of a
// function declared in this file, whose argument at position i is an account
// by one of the three routes above, makes that function's i-th parameter an
// account. Nothing here keys on `account`, `acct` or any other spelling —
// which is the same reason wmaMemberPattern reads the check instead of
// retyping it.
//
// ONE HOP, the same depth as every other reading in this file, and the param
// map is deliberately nil in the recursive position below: an account that
// reaches a parameter FROM another parameter is two hops and is not read. That
// also makes this reading independent of the order the walk meets the call
// sites in — with the fourth route fed back in, a two-hop chain would bind or
// not depending on which call ast.Inspect reached first. The floors in arm 2
// are what notice a route going quiet.
func wmaParamAccounts(f wmaFile, runner, folded map[string]bool) map[string]bool {
	params := map[string]bool{}
	decls := wmaDecls(f.ast)
	ast.Inspect(f.ast, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		// A spread reorders nothing but hides which parameter each argument
		// lands in, which is arm 2's own reason for refusing to guess.
		if !ok || c.Ellipsis.IsValid() {
			return true
		}
		callee := wmaCallee(c)
		if callee == "" {
			return true
		}
		for i, a := range c.Args {
			if _, _, isAccount := wmaAccountArg(a, runner, folded, nil); !isAccount {
				continue
			}
			for _, fn := range decls {
				if fn.Name.Name != callee {
					continue
				}
				if name := wmaParamAt(fn, i); name != "" {
					params[name] = true
				}
			}
		}
		return true
	})
	return params
}

// wmaAccountArg reports whether one argument of a format call IS a seat's
// account, and by which route — which is the whole of arm 2's key. The four
// routes are kept apart so arm 2 can floor each of them: a route that has
// gone quiet is a route a later narrowing could delete while the census stays
// green, which is ranger-base-hhps5's lesson one arm over.
func wmaAccountArg(x ast.Expr, runner, folded, param map[string]bool) (name, route string, ok bool) {
	if id, isID := x.(*ast.Ident); isID {
		switch {
		case runner[id.Name]:
			return id.Name, "runner local", true
		case folded[id.Name]:
			return id.Name, "folded local", true
		case param[id.Name]:
			return id.Name, "parameter", true
		}
		return "", "", false
	}
	if c, isCall := x.(*ast.CallExpr); isCall && wmaAccountFolds[wmaCallee(c)] {
		return wmaCallee(c) + "(…)", "inline fold", true
	}
	return "", "", false
}

// wmaFormat reconstructs a format string from a literal or a concatenation of
// literals, so a format spelled across a `+` is read too.
func wmaFormat(x ast.Expr) (string, bool) {
	switch e := x.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.ParenExpr:
		return wmaFormat(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		l, ok := wmaFormat(e.X)
		if !ok {
			return "", false
		}
		r, ok := wmaFormat(e.Y)
		if !ok {
			return "", false
		}
		return l + r, true
	}
	return "", false
}

// wmaMsg is one member message that interpolates a seat's account: where go
// test will print it, the format string, and which argument the account is.
type wmaMsg struct {
	file     string // the member file, as the Glob found it
	line     int    // the line go test prefixes the message with
	verb     string // the testing method
	text     string // the format string
	argIdx   int    // the account's 0-based place among the arguments
	name     string // how the account is spelled at this call site
	route    string // which arm of the key read it
	ellipsis bool   // the call ends in a variadic spread
	helper   bool   // rendered inside a t.Helper(), so go test prints the CALLER's file
	helperIn string // the innermost function containing it, for the report
}

// wmaAccountMessages is THE KEY, in ONE place: every format call in one member
// file that interpolates a seat's account, with the account's own argument
// position. Arm 2 reads which LINE the account lands on and arm 6 reads how
// many CHARACTERS are spent before it, both off this one walk — two spellings
// of "which messages carry an account" are two answers waiting to disagree
// about what this census grades, which is the register arm 5 refuses one layer
// down and the register this file's head comment refuses when it declines an
// exemption table.
//
// EVERY member file, and not only the ones that bind an account to a local: an
// inline fold IS an account, and a file that hands one straight to a verb binds
// nothing. Gating the walk on the locals was the narrowness arm 2 was found
// with one layer down — it hid wallunixlive_test.go's three deferred-prose
// messages, which are three quarters of what witnesses arm 2's rule
// (ranger-base-momix).
//
// AND THE FOURTH ROUTE IS READ HERE, in the one place, which is why that
// register matters (ranger-base-x1ofd): an account handed to a function as a
// PARAMETER is bound by no `:=` anywhere, so neither local map can hold it,
// and it reached the key not at all. Both arms read it the moment this walk
// does.
func wmaAccountMessages(f wmaFile) []wmaMsg {
	runner, folded := wmaAccountLocals(f), wmaFoldLocals(f)
	param := wmaParamAccounts(f, runner, folded)
	spans := wmaFuncSpans(f.ast)
	var out []wmaMsg
	ast.Inspect(f.ast, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) < 2 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !wmaFormatVerbs[sel.Sel.Name] {
			return true
		}
		text, ok := wmaFormat(c.Args[0])
		if !ok {
			return true
		}
		helper, helperIn := wmaRenderedInHelper(spans, c.Pos())
		for i, a := range c.Args[1:] {
			name, route, isAccount := wmaAccountArg(a, runner, folded, param)
			if !isAccount {
				continue
			}
			out = append(out, wmaMsg{
				file:     f.path,
				line:     f.fset.Position(c.Pos()).Line,
				verb:     sel.Sel.Name,
				text:     text,
				argIdx:   i,
				name:     name,
				route:    route,
				ellipsis: c.Ellipsis.IsValid(),
				helper:   helper,
				helperIn: helperIn,
			})
		}
		return true
	})
	return out
}

// wmaEvidenceRe reads the slice evidence() takes of a member's line. Read
// from the check and never retyped, for the reason wmaMemberPattern is: a
// budget this file typed is a budget that can disagree with the one the
// launcher actually puts on the bead.
var wmaEvidenceRe = regexp.MustCompile(`text\[:([0-9]+)\]`)

// wmaEvidenceBudget is how many CHARACTERS of a member's first line reach the
// bead. `evidence()` is Python and slices a `str` decoded from `go test
// -json`, so the budget is Unicode code points and not bytes — the member
// set's prose is em-dash heavy (one to three per message, 3 bytes each), so a
// rule here counting `len(string)` would over-read every message it grades
// (MEASURED 2026-11-22, ranger-base-uu303: the first pass of that reading counted bytes and put the widest message 2 too high). Arm 6
// counts runes everywhere.
func wmaEvidenceBudget(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(wmaCheck)
	if err != nil {
		t.Fatal(err)
	}
	ms := wmaEvidenceRe.FindAllStringSubmatch(string(b), -1)
	if len(ms) == 0 {
		t.Fatalf("%s no longer slices its evidence line as `text[:N]`, so this census cannot read the budget it is a census of. That slice is ADR 0097 D3's 300 characters and arm 6 reads it rather than typing it.", wmaCheck)
	}
	n, cerr := strconv.Atoi(ms[0][1])
	if cerr != nil || n <= 0 {
		t.Fatalf("%s's evidence slice reads %q, which is not a budget", wmaCheck, ms[0][1])
	}
	for _, m := range ms[1:] {
		if m[1] != ms[0][1] {
			t.Errorf("%s slices its evidence line to %s characters in one place and %s in another, so there is no one budget for arm 6 to read. evidence() has one rule for a failure line and for a skip line (its own docstring) and both halves must spend the same 300.", wmaCheck, ms[0][1], m[1])
		}
	}
	return n
}

// wmaSpentBefore is how many characters of a member's line a reader has spent
// before the account's own verb, counted over the FIRST line only — which is
// all evidence() carries — with every interpolated verb counted as ZERO. So
// the figure is a FLOOR: the literal preamble and nothing else. That is
// deliberate and it is the half ranger-base-uu303's reading says no static
// census can close: three member messages blow the 300 on interpolated fixture
// paths of 124 to 174 characters (ranger-base-bj1hh), and no reading of a
// format string can see a path. A floor is still the right NECESSARY
// condition, because a message whose LITERAL preamble already spends the
// budget spends it on every box there is.
//
// `%%` is a literal percent and consumes no argument, so it counts one
// character and not a verb — the same handling wmaVerbs gets by removing it
// first, done here by position because arm 6 needs the offset and not the
// count. A lone `%` that starts no verb is one literal character too.
func wmaSpentBefore(firstLine string, argIdx int) (spent int, ok bool) {
	rest, arg := firstLine, 0
	for {
		i := strings.IndexByte(rest, '%')
		if i < 0 {
			return 0, false
		}
		spent += len([]rune(rest[:i]))
		rest = rest[i:]
		if strings.HasPrefix(rest, "%%") {
			spent++
			rest = rest[2:]
			continue
		}
		m := wmaVerbRe.FindString(rest)
		if m == "" || !strings.HasPrefix(rest, m) {
			spent++
			rest = rest[1:]
			continue
		}
		if arg == argIdx {
			return spent, true
		}
		arg++
		rest = rest[len(m):]
	}
}

// wmaLinePrefix is what go test itself spends at the head of a member's line
// before one character of the message: `<basename>:<line>: `, which is
// testing's own decoration (its decorate() writes the BASENAME, the line and
// `": "`). evidence() keeps all of it, because `line.strip()` strips the
// indentation and nothing else — which is exactly how ranger-base-mklu0 spent
// 261 of its 300 before the account on a message whose own format string was
// shorter than that.
//
// THE BASENAME, because the check runs `go test` without `-test.fullpath`
// (its own pins_argv, MEASURED 2026-11-22) and testing's formatter is
// `filepath.Base(file)` under exactly that condition. A check that ever added
// that flag would make this whole term an under-read, by the length of the
// path in front of every member file.
//
// AND IT IS THE PREFIX FOR A MESSAGE RENDERED WHERE IT IS WRITTEN, which is a
// condition and not a simplification: testing's frame walk returns the first
// NON-helper frame (frameSkip, formatted by callSite — the `decorate` this
// file's older prose names, renamed upstream; go1.27.3 on this box), so a
// message rendered inside a `t.Helper()` carries the CALLING file and its line
// and this term would charge a basename that never appears on the line.
// wmaRenderedInHelper is which case a message is and wmaHelperPrefix is the
// other one's charge (ranger-base-q41e8). This docstring said "nothing arm 6
// grades is in such a helper today" until that bead measured it: eight of the
// 65 did, and the head comment names them. NINE of 66 do since
// ranger-base-x1ofd's fourth route, whose one message is itself rendered
// inside a t.Helper() — it is the narrowest head of the nine, MEASURED
// 2026-11-22 at 814776ed, and the arm's own Logf prints the count.
//
// It is a named reader and not two lines inside arm 6 because this tree does
// not witness it: MEASURED 2026-11-22 (mutant 8 of docs/notes.d/
// ranger-base-dxaxt.md's battery), with the whole term replaced by a constant
// 0 the member set still fits inside the budget and arm 6 stays green. A term
// no tree can red is held hermetically by the arm that drives the readers over
// planted input, which is ranger-base-8752f's ruling one arm over: say the
// claim where something can check it, and do not assert a bound nothing
// witnesses.
func wmaLinePrefix(file string, line int) int {
	return len([]rune(filepath.Base(file))) + len(":") + len(strconv.Itoa(line)) + len(": ")
}

// wmaHelperMethod is the one call that makes a function a helper frame, and it
// is matched by NAME on any receiver — the same way wmaSayers matches a
// testing method, because a fixture's helper takes its `testing.TB` under
// whatever name it likes (`t`, `tb`, a field).
const wmaHelperMethod = "Helper"

// wmaHelperLineDigits is the line-number width charged to a helper-rendered
// message, and the decision is FOUR (ranger-base-x1ofd). The decorated number
// is the CALLER's line in the calling member file, and the member set is a set
// of thousand-line files: MEASURED 2026-11-22 at 814776ed, the longest member
// file is wallunixlive_test.go at 1217 lines and two others are past 1000
// (wallclaudesessionstate_qa_test.go at 1200, bwrapgitrefslive_test.go at
// 1026), so a caller's line is three or four digits and four is the wider of
// them — and it stays four while the longest member file is any length between
// 1000 and 9999, which is why this reading is a sanity check and not the
// decision. It is a constant and not a reading of the files' own lengths
// because this term rides with the WIDEST BASENAME, which is already a
// different file from the widest caller — pairing the two would read as a
// precision this charge does not claim. A member file past 9999 lines is what
// would make it one short.
const wmaHelperLineDigits = 4

// wmaWidestMemberBase is the widest basename in the member set, in runes,
// derived from the set and never typed — for the reason wmaMemberPattern is
// read rather than retyped and wmaFixedAccount's width is read rather than
// carried: a basename this file spelled as a literal is a basename that drifts
// the moment a member file is added or renamed, and a drifted one here grades
// LESS. Ties keep the first, which the Glob's own order makes deterministic.
func wmaWidestMemberBase(files []wmaFile) (base string, chars int) {
	for _, f := range files {
		if b := filepath.Base(f.path); len([]rune(b)) > chars {
			base, chars = b, len([]rune(b))
		}
	}
	return base, chars
}

// wmaHelperPrefix is what go test spends at the head of a message rendered
// inside a t.Helper(), where the file it prints is NOT the file the message is
// written in. testing's decorate() writes the first non-helper frame, so the
// line carries the CALLING member file and its line.
//
// IT CHARGES THE WIDEST BASENAME IN THE MEMBER SET (ranger-base-x1ofd's
// decision, landed under ranger-base-q41e8). The member set's sharpest case is
// bgSeatAbstainAccount's abstention line, whose whole chain calls t.Helper()
// with 52 call sites between bgSeatAbstainIfOutside and wwAbstainIfOutside at
// 7dc3313e — so the actual caller cannot be known statically and a narrower
// charge under-reads. The widest keeps the rule a FLOOR either way: a line
// that fits at its own caller and not at the widest is still one caller away
// from being cut.
//
// THE RESIDUAL, named rather than hidden: the first non-helper frame need not
// be in a member file at all — a non-helper function in another file of
// internal/posse, called from a member, would decorate the line with its own
// basename, which this charge would under-read if it were wider. The member
// set is the one subject this census has (wmaMemberFiles), and a reading over
// the whole package would be a wider subject than the check's own definition
// of a member, which is what wmaMemberPattern exists to refuse.
func wmaHelperPrefix(base string) int {
	return len([]rune(base)) + len(":") + wmaHelperLineDigits + len(": ")
}

// wmaChargedPrefix is the prefix term arm 6 charges ONE message, and it is the
// branch between the two readers above rather than two lines inside the arm —
// for the reason wmaLinePrefix is a named reader at all: arm 6 is green at
// either charge over this tree (its own mutant 8), so the branch has to be
// somewhere arm 7 can red it over a planted message. `how` is the prefix as a
// reader would meet it on the line, which is also the report's own account of
// which file it charged.
func wmaChargedPrefix(m wmaMsg, widestBase string) (chars int, how string) {
	if m.helper {
		return wmaHelperPrefix(widestBase), fmt.Sprintf("%s:%s: — %s calls t.Helper(), so go test prints the CALLING member file and the widest basename in the set is what this arm can honestly charge", widestBase, strings.Repeat("N", wmaHelperLineDigits), m.helperIn)
	}
	return wmaLinePrefix(m.file, m.line), fmt.Sprintf("%s:%d: ", filepath.Base(m.file), m.line)
}

// wmaFuncSpan is one function in a member file — a FuncDecl or a FuncLit,
// because a closure is its own frame — with whether its OWN body marks itself
// a helper.
//
// OWN BODY, AND A NESTED CLOSURE IS NOT IT: t.Helper() marks the function that
// CALLS it, so a closure inside a helper is an ordinary frame unless it calls
// t.Helper() itself, and a helper's caller is reached by the CALL stack and
// not by this lexical one. That distinction is the whole of what this reading
// had to get right — MEASURED 2026-11-22 over the member set, the sloppier
// "a Helper() call anywhere under the enclosing FuncDecl" reads nine of arm
// 6's 65 messages where this reads eight, and the one it adds
// (wallwatchlive_test.go:614) is written in a test body beside a closure that
// is a helper, so go test decorates it with its own file and line.
type wmaFuncSpan struct {
	pos, end token.Pos
	name     string
	helper   bool
}

// wmaFuncSpans is every function body in a file with that reading.
func wmaFuncSpans(f *ast.File) []wmaFuncSpan {
	var out []wmaFuncSpan
	add := func(body *ast.BlockStmt, name string) {
		if body == nil {
			return
		}
		helper := false
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				return false // its own frame, and it answers for itself
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == wmaHelperMethod && len(x.Args) == 0 {
					helper = true
				}
			}
			return true
		})
		out = append(out, wmaFuncSpan{pos: body.Pos(), end: body.End(), name: name, helper: helper})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			add(fn.Body, fn.Name.Name)
		case *ast.FuncLit:
			add(fn.Body, "a func literal")
		}
		return true
	})
	return out
}

// wmaRenderedInHelper reports whether the call at pos is rendered inside a
// helper, and names the function it is in. The INNERMOST function containing
// pos is the frame go test starts from, and spans nest, so the innermost is
// the containing one with the greatest pos.
func wmaRenderedInHelper(spans []wmaFuncSpan, pos token.Pos) (helper bool, in string) {
	best := -1
	for i, s := range spans {
		if pos < s.pos || pos >= s.end {
			continue
		}
		if best < 0 || s.pos > spans[best].pos {
			best = i
		}
	}
	if best < 0 {
		return false, ""
	}
	return spans[best].helper, spans[best].name
}

// wmaConcatParts splits a string expression into its literal and identifier
// parts, and refuses anything else — a call, an index, a conversion. It is how
// arm 6 tells bgSeatAccount's two returns apart: one is `status` and literals,
// whose width is fully determined by the source, and the other ends in a fold
// of whatever the seat said, whose width is the seat's.
func wmaConcatParts(x ast.Expr) (lits, idents []string, ok bool) {
	switch e := x.(type) {
	case *ast.ParenExpr:
		return wmaConcatParts(e.X)
	case *ast.BasicLit:
		s, isStr := wmaStringLit(e)
		if !isStr {
			return nil, nil, false
		}
		return []string{s}, nil, true
	case *ast.Ident:
		return nil, []string{e.Name}, true
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return nil, nil, false
		}
		ll, li, lok := wmaConcatParts(e.X)
		rl, ri, rok := wmaConcatParts(e.Y)
		if !lok || !rok {
			return nil, nil, false
		}
		return append(ll, rl...), append(li, ri...), true
	}
	return nil, nil, false
}

// wmaIdentLitFloor is the shortest string literal the body assigns to the
// named identifier, in runes. For bgSeatAccount's `status` that is `exit 0`
// (6) — the one status its own source spells, the nil-error case — and every
// status a real failure produces is longer, which is what makes it a floor:
// MEASURED 2026-11-22 on this box, `signal: killed` is 14 and `exit status
// 128` is 15 (ranger-base-uu303).
func wmaIdentLitFloor(fn *ast.FuncDecl, name string) (int, string, bool) {
	best, lit, found := 0, "", false
	record := func(lhs, rhs []ast.Expr) {
		for i, r := range rhs {
			if i >= len(lhs) {
				continue
			}
			id, isID := lhs[i].(*ast.Ident)
			if !isID || id.Name != name {
				continue
			}
			s, isStr := wmaStringLit(r)
			if !isStr {
				continue
			}
			if n := len([]rune(s)); !found || n < best {
				best, lit, found = n, s, true
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return best, lit, found
}

// wmaFixedAccount is the ONE answer bgSeatAccount can give whose width is
// fully determined by its own source, and arm 6's whole premise.
//
// WHY THIS ONE AND NOT A DISTRIBUTION. bgSeatAccount returns `<status>` plus
// one of two tails. The words tail ends in a fold of whatever the seat said,
// so its width is the seat's and there is nothing here to read. The SILENCE
// tail is a fixed literal, and it is the account for the case that most needs
// reporting — a seat that said nothing, where the status is the whole finding
// (ranger-base-axt1f). So arm 6's rule is: there must be room on the line for
// the one account this function can produce in full, at the shortest status it
// spells. Nothing is fitted and nothing is typed: a tail that grows shrinks
// every member's budget by exactly what it grew, which is the ratchet
// ranger-base-dxaxt put here instead of a number.
//
// IT IS READ, NEVER TYPED, for the reason wmaAccountFolds is read back by arm
// 5: a width this file carried as a constant is a width that drifts the moment
// someone edits the prose, and a drifted one here does not red — it grades
// LESS, which is the shape this class has been found in twice.
func wmaFixedAccount(t *testing.T, files []wmaFile) (chars int, how string) {
	t.Helper()
	chars, how, err := wmaReadFixedAccount(files)
	if err != nil {
		t.Fatal(err)
	}
	return chars, how
}

// wmaReadFixedAccount is that reading with its refusals as values, so arm 7
// can drive them over planted source. A reader whose "I cannot read this"
// branch is a t.Fatalf is a branch no fixture can reach, and those are the
// branches that matter here: the whole point of arm 6 is that it never guesses
// a width.
func wmaReadFixedAccount(files []wmaFile) (chars int, how string, err error) {
	var fn *ast.FuncDecl
	for _, f := range files {
		for _, d := range wmaDecls(f.ast) {
			if d.Recv == nil && d.Name.Name == wmaAccountFunc {
				fn = d
			}
		}
	}
	if fn == nil {
		return 0, "", fmt.Errorf("no member file declares %s, so arm 6 has no account to budget for. Arm 5 says what that means", wmaAccountFunc)
	}
	type fixed struct {
		lits  []string
		ident string
	}
	var found []fixed
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		lits, idents, ok := wmaConcatParts(ret.Results[0])
		if !ok || len(idents) != 1 || len(lits) == 0 {
			return true
		}
		found = append(found, fixed{lits: lits, ident: idents[0]})
		return true
	})
	if len(found) != 1 {
		return 0, "", fmt.Errorf("%s has %d return(s) whose value is one identifier plus string literals, want exactly 1 — the silence account, whose width is fully determined by this function's own source. Arm 6 budgets for that one answer and cannot read a shape it does not recognise: if the function now builds its answer some other way (a Sprintf, a table, a second fold), say what its fixed-width answer is and teach this reader to find it. Do NOT type the width in here", wmaAccountFunc, len(found))
	}
	statusFloor, statusLit, ok := wmaIdentLitFloor(fn, found[0].ident)
	if !ok {
		return 0, "", fmt.Errorf("%s builds its fixed-width answer from %s plus literals, and assigns no string literal to %s anywhere in its body — so arm 6 cannot floor the status half of that account out of this source", wmaAccountFunc, found[0].ident, found[0].ident)
	}
	tail := 0
	for _, s := range found[0].lits {
		tail += len([]rune(s))
	}
	return statusFloor + tail, fmt.Sprintf("%s's silence account: %q (%d) + a fixed %d-character tail", wmaAccountFunc, statusLit, statusFloor, tail), nil
}

// ARM 1 — THE ACCOUNT. Every function in a member file that launches a bwrap
// seat renders that seat's error. This is ranger-base-axt1f's half: a runner
// that answers `err == nil` has destroyed the exit status before any message
// exists, and the line budget is then irrelevant.
func TestQAEveryWallMemberSeatLaunchRendersItsChildsError(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	launchers, graded, argvEntry := 0, 0, 0
	var wrapped []string
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			direct, viaWrapper := wmaSeatLaunches(fn)
			if !direct && !viaWrapper {
				continue
			}
			launchers++
			if direct {
				argvEntry++
			}
			if viaWrapper {
				wrapped = append(wrapped, f.path+":"+strconv.Itoa(f.fset.Position(fn.Pos()).Line)+" "+fn.Name.Name)
			}
			errs := wmaChildErrs(fn)
			if len(errs) == 0 {
				// A launch whose waiter's error is dropped at the
				// assignment is the same hole one step earlier.
				t.Errorf("%s:%d: %s launches a bwrap seat and binds its waiter's error to nothing a reader can be shown. `scripts/posse-wall-check` carries a failed member's FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), so the status is the one row that discriminates `exit status 128` (the wall or the command) from `signal: killed` (the box) — and an error that is never bound cannot be rendered by any message. Build the seat's answer with %s, as bgLive.walled does.",
					f.path, f.fset.Position(fn.Pos()).Line, fn.Name.Name, wmaAccountFunc)
				continue
			}
			for name := range errs {
				graded++
				if wmaSaid(fn, name) {
					continue
				}
				t.Errorf("%s:%d: %s launches a bwrap seat and renders %s nowhere — it is reduced to a bool or dropped before any message is formatted. That is ranger-base-axt1f exactly: a git that exited 128 at `cannot lock ref` and a seat ended from outside then produce byte-identical reports, and the one line `scripts/posse-wall-check` carries to a P1 bead (ADR 0097 D3) cannot say which it was. No line budget brings a destroyed status back. Return %s(%s, out) beside the raw output, as bsLive.in and srLive.in do.",
					f.path, f.fset.Position(fn.Pos()).Line, fn.Name.Name, name, wmaAccountFunc, name)
			}
		}
	}
	// THE CONTROLS. Both directions are the bug: a census that found no
	// member files, or no seat launches in them, would pass having read
	// nothing — which is the failure mode an absence pin has (see this
	// package's absencerules_qa_test.go).
	if len(files) < 20 {
		t.Errorf("this census found %d member file(s) in %s, want at least 20 (24 on 2026-11-21) — the member pattern read from %s has stopped matching this package and the arms above graded almost nothing", len(files), wmaPkgGlob, wmaCheck)
	}
	if launchers < 25 {
		t.Errorf("this census found %d bwrap-launching function(s) across those files, want at least 25 (30 on 2026-11-21) — a seat is started some way this pin cannot see, so the class has members it is not reading", launchers)
	}
	if graded == 0 {
		t.Errorf("this census graded 0 child errors over %d launching function(s) — the waiter set %v no longer names how this package waits on a seat", launchers, wmaWaiters)
	}
	// AND THE CONTROL FOR THE WIDENING (ranger-base-hhps5). The floor above
	// cannot ask whether the launch KEY still reads every spelling of a
	// launch, for the reason that finding exists: an invisible launcher does
	// not lower the count of the visible ones, so arm 1 was green over the
	// wrapper route for as long as it could not see it. This floor asks the
	// one question that floor cannot — does the tree still carry a launch the
	// old literal-argv key could not see — and arm 3's planted pair is the
	// hermetic half of the same claim.
	if len(wrapped) == 0 {
		t.Errorf("this census found %d launch(es) of a seat spelled some way other than a literal `bwrap` argv entry, want at least 1 (2 on 2026-11-22: grokleader_qa_test.go's pty arm, which joins the render's argv into a line for `script -qfec`, and wallunixlive_test.go's `sh -c 'bwrap \"$@\"; :'` arm). Those two are why the key is not the argv entry alone (ranger-base-hhps5); with neither in the tree, nothing but arm 3's planted pair holds the widening, and a later narrowing of wmaSeatLaunches would be green here. If the routes really have gone, say so and take this floor out — do not lower it.", len(wrapped))
	}
	// The other direction — a key that has stopped meaning `this function
	// starts a seat` — is arm 3's and not a bound here: see the head comment,
	// which carries the measurement that no bound over this tree can read it.
	// What this arm owes a reader is the census it actually took.
	t.Logf("%d member file(s), %d launching function(s) (%d handing exec.Command a literal `bwrap` argv entry, %d through the wrapper route), %d child error(s) graded. The wrapper reads were: %v",
		len(files), launchers, argvEntry, len(wrapped), graded, wrapped)
}

// ARM 2 — THE LINE. A message carrying a seat's account carries it where the
// bead will read it. This is ranger-base-a688i's half, scoped to the one
// value that cannot be recovered after the fact.
//
// THE ACCOUNT IS RENDERED BEFORE THE FIRST NEWLINE, and not "the message has
// no newline at all". `scripts/posse-wall-check`'s evidence() carries that
// first line and nothing after it (ADR 0097 D3), so the newline is what costs
// a reader the TAIL and never the account in front of it — and the account in
// front with the supporting prose behind is the shape D3 and
// ranger-base-mklu0's fix both prescribe. The head comment carries the
// measurement that decided this against the rule that stood here before.
// WHERE inside the first line the account lands is arm 6's, off this same key
// (wmaAccountMessages) — this arm says the account is on the line evidence()
// carries, and that one says the line's 300 characters reach it.
func TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	withAccounts, read := 0, 0
	// THE DEFERRED SET IS LISTED AND NOT COUNTED (ranger-base-yem73): the
	// floor below names its own examples as a literal, because the state it
	// reds in is the state where the tree has none to print — and a typed list
	// goes stale at the next landing that moves a line, which is twice now.
	// The Logf carries the live set so the reading is a `-v` run away.
	var deferred []string
	routes := map[string]int{}
	for _, f := range files {
		msgs := wmaAccountMessages(f)
		if len(msgs) > 0 {
			withAccounts++
		}
		for _, m := range msgs {
			read++
			routes[m.route]++
			// A variadic spread reorders nothing but hides how many
			// arguments the verbs before the newline consumed, so it is
			// the same unreadable case as an indexed verb.
			before, readable, firstLine := wmaAccountBeforeNewline(m.text, m.argIdx)
			if m.ellipsis {
				readable = false
			}
			switch {
			case !readable:
				t.Errorf("%s:%d: this t.%s carries the seat's account %s in a format string that has a newline in it AND a verb whose argument positions cannot be read off the text (an indexed `%%[n]s`, a star width, or a variadic spread). `scripts/posse-wall-check` carries the FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), so whether the account survives depends on where it lands in %q — and this census will not guess. Spell the verbs in order, or fold the message onto one line.",
					m.file, m.line, m.verb, m.name, firstLine)
			case !before:
				t.Errorf("%s:%d: this t.%s renders the seat's account %s AFTER the first newline of its format string. `scripts/posse-wall-check` reports a failed member by its FIRST `file.go:NN:` line and nothing after it (ADR 0097 D3), and the launcher puts that one line on a P1 bead — so this message reaches the bead as %q and the account really is dropped in transit. That is ranger-base-a688i exactly: a symptom, a colon and nothing. Move the account in front of the newline; the prose behind it costs nothing.",
					m.file, m.line, m.verb, m.name, firstLine)
			case strings.ContainsAny(m.text, "\n\r"):
				// The shape the rule before this one refused, and the
				// reason the floor below exists.
				deferred = append(deferred, fmt.Sprintf("%s:%d", m.file, m.line))
			}
		}
	}
	// The controls, again both ways.
	if withAccounts < 6 {
		t.Errorf("this arm found seat accounts in %d member file(s), want at least 6 (8 on 2026-11-22: bwrapgitrefslive_test.go, bwrapseatstorelive_test.go, storerepogitwalllive_test.go, wallclaudeprojects_qa_test.go, wallclaudeprojectsattack_qa_test.go, wallunixlive_test.go, wallwatchbackstopwiring_linux_test.go and wallwatchlive_test.go — 3 of them under the key before ranger-base-momix widened it) — the shapes this arm reads have changed and it is grading almost nothing", withAccounts)
	}
	if read < 45 {
		t.Errorf("this arm read %d message(s) interpolating a seat account, want at least 45 (66 on 2026-11-22, of which 32 were all the key read before ranger-base-momix and 65 all it read before ranger-base-x1ofd) — an account reaches a reader by some route this arm cannot see", read)
	}
	// AND A FLOOR PER ROUTE, for the reason arm 1 floors the wrapper route
	// (ranger-base-hhps5): a key that reads four spellings and finds three of
	// them in the tree is a key whose fourth arm a later narrowing could
	// delete with every arm here still green. An account this census cannot
	// see does not lower the count of the ones it can.
	//
	// THE PARAMETER ROW IS A FLOOR OF ONE AND THAT IS THE WHOLE POINT
	// (ranger-base-x1ofd). It is the thinnest row here — one message, in one
	// file — and it is the row that went ungraded longest: the key read three
	// routes while the member set used four, so nothing held
	// bgSeatAbstainAccount's NOTHING MEASURED line anywhere. A route with one
	// witness is exactly the route a narrowing deletes for free.
	for _, route := range []string{"runner local", "folded local", "inline fold", "parameter"} {
		if routes[route] == 0 {
			t.Errorf("this arm read 0 account(s) by the %q route, want at least 1 (on 2026-11-22: 32 runner local, 8 folded local, 25 inline fold, 1 parameter). That route is in the key because the member set used it — the folded routes are ranger-base-momix's, which is how wallclaudeprojectsattack_qa_test.go came to be graded by nothing after this class had been found in it twice, and the parameter route is ranger-base-x1ofd's, which is how bgSeatAbstainAccount's abstention line came to be graded by nothing at all. If the spelling really has gone from the tree, say so and take the route out of wmaAccountArg; do not leave an arm of the key with nothing witnessing it.", route)
		}
	}
	// THE WITNESS FOR THE RULE ITSELF. The rule above is POSITIONAL, and the
	// only thing in this tree that tells a positional rule from the "no
	// newline at all" rule it replaced is a message that HAS a newline with
	// its account in front of it. With none in the member set, a later
	// re-tightening would be green here and would red four messages whose
	// shape ADR 0097 D3 prescribes — which is the state ranger-base-momix
	// found and the reason that re-tightening is easy to type.
	if len(deferred) == 0 {
		t.Errorf("this arm read %d account message(s) that defer their prose to a second line, want at least 1 (6 at 814776ed on 2026-11-22: wallclaudeprojects_qa_test.go:445, wallclaudeprojectsattack_qa_test.go:386 and :403, and wallunixlive_test.go:237, :526 and :1130 — four of them ranger-base-momix's reading at e5221de6, the other two ranger-base-72z36's, and the line numbers are this tip's and not a promise about yours: the Logf below lists whatever the tree holds now). Those are what distinguish this arm's rule from `no newline at all`; with none of them in the tree, nothing here witnesses that the rule is positional and arm 3's planted rows are the whole of it.", len(deferred))
	}
	t.Logf("%d member file(s), %d of them carrying a seat account, %d account message(s) read (%d by a runner local, %d by a folded local, %d by an inline fold, %d by a parameter), %d of which defer prose to a second line with the account in front of it: %v",
		len(files), withAccounts, read, routes["runner local"], routes["folded local"], routes["inline fold"], routes["parameter"], len(deferred), deferred)
}

// ARM 3 — THE RULES SAY NO. Both rules driven over planted source, each with
// its own negative case, through the SAME functions the two arms above read
// the tree with. A rule that quietly stopped meaning what this file says it
// means reds here with no bwrap, no venue and no member set.
func TestQATheWallMemberAccountRulesMatchWhatTheyClaim(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name            string
		src             string
		launches, rends bool
	}{
		{"a seat runner that renders its status", `func run(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}`, true, true},
		{"the ranger-base-axt1f shape, reduced to a bool", `func run(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}`, true, false},
		{"a seat run whose error reaches a fatal", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	if err := cmd.Run(); err != nil {
		t.Fatalf("the seat refused: %v", err)
	}
}`, true, true},
		{"a seat run whose error is dropped at the assignment", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, true, false},
		{"a seat run that says a DIFFERENT error and swallows its own", `func run(t *testing.T) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	if ferr := os.WriteFile("x", out, 0o644); ferr != nil {
		t.Fatalf("the fixture: %v", ferr)
	}
	_ = err
}`, true, false},
		{"a run of something that is not a seat", `func run(t *testing.T) {
	cmd := exec.Command("git", "status")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		// ranger-base-hhps5's planted pair, which is the whole of that
		// finding: the same probe twice, the only difference being how the
		// seat's argv is spelled. The first was RED before the widening and
		// the second GREEN, with nothing else between them (MEASURED
		// 2026-11-22 at 2866fdf4 by planting both into internal/posse as a
		// member file). Both are launches now.
		{"a seat launched through a shell, the literal inside the command line", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "bwrap --dev-bind / / /bin/true")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, true, false},
		{"a seat launched through a pty wrapper, the argv joined into a line one hop away", `func run(t *testing.T) {
	var q []string
	q = append(q, "bwrap")
	for _, a := range argv {
		q = append(q, shellQuote(a))
	}
	cmd := exec.Command(script, "-qfec", strings.Join(q, " ")+" "+line, "/dev/null")
	if err := cmd.Start(); err != nil {
		t.Fatalf("the seat never started: %v", err)
	}
	_ = cmd.Wait()
}`, true, true},
		{"a seat launched by its absolute path inside a command line", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exec /usr/bin/bwrap --dev-bind / / /bin/true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("the seat refused: %v", err)
	}
}`, true, true},
		// And the negative cases the widening needs, because a key read as a
		// SUBSTRING would put every one of these under arm 1's rule:
		{"a shell line that is not a seat", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "git status --porcelain")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		{"a shell line naming a neighbour of bwrap", `func run(t *testing.T) {
	cmd := exec.Command("sh", "-c", "bwraptest && bwrap-wrapper && echo bwrap.sh")
	out, _ := cmd.CombinedOutput()
	t.Logf("%s", out)
}`, false, false},
		{"a skip message that says bwrap is absent", `func run(t *testing.T) {
	why := "NOTHING MEASURED: no bwrap on this host"
	cmd := exec.Command("git", "status")
	out, _ := cmd.CombinedOutput()
	t.Skipf("%s: %s", why, out)
}`, false, false},
		{"a seat started through CommandContext", `func run(t *testing.T, ctx context.Context) {
	cmd := exec.CommandContext(ctx, "bwrap", "x")
	if err := cmd.Start(); err != nil {
		t.Fatalf("the seat never started: %v", err)
	}
}`, true, true},
		{"an err.Error() is a rendering of its own", `func run(t *testing.T) string {
	cmd := exec.Command("bwrap", "x")
	err := cmd.Run()
	if err != nil {
		return err.Error()
	}
	return ""
}`, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			fn, _ := wmaFixture(t, c.src, "run")
			if got := wmaLaunchesSeat(fn); got != c.launches {
				t.Fatalf("wmaLaunchesSeat = %v, want %v — the seat-launch key no longer selects what this file says it selects:\n%s", got, c.launches, c.src)
			}
			if !c.launches {
				return
			}
			errs := wmaChildErrs(fn)
			rendered := false
			for name := range errs {
				if wmaSaid(fn, name) {
					rendered = true
				}
			}
			if rendered != c.rends {
				t.Errorf("the child-error reading says rendered=%v, want %v (bound names %v) — arm 1's key no longer means what this file says it means:\n%s", rendered, c.rends, errs, c.src)
			}
		})
	}

	// And arm 2's one hop, the same way: a local bound to a runner's account
	// is found, and a local bound to something else is not.
	for _, c := range []struct {
		name string
		src  string
		want []string
	}{
		{"the account local of a three-result runner", `func (f fx) in(t *testing.T) (bool, string, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, string(out), bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, out, acct := f.in(t); _, _, _ = ok, out, acct }`, []string{"acct"}},
		{"the account local of a two-result runner", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, acct := f.in(t); _, _ = ok, acct }`, []string{"acct"}},
		{"a blanked account binds nothing", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("bwrap", "x")
	out, err := cmd.CombinedOutput()
	return err == nil, bgSeatAccount(err, string(out))
}
func TestX(t *testing.T) { ok, _ := f.in(t); _ = ok }`, nil},
		{"a helper that launches no seat is not a runner", `func (f fx) in(t *testing.T) (bool, string) {
	cmd := exec.Command("git", "status")
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}
func TestX(t *testing.T) { ok, acct := f.in(t); _, _ = ok, acct }`, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, f := wmaFixture(t, c.src, "")
			got := wmaAccountLocals(f)
			if len(got) != len(c.want) {
				t.Fatalf("wmaAccountLocals found %v, want %v — arm 2's one hop no longer binds what this file says it binds:\n%s", got, c.want, c.src)
			}
			for _, w := range c.want {
				if !got[w] {
					t.Errorf("wmaAccountLocals found %v, want it to name %q:\n%s", got, w, c.src)
				}
			}
		})
	}

	// And arm 2's OTHER two routes, which are ranger-base-momix's: an account
	// the fold produced, bound to a local or handed to the verb inline. The
	// rows are the two spellings
	// internal/posse/wallclaudeprojectsattack_qa_test.go actually uses, and
	// the negatives are the ones that make the key a key: a fold of something
	// that is not a seat's words is still read (the fold exists to make a
	// value fit a member's one line, so the rule is right for it too), but a
	// local bound to anything else is not.
	for _, c := range []struct {
		name  string
		src   string
		local []string
		arg   []bool
	}{
		{"an account folded into a local, from a runner this census cannot recognise", `func TestX(t *testing.T) {
	out := run(t, nonOwn)
	said := wwOneLine(out)
	t.Fatalf("claude -p under the wall did not end in 90s; it said %s\ncwd %s", said, cwd)
}`, []string{"said"}, []bool{true, false}},
		{"an account built by the account func and bound to a local", `func TestX(t *testing.T) {
	acct := bgSeatAccount(runErr, outBuf.String())
	t.Logf("the seat ended badly: %s", acct)
}`, []string{"acct"}, []bool{true}},
		{"an account handed to the verb inline", `func TestX(t *testing.T) {
	t.Logf("the seat ended badly, which this pin does not assert on: %s", bgSeatAccount(runErr, outBuf.String()))
}`, nil, []bool{true}},
		{"a fold handed to the verb inline", `func TestX(t *testing.T) {
	t.Fatalf("the seat exited badly: %s\nargv: bwrap %s", wwOneLine(string(out)), argv)
}`, nil, []bool{true, false}},
		{"a local bound to something that is not a fold", `func TestX(t *testing.T) {
	out := strings.TrimSpace(string(raw))
	t.Fatalf("the seat said:\n%s", out)
}`, nil, []bool{false}},
		{"a blanked fold binds nothing", `func TestX(t *testing.T) {
	_ = wwOneLine(out)
	t.Logf("%s", out)
}`, nil, []bool{false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, f := wmaFixture(t, c.src, "")
			got := wmaFoldLocals(f)
			if len(got) != len(c.local) {
				t.Fatalf("wmaFoldLocals found %v, want %v — arm 2's folded-local route no longer binds what this file says it binds:\n%s", got, c.local, c.src)
			}
			for _, w := range c.local {
				if !got[w] {
					t.Errorf("wmaFoldLocals found %v, want it to name %q:\n%s", got, w, c.src)
				}
			}
			// And the per-argument reading, which is what arm 2 keys on.
			runner := wmaAccountLocals(f)
			var isAccount []bool
			ast.Inspect(f.ast, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !wmaFormatVerbs[sel.Sel.Name] {
					return true
				}
				for _, a := range call.Args[1:] {
					_, _, is := wmaAccountArg(a, runner, got, nil)
					isAccount = append(isAccount, is)
				}
				return true
			})
			if len(isAccount) != len(c.arg) {
				t.Fatalf("wmaAccountArg read %d format argument(s), want %d:\n%s", len(isAccount), len(c.arg), c.src)
			}
			for i := range c.arg {
				if isAccount[i] != c.arg[i] {
					t.Errorf("wmaAccountArg says argument %d is an account = %v, want %v — arm 2's key no longer means what this file says it means:\n%s", i, isAccount[i], c.arg[i], c.src)
				}
			}
		})
	}

	// And arm 2's FOURTH route, which is ranger-base-x1ofd's: the account
	// arrives as a PARAMETER of the function that renders it, bound by no `:=`
	// anywhere, so neither local map can hold it. The first row is the site
	// the route exists for — bgSeatAbstainAccount's shape, a grouped
	// `what, account string` list whose account comes from an inline fold at
	// the call site — and the negatives are what keep the route from reading
	// a position no account reaches: the `what` beside it, a parameter the
	// call site hands something else, a variadic, a blank, a non-string, and
	// the second hop.
	for _, c := range []struct {
		name  string
		src   string
		param []string
		arg   []bool
	}{
		{"the account parameter of an abstention, from an inline fold at the call site", `func abstain(t *testing.T, what, account string) {
	t.Skipf("NOTHING MEASURED: %s — %s. the seat was ended from outside", what, account)
}
func TestX(t *testing.T) { abstain(t, "a seat running git", bgSeatAccount(err, out)) }`, []string{"account"}, []bool{false, true}},
		{"the account parameter, from a folded local at the call site", `func say(t *testing.T, account string) {
	t.Fatalf("the seat ended badly: %s", account)
}
func TestX(t *testing.T) {
	acct := wwOneLine(out)
	say(t, acct)
}`, []string{"account"}, []bool{true}},
		{"a parameter the call site hands something that is not an account", `func say(t *testing.T, msg string) {
	t.Fatalf("the seat said: %s", msg)
}
func TestX(t *testing.T) { say(t, strings.TrimSpace(string(raw))) }`, nil, []bool{false}},
		{"a variadic parameter binds nothing", `func say(t *testing.T, parts ...string) {
	t.Fatalf("the seat said: %s", parts)
}
func TestX(t *testing.T) { say(t, bgSeatAccount(err, out)) }`, nil, []bool{false}},
		{"a blank parameter binds nothing", `func say(t *testing.T, _ string) {
	t.Fatalf("the seat said something this function cannot read: %s", elsewhere)
}
func TestX(t *testing.T) { say(t, bgSeatAccount(err, out)) }`, nil, []bool{false}},
		{"a parameter that is not a string cannot be holding an account", `func say(t *testing.T, code int) {
	t.Fatalf("the seat ended: %s", code)
}
func TestX(t *testing.T) {
	got := wwOneLine(out)
	say(t, got)
}`, nil, []bool{false}},
		// THE SECOND HOP, IN BOTH SOURCE ORDERS, and the orders are the row
		// (ranger-base-x1ofd). Fed back into its own reading the route stops
		// being one hop and starts being a fixpoint reached in walk order: the
		// mutant that feeds the param map back in reds the first of these two
		// rows and NOT the second, because with the inner call written first
		// ast.Inspect meets it before anything has bound the outer parameter.
		// One row would have left the one-hop rule witnessed by nothing — it
		// did, until this battery was run.
		{"an account two hops from the fold is not read, the outer call first", `func TestX(t *testing.T) { outer(t, bgSeatAccount(err, out)) }
func outer(t *testing.T, acct string) { inner(t, acct) }
func inner(t *testing.T, account string) {
	t.Fatalf("the seat ended badly: %s", account)
}`, []string{"acct"}, []bool{false}},
		{"an account two hops from the fold is not read, the inner call first", `func inner(t *testing.T, account string) {
	t.Fatalf("the seat ended badly: %s", account)
}
func outer(t *testing.T, acct string) { inner(t, acct) }
func TestX(t *testing.T) { outer(t, bgSeatAccount(err, out)) }`, []string{"acct"}, []bool{false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, f := wmaFixture(t, c.src, "")
			runner, folded := wmaAccountLocals(f), wmaFoldLocals(f)
			got := wmaParamAccounts(f, runner, folded)
			if len(got) != len(c.param) {
				t.Fatalf("wmaParamAccounts found %v, want %v — arm 2's parameter route no longer binds what this file says it binds:\n%s", got, c.param, c.src)
			}
			for _, w := range c.param {
				if !got[w] {
					t.Errorf("wmaParamAccounts found %v, want it to name %q:\n%s", got, w, c.src)
				}
			}
			// And the per-argument reading, which is what arm 2 keys on.
			var isAccount []bool
			ast.Inspect(f.ast, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !wmaFormatVerbs[sel.Sel.Name] {
					return true
				}
				for _, a := range call.Args[1:] {
					_, _, is := wmaAccountArg(a, runner, folded, got)
					isAccount = append(isAccount, is)
				}
				return true
			})
			if len(isAccount) != len(c.arg) {
				t.Fatalf("wmaAccountArg read %d format argument(s), want %d:\n%s", len(isAccount), len(c.arg), c.src)
			}
			for i := range c.arg {
				if isAccount[i] != c.arg[i] {
					t.Errorf("wmaAccountArg says argument %d is an account = %v, want %v — arm 2's parameter route no longer means what this file says it means:\n%s", i, isAccount[i], c.arg[i], c.src)
				}
			}
		})
	}

	// THE POSITIONAL RULE, driven over the text (ranger-base-momix). The row
	// that matters most is the third: an account in FRONT of a newline is
	// accepted, which is the shape ADR 0097 D3 prescribes and the shape the
	// rule that stood here before refused.
	for _, c := range []struct {
		name, text       string
		argIdx           int
		before, readable bool
		firstLine        string
	}{
		{"one line, one verb", "the seat refused: %s", 0, true, true, "the seat refused: %s"},
		{"ranger-base-a688i's own shape, the account after the newline", "the seat ended before it ran (exit 1):\n%s", 0, false, true, "the seat ended before it ran (exit 1):"},
		{"the account in front, the prose behind", "it said %s\ncwd %s — D3 assumes an unwritable transcript dir is survivable", 0, true, true, "it said %s"},
		{"the second argument is behind the newline", "it said %s\ncwd %s", 1, false, true, "it said %s"},
		{"a literal percent consumes no argument", "the wall took 100%% of it:\n%s", 0, false, true, "the wall took 100%% of it:"},
		{"a carriage return is a newline too", "the seat refused:\r%s", 0, false, true, "the seat refused:"},
		{"an indexed verb cannot be read positionally", "it said %[1]s\ncwd %[2]s", 0, false, false, "it said %[1]s"},
		{"a star width cannot be read positionally", "it said %*s\ncwd %s", 1, false, false, "it said %*s"},
		{"an unreadable verb in a ONE-LINE message is no finding", "it said %[1]s and %[1]s again", 0, true, true, "it said %[1]s and %[1]s again"},
	} {
		t.Run(c.name, func(t *testing.T) {
			before, readable, firstLine := wmaAccountBeforeNewline(c.text, c.argIdx)
			if before != c.before || readable != c.readable || firstLine != c.firstLine {
				t.Errorf("wmaAccountBeforeNewline(%q, %d) = %v,%v,%q want %v,%v,%q — arm 2's line rule no longer reads what this file says it reads",
					c.text, c.argIdx, before, readable, firstLine, c.before, c.readable, c.firstLine)
			}
		})
	}

	// The format reader, including the shape a `+` concatenation hides a
	// newline in: a format spelled in two literals is one message.
	for _, c := range []struct {
		src  string
		want string
		ok   bool
	}{
		{`"one line"`, "one line", true},
		{`"first:\n" + "second"`, "first:\nsecond", true},
		{"`a raw\nstring`", "a raw\nstring", true},
		{`"a " + name`, "", false},
	} {
		expr, perr := parser.ParseExpr(c.src)
		if perr != nil {
			t.Fatalf("parse %s: %v", c.src, perr)
		}
		got, ok := wmaFormat(expr)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("wmaFormat(%s) = %q,%v want %q,%v", c.src, got, ok, c.want, c.ok)
		}
	}
}

// wmaFixture parses one planted snippet and hands back the named func (when
// name is set) and the file. It goes through the same parser the tree arms
// use, so a fixture cannot pass against a second, friendlier copy of a rule.
func wmaFixture(t *testing.T, src, name string) (*ast.FuncDecl, wmaFile) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture_test.go", "package posse\n"+src+"\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v\n%s", err, src)
	}
	f := wmaFile{path: "fixture_test.go", fset: fset, ast: file}
	if name == "" {
		return nil, f
	}
	for _, fn := range wmaDecls(file) {
		if fn.Name.Name == name {
			return fn, f
		}
	}
	t.Fatalf("fixture declares no %s:\n%s", name, src)
	return nil, f
}

// ARM 4 — the member pattern this census reads is the one the check runs.
// Read back as a REGEXP against the real member names, because a pattern that
// compiled but matched nothing would leave every arm above green over an
// empty set.
func TestQATheWallMemberCensusReadsTheChecksOwnPattern(t *testing.T) {
	t.Parallel()
	re := wmaMemberPattern(t)
	for _, name := range []string{
		"TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead",
		"TestLiveBwrapGitRefsCommitsAfterAnOutsidePackRefs",
		"TestQALiveWallSomethingNobodyHasWrittenYet",
	} {
		if !re.MatchString(name) {
			t.Errorf("the pattern read from %s does not match %q, which is the shape of a wall-check member", wmaCheck, name)
		}
	}
	// And it refuses what is not one. bwrapgitrefsaccount_qa_test.go's own
	// pins are deliberately NOT named like members: they measure no wall.
	for _, name := range []string{
		"TestQABwrapGitRefsSeatAccountNamesItsStatusOnOneLine",
		"TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine",
		"TestLiveSomethingThatIsNoWall",
	} {
		if re.MatchString(name) {
			t.Errorf("the pattern read from %s matches %q, which is not a wall-check member — the set this census reads has widened past the set the check measures", wmaCheck, name)
		}
	}
}

// ARM 5 — THE ACCOUNT VOCABULARY IS THE TREE'S AND NOT THIS FILE'S
// (ranger-base-momix). Arm 2's key reads a value folded by one of two names,
// and a typed pair of names is exactly the register this file's head comment
// refuses when it declines an exemption table: a list nobody reads back drifts
// quietly, and a drifted list here does not red — it GRADES LESS, which is the
// state that let this class be found twice in one file.
//
// So the pair is read back out of the member set. Each name is declared there
// as a function returning exactly one string, and bgSeatAccount's body calls
// wwOneLine — which is the sentence bwrapgitrefslive_test.go writes about
// itself ("THE FOLD IS wwOneLine's AND NOT A SECOND SPELLING OF IT"). What it
// cannot see is a THIRD fold nobody told it about; arm 2's per-route floors
// are what notice a route going quiet, and this arm is what notices the two
// names it does have coming apart.
func TestQATheWallMemberAccountFoldsAreTheOnesThePackageDeclares(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	decls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, fn := range wmaDecls(f.ast) {
			if fn.Recv != nil || !wmaAccountFolds[fn.Name.Name] {
				continue
			}
			if prev, seen := decls[fn.Name.Name]; seen {
				t.Errorf("%s declares a second %s (the first is at %s) — two foldings of \"put this on one line\" are two answers waiting to disagree about what a member's line may contain (bwrapgitrefslive_test.go), and arm 2 reads them as one",
					f.path, fn.Name.Name, prev.Name.Name)
			}
			decls[fn.Name.Name] = fn
		}
	}
	for name := range wmaAccountFolds {
		fn, ok := decls[name]
		if !ok {
			t.Errorf("no member file declares %s, which arm 2's key names as a spelling of a seat's account. Either it was renamed — in which case arm 2 is now grading the messages that used it by nothing, silently, which is the shape ranger-base-momix found — or the fold has moved out of the member set and this census can no longer see it.", name)
			continue
		}
		res := fn.Type.Results
		if res == nil || len(res.List) != 1 || len(res.List[0].Names) > 1 {
			t.Errorf("%s no longer returns exactly one value, so what arm 2 reads at its call sites is not the one-line account this file says it is", name)
			continue
		}
		if id, isID := res.List[0].Type.(*ast.Ident); !isID || id.Name != "string" {
			t.Errorf("%s no longer returns a string, so arm 2's reading of its call sites as a rendered account is wrong", name)
		}
	}
	// AND THE PAIR IS ONE FOLD AND ONE ACCOUNT BUILT ON IT, which is the whole
	// reason this list is two names long and not one or three.
	if fn, ok := decls[wmaAccountFunc]; ok && !wmaBodyCalls(fn, wmaFoldFunc) {
		t.Errorf("%s no longer calls %s, so this package has two independent foldings of \"put this on one line\" and arm 2 reads them as one vocabulary. bwrapgitrefslive_test.go's own head comment is the contract: the fold is %s's and not a second spelling of it.",
			wmaAccountFunc, wmaFoldFunc, wmaFoldFunc)
	}
}

// ARM 6 — THE 300, which is the half arm 2 could not read
// (ranger-base-uu303's reading, ranger-base-dxaxt's lever).
//
// An account rendered before the first newline is only ON THE BEAD if
// evidence()'s budget reaches it. `scripts/posse-wall-check` carries that
// first line with the `file.go:NN: ` prefix INSIDE the slice — it strips
// indentation and nothing else — so a message whose literal preamble is wide
// enough cuts its own account while satisfying arm 2 in full. That is
// ranger-base-mklu0's finding 2 exactly: 492 characters rendered, 300
// carried, 261 of the 300 spent before the account.
//
// THE RULE IS DERIVED AND NOT FITTED: `prefix + preamble + the account's own
// fixed width <= the check's own budget`. Every term is read out of the tree —
// the budget out of evidence() (wmaEvidenceBudget), the account out of
// bgSeatAccount's source (wmaFixedAccount), the prefix out of the message's
// own position or, for a message rendered inside a t.Helper(), out of the
// widest basename in the member set (wmaChargedPrefix, ranger-base-q41e8: go
// test decorates such a line with the first NON-helper frame, so the file it
// prints is the CALLER's and not this one), the preamble out of the format
// string with every interpolated verb counted as zero. There is no number in
// this arm and no exemption table,
// which is what ranger-base-uu303 was filed to avoid: a FITTED preamble bound
// of 128 reds 11 of the old key's 32 at a precision of 3/11, and it has no
// derivation at all.
//
// IT IS NECESSARY AND NOT SUFFICIENT, by measurement. Three member messages
// blow the budget on interpolated fixture paths of 124 to 174 characters, and
// in ADR 0097 D1's venue every `t.TempDir()` path is +50 (that venue's TMPDIR
// is 54 characters against /tmp's 4, MEASURED 2026-11-22) — so their accounts
// arrive as nothing at all and no reading of a format string can see why.
// Those three are ranger-base-bj1hh. Sufficiency needs each member's messages
// RENDERED under its fixture and sliced as evidence() does, which is its own
// venue; ranger-base-uu303 records it rather than filing it, so the next attempt does not re-try a static rule.
//
// WHAT THIS ARM COST TO LAND, and why the lever was the account and not the
// messages. At bgSeatAccount's original 199-character silence tail the derived
// budget for a whole message was 95 characters, and this arm red 28 of the 65
// messages it grades — every `CONTROL:` message whose preamble is a complete
// claim ADR 0097 D3 wants on line 1 among them. Correct, and unlandable
// without 28 prose edits. The 199 characters were REASONING and not evidence
// (the status and `said NOTHING` are the finding) and the clause had become
// wrong on the only path that still reaches a bead with it: silence plus a
// supervisor's signal is routed to ADR 0097 D3's abstention now
// (bgSeatAbstainIfOutside), whose own sentence says the ended-from-outside
// half, so what is left carrying the account as a RED is a status the house
// reads as the wall's own answer — and "no error from the command" read
// directly after `exit status 128` was wrong and not merely redundant.
// Shortening the tail to 27 characters moved the budget to 267 and left this
// arm red on nothing, at 58 characters of headroom on the widest head it
// charges. MEASURED 2026-11-22, both readings, on this box
// (linux 7.2.15-1-omarchy, go1.27.3) at tip 3ef2d977 for the two tails and at
// 7dc3313e for that headroom, which the head comment prices and this arm's own
// Logf prints.
func TestQAEveryWallMemberAccountFitsInsideTheEvidenceBudget(t *testing.T) {
	t.Parallel()
	files := wmaMemberFiles(t)
	budget := wmaEvidenceBudget(t)
	account, how := wmaFixedAccount(t, files)
	widestBase, _ := wmaWidestMemberBase(files)
	graded, widest, widestAt := 0, 0, ""
	// LISTED AND NOT COUNTED, for ranger-base-yem73's reason: the head
	// comment's account of which eight messages these are went stale at the
	// landing before the one that typed it, and a count cannot be checked
	// against a list. The floor stays off it (see the Logf below for why).
	var inHelper []string
	for _, f := range files {
		for _, m := range wmaAccountMessages(f) {
			// Arm 2's cases, and arm 2 reports them. A message whose account
			// is behind the newline or whose positions cannot be read has no
			// place on the first line for this arm to measure.
			before, readable, firstLine := wmaAccountBeforeNewline(m.text, m.argIdx)
			if m.ellipsis || !readable || !before {
				continue
			}
			spent, ok := wmaSpentBefore(firstLine, m.argIdx)
			if !ok {
				continue
			}
			graded++
			// THE PREFIX IS CHARGED TO THE FILE go test WILL PRINT.
			// testing's decorate() writes the first NON-helper frame, so a
			// message rendered inside a t.Helper() carries the CALLING member
			// file and its line and not this one — and with 52 call sites on
			// the member set's own abstention chain there is no one caller to
			// read, so the charge is the widest basename in the set
			// (ranger-base-q41e8; wmaHelperPrefix says why that is the
			// strictest honest reading and still a floor).
			prefix, prefixHow := wmaChargedPrefix(m, widestBase)
			if m.helper {
				inHelper = append(inHelper, fmt.Sprintf("%s:%d (in %s, head %d)", m.file, m.line, m.helperIn, prefix+spent))
			}
			if prefix+spent > widest {
				widest, widestAt = prefix+spent, fmt.Sprintf("%s:%d", m.file, m.line)
			}
			if prefix+spent+account <= budget {
				continue
			}
			arrives := budget - prefix - spent
			gets := fmt.Sprintf("%d character(s) of it", arrives)
			if arrives <= 0 {
				gets = "NONE of it — the account's own verb is past the slice, so it reaches the bead as nothing at all"
			}
			t.Errorf("%s:%d: this t.%s spends %d of `scripts/posse-wall-check`'s %d characters before the seat's account %s even starts (%d for go test's own `%s` prefix, which evidence() keeps because it strips only indentation, and %d on the literal preamble — every interpolated verb counted as ZERO, so this is a FLOOR). The account needs %d (%s), so the bead gets %s. ADR 0097 D3's evidence() carries the first `file.go:NN:` line truncated to %d characters and the launcher puts that one line on a P1 bead, so this is ranger-base-mklu0's finding 2: arm 2 is satisfied and the account is cut anyway. THE CHEAPEST FIX IS USUALLY NOT THIS MESSAGE — %s is one function and the single largest consumer in this class, and shortening its fixed tail widens every member's budget by what it gives back (ranger-base-dxaxt). Otherwise move this preamble's prose behind the newline, where D3 says it costs nothing.",
				m.file, m.line, m.verb, prefix+spent, budget, m.name,
				prefix, prefixHow, spent,
				account, how, gets, budget, wmaAccountFunc)
		}
	}
	// THE CONTROL, which is this arm's whole claim to have read anything: it
	// grades off wmaAccountMessages, the same walk arm 2 reads, and a key that
	// went quiet would leave this arm green over nothing. Arm 2 floors the
	// same count for the same reason; this one is here because arm 6 DROPS the
	// messages arm 2 reports — an account behind the newline, or behind a verb
	// this census will not position — so the two counts need not be the same
	// number, and a drop that swallowed the set would show here and not there.
	// They ARE the same number today (66 on 2026-11-22, 65 before
	// ranger-base-x1ofd's fourth route: every account in the member set is on
	// the first line, which is arm 2's green said the other way round) and
	// this floor does not assume they stay that way.
	if graded < 45 {
		t.Errorf("this arm graded %d account message(s) against the %d-character budget, want at least 45 (66 on 2026-11-22, the whole of what arm 2 reads). An account reaches a reader by some route this arm no longer reads, or every account in the set has moved somewhere this arm declines to measure — and arm 2's own report is what says which.", graded, budget)
	}
	// THE HELPER-RENDERED COUNT IS LOGGED AND NOT FLOORED, which is the one
	// place this arm departs from arm 2's register and does so on arm 3's
	// ruling. A floor on it would be a bound on the TREE's prose — a member
	// set whose helpers all grew their own messages would red it while every
	// line still fitted — and the term it would be protecting is held
	// hermetically in arm 7 instead, where a reading that stopped recognising
	// a helper reds over planted source with no member set at all. 9 of 66 on
	// 2026-11-22, the ninth being ranger-base-x1ofd's parameter-route message,
	// which was 8 of 65 before that route landed — and the Logf LISTS them,
	// which the head comment's own copy of that list could not do and went
	// stale for (ranger-base-yem73).
	t.Logf("%d member file(s), %d account message(s) graded against %s's %d characters, %d of them rendered inside a t.Helper() and charged `%s:%s: ` (%d) for the caller go test prints instead of their own position: %v; the account itself needs %d (%s), leaving a message %d; the widest head (prefix + literal preamble) is %d at %s.",
		len(files), graded, wmaCheck, budget, len(inHelper), widestBase, strings.Repeat("N", wmaHelperLineDigits), wmaHelperPrefix(widestBase), inHelper, account, how, budget-account, widest, widestAt)
}

// ARM 7 — ARM 6'S READERS SAY WHAT THIS FILE SAYS THEY SAY, hermetically,
// through the same functions arm 6 reads the tree with. Arm 3's reason one arm
// over: a reader that quietly stopped meaning what the prose claims reds here
// with no member set, no check and no venue.
func TestQATheWallMemberBudgetReadersMatchWhatTheyClaim(t *testing.T) {
	t.Parallel()
	// THE PREAMBLE, counted in runes, with the interpolated verbs free.
	for _, c := range []struct {
		name  string
		text  string
		arg   int
		spent int
		ok    bool
	}{
		{"the account is the only argument", "the seat said %s", 0, 14, true},
		{"an earlier verb costs its literal and nothing else", "the seat at %s said %s", 1, 18, true},
		{"an em-dash is ONE character and not three", "a — b %s", 0, 6, true},
		// `100%% done ` is eleven characters of SOURCE and ten of rendered
		// line, and the rendered one is what evidence() slices.
		{"a literal percent is one character", "100%% done %s", 0, 10, true},
		{"a trailing percent that starts no verb is one character", "50% %s", 0, 4, true},
		{"only the first line is counted", "head %s\ntail %s", 0, 5, true},
		{"an account past the first line is not on it", "head\ntail %s", 0, 0, false},
		{"no verb at all", "nothing interpolated here", 0, 0, false},
	} {
		firstLine := c.text
		if i := strings.IndexAny(firstLine, "\n\r"); i >= 0 {
			firstLine = firstLine[:i]
		}
		spent, ok := wmaSpentBefore(firstLine, c.arg)
		if spent != c.spent || ok != c.ok {
			t.Errorf("%s: wmaSpentBefore(%q, %d) = %d,%v want %d,%v — arm 6's preamble reading no longer counts what this file says it counts",
				c.name, firstLine, c.arg, spent, ok, c.spent, c.ok)
		}
	}
	// THE PREFIX go test spends before the message, which is the one term of
	// arm 6's rule this TREE does not witness (mutant 8: replaced by a
	// constant 0, the member set still fits and arm 6 stays green). So it is
	// said here, where a fixture can check it, rather than asserted as a bound
	// nothing can red — ranger-base-8752f's ruling one arm over.
	for _, c := range []struct {
		file string
		line int
		want int
	}{
		// `bwrapseatstorelive_test.go:751: ` — 26 + 1 + 3 + 2.
		{"internal/posse/bwrapseatstorelive_test.go", 751, 32},
		// The basename and never the path: evidence() reads go test's own
		// decoration, which is filepath.Base and a line number. `x_test.go:7: `
		// — 9 + 1 + 1 + 2, whatever the path in front of it.
		{"a/very/deeply/nested/path/x_test.go", 7, 13},
		// And the line number's own digits, which a thousand-line member file
		// spends four of: `x_test.go:1004: ` — 9 + 1 + 4 + 2.
		{"x_test.go", 1004, 16},
	} {
		if got := wmaLinePrefix(c.file, c.line); got != c.want {
			t.Errorf("wmaLinePrefix(%q, %d) = %d, want %d — arm 6 no longer charges a message for `<basename>:<line>: `, which go test writes and evidence() keeps because `line.strip()` strips indentation and nothing else. That is 261 of ranger-base-mklu0's 300 arriving before its account.",
				c.file, c.line, got, c.want)
		}
	}
	// THE SAME TERM FOR A HELPER-RENDERED MESSAGE, which is the one go test
	// decorates with the CALLING file — so it is charged to the widest
	// basename in the member set and not to the file the message is written in
	// (ranger-base-q41e8). This tree witnesses it no better than it witnesses
	// the term above: a mutant answering "not a helper" everywhere charges the
	// eight such messages 27 to 41 where the decision charges 44, and arm 6 is
	// green at both. So the reading and the arithmetic are both here.
	//
	// THE READING, over one planted file carrying all five shapes. The fourth
	// and fifth are the pair that decided it: t.Helper() marks the function
	// that CALLS it, so a closure inside a helper is an ordinary frame and a
	// closure that marks itself is a helper wherever it sits.
	helperSrc := `func writtenHere(t *testing.T) {
	t.Logf("PLAIN FUNC %s", "x")
}

func marked(t *testing.T) {
	t.Helper()
	t.Logf("MARKED FUNC %s", "x")
}

func markedWithAClosure(t *testing.T) {
	t.Helper()
	run := func() {
		t.Logf("CLOSURE INSIDE A HELPER %s", "x")
	}
	run()
}

func TestPlanted(t *testing.T) {
	run := func(tb testing.TB) {
		tb.Helper()
		tb.Logf("CLOSURE THAT MARKS ITSELF %s", "x")
	}
	run(t)
	t.Logf("BESIDE THAT CLOSURE %s", "x")
}`
	_, hf := wmaFixture(t, helperSrc, "")
	spans := wmaFuncSpans(hf.ast)
	callAt := func(marker string) (token.Pos, bool) {
		var at token.Pos
		found := false
		ast.Inspect(hf.ast, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) == 0 {
				return true
			}
			if s, isStr := wmaStringLit(c.Args[0]); isStr && strings.Contains(s, marker) {
				at, found = c.Pos(), true
			}
			return true
		})
		return at, found
	}
	for _, c := range []struct {
		marker string
		helper bool
		in     string
		why    string
	}{
		{"PLAIN FUNC", false, "writtenHere", "a message in a function that marks nothing is printed at its own file and line"},
		{"MARKED FUNC", true, "marked", "a message in a t.Helper() is printed at its caller's"},
		{"CLOSURE INSIDE A HELPER", false, "a func literal", "a closure inside a helper is its OWN frame — it did not call t.Helper(), so go test stops at it"},
		{"CLOSURE THAT MARKS ITSELF", true, "a func literal", "a closure that calls t.Helper() is a helper frame, and the receiver's name is not the key — this one is `tb`"},
		{"BESIDE THAT CLOSURE", false, "TestPlanted", "the test body beside a helper closure is not itself a helper. This is wallwatchlive_test.go:614, the one message the sloppier `a Helper() call anywhere under the enclosing FuncDecl` reading adds"},
	} {
		at, ok := callAt(c.marker)
		if !ok {
			t.Fatalf("the planted helper fixture no longer carries a %q message, so this row grades nothing:\n%s", c.marker, helperSrc)
		}
		helper, in := wmaRenderedInHelper(spans, at)
		if helper != c.helper || in != c.in {
			t.Errorf("%q: wmaRenderedInHelper = %v,%q want %v,%q — %s. Arm 6 charges a helper-rendered message the widest basename in the member set and every other message its own, so a reading that picks wrong charges the wrong file either way: too few characters for a helper's line (27 to 41 against 44 over the member set on 2026-11-22) or a basename that never appears on the line for anything else.",
				c.marker, helper, in, c.helper, c.in, c.why)
		}
	}
	// THE WIDEST BASENAME, derived from a planted set and never typed, and the
	// prefix it charges: 25 + `:` + four digits + `: `.
	planted := []wmaFile{
		{path: "internal/posse/short_test.go"},
		{path: "a/deep/path/the_widest_member_test.go"},
		{path: "internal/posse/a_middling_name_test.go"},
	}
	if base, chars := wmaWidestMemberBase(planted); base != "the_widest_member_test.go" || chars != 25 {
		t.Errorf("wmaWidestMemberBase over a planted set = %q,%d want %q,25 — arm 6's helper prefix charges the widest BASENAME in the member set and the path in front of it is not part of what go test prints, so a reader comparing whole paths would charge the deepest file instead of the widest-named one.", base, chars, "the_widest_member_test.go")
	}
	if got := wmaHelperPrefix("the_widest_member_test.go"); got != 32 {
		t.Errorf("wmaHelperPrefix(%q) = %d, want 32 — 25 for the basename, 1 for `:`, %d for the line number the decision charges and 2 for `: `. The four digits are the member set's own shape (its longest file is past 1200 lines) and the widest basename is ranger-base-x1ofd's decision, which is a FLOOR: a line that fits at its own caller and not at the widest is one caller away from being cut.",
			"the_widest_member_test.go", got, wmaHelperLineDigits)
	}
	// AND THE BRANCH BETWEEN THE TWO, over a planted message, because that is
	// the half arm 6 cannot red: both charges fit the member set today, so a
	// mutant that takes the helper arm out leaves arm 6 green and the term
	// quietly wrong by 14 characters.
	for _, c := range []struct {
		name  string
		msg   wmaMsg
		chars int
		names string
	}{
		{"a message rendered where it is written is charged its own position",
			wmaMsg{file: "internal/posse/mid_test.go", line: 751}, 17, "mid_test.go:751: "},
		{"a helper-rendered message is charged the widest basename and a four-digit line",
			wmaMsg{file: "internal/posse/mid_test.go", line: 751, helper: true, helperIn: "aHelper"}, 32, "the_widest_member_test.go:NNNN: "},
	} {
		chars, how := wmaChargedPrefix(c.msg, "the_widest_member_test.go")
		if chars != c.chars || !strings.HasPrefix(how, c.names) {
			t.Errorf("%s: wmaChargedPrefix = %d,%q want %d and a report opening %q — arm 6 adds this term to every message's preamble, and it is charged to the file `go test` will PRINT: testing's decorate() writes the first NON-helper frame, so a helper's own basename is a file that never appears on the line (ranger-base-q41e8).",
				c.name, chars, how, c.chars, c.names)
		}
	}
	// THE ACCOUNT'S FIXED WIDTH, read out of a planted bgSeatAccount: the
	// silence branch and never the words branch, at the shortest status the
	// source spells.
	fixed := `func wwOneLine(s string) string { return s }
func bgSeatAccount(err error, out string) string {
	status := "exit 0"
	if err != nil {
		status = "long status from the error"
	}
	if len(out) == 0 {
		return status + ", and the seat said NOTHING"
	}
	return status + ", and the seat said " + wwOneLine(out)
}`
	_, f := wmaFixture(t, fixed, "")
	if got, _, err := wmaReadFixedAccount([]wmaFile{f}); err != nil || got != 33 {
		t.Errorf("wmaFixedAccount over a planted %s reads %d, want 33 — `exit 0` (6) plus a 27-character silence tail. Arm 6 must read the SILENCE branch at the SHORTEST status the source spells: the words branch ends in a fold of whatever the seat said and has no width to read, and a longer status is not a floor.", wmaAccountFunc, got)
	}
	// And the two shapes it must refuse rather than guess at: a fixed-width
	// answer built some other way, and a status with no literal to floor.
	for _, c := range []struct{ name, src string }{
		{"a Sprintf answer has no concatenation to read", `func bgSeatAccount(err error, out string) string {
	status := "exit 0"
	return fmt.Sprintf("%s, and the seat said NOTHING", status)
}`},
		{"a status with no string literal cannot be floored", `func bgSeatAccount(err error, out string) string {
	status := statusOf(err)
	return status + ", and the seat said NOTHING"
}`},
	} {
		_, g := wmaFixture(t, c.src, "")
		if got, _, err := wmaReadFixedAccount([]wmaFile{g}); err == nil {
			t.Errorf("%s: wmaReadFixedAccount read %d out of a planted %s it cannot read, instead of refusing. Arm 6 must say so and stop, never guess a width — a guessed width is the number nobody measured that ranger-base-uu303 was filed to avoid.", c.name, got, wmaAccountFunc)
		}
	}
}

// wmaBodyCalls reports whether fn's body calls the named function.
func wmaBodyCalls(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && wmaCallee(c) == name {
			found = true
		}
		return true
	})
	return found
}
