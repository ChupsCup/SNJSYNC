package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func regexCodeMatches(s string) [][2]string {
	var out [][2]string
	for _, m := range reCodePattern.FindAllStringSubmatch(s, -1) {
		if len(m) >= 3 {
			out = append(out, [2]string{m[1], m[2]})
		}
	}
	return out
}

func regexSpecMatches(s string) [][2]string {
	var out [][2]string
	for _, m := range reSpecPattern.FindAllStringSubmatch(s, -1) {
		if len(m) >= 3 {
			out = append(out, [2]string{m[1], m[2]})
		}
	}
	return out
}

func eqMatches(a, b [][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustMatchScan(t *testing.T, s string) {
	t.Helper()
	if got, want := scanCodePattern(s), regexCodeMatches(s); !eqMatches(got, want) {
		t.Fatalf("scanCodePattern(%q) = %v, regex = %v", s, got, want)
	}
	if got, want := scanSpecPattern(s), regexSpecMatches(s); !eqMatches(got, want) {
		t.Fatalf("scanSpecPattern(%q) = %v, regex = %v", s, got, want)
	}
}

func TestScanPatternSetaraRegex(t *testing.T) {
	cases := []string{
		"", " ", "fq11", "FQ-11", "fq 11", "hq41", "cm-01", "cm001", "fq11b",
		"95cl", "90vl", "65cl", "18kg", "5.2kg", "0x1a2b3c",
		"a1", "ab12", "abc123", "abcd1234", "abcd12345", "abc123456",
		"1234abcd", "12345abc", "123abc", "12ab", "1a",
		"abc_def12", "abc-def12", "a-b", "a - b", "a -b", "a- b",
		"310010-14.074in.00999 slice (amber)---fq11 bkr---est. in",
		"120080-14.146in.03675 topside (black gems)---hq41 bkr---est. in",
		"swift - bnls beef short plate st (vacuum pack) - 31718 - est 969",
		"50 cl est 239", "est.164 in", "establishment 12345",
		"1. box putih", "allana buffalo fq-11 @18 kg (trays slice 900gr) - bkr",
		"  spaced   fq - 11  ", "fq-11b", "@18kg ppi", "kwh1234",
		"123456789012345", "abc", "123", "-", "_", "___", "a1b2c3d4e5",
		"rib eye slice 1kg", "daging kita (aov) --- cm01 ppi--- est. in",
		"900gr", "350 gr", "est 969g", "58446 - 8247", "0x1",
	}
	for _, s := range cases {
		mustMatchScan(t, s)
	}

	rnd := rand.New(rand.NewSource(42))
	alpha := []byte("abcxy019-_. ()")
	for i := 0; i < 40000; i++ {
		n := rnd.Intn(24)
		b := make([]byte, n)
		for j := range b {
			b[j] = alpha[rnd.Intn(len(alpha))]
		}
		mustMatchScan(t, string(b))
	}
}

func TestScanPatternDataAsli(t *testing.T) {
	root := "D:/Website/SNJSYNC"
	if v := os.Getenv("CTF_ROOT"); v != "" {
		root = v
	}
	raw, err := os.ReadFile(root + "/data/photo_list.json")
	if err != nil {
		t.Skip("photo_list.json tidak ada: " + err.Error())
	}
	var pl struct {
		Files []PhotoEntry `json:"files"`
	}
	if err := json.Unmarshal(raw, &pl); err != nil {
		t.Fatal(err)
	}
	strs := make([]string, 0, len(pl.Files)*4)
	for _, e := range pl.Files {
		var bc []string
		for _, b := range e.Barcodes {
			bc = append(bc, b.Text)
		}
		strs = append(strs,
			normText(e.Dir), normText(e.Name), normText(e.Text),
			normText(joinForTest(bc)))
	}
	for i, s := range strs {
		if got, want := scanCodePattern(s), regexCodeMatches(s); !eqMatches(got, want) {
			t.Fatalf("entry #%d code scan=%v regex=%v string=%.120q", i, got, want, s)
		}
		if got, want := scanSpecPattern(s), regexSpecMatches(s); !eqMatches(got, want) {
			t.Fatalf("entry #%d spec scan=%v regex=%v string=%.120q", i, got, want, s)
		}
	}
	fmt.Printf("setara: %d string dari %d entri\n", len(strs), len(pl.Files))
}

func joinForTest(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

func regexEsts(values ...string) []string {
	out := newSet()
	for _, t := range values {
		s := strings.ToLower(t)
		for _, m := range reEstNo.FindAllStringSubmatch(s, -1) {
			out.Add(stripLeadingZeros(m[1]))
		}
		for _, m := range reEstWord.FindAllStringSubmatch(s, -1) {
			out.Add(stripLeadingZeros(m[1]))
		}
	}
	return out.List()
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqStringSet(a, b []string) bool {
	ma, mb := map[string]bool{}, map[string]bool{}
	for _, v := range a {
		ma[v] = true
	}
	for _, v := range b {
		mb[v] = true
	}
	if len(ma) != len(mb) {
		return false
	}
	for v := range ma {
		if !mb[v] {
			return false
		}
	}
	return true
}

func TestExtractEstsSetaraRegex(t *testing.T) {
	cases := [][]string{
		{"est 12345", "EST.969 US", "est no. 12", "est no 12", "establishment 12345"},
		{"est 1234567", "x establishment   42 y", "best 99", "est 0007", "est"},
		{"2. est 969/swift", "est. 164 in", "establishment___164", "establishment__________164"},
		{"establishment___________164", "a est - 7 b", "est  0", "no est"},
		{"FOTO PRODUCT SNJ GROUP/SAPI/SAPI 2024/SWIFT - USA/1. EST 969/SWIFT - BNLS BEEF SHORT PLATE ST ( VACUUM PACK ) - 31718 - EST 969"},
	}
	for _, cs := range cases {
		got, want := extractEsts(cs...), regexEsts(cs...)
		if !eqStringSet(got, want) {
			t.Fatalf("extractEsts(%q) = %v, regex = %v", cs, got, want)
		}
	}
	rnd := rand.New(rand.NewSource(7))
	alpha := []byte("estno. 019_-abc")
	for i := 0; i < 20000; i++ {
		n := rnd.Intn(40)
		b := make([]byte, n)
		for j := range b {
			b[j] = alpha[rnd.Intn(len(alpha))]
		}
		s := string(b)
		got, want := extractEsts(s), regexEsts(s)
		if !eqStringSet(got, want) {
			t.Fatalf("extractEsts(%q) = %v, regex = %v", s, got, want)
		}
	}
}

func TestDigitChunksSetaraRegex(t *testing.T) {
	check := func(s string) {
		t.Helper()
		if got, want := digitChunks(s, 1, 6), reDigits1to6.FindAllString(s, -1); !eqStrings(got, want) {
			t.Fatalf("digitChunks(1,6)(%q) = %v, regex = %v", s, got, want)
		}
		if got, want := digitChunks(s, 7, 15), reDigits7to15.FindAllString(s, -1); !eqStrings(got, want) {
			t.Fatalf("digitChunks(7,15)(%q) = %v, regex = %v", s, got, want)
		}
	}
	for _, s := range []string{
		"", "1", "1234567", "1234567890123456", "12345678901234567890",
		"a1b12c123d1234e123456f", "12 345 67890", "abc", "0000000",
	} {
		check(s)
	}
	rnd := rand.New(rand.NewSource(9))
	alpha := []byte("0123456789abc ")
	for i := 0; i < 20000; i++ {
		n := rnd.Intn(30)
		b := make([]byte, n)
		for j := range b {
			b[j] = alpha[rnd.Intn(len(alpha))]
		}
		check(string(b))
	}
}
