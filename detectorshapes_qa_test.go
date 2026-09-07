package posse

// ranger-base-m0ce1. INCIDENT 2026-09-02: commit 34a27b4
// (memoryland_credshapes_qa_test.go) shaped its Slack fixtures
// xox?-<twelve digits>-<twelve digits>-<sixteen letters> — exactly the shape
// GitHub's own push protection (GH013, "Slack API Token") is built to catch,
// because that is what a real Slack token's workspace/bot segments look
// like. GitHub refused every push of main until the commit was rewritten
// (now 6a230eb) with xox?-AAAA…-AAAA-… bodies. Every other row in that table
// was already letter-only and passed.
//
// This is the pin that stops it recurring anywhere in the tree, not just in
// that one file: GitHub's partner regexes are not published, so the shapes
// below are not an attempt to reproduce them byte for byte — the bar here is
// narrower and does not need to be. A test fixture never needs a REALISTIC
// value to exercise the shape it is testing, so any occurrence of one of
// these vendor prefixes with a DIGIT inside the credential body is
// disallowed outright, wherever it sits in the tracked tree. The letter-only
// AAAA… convention every fixture in this tree already uses never trips it —
// confirmed against the live corpus by the grep this pin is built from.
//
// The Anthropic and GitHub-PAT shapes carry fixed digits in their own
// prefix (sk-ant-apiNN-, the fine-grained PAT's two-part length) that are
// not the credential body; those are outside the capture group this pin
// actually checks, on purpose, so the format itself never fails a fixture
// that already spells it correctly.

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// detectorShapes name the six vendors ranger-base-m0ce1 lists. re's first
// capture group is the credential BODY — the part a real secret's entropy
// lives in and a fixture never needs to imitate.
var detectorShapes = []struct {
	what string
	re   *regexp.Regexp
}{
	// Slack carries no fixed-digit prefix, so the whole body is the check —
	// this is the exact shape 34a27b4 tripped.
	{"a Slack token", regexp.MustCompile(`\bxox[abeprs]-([A-Za-z0-9-]{16,})`)},
	// OpenAI. sk-ant- is a separate, longer prefix handled below; its "ant"
	// is too short to satisfy this pattern's body length on its own, so the
	// two patterns never compete for the same match.
	{"an OpenAI key", regexp.MustCompile(`\bsk-(?:proj-)?([A-Za-z0-9]{20,})`)},
	// Anthropic. apiNN is a fixed part of the real prefix, not the body:
	// sk-ant-api03-AAAA… is the shape every existing fixture already uses,
	// and the digits in "api03" must never be why one fails.
	{"an Anthropic key", regexp.MustCompile(`\bsk-ant-api\d{2}-([A-Za-z0-9_-]{16,})`)},
	{"a GitHub token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_([A-Za-z0-9]{20,})`)},
	// The fine-grained PAT's real shape is two underscore-separated groups
	// of exact length. Anchoring to that shape, rather than a loose run off
	// the prefix, is what keeps this from flagging the shorter fixtures
	// this tree already uses that merely start with github_pat_.
	{"a GitHub token", regexp.MustCompile(`\bgithub_pat_([A-Za-z0-9]{22}_[A-Za-z0-9]{59})\b`)},
	{"an AWS access key id", regexp.MustCompile(`\b(?:AKIA|ASIA)([0-9A-Z]{16})\b`)},
	{"a Linear key", regexp.MustCompile(`\blin_api_([A-Za-z0-9]{20,})`)},
}

// hasDigit is the whole rule: a credential body with a digit in it is what
// reads as realistic to a detector built to catch real ones.
var hasDigit = regexp.MustCompile(`[0-9]`)

// awsPublishedExamples are AWS's own documentation placeholders. ...EXAMPLE
// is AWS's own convention for "this is not a real key" — published in AWS's
// docs, not invented here — and this tree already uses it
// (memoryland_credshapes_qa_test.go), so it is exempt by literal value
// rather than by loosening the AWS shape for everyone else.
var awsPublishedExamples = map[string]bool{
	"AKIAIOSFODNN7EXAMPLE": true,
	"ASIAIOSFODNN7EXAMPLE": true,
}

// pinSourceFile is this file's own path. TestTheDigitRealisticScanIsNotVacuous
// below plants a real digit-realistic example to prove the scan fires, and
// the tree scan must skip this one file or its own control would flag itself.
const pinSourceFile = "detectorshapes_qa_test.go"

func trackedFiles(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v — this pin needs the checkout to name what posse ships", err)
	}
	files := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var keep []string
	for _, f := range files {
		if f != "" {
			keep = append(keep, f)
		}
	}
	if len(keep) < 100 {
		t.Fatalf("git tracks %d files — the listing failed, so a clean result here is not evidence", len(keep))
	}
	return keep
}

// digitRealisticHits scans body for detectorShapes matches whose credential
// BODY carries a digit, skipping the AWS published examples by literal
// value. Shared between the tree scan and its own control so both measure
// the same rule.
func digitRealisticHits(body []byte) []string {
	var hits []string
	for _, shape := range detectorShapes {
		for _, loc := range shape.re.FindAllSubmatchIndex(body, -1) {
			whole := string(body[loc[0]:loc[1]])
			cred := string(body[loc[2]:loc[3]])
			if awsPublishedExamples[whole] {
				continue
			}
			if hasDigit.MatchString(cred) {
				line := strings.Count(string(body[:loc[0]]), "\n") + 1
				hits = append(hits, fmt.Sprintf("line %d: %s %q looks digit-realistic", line, shape.what, whole))
			}
		}
	}
	return hits
}

func TestNoTrackedFileCarriesADigitRealisticDetectorShape(t *testing.T) {
	var findings []string
	for _, rel := range trackedFiles(t) {
		if rel == pinSourceFile {
			continue
		}
		body, err := os.ReadFile(rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, hit := range digitRealisticHits(body) {
			findings = append(findings, rel+":"+hit)
		}
	}
	if len(findings) > 0 {
		t.Errorf("%d tracked file(s) carry a digit-realistic detector shape — GitHub's own push "+
			"protection blocks the push of the WHOLE branch on exactly this (ranger-base-m0ce1, "+
			"commit 34a27b4 / 6a230eb). Use a letter-only body — the AAAA… convention every other "+
			"fixture in this tree already uses:\n  %s",
			len(findings), strings.Join(findings, "\n  "))
	}
}

// The control: the shared matcher must actually fire on the shape that
// caused the incident, and must not fire on the letter-only convention the
// fix landed in, or a clean result above is a clean result over a scan that
// looks at nothing (or one that would hold every future commit).
func TestTheDigitRealisticScanIsNotVacuous(t *testing.T) {
	planted := []byte("SLACK_BOT_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnop")
	if hits := digitRealisticHits(planted); len(hits) == 0 {
		t.Fatal("control: the digit-realistic Slack shape that blocked 34a27b4 does not trip the scan — every clean verdict above is empty")
	}

	safe := []byte("SLACK_BOT_TOKEN=xoxb-AAAAAAAAAAAA-AAAAAAAAAAAA-abcdefghijklmnop")
	if hits := digitRealisticHits(safe); len(hits) != 0 {
		t.Errorf("control: the letter-only body every fixture in this tree uses reads as digit-realistic (%v) — this scan would hold every commit", hits)
	}

	example := []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE")
	if hits := digitRealisticHits(example); len(hits) != 0 {
		t.Errorf("control: AWS's own published EXAMPLE key reads as a finding (%v) — it is a documented placeholder, not a realistic body", hits)
	}
}
