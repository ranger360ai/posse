package posse

// ADR 0072 — bd 0.50.3 reads a store's issues.jsonl through a 64 KB
// bufio.Scanner and can neither delete nor edit a comment, so the write
// that pushes one issue's line over 65,536 bytes is the last point anyone
// holds a remedy bd can perform. This file is the one judge both the
// write-time refusal (D3, below, wired into Bd.Comment/SetDescription/
// Create/Close) and the read-time report (D4, wired into Bd.runOnce/run)
// consult, and it is the judge the bd-shim arm (ADR 0072 B2, not built
// here) will call too.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BdLineLimit is MEASURED (docs/adr/0072-bd-line-cliff.probe.sh): a 65,536
// byte line, newline included, is the last one `bd --no-daemon list --json`
// reads; 65,537 fails every verb, including the one shrink bd has
// (`update -d <short>` in no-db mode).
const BdLineLimit = 65536

// BdLineHeadroom is ASSUMED (ADR 0072 D1): it covers unflushed DB writes
// the census does not yet show, and bd reordering a row's fields — or
// dropping an empty `labels`/`dependencies` it would otherwise omit — when
// it rewrites the line. A write is refused once its projection crosses
// BdLineLimit - BdLineHeadroom, never at BdLineLimit itself.
const BdLineHeadroom = 2048

// BdLineWarn is G16's lane threshold (ADR 0072 D5, built in ranger-base-m7e0d):
// a line past this many bytes is worth a status row before it is worth a
// refusal.
const BdLineWarn = 49152

// LongLine is one census line CensusLongLines found past its threshold.
type LongLine struct {
	Line  int    // 1-based line number in the census
	ID    string // from the line's `{"id":"<id>",` prefix; "" if it has none
	Bytes int    // the line's length, newline included
}

// CensusLongLines scans path (an issues.jsonl census) for every line longer
// than over bytes. It reads with bufio.Reader.ReadSlice and no size limit —
// the same idiom censusHasRecord uses — because a bead's description or a
// comment makes a line of unbounded length, and that is exactly the shape
// bd's own bufio.Scanner cannot read past 64 KB. MEASURED cost: ~2.3 ms over
// a 22.9 MB, 2,963-line census (ADR 0072 Context).
func CensusLongLines(path string, over int) ([]LongLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 4096)
	var out []LongLine
	lineNo := 0
	for {
		first, rerr := r.ReadSlice('\n')
		n := len(first)
		id := censusLineID(first)
		for rerr == bufio.ErrBufferFull {
			var more []byte
			more, rerr = r.ReadSlice('\n')
			n += len(more)
		}
		if n == 0 && rerr != nil {
			break
		}
		lineNo++
		if n > over {
			out = append(out, LongLine{Line: lineNo, ID: id, Bytes: n})
		}
		if rerr != nil {
			break
		}
	}
	return out, nil
}

// censusLineID reads a census line's id from its `{"id":"<id>",` prefix —
// every live row begins with it (ADR 0072 Context: all 2,963 lines on the
// store of record). "" for a line that does not, rather than guessing.
func censusLineID(line []byte) string {
	const prefix = `{"id":"`
	if !bytes.HasPrefix(line, []byte(prefix)) {
		return ""
	}
	rest := line[len(prefix):]
	end := bytes.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return string(rest[:end])
}

// censusExactLine returns the full bytes (newline included) of the one
// census line whose `{"id":"<id>",` prefix matches id exactly, reading only
// as much of every OTHER line as it takes to find its newline (the same
// "skip without allocating" idiom censusHasRecord uses). ok is false, with
// no error, when the census does not exist or holds no such line.
func censusExactLine(census, id string) (line []byte, ok bool, err error) {
	f, err := os.Open(census)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()
	want := []byte(`{"id":"` + id + `",`)
	r := bufio.NewReaderSize(f, 4096)
	for {
		chunk, rerr := r.ReadSlice('\n')
		match := bytes.HasPrefix(chunk, want)
		var full []byte
		if match {
			full = append(full, chunk...)
		}
		for rerr == bufio.ErrBufferFull {
			chunk, rerr = r.ReadSlice('\n')
			if match {
				full = append(full, chunk...)
			}
		}
		if match {
			return full, true, nil
		}
		if rerr != nil {
			return nil, false, nil
		}
	}
}

