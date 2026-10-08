package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func mathLog(v float64) float64 { return math.Log(v) }

var reWS = regexp.MustCompile(`\s+`)

func deaccent(s string) string {
	d := norm.NFKD.String(s)
	var b strings.Builder
	b.Grow(len(d))
	for _, r := range d {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func foldBasic(s string, dashMode bool, quoteMap bool) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range norm.NFKD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r >= 0x2010 && r <= 0x2015, r == 0x2212:
			if dashMode {
				b.WriteRune('-')
			} else {
				b.WriteRune(' ')
			}
		case quoteMap && (r == 0x2018 || r == 0x2019 || r == 0x201c || r == 0x201d):
			b.WriteRune('\'')
		case r == 0x00b7, r == 0x2022:
			b.WriteRune(' ')
		case r < 0x20 || r > 0x7e:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normStr(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(foldBasic(s, true, false), " "))
}

func normText(s string) string {
	return foldBasic(s, true, true)
}

func batchNorm(s string) string {
	return foldBasic(s, false, false)
}

var reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func aliasTokenize(s string) string {
	t := reNonAlnum.ReplaceAllString(foldBasic(s, false, false), " ")
	return t
}

func splitFields(s string, isSep func(rune) bool) []string {
	out := make([]string, 0, 8)
	cur := strings.Builder{}
	has := false
	for _, r := range s {
		if isSep(r) {
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
			continue
		}
		cur.WriteRune(r)
		has = true
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

func isNonAlnum(r rune) bool {
	return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
}

func splitAlphaNum(s string) []string { return splitFields(s, isNonAlnum) }

func trimNonAlnum(s string) string {
	rs := []rune(s)
	for len(rs) > 0 && !isAlnumLower(rs[0]) {
		rs = rs[1:]
	}
	for len(rs) > 0 && !isAlnumLower(rs[len(rs)-1]) {
		rs = rs[:len(rs)-1]
	}
	return string(rs)
}

func isAlnumLower(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func normKode(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f', 0x00a0, 0xfeff, 0x2007, 0x202f:
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

func normBrand(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type strSet struct {
	m map[string]bool
	s []string
}

func newSet() *strSet { return &strSet{m: map[string]bool{}} }

func (o *strSet) Add(v string) bool {
	if o.m == nil {
		o.m = map[string]bool{}
	}
	if o.m[v] {
		return false
	}
	o.m[v] = true
	o.s = append(o.s, v)
	return true
}

func (o *strSet) Has(v string) bool { return o.m != nil && o.m[v] }
func (o *strSet) Len() int          { return len(o.s) }
func (o *strSet) List() []string    { return o.s }

func (o *strSet) Delete(v string) { delete(o.m, v) }

func (o *strSet) Values() []string {
	out := make([]string, 0, len(o.s))
	for _, v := range o.s {
		if o.m[v] {
			out = append(out, v)
		}
	}
	return out
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type levCacheT struct {
	m map[string]int
}

func (l *levCacheT) get(a, b string) (int, bool) {
	v, ok := l.m[a+" "+b]
	return v, ok
}
func (l *levCacheT) set(a, b string, d int) {
	if len(l.m) > 100000 {
		l.m = map[string]int{}
	}
	l.m[a+" "+b] = d
}

func levDistFast(lc *levCacheT, a, b string) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	m, n := len(a), len(b)
	if m == 0 {
		return n
	}
	if n-m > 1 {
		return n
	}
	if d, ok := lc.get(a, b); ok {
		return d
	}
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for i := 0; i <= m; i++ {
		prev[i] = i
	}
	for j := 1; j <= n; j++ {
		cur[0] = j
		for i := 1; i <= m; i++ {
			if a[i-1] == b[j-1] {
				cur[i] = prev[i-1]
			} else {
				x := cur[i-1]
				if prev[i] < x {
					x = prev[i]
				}
				if prev[i-1] < x {
					x = prev[i-1]
				}
				cur[i] = 1 + x
			}
		}
		copy(prev, cur)
	}
	d := prev[m]
	lc.set(a, b, d)
	return d
}

func levDistSimple(a, b string) int {
	la, lb := len(a), len(b)
	if absInt(la-lb) > 2 {
		return 3
	}
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			x := dp[i-1][j] + 1
			y := dp[i][j-1] + 1
			z := dp[i-1][j-1] + c
			m := x
			if y < m {
				m = y
			}
			if z < m {
				m = z
			}
			dp[i][j] = m
		}
	}
	return dp[la][lb]
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func cmpName(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}