// Projection is what a bd write would do to one census line: ID is "" for a
// `create` (bd has not assigned one yet), Before is 0 for the same reason,
// and After is the byte length (newline included) ProjectBdWrite computed
// the write would leave behind.
type Projection struct {
	ID     string
	Before int
	After  int
}

// bdComment is bd's own comment object (field order and names MEASURED
// against a real write, ADR 0072 Context) — id, issue_id, author, text,
// created_at, in that order.
type bdComment struct {
	ID        int    `json:"id"`
	IssueID   string `json:"issue_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// bdWorstCommentID is a placeholder for the comment id bd assigns — the
// next value of a store-wide auto-increment sequence ProjectBdWrite cannot
// see without asking bd (which would cost an exec per judged write). Nine
// digits is generous against any real store's count; BdLineHeadroom covers
// the rest.
const bdWorstCommentID = 999999999

// bdWorstNewID and bdWorstActor are the same kind of placeholder for a
// `create`'s not-yet-assigned id and its owner/created_by fields: long
// enough that a real value is never longer, so the projection never
// UNDERestimates — the direction that would let a crossing write through.
const (
	bdWorstNewID = "xxxxxxxxxxxxxxxxxxxxxxxx"         // 24 bytes
	bdWorstActor = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" // 32 bytes
)

// bdWorstTimestamp is bd's own timestamp layout at its longest: nine
// fractional digits — "000000000" never trims, unlike RFC3339Nano's
// "999999999" — and a numeric zone offset ("+00:00", 6 bytes) rather than
// "Z" (1 byte). 35 bytes, regardless of this box's actual clock or zone
// (MEASURED shape: a no-db write here produced
// "2026-10-12T08:24:40.540859-04:00"; this is that shape at its widest).
func bdWorstTimestamp() string {
	return time.Now().Format("2006-01-02T15:04:05.000000000-07:00")
}

// bdEncodeCompact marshals v the way bd's own encoder does — compact,
// HTML-escaping OFF (MEASURED: a literal `<` survives, `\n`/`\t`/`"` cost
// two bytes each) — and trims the trailing newline json.Encoder.Encode
// always appends.
func bdEncodeCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// bdConsumeFlag reports whether args[i] is one of names, in bd's own
// spellings: the separate-word form (`--description value`) and, for every
// long (`--`) name, the `=` form (`--description=value`). A match whose
// separate-word form has no following word to consume reports matched=true
// with next=-1, which every caller reads as "do not judge" rather than
// guessing a value that is not there.
func bdConsumeFlag(args []string, i int, names ...string) (value string, next int, matched bool) {
	tok := args[i]
	for _, n := range names {
		if tok == n {
			if i+1 >= len(args) {
				return "", -1, true
			}
			return args[i+1], i + 2, true
		}
		if strings.HasPrefix(n, "--") && strings.HasPrefix(tok, n+"=") {
			return tok[len(n)+1:], i + 1, true
		}
	}
	return "", i, false
}

// bdIsStdinPath matches ADR 0069 D2's rule for what a `-f`/`--file`/
// `--body-file` value is never read as a regular file: stdin itself, or a
// path under /dev or /proc that could be standing in for it.
func bdIsStdinPath(p string) bool {
	return p == "-" || strings.HasPrefix(p, "/dev/") || strings.HasPrefix(p, "/proc/")
}

// bdReadRegularFile reads path the way ADR 0069 D2's shim arm does — only
// when it is a regular file, shell `[ -f ]`'s own test (os.Stat, so a
// symlink is judged by what it resolves to).
func bdReadRegularFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	return os.ReadFile(path)
}

// bdBenignFlags are the boolean, no-value options our own callers' argv
// carries that never change a projected line's length — today just the
// `--json` every posse call adds. Anything else unrecognized makes the
// parser bail (ok=false) rather than guess an arity it was not told.
var bdBenignFlags = map[string]bool{"--json": true}

// parseCommentAddArgs parses `comments add`'s own argv (bd's global options
// already stripped, and "add" itself already consumed): the id, then
// either a positional text or a `-f`/`--file`/`--body-file` source, plus an
// optional `-a`/`--author`. ok is false — do not judge — for anything this
// cannot resolve to an actual text value: an unrecognized flag, a missing
// flag value, a file that is not a regular one, or stdin.
func parseCommentAddArgs(args []string) (id, text, author string, ok bool) {
	var file string
	haveFile := false
	var positional []string
	i := 0
	for i < len(args) {
		if v, next, m := bdConsumeFlag(args, i, "-f", "--file", "--body-file"); m {
			if next < 0 {
				return "", "", "", false
			}
			file, haveFile, i = v, true, next
			continue
		}
		if v, next, m := bdConsumeFlag(args, i, "-a", "--author"); m {
			if next < 0 {
				return "", "", "", false
			}
			author, i = v, next
			continue
		}
		tok := args[i]
		if strings.HasPrefix(tok, "-") && tok != "-" {
			return "", "", "", false
		}
		positional = append(positional, tok)
		i++
	}
	if len(positional) == 0 {
		return "", "", "", false
	}
	id = positional[0]
	if len(positional) > 1 {
		text = positional[1]
	}
	if haveFile {
		if bdIsStdinPath(file) {
			return "", "", "", false
		}
		data, rerr := bdReadRegularFile(file)
		if rerr != nil {
			return "", "", "", false
		}
		text = string(data)
	}
	return id, text, author, true
}

// bdUpdateTextFlags are `update`'s text-bearing flags, bd's own flag name
// to the JSON key it writes (MEASURED: `--acceptance` lands as
// "acceptance_criteria", every other one keeps its flag's own name).
var bdUpdateTextFlags = []struct {
	names []string
	key   string
}{
	{[]string{"-d", "--description"}, "description"},
	{[]string{"--design"}, "design"},
	{[]string{"--notes"}, "notes"},
	{[]string{"--acceptance"}, "acceptance_criteria"},
	{[]string{"--title"}, "title"},
}

// parseUpdateArgs parses `update`'s own argv for the fields ADR 0072 D1
// names: the id, plus whatever of -d/--description/--design/--notes/
// --acceptance/--title/--body-file it sets. sets is empty when the call
// touches none of them (e.g. a bare `--priority` edit) — not a reason to
// refuse the parse, just nothing for the line-cliff judge to project.
func parseUpdateArgs(args []string) (id string, sets map[string]string, ok bool) {
	sets = map[string]string{}
	var positional []string
	i := 0
	for i < len(args) {
		matched := false
		for _, f := range bdUpdateTextFlags {
			if v, next, m := bdConsumeFlag(args, i, f.names...); m {
				if next < 0 {
					return "", nil, false
				}
				sets[f.key] = v
				i = next
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if v, next, m := bdConsumeFlag(args, i, "--body-file"); m {
			if next < 0 {
				return "", nil, false
			}
			if bdIsStdinPath(v) {
				return "", nil, false
			}
			data, rerr := bdReadRegularFile(v)
			if rerr != nil {
				return "", nil, false
			}
			sets["description"] = string(data)
			i = next
			continue
		}
		tok := args[i]
		if bdBenignFlags[tok] {
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			return "", nil, false
		}
		positional = append(positional, tok)
		i++
	}
	if len(positional) == 0 {
		return "", nil, false
	}
	return positional[0], sets, true
}

// parseCloseArgs parses `close`'s own argv for its id and an optional
// `-r`/`--reason`.
func parseCloseArgs(args []string) (id, reason string, ok bool) {
	var positional []string
	i := 0
	for i < len(args) {
		if v, next, m := bdConsumeFlag(args, i, "-r", "--reason"); m {
			if next < 0 {
				return "", "", false
			}
			reason, i = v, next
			continue
		}
		tok := args[i]
		if bdBenignFlags[tok] {
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			return "", "", false
		}
		positional = append(positional, tok)
		i++
	}
	if len(positional) == 0 {
		return "", "", false
	}
	return positional[0], reason, true
}

// bdCreateValueFlags are `create`'s other recognized value-taking options —
// the ones BdNew builds (assignee, labels, deps, priority, type) — skipped
// rather than projected, because none of them carries unbounded text.
var bdCreateValueFlags = []string{"-a", "--assignee", "-l", "--labels", "--deps", "-p", "--priority", "-t", "--type"}

// parseCreateArgs parses `create`'s own argv: the title (bd's one
// positional) and a description, from `-d`/`--description` or
// `--body-file`. `-f`/`--file` is bd's OTHER meaning on this verb — create
// multiple issues from a markdown file — a different shape this does not
// model, so it bails rather than guess.
func parseCreateArgs(args []string) (title, desc string, ok bool) {
	var positional []string
	i := 0
	for i < len(args) {
		if v, next, m := bdConsumeFlag(args, i, "-d", "--description"); m {
			if next < 0 {
				return "", "", false
			}
			desc, i = v, next
			continue
		}
		if v, next, m := bdConsumeFlag(args, i, "--body-file"); m {
			if next < 0 {
				return "", "", false
			}
			if bdIsStdinPath(v) {
				return "", "", false
			}
			data, rerr := bdReadRegularFile(v)
			if rerr != nil {
				return "", "", false
			}
			desc, i = string(data), next
			continue
		}
		tok := args[i]
		if tok == "-f" || tok == "--file" || strings.HasPrefix(tok, "--file=") {
			return "", "", false
		}
		if _, next, m := bdConsumeFlag(args, i, bdCreateValueFlags...); m {
			if next < 0 {
				return "", "", false
			}
			i = next
			continue
		}
		if bdBenignFlags[tok] {
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			return "", "", false
		}
		positional = append(positional, tok)
		i++
	}
	if len(positional) == 0 {
		return "", "", false
	}
	return positional[0], desc, true
}

// stripBdGlobalOpts skips the global options bd accepts before the verb —
// globalValueOpts["bd"]'s pairs, and every other leading "-token" singly,
// the same walk renderShim's own matcher does in sh — and reports the
// `--actor` value, if any: the audit actor a comment's author defaults to
// when the verb names none of its own (ADR 0072 D1, MEASURED).
func stripBdGlobalOpts(argv []string) (rest []string, actor string) {
	pairs := globalValueOpts["bd"]
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			break
		}
		matched := false
		for _, name := range pairs {
			if tok == name {
				if i+1 >= len(argv) {
					return argv[i+1:], actor
				}
				if name == "--actor" {
					actor = argv[i+1]
				}
				i += 2
				matched = true
				break
			}
			if strings.HasPrefix(tok, name+"=") {
				if name == "--actor" {
					actor = tok[len(name)+1:]
				}
				i++
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		i++
	}
	return argv[i:], actor
}

// bdApplyFieldWrite re-encodes census's exact line for id with every key in
// sets replaced (or added, for a key the row did not carry yet), bd's
// encoder's way: untouched fields are carried as json.RawMessage, verbatim,
// so this never re-derives bytes bd itself did not just rewrite.
func bdApplyFieldWrite(census, id string, sets map[string]string) (Projection, bool, error) {
	if id == "" || len(sets) == 0 {
		return Projection{}, false, nil
	}
	line, ok, err := censusExactLine(census, id)
	if err != nil {
		return Projection{}, false, err
	}
	if !ok {
		return Projection{}, false, nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(line, &m) != nil {
		return Projection{}, false, nil
	}
	for key, val := range sets {
		raw, eerr := bdEncodeCompact(val)
		if eerr != nil {
			return Projection{}, false, eerr
		}
		m[key] = json.RawMessage(raw)
	}
	after, eerr := bdEncodeCompact(m)
	if eerr != nil {
		return Projection{}, false, eerr
	}
	return Projection{ID: id, Before: len(line), After: len(after) + 1}, true, nil
}

// bdApplyComment re-encodes census's exact line for id with one more
// comment appended to its `comments` array (absent today, bd omits an
// empty one — MEASURED), built with a worst-case id, timestamp and the
// given author/text.
func bdApplyComment(census, id, text, author string) (Projection, bool, error) {
	if id == "" {
		return Projection{}, false, nil
	}
	line, ok, err := censusExactLine(census, id)
	if err != nil {
		return Projection{}, false, err
	}
	if !ok {
		return Projection{}, false, nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(line, &m) != nil {
		return Projection{}, false, nil
	}
	var existing []json.RawMessage
	if raw, has := m["comments"]; has && len(bytes.TrimSpace(raw)) > 2 {
		if json.Unmarshal(raw, &existing) != nil {
			return Projection{}, false, nil
		}
	}
	cRaw, eerr := bdEncodeCompact(bdComment{
		ID:        bdWorstCommentID,
		IssueID:   id,
		Author:    author,
		Text:      text,
		CreatedAt: bdWorstTimestamp(),
	})
	if eerr != nil {
		return Projection{}, false, eerr
	}
	existing = append(existing, json.RawMessage(cRaw))
	arrRaw, eerr := bdEncodeCompact(existing)
	if eerr != nil {
		return Projection{}, false, eerr
	}
	m["comments"] = json.RawMessage(arrRaw)
	after, eerr := bdEncodeCompact(m)
	if eerr != nil {
		return Projection{}, false, eerr
	}
	return Projection{ID: id, Before: len(line), After: len(after) + 1}, true, nil
}

// bdNewRow is the shape of a freshly created issue (MEASURED: `create` with
// no description omits the key entirely, so Description carries
// `omitempty`).
type bdNewRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Priority    int    `json:"priority"`
	IssueType   string `json:"issue_type"`
	Owner       string `json:"owner"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	UpdatedAt   string `json:"updated_at"`
}

// bdApplyCreate projects the row a `create` would write. There is no
// census line to read — bd has not assigned the id yet — so Before is 0 and
// the placeholders above stand in for the handful of bytes that will not be
// exactly these, all well inside BdLineHeadroom.
func bdApplyCreate(title, desc string) (Projection, bool, error) {
	if title == "" {
		return Projection{}, false, nil
	}
	ts := bdWorstTimestamp()
	after, err := bdEncodeCompact(bdNewRow{
		ID:          bdWorstNewID,
		Title:       title,
		Description: desc,
		Status:      "open",
		Priority:    2,
		IssueType:   "task",
		Owner:       bdWorstActor,
		CreatedAt:   ts,
		CreatedBy:   bdWorstActor,
		UpdatedAt:   ts,
	})
	if err != nil {
		return Projection{}, false, err
	}
	return Projection{ID: "", Before: 0, After: len(after) + 1}, true, nil
}

// ProjectBdWrite is ADR 0072 D1's one judge: the line length a bd call
// (argv, global options included) would leave behind, read from census
// (the target's beadsHome/issues.jsonl) rather than asked of bd. ok is
// false — do not judge, let the real call proceed — whenever this cannot
// resolve a definite text value: the census is absent, there is no exact
// id line, the verb or its flags are not one of the ones named here, or the
// text would arrive on bd's stdin.
func ProjectBdWrite(census string, argv []string) (Projection, bool, error) {
	rest, actor := stripBdGlobalOpts(argv)
	if len(rest) == 0 {
		return Projection{}, false, nil
	}
	switch rest[0] {
	case "comments":
		if len(rest) < 2 || rest[1] != "add" {
			return Projection{}, false, nil
		}
		id, text, author, ok := parseCommentAddArgs(rest[2:])
		if !ok {
			return Projection{}, false, nil
		}
		if author == "" {
			author = actor
		}
		if author == "" {
			author = os.Getenv("BD_ACTOR")
		}
		return bdApplyComment(census, id, text, author)
	case "update":
		id, sets, ok := parseUpdateArgs(rest[1:])
		if !ok || len(sets) == 0 {
			return Projection{}, false, nil
		}
		return bdApplyFieldWrite(census, id, sets)
	case "close":
		id, reason, ok := parseCloseArgs(rest[1:])
		if !ok || reason == "" {
			return Projection{}, false, nil
		}
		return bdApplyFieldWrite(census, id, map[string]string{"close_reason": reason})
	case "create":
		title, desc, ok := parseCreateArgs(rest[1:])
		if !ok {
			return Projection{}, false, nil
		}
		return bdApplyCreate(title, desc)
	default:
		return Projection{}, false, nil
	}
}

// refuseIfCrossing is D3's wiring: ProjectBdWrite over argv, refused when
// the projection crosses BdLineLimit-BdLineHeadroom. A judging error or
// ok=false both mean "do not judge" and let the real call proceed — the
// same fail-open posture the shim arm this sits beside takes when ITS
// posse binary has gone missing (ADR 0072 D2): a wall that blocks every
// write because this judge could not read the census is worse than the gap
// BdLineHeadroom and ADR 0072 D4's read-time report already cover.
func (b Bd) refuseIfCrossing(dir string, argv []string) error {
	census := filepath.Join(beadsHome(dir), beadsJSONL)
	proj, ok, err := ProjectBdWrite(census, argv)
	if err != nil || !ok {
		return nil
	}
	if proj.After > BdLineLimit-BdLineHeadroom {
		return &BdLineWouldCrossError{ID: proj.ID, Before: proj.Before, After: proj.After}
	}
	return nil
}

// BdLineWouldCrossError is refuseIfCrossing's refusal (ADR 0072 D3): the
// write posse was about to make would push a census line past bd's 64 KB
// scanner limit. Typed and errors.As-able for the reason every other bd
// error here is: "refused before it happened" and "bd itself failed" are
// different facts.
type BdLineWouldCrossError struct {
	ID     string
	Before int
	After  int
}

func (e *BdLineWouldCrossError) Error() string {
	if e.ID == "" {
		return fmt.Sprintf("refused: this create would write a %d-byte line; bd 0.50.3 cannot read a line over %d, and it cannot delete a comment, so the store would stay unreadable (ADR 0072) — put the bulk in a committed file and comment a pointer to it instead",
			e.After, BdLineLimit)
	}
	return fmt.Sprintf("refused: this write would make %s's line %d bytes; bd 0.50.3 cannot read a line over %d, and it cannot delete a comment, so the store would stay unreadable (ADR 0072) — put the bulk in a committed file or a new bead (`bd create … --deps discovered-from:%s`) and comment the pointer",
		e.ID, e.After, BdLineLimit, e.ID)
}

// IsBdLineWouldCross reports whether err is a refused crossing write.
func IsBdLineWouldCross(err error) bool {
	var le *BdLineWouldCrossError
	return errors.As(err, &le)
}

// BdStoreLineTooLongError is a failed bd call, checked against the census
// rather than against bd's wording (ADR 0072 D4 — ADR 0056 D1's single
// prose tripwire stays single: this never matches "token too long"). Lines
// is CensusLongLines' own report, cheapest-line-number first.
type BdStoreLineTooLongError struct {
	Store string
	Lines []LongLine
	Err   error // the bd failure this wraps
}

func (e *BdStoreLineTooLongError) Error() string {
	l := e.Lines[0]
	return fmt.Sprintf("bd store unreadable: %s line %d (%s) is %d bytes > %d — bd 0.50.3 reads the jsonl through a 64 KB scanner (ADR 0072); posse does not repair it — the operator moves the bulk out of that line by hand",
		e.Store, l.Line, l.ID, l.Bytes, BdLineLimit)
}

func (e *BdStoreLineTooLongError) Unwrap() error { return e.Err }

// IsBdStoreLineTooLong reports whether err is a read refused because its
// census carries a line over bd's scanner limit.
func IsBdStoreLineTooLong(err error) bool {
	var le *BdStoreLineTooLongError
	return errors.As(err, &le)
}

// checkLineCliff is ADR 0072 D4's other half: called over every bd failure
// runOnce is about to return (except the one staleness message run's own
// self-heal must see raw — see run, beads.go). dir is the caller's
// directory; the census is the same store bd itself just failed to read.
func (b Bd) checkLineCliff(dir string, cause error) error {
	census := filepath.Join(beadsHome(dir), beadsJSONL)
	lines, cerr := CensusLongLines(census, BdLineLimit)
	if cerr != nil || len(lines) == 0 {
		return cause
	}
	return &BdStoreLineTooLongError{Store: census, Lines: lines, Err: cause}
}
