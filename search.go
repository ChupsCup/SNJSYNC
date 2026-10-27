package main

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type searchEntry struct {
	e      PhotoEntry
	hay    string
	words  []string
	wSrc   map[string]float64
	digits []string
	ests   *strSet

	codeW    map[string]float64
	cutUnits []string
}

type SearchIndex struct {
	entries  []*searchEntry
	wIndex   map[string][]int
	dIndex   map[string][]int
	fz       map[string]*strSet
	estIndex map[string][]int
}

var srcWeight = map[string]float64{"dir": 3, "name": 2, "text": 1.5, "bc": 1}

var cutCodeSynonyms = map[string][]string{
	"fq11":  {"slice", "forequarter", "prosot"},
	"fq11b": {"slice", "forequarter", "prosot"},
	"cm01":  {"slice", "daging kita", "dagingkita", "compensated"},
	"cm001": {"slice", "daging kita", "dagingkita", "compensated"},
	"hq41":  {"topside", "penutup"},
	"hq42":  {"thick flank", "thickflank", "flank"},
	"hq44":  {"silverside", "gandik"},
	"hq45":  {"rump", "rump steak", "rumpsteak", "tanjakan"},
	"hq46":  {"striploin", "sirloin", "has luar"},
	"hq31":  {"tenderloin", "has dalam"},
	"fq18":  {"trimming", "bulk", "kasaran"},
	"fq19":  {"square flank", "flank"},
	"fq64":  {"chuck tender", "chucktender", "paha depan"},
	"fq65":  {"blade", "shoulder blade"},
	"fq66":  {"trimming", "trim"},
	"fq67":  {"cube roll", "cuberoll", "ribeye", "rib eye"},
	"fq106": {"shin shank", "shinshank", "sengkel"},
	"fq152": {"trimming", "trim"},
}

var reCodePattern = regexp.MustCompile(`(?i)\b([a-z]{1,4})[- ]?(\d{1,4}[a-z]?)\b`)
var reSpecPattern = regexp.MustCompile(`(?i)\b(\d{1,3})[- ]?([a-z]{1,3})\b`)

var cutUnitCodes = func() map[string][]string {
	m := map[string][]string{}
	for code, syns := range cutCodeSynonyms {
		for _, u := range syns {
			m[u] = append(m[u], code)
		}
	}
	return m
}()

var cutWordUnits = func() map[string][]string {
	m := map[string][]string{}
	for u := range cutUnitCodes {
		for _, w := range splitFields(u, isNonAlnum) {
			if w == "" {
				continue
			}
			if !strInSlice(m[w], u) {
				m[w] = append(m[w], u)
			}
		}
	}
	return m
}()

func strInSlice(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func famWord(w string) []string {
	var out []string
	for _, u := range cutWordUnits[w] {
		out = append(out, cutUnitCodes[u]...)
	}
	return out
}

func famHas(fam []string, code string) bool {
	for _, c := range fam {
		if c == code {
			return true
		}
	}
	return false
}

func famIntersect(a, b []string) bool {
	for _, x := range a {
		if famHas(b, x) {
			return true
		}
	}
	return false
}

var softLabels = newSet()
var defaultCountry = map[string][]string{
	"in": {"india"},
	"br": {"brazil"},
	"nz": {"new zealand", "newzealand"},
	"us": {"united states", "usa", "us.a"},
	"au": {"australia"},
	"jp": {"japan", "jepang"},
}

func init() {
	for _, s := range []string{"est", "me", "sif", "msa", "usda", "halal", "jkt"} {
		softLabels.Add(s)
	}
}

func (a *App) countryMap() map[string][]string {
	a.mu.RLock()
	cc := a.cfg.CountryCodes
	a.mu.RUnlock()
	src := defaultCountry
	if len(cc) > 0 {
		src = cc
	}
	m := make(map[string][]string, len(src))
	for k, v := range src {
		lv := make([]string, 0, len(v))
		for _, x := range v {
			lv = append(lv, strings.ToLower(x))
		}
		m[strings.ToLower(k)] = lv
	}
	return m
}

func (a *App) getSearchIndex() *SearchIndex {
	for {
		a.searchMu.Lock()
		if a.searchIx != nil {
			ix := a.searchIx
			a.searchMu.Unlock()
			return ix
		}
		if !a.searchBuilding {
			a.searchBuilding = true
			a.searchDirty = false
			a.searchMu.Unlock()
			ix := buildSearchEntries(a.searchSource())
			a.searchMu.Lock()
			a.searchIx = ix
			a.searchStale = nil
			if a.searchDirty {

				go a.searchBuildLoop()
			} else {
				a.searchBuilding = false
			}
			a.searchMu.Unlock()
			return ix
		}
		stale := a.searchStale
		a.searchMu.Unlock()
		if stale != nil {
			return stale
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (a *App) searchSource() []PhotoEntry {
	src := a.snapshotUniverse()
	if len(src) == 0 {
		src = a.snapshotIdx()
	}
	return src
}

func (a *App) resetSearchIndex() {
	a.searchMu.Lock()
	if a.searchIx != nil {
		a.searchStale = a.searchIx
		a.searchIx = nil
	}
	a.searchDirty = true
	if !a.searchBuilding {
		a.searchBuilding = true
		go a.searchBuildLoop()
	}
	a.searchMu.Unlock()
}

func (a *App) searchBuildLoop() {
	for {
		a.searchMu.Lock()
		if !a.searchDirty && a.searchIx != nil {
			a.searchBuilding = false
			a.searchMu.Unlock()
			return
		}
		a.searchDirty = false
		a.searchMu.Unlock()

		ix := buildSearchEntries(a.searchSource())

		a.searchMu.Lock()
		a.searchIx = ix
		a.searchStale = nil
		a.searchMu.Unlock()
	}
}

var reDigits1to6 = regexp.MustCompile(`\d{1,6}`)
var reDigits7to15 = regexp.MustCompile(`\d{7,15}`)

var (
	reEstNo   = regexp.MustCompile(`\best\s*(?:no\.?)?\s*\.?\s*(\d{1,6})`)
	reEstWord = regexp.MustCompile(`establishment[^a-z0-9]{0,20}(\d{1,6})`)
)

func hasLetterDigit(s string) bool {
	hasL, hasD := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasD = true
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasL = true
		}
		if hasL && hasD {
			return true
		}
	}
	return false
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlnumLowerByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func splitWords(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if isAlnumLowerByte(s[i]) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			out = append(out, s[start:i])
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func isWordByte(c byte) bool {
	return isASCIILetter(c) || isASCIIDigit(c) || c == '_'
}

func boundaryAt(s string, p int) bool {
	return p >= len(s) || !isWordByte(s[p])
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func tryCode(s string, start int) (int, string, string, bool) {
	n := len(s)
	lrun := 0
	for start+lrun < n && isASCIILetter(s[start+lrun]) {
		lrun++
	}
	maxL := lrun
	if maxL > 4 {
		maxL = 4
	}
	for nl := maxL; nl >= 1; nl-- {
		p := start + nl
		dstart := p
		if p < n && (s[p] == '-' || s[p] == ' ') {
			dstart = p + 1
		}
		drun := 0
		for dstart+drun < n && isASCIIDigit(s[dstart+drun]) {
			drun++
		}
		if drun == 0 {
			continue
		}
		maxD := drun
		if maxD > 4 {
			maxD = 4
		}
		for nd := maxD; nd >= 1; nd-- {
			q := dstart + nd
			if q < n && isASCIILetter(s[q]) {

				if boundaryAt(s, q+1) {
					return q + 1, s[start : start+nl], s[dstart : q+1], true
				}
				if boundaryAt(s, q) {
					return q, s[start : start+nl], s[dstart:q], true
				}
			} else if boundaryAt(s, q) {
				return q, s[start : start+nl], s[dstart:q], true
			}
		}
	}
	return 0, "", "", false
}

func trySpec(s string, start int) (int, string, string, bool) {
	n := len(s)
	drun := 0
	for start+drun < n && isASCIIDigit(s[start+drun]) {
		drun++
	}
	maxD := drun
	if maxD > 3 {
		maxD = 3
	}
	for nd := maxD; nd >= 1; nd-- {
		p := start + nd
		lstart := p
		if p < n && (s[p] == '-' || s[p] == ' ') {
			lstart = p + 1
		}
		lrun := 0
		for lstart+lrun < n && isASCIILetter(s[lstart+lrun]) {
			lrun++
		}
		if lrun == 0 {
			continue
		}
		maxL := lrun
		if maxL > 3 {
			maxL = 3
		}
		for nl := maxL; nl >= 1; nl-- {
			q := lstart + nl
			if boundaryAt(s, q) {
				return q, s[start : start+nd], s[lstart:q], true
			}
		}
	}
	return 0, "", "", false
}

func scanCodePattern(s string) [][2]string {
	if !isASCII(s) {
		var out [][2]string
		for _, m := range reCodePattern.FindAllStringSubmatch(s, -1) {
			if len(m) >= 3 {
				out = append(out, [2]string{m[1], m[2]})
			}
		}
		return out
	}
	var out [][2]string
	for i := 0; i < len(s); i++ {
		if !isASCIILetter(s[i]) || (i > 0 && isWordByte(s[i-1])) {
			continue
		}
		if end, g1, g2, ok := tryCode(s, i); ok {
			out = append(out, [2]string{g1, g2})
			i = end - 1
		}
	}
	return out
}

func scanSpecPattern(s string) [][2]string {
	if !isASCII(s) {
		var out [][2]string
		for _, m := range reSpecPattern.FindAllStringSubmatch(s, -1) {
			if len(m) >= 3 {
				out = append(out, [2]string{m[1], m[2]})
			}
		}
		return out
	}
	var out [][2]string
	for i := 0; i < len(s); i++ {
		if !isASCIIDigit(s[i]) || (i > 0 && isWordByte(s[i-1])) {
			continue
		}
		if end, g1, g2, ok := trySpec(s, i); ok {
			out = append(out, [2]string{g1, g2})
			i = end - 1
		}
	}
	return out
}

func digitChunks(s string, min, max int) []string {
	var out []string
	i := 0
	for i < len(s) {
		if !isASCIIDigit(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isASCIIDigit(s[j]) {
			j++
		}
		for j-i >= min {
			end := i + max
			if end > j {
				end = j
			}
			out = append(out, s[i:end])
			i = end
		}
		i = j
	}
	return out
}

func scanEstNo(s string, out *strSet) {
	for i := 0; i+3 <= len(s); i++ {
		if s[i] != 'e' || s[i+1] != 's' || s[i+2] != 't' {
			continue
		}
		if i > 0 && isWordByte(s[i-1]) {
			continue
		}
		q := i + 3
		for q < len(s) && isSpaceByte(s[q]) {
			q++
		}
		if q+2 <= len(s) && s[q] == 'n' && s[q+1] == 'o' {
			q += 2
			if q < len(s) && s[q] == '.' {
				q++
			}
		}
		for q < len(s) && isSpaceByte(s[q]) {
			q++
		}
		if q < len(s) && s[q] == '.' {
			q++
			for q < len(s) && isSpaceByte(s[q]) {
				q++
			}
		}
		d := q
		for q < len(s) && isASCIIDigit(s[q]) && q-d < 6 {
			q++
		}
		if q > d {
			out.Add(stripLeadingZeros(s[d:q]))
			i = q - 1
		}
	}
}

func scanEstWord(s string, out *strSet) {
	for i := 0; i+13 <= len(s); i++ {
		if s[i:i+13] != "establishment" {
			continue
		}
		q := i + 13
		for q < len(s) && !isAlnumLowerByte(s[q]) && q-(i+13) < 20 {
			q++
		}

		if q < len(s) && isASCIIDigit(s[q]) && q-(i+13) <= 20 {
			d := q
			for q < len(s) && isASCIIDigit(s[q]) && q-d < 6 {
				q++
			}
			if q > d {
				out.Add(stripLeadingZeros(s[d:q]))
				i = q - 1
				continue
			}
		}
	}
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func extractEsts(values ...string) []string {
	out := newSet()
	for _, t := range values {
		s := strings.ToLower(t)

		if !strings.Contains(s, "est") {
			continue
		}
		scanEstNo(s, out)
		scanEstWord(s, out)
	}
	return out.List()
}

func stripLeadingZeros(s string) string {
	i := 0
	for i < len(s) && s[i] == '0' {
		i++
	}
	return s[i:]
}

func cutCanonField(s string, cache map[string][]string) []string {
	if v, ok := cache[s]; ok {
		return v
	}
	wds := splitWords(s)
	out := []string{}
	seen := map[string]bool{}
	for i := 0; i < len(wds); i++ {
		w := wds[i]
		if len(w) < 3 || !cutPrefix3[strings.ToUpper(w[:3])] {
			continue
		}
		for n := 1; n <= 3 && i+n <= len(wds); n++ {
			k := cutCanon(wds[i : i+n])
			if k == "" {
				continue
			}
			low := strings.ToLower(k)
			if !seen[low] {
				seen[low] = true
				out = append(out, low)
			}
		}
	}
	cache[s] = out
	return out
}

func dirLabel(raw string) string {
	d := strings.ReplaceAll(raw, "\\", "/")
	parts := make([]string, 0, 2)
	for _, p := range strings.Split(d, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return raw
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[len(parts)-2] + " " + parts[len(parts)-1]
}

func buildSearchEntries(idx []PhotoEntry) *SearchIndex {
	si := &SearchIndex{
		entries:  make([]*searchEntry, 0, len(idx)),
		wIndex:   map[string][]int{},
		dIndex:   map[string][]int{},
		fz:       map[string]*strSet{},
		estIndex: map[string][]int{},
	}
	canonCache := map[string][]string{}
	for _, e := range idx {
		bcTexts := make([]string, 0, len(e.Barcodes))
		for _, b := range e.Barcodes {
			bcTexts = append(bcTexts, b.Text)
		}

		fields := [4]string{
			normText(e.Dir),
			normText(e.Name),
			normText(e.Text),
			normText(strings.Join(bcTexts, " ")),
		}
		ws := [4]float64{srcWeight["dir"], srcWeight["name"], srcWeight["text"], srcWeight["bc"]}
		en := &searchEntry{e: e, wSrc: map[string]float64{}, codeW: map[string]float64{}, ests: newSet()}
		var hayParts []string
		for i := 0; i < len(fields); i++ {
			v, w := fields[i], ws[i]
			hayParts = append(hayParts, v)
			for _, wd := range splitWords(v) {
				if wd == "" {
					continue
				}
				if w > en.wSrc[wd] {
					en.wSrc[wd] = w
				}
			}

			if hasLetterDigit(v) {
				for _, m := range scanCodePattern(v) {
					codeNorm := m[0] + m[1]
					codeHyphen := m[0] + "-" + m[1]
					if w > en.codeW[codeNorm] {
						en.codeW[codeNorm] = w
					}
					if w > en.wSrc[codeNorm] {
						en.wSrc[codeNorm] = w
					}
					if w > en.wSrc[codeHyphen] {
						en.wSrc[codeHyphen] = w
					}
					hayParts = append(hayParts, codeNorm, codeHyphen)
				}
				for _, m := range scanSpecPattern(v) {
					specNorm := m[0] + m[1]
					specHyphen := m[0] + "-" + m[1]
					if w > en.wSrc[specNorm] {
						en.wSrc[specNorm] = w
					}
					if w > en.wSrc[specHyphen] {
						en.wSrc[specHyphen] = w
					}
					hayParts = append(hayParts, specNorm, specHyphen)
				}
			}
		}
		en.hay = strings.Join(hayParts, " ")

		for fi := 0; fi < 2; fi++ {
			src := fields[fi]
			if fi == 0 {
				src = dirLabel(e.Dir)
			}
			for _, low := range cutCanonField(src, canonCache) {
				if ws[fi] > en.wSrc[low] {
					en.wSrc[low] = ws[fi]
				}
			}
		}

		en.words = make([]string, 0, len(en.wSrc))
		for wd := range en.wSrc {
			en.words = append(en.words, wd)
		}

		folderBlob := " " + strings.Join(splitWords(fields[0]+" "+fields[1]), " ") + " "
		for u := range cutUnitCodes {
			if strings.Contains(folderBlob, " "+u+" ") {
				en.cutUnits = append(en.cutUnits, u)
			}
		}
		digits := digitChunks(fields[0], 1, 6)
		digits = append(digits, digitChunks(fields[2], 1, 6)...)
		digits = append(digits, digitChunks(fields[0], 7, 15)...)
		digits = append(digits, digitChunks(fields[2], 7, 15)...)
		en.digits = digits
		for _, v := range extractEsts(fields[0], fields[2], fields[1]) {
			en.ests.Add(v)
		}
		si.entries = append(si.entries, en)
		ei := len(si.entries) - 1
		for _, w := range en.words {
			si.wIndex[w] = append(si.wIndex[w], ei)

			if strings.ContainsAny(w, "0123456789") || len(w) < 5 {
				continue
			}
			key := w[:1] + "|" + itoa(len(w))
			s := si.fz[key]
			if s == nil {
				s = newSet()
				si.fz[key] = s
			}
			s.Add(w)
		}
		for _, d := range digits {
			si.dIndex[d] = append(si.dIndex[d], ei)
		}
		for _, es := range en.ests.List() {
			si.estIndex[es] = append(si.estIndex[es], ei)
		}
	}
	return si
}

type queryToken struct {
	text    string
	num     bool
	soft    bool
	aliases []string
	country bool
	syn     bool
}

type codeHit struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type groupHit struct {
	Code string   `json:"code"`
	Any  []string `json:"any"`
}

type tokOut struct {
	Text string `json:"text"`
	Num  bool   `json:"num"`
	Soft bool   `json:"soft"`
}

var reNumOnly = regexp.MustCompile(`^\d+$`)

func parseQuery(q string, countries map[string][]string) ([]queryToken, []codeHit, []string) {
	rawTerms := splitFields(strings.ToLower(q), func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
	})
	var tokens []queryToken
	var codes []codeHit

	var processedTerms []string
	for i := 0; i < len(rawTerms); i++ {
		clean := trimNonAlnum(rawTerms[i])
		if clean == "" {
			continue
		}

		if m := reCodePattern.FindStringSubmatch(clean); len(m) >= 3 && (m[0] == clean || strings.ReplaceAll(clean, "-", "") == m[1]+m[2]) {
			processedTerms = append(processedTerms, m[1]+m[2])
			continue
		}
		if m := reSpecPattern.FindStringSubmatch(clean); len(m) >= 3 && (m[0] == clean || strings.ReplaceAll(clean, "-", "") == m[1]+m[2]) {
			processedTerms = append(processedTerms, m[1]+m[2])
			continue
		}

		if i+1 < len(rawTerms) {
			next := trimNonAlnum(rawTerms[i+1])
			if next != "" {

				if _, isCountry := countries[next]; !isCountry {
					if _, isCountryClean := countries[clean]; !isCountryClean {
						combined := clean + next
						if m := reCodePattern.FindStringSubmatch(combined); len(m) >= 3 && m[0] == combined {
							processedTerms = append(processedTerms, combined)
							i++
							continue
						}
						if m := reSpecPattern.FindStringSubmatch(combined); len(m) >= 3 && m[0] == combined {
							processedTerms = append(processedTerms, combined)
							i++
							continue
						}
					}
				}
			}
		}

		joined := ""
		skip := 0
		for n := 3; n >= 2; n-- {
			if i+n > len(rawTerms) {
				continue
			}
			parts := make([]string, 0, n)
			for k := 0; k < n; k++ {
				p := trimNonAlnum(rawTerms[i+k])
				if p == "" {
					parts = nil
					break
				}
				parts = append(parts, p)
			}
			if parts == nil {
				continue
			}
			if kk := cutCanon(parts); kk != "" {
				joined = strings.ToLower(kk)
				skip = n - 1
				break
			}
		}
		if joined != "" {
			processedTerms = append(processedTerms, joined)
			i += skip
			continue
		}

		processedTerms = append(processedTerms, clean)
	}

	for _, clean := range processedTerms {
		for _, sub := range splitFields(clean, isNonAlnum) {
			if sub == "" {
				continue
			}
			if al, ok := countries[sub]; ok {
				lo := make([]string, 0, len(al))
				for _, v := range al {
					lo = append(lo, strings.ToLower(v))
				}

				tokens = append(tokens, queryToken{text: sub, num: false, soft: true, aliases: lo, country: true})
				label := ""
				if len(lo) > 0 {
					label = lo[0]
				}
				codes = append(codes, codeHit{Code: sub, Label: label})
			} else {
				txt := sub
				if k := cutCanon([]string{sub}); k != "" {
					txt = strings.ToLower(k)
				}
				isNum := reNumOnly.MatchString(txt)
				tokens = append(tokens, queryToken{text: txt, num: isNum, soft: !isNum && softLabels.Has(txt)})
			}
		}
	}
	return tokens, codes, rawTerms
}

type aliasRow struct {
	A     string   `json:"a"`
	B     string   `json:"b"`
	AToks []string `json:"aToks"`
	BToks []string `json:"bToks"`
}

var (
	aliasOnce sync.Once
	aliasData []aliasRow
)

func (a *App) loadAliasMap() []aliasRow {
	aliasOnce.Do(func() {
		if b := readJSONSafe(filepath.Join(a.dataDir, "alias_map.json")); b != nil {
			var arr []aliasRow
			if json.Unmarshal(b, &arr) == nil {
				aliasData = arr
			}
		}
	})
	return aliasData
}

func aliasExpansion(tokens []queryToken, rows []aliasRow) (req, opt *strSet) {
	qSet := newSet()
	for _, t := range tokens {
		qSet.Add(t.text)
	}
	type scored struct {
		row  aliasRow
		cov  float64
		covA float64
		covB float64
	}
	var sc []scored
	for _, row := range rows {
		aW, bW := row.AToks, row.BToks
		if len(aW) < 3 || len(bW) < 3 {
			continue
		}
		aHit := 0
		for _, w := range aW {
			if qSet.Has(w) {
				aHit++
			}
		}
		bHit := 0
		for _, w := range bW {
			if qSet.Has(w) {
				bHit++
			}
		}
		covA := float64(aHit) / float64(len(aW))
		covB := float64(bHit) / float64(len(bW))
		cov := maxF(covA, covB)
		if cov >= 0.6 {
			sc = append(sc, scored{row: row, cov: cov, covA: covA, covB: covB})
		}
	}
	if len(sc) == 0 {
		return newSet(), newSet()
	}
	maxCov := sc[0].cov
	for _, s := range sc {
		if s.cov > maxCov {
			maxCov = s.cov
		}
	}
	req, opt = newSet(), newSet()
	for _, s := range sc {
		if s.cov < maxCov-0.15 {
			continue
		}
		for _, w := range s.row.AToks {
			req.Add(w)
		}
		for _, w := range s.row.BToks {
			opt.Add(w)
		}
	}

	return req, opt
}

func fuzzyCands(fz map[string]*strSet, t string) []string {

	if t == "" || len(t) < 5 || strings.ContainsAny(t, "0123456789") {
		return nil
	}
	maxD := 1
	if len(t) >= 7 {
		maxD = 2
	}
	out := []string{}
	seen := newSet()
	for dl := -1; dl <= 1; dl++ {
		set := fz[t[:1]+"|"+itoa(len(t)+dl)]
		if set == nil {
			continue
		}
		for _, w := range set.List() {
			if seen.Has(w) {
				continue
			}
			seen.Add(w)
			if absInt(len(w)-len(t)) > maxD {
				continue
			}
			if levDistSimple(t, w) <= maxD {
				out = append(out, w)
			}
		}
	}
	return out
}

func (en *searchEntry) cutInfer(t string) float64 {
	units := cutWordUnits[t]
	if len(units) == 0 {
		return 0
	}
	fam := famWord(t)
	codeBest, hasCode := 0.0, false
	for c, w := range en.codeW {
		if famHas(fam, c) {
			hasCode = true
			codeBest = maxF(codeBest, w)
		}
	}
	unitsOK := true
	for _, u := range en.cutUnits {
		if !famIntersect(cutUnitCodes[u], fam) {
			unitsOK = false
		}
	}
	if hasCode && unitsOK {
		s := codeBest * 0.15
		if s > 0.5 {
			s = 0.5
		}
		return s
	}

	for _, u := range en.cutUnits {
		if famIntersect(cutUnitCodes[u], fam) {
			if len(en.codeW) == 0 || hasCode {
				return 0.5
			}
		}
	}
	return 0
}

func matchToken(en *searchEntry, tk queryToken, cands []string) float64 {
	if tk.num {
		if len(tk.text) >= 2 && en.ests.Has(tk.text) {
			return 1.7
		}
		best := 0.0
		for _, d := range en.digits {
			if d == tk.text {
				best = maxF(best, 1)
				continue
			}
			if len(tk.text) <= 6 && len(d) <= len(tk.text)+2 && strings.Contains(d, tk.text) {
				best = maxF(best, 0.7)
			}
		}
		if strings.Contains(en.hay, tk.text) {
			best = maxF(best, 1)
		}
		return best
	}
	if tk.aliases != nil {
		best := 0.0
		for _, al := range tk.aliases {
			if w, ok := en.wSrc[al]; ok && w > 0 {
				best = maxF(best, w)
			} else if strings.Contains(en.hay, al) {
				best = maxF(best, 0.8)
			}
		}
		if w, ok := en.wSrc[tk.text]; ok && w > 0 {
			best = maxF(best, w)
		} else if strings.Contains(en.hay, tk.text) {
			best = maxF(best, 0.6)
		}
		return best
	}
	t := tk.text
	if w, ok := en.wSrc[t]; ok && w > 0 {
		return w
	}
	if strings.Contains(en.hay, t) {
		return 0.6
	}

	if _, ok := cutWordUnits[t]; ok {
		if s := en.cutInfer(t); s > 0 {
			return s
		}
	}
	if len(t) >= 5 && cands != nil {
		for _, c := range cands {
			if w, ok := en.wSrc[c]; ok && w > 0 {
				return w * 0.5
			}
		}
	}
	return 0
}

type photoResult struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Dir      string    `json:"dir"`
	Size     int64     `json:"size"`
	Text     string    `json:"text"`
	Barcodes []Barcode `json:"barcodes"`
	Path     string    `json:"path"`
	Matched  int       `json:"matched"`
	Total    int       `json:"total"`
}

type searchResp struct {
	Total        int           `json:"total"`
	Terms        []string      `json:"terms"`
	Groups       []groupHit    `json:"groups"`
	Codes        []codeHit     `json:"codes"`
	Results      []photoResult `json:"results"`
	ExactCount   int           `json:"exactCount"`
	Partial      []photoResult `json:"partial"`
	PartialCount int           `json:"partialCount"`
	Tokens       []tokOut      `json:"tokens"`
	MS           int64         `json:"ms"`
	Browse       bool          `json:"browse"`
}

type scoredHit struct {
	idx     int
	score   float64
	matched int
	total   int
}

func (a *App) browsePhotos(limit int) searchResp {
	t0 := time.Now()
	u := a.snapshotUniverse()
	if len(u) == 0 {
		return searchResp{
			Total: 0, Terms: []string{}, Groups: []groupHit{}, Codes: []codeHit{},
			Results: []photoResult{}, Partial: []photoResult{}, Browse: true,
		}
	}
	if limit <= 0 || limit > len(u) {
		limit = len(u)
	}
	step := len(u) / limit
	if step < 1 {
		step = 1
	}
	results := make([]photoResult, 0, limit)
	for i := 0; i < len(u) && len(results) < limit; i += step {
		e := u[i]
		results = append(results, photoResult{
			ID: e.ID, Name: e.Name, Dir: e.Dir, Size: e.Size,
			Text: e.Text, Barcodes: e.Barcodes, Path: e.Path,
		})
	}
	return searchResp{
		Total: len(u), Terms: []string{}, Groups: []groupHit{}, Codes: []codeHit{},
		Results: results, ExactCount: len(results), Browse: true,
		Partial: []photoResult{}, Tokens: []tokOut{},
		MS: time.Since(t0).Milliseconds(),
	}
}

func (a *App) search(q string) searchResp {
	t0 := time.Now()
	si := a.getSearchIndex()
	tokens, codes, rawTerms := parseQuery(q, a.countryMap())
	req, opt := aliasExpansion(tokens, a.loadAliasMap())

	baseToks := make([]queryToken, len(tokens))
	copy(baseToks, tokens)

	hardLeft := 0
	for _, t := range baseToks {
		if !t.soft {
			hardLeft++
		}
	}
	if hardLeft == 0 {
		best, bestW := -1, 0.0
		for i, t := range baseToks {
			w := srcWeight["text"]
			if t.num {
				w = srcWeight["bc"] + 0.5
			}
			if w > bestW || best < 0 {
				best, bestW = i, w
			}
		}
		if best >= 0 {
			baseToks[best].soft = false
			baseToks[best].syn = false
		}
	}
	haveText := func(w string) bool {
		for _, t := range baseToks {
			if t.text == w {
				return true
			}
		}
		return false
	}
	var extraToks []queryToken
	seenExtra := newSet()
	addExtras := func(w string, soft bool) {
		if seenExtra.Has(w) || haveText(w) {
			return
		}
		seenExtra.Add(w)
		isNum := reNumOnly.MatchString(w)
		extraToks = append(extraToks, queryToken{text: w, num: isNum, soft: soft, syn: true})
	}
	for _, w := range req.List() {

		addExtras(w, true)
	}
	for _, w := range opt.List() {
		addExtras(w, true)
	}
	allToks := append(append([]queryToken{}, baseToks...), extraToks...)
	var nonSoft []queryToken
	for _, t := range allToks {
		if !t.soft {
			nonSoft = append(nonSoft, t)
		}
	}
	entries := si.entries
	tokenCands := make([][]string, len(allToks))
	for i, t := range allToks {
		if t.num || t.aliases != nil || len(t.text) < 5 || strings.ContainsAny(t.text, "0123456789") {
			tokenCands[i] = nil
		} else {
			tokenCands[i] = fuzzyCands(si.fz, t.text)
		}
	}

	anchor := make([]bool, len(allToks))
	for i, t := range allToks {
		if t.num || strings.ContainsAny(t.text, "0123456789") {
			anchor[i] = true
			continue
		}
		if _, ok := cutWordUnits[t.text]; ok {
			anchor[i] = true
			continue
		}
		if cutKnown(cutKey(t.text)) {
			anchor[i] = true
		}
	}

	minPartial := 0
	if len(nonSoft) >= 2 {
		minPartial = len(nonSoft) - 1
	}
	var exact, partial []scoredHit
	if len(nonSoft) > 0 && len(entries) > 0 {
		for i, en := range entries {
			matched := 0
			score := 0.0
			allStrong := true
			anchorMiss := false
			for k := range allToks {
				s := matchToken(en, allToks[k], tokenCands[k])
				if allToks[k].soft {
					if s > 0 {
						score += 0.3 + s*0.05
					}
					continue
				}
				if s == 0 {
					allStrong = false
					if anchor[k] {
						anchorMiss = true
					}
					continue
				}
				matched++
				score += s
			}
			if matched == 0 {
				continue
			}
			if allStrong {
				exact = append(exact, scoredHit{idx: i, score: score, matched: matched, total: len(nonSoft)})
			} else if !anchorMiss && matched >= minPartial {

				partial = append(partial, scoredHit{idx: i, score: score, matched: matched, total: len(nonSoft)})
			}
		}
		sort.SliceStable(exact, func(i, j int) bool {
			if exact[i].score != exact[j].score {
				return exact[i].score > exact[j].score
			}
			di, dj := entries[exact[i].idx].e.Dir, entries[exact[j].idx].e.Dir
			if di != dj {
				return di < dj
			}
			return cmpName(entries[exact[i].idx].e.Name, entries[exact[j].idx].e.Name)
		})
		sort.SliceStable(partial, func(i, j int) bool {
			a1, b1 := partial[i], partial[j]
			if a1.matched != b1.matched {
				return a1.matched > b1.matched
			}
			if a1.score != b1.score {
				return a1.score > b1.score
			}
			return cmpName(entries[a1.idx].e.Name, entries[b1.idx].e.Name)
		})
	} else {
		for i := range entries {
			exact = append(exact, scoredHit{idx: i})
		}
		sort.SliceStable(exact, func(i, j int) bool {
			return cmpName(entries[exact[i].idx].e.Name, entries[exact[j].idx].e.Name)
		})
	}
	mk := func(m scoredHit) photoResult {
		e := entries[m.idx].e
		bc := e.Barcodes
		if bc == nil {
			bc = []Barcode{}
		}
		return photoResult{
			ID: e.ID, Name: e.Name, Dir: e.Dir, Size: e.Size, Text: e.Text,
			Barcodes: bc, Path: e.Path, Matched: m.matched, Total: len(nonSoft),
		}
	}
	groups := make([]groupHit, 0, len(tokens))
	tokOuts := make([]tokOut, 0, len(tokens))
	for _, t := range tokens {
		var code string
		if t.country {
			code = t.text
		}
		any := t.aliases
		if any == nil {
			any = []string{t.text}
		}
		groups = append(groups, groupHit{Code: code, Any: any})
		tokOuts = append(tokOuts, tokOut{Text: t.text, Num: t.num, Soft: t.soft})
	}
	if codes == nil {
		codes = []codeHit{}
	}
	if rawTerms == nil {
		rawTerms = []string{}
	}
	res := make([]photoResult, 0, len(exact))
	for _, m := range exact {
		res = append(res, mk(m))
	}
	part := make([]photoResult, 0, 200)
	for i, m := range partial {
		if i >= 200 {
			break
		}
		part = append(part, mk(m))
	}
	if part == nil {
		part = []photoResult{}
	}
	return searchResp{
		Total: len(entries), Terms: rawTerms, Groups: groups, Codes: codes,
		Results: res, ExactCount: len(exact), Partial: part, PartialCount: len(partial),
		Tokens: tokOuts, MS: time.Since(t0).Milliseconds(),
	}
}

func batchTokens(s string) []string {
	f := foldBasic(s, true, false)
	parts := splitFields(f, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f', '-', '/', '\\', '.', ',', ';', ':', '(', ')':
			return true
		}
		return false
	})
	out := make([]string, 0, len(parts))
	for _, w := range parts {
		w = trimNonAlnum(w)
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func batchMatch(hay, tok string) bool {
	if strings.Contains(hay, tok) {
		return true
	}
	if reNumOnly.MatchString(tok) && len(tok) > 1 {
		noLead := stripLeadingZeros(tok)
		if noLead != tok && strings.Contains(hay, noLead) {
			return true
		}
		if strings.Contains(hay, " "+tok+" ") || strings.Contains(hay, " "+tok) || strings.Contains(hay, tok+" ") {
			return true
		}
	}
	return false
}

type batchCand struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Dir        string   `json:"dir"`
	Path       string   `json:"path"`
	Text       string   `json:"text"`
	Folder     string   `json:"folder"`
	BCCount    int      `json:"bcCount"`
	LabelScore int      `json:"labelScore"`
	OCRConf    float64  `json:"ocrConf"`
	Hits       []string `json:"hits"`
	Missing    []string `json:"missing"`
	EstHit     []string `json:"estHit"`
	Exact      bool     `json:"exact"`
	Score      int      `json:"score"`
}

type batchRowOut struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type batchRowResult struct {
	Row        batchRowOut `json:"row"`
	Ests       []string    `json:"ests"`
	Tokens     []string    `json:"tokens"`
	Status     string      `json:"status"`
	Note       string      `json:"note"`
	ExactCount int         `json:"exactCount"`
	Cands      []batchCand `json:"cands"`
}

type batchResp struct {
	Total   int              `json:"total"`
	Ada     int              `json:"ada"`
	Tidak   int              `json:"tidak"`
	MS      int64            `json:"ms"`
	Results []batchRowResult `json:"results"`
}

type batchRowIn struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

func (a *App) batchCheckRow(row batchRowIn) batchRowResult {
	var qparts []string
	if row.Name != "" {
		qparts = append(qparts, row.Name)
	}
	if row.Desc != "" {
		qparts = append(qparts, row.Desc)
	}
	query := strings.Join(qparts, " ")
	sr := a.search(query)
	results := sr.Results
	ests := extractEsts(row.Name, row.Desc)
	toks := batchTokens(query)
	var importantToks []string
	for _, t := range toks {
		if len(t) >= 2 {
			importantToks = append(importantToks, t)
		}
	}
	cands := make([]batchCand, 0, len(results))
	for _, r := range results {
		dir := batchNorm(r.Dir)
		folder := ""
		parts := splitFields(r.Dir, func(rr rune) bool { return rr == '/' || rr == '\\' })
		if len(parts) > 0 {
			folder = parts[len(parts)-1]
		}
		txt := batchNorm(r.Text)
		var hits, missing []string
		for _, t := range importantToks {
			if batchMatch(dir, t) || batchMatch(txt, t) {
				hits = append(hits, t)
			} else {
				missing = append(missing, t)
			}
		}
		allHit := len(hits) == len(importantToks)
		rawTxt := r.Text
		ocrConf := 0.0
		if len(rawTxt) > 10 && !strings.HasPrefix(rawTxt, "[OCR GAGAL]") {
			ocrConf = 0.5
		}
		labelScore := 0
		if ocrConf > 0 {
			labelScore = int(ocrConf*100 + 0.5)
		}
		queryLower := strings.TrimSpace(batchNorm(query))
		folderScore := 0
		switch {
		case strings.Contains(dir, queryLower):
			folderScore = 90
		case everyMatch(importantToks, func(t string) bool { return batchMatch(dir, t) }):
			folderScore = 70
		case len(hits) >= int(mathCeil(float64(len(importantToks))*0.6)):
			folderScore = 40
		}
		displayScore := labelScore
		if displayScore == 0 {
			displayScore = folderScore
		}
		if hits == nil {
			hits = []string{}
		}
		if missing == nil {
			missing = []string{}
		}
		estHit := []string{}
		for _, es := range ests {
			if batchMatch(dir, es) || batchMatch(txt, es) {
				estHit = append(estHit, es)
			}
		}
		cands = append(cands, batchCand{
			ID: r.ID, Name: r.Name, Dir: r.Dir, Path: r.Path, Text: rawTxt,
			Folder: folder, BCCount: len(r.Barcodes), LabelScore: displayScore, OCRConf: ocrConf,
			Hits: hits, Missing: missing, EstHit: estHit, Exact: allHit, Score: len(hits),
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a1, b1 := cands[i], cands[j]
		if a1.Exact != b1.Exact {
			return a1.Exact
		}
		if a1.LabelScore != b1.LabelScore {
			return a1.LabelScore > b1.LabelScore
		}
		return a1.Score > b1.Score
	})
	var exactCands, labelCands []batchCand
	for _, c := range cands {
		if c.Exact {
			exactCands = append(exactCands, c)
		} else if c.LabelScore >= 70 {
			labelCands = append(labelCands, c)
		}
	}
	var best *batchCand
	if len(exactCands) > 0 {
		best = &exactCands[0]
	} else if len(labelCands) > 0 {
		best = &labelCands[0]
	} else if len(cands) > 0 {
		best = &cands[0]
	}
	status := "tidak"
	if len(exactCands) > 0 || len(labelCands) > 0 {
		status = "ada"
	}
	note := ""
	if status == "ada" {
		if len(exactCands) > 1 {
			note = itoa(len(exactCands)) + " foto cocok persis"
		} else if len(labelCands) > 0 {
			note = "Cocok (folder match)"
		} else {
			note = "Cocok persis"
		}
	} else if best != nil && len(best.Missing) > 0 {
		m := best.Missing
		if len(m) > 5 {
			m = m[:5]
		}
		note = "Kata tidak ketemu: " + strings.Join(m, ", ")
	} else {
		note = "Tidak ada foto yang cocok"
	}
	if len(cands) > 20 {
		cands = cands[:20]
	}
	if cands == nil {
		cands = []batchCand{}
	}
	if ests == nil {
		ests = []string{}
	}
	if toks == nil {
		toks = []string{}
	}
	return batchRowResult{
		Row: batchRowOut{Name: row.Name, Desc: row.Desc}, Ests: ests, Tokens: toks,
		Status: status, Note: note, ExactCount: len(exactCands), Cands: cands,
	}
}

func (a *App) batchCheck(rows []batchRowIn) batchResp {
	t0 := time.Now()
	results := make([]batchRowResult, 0, len(rows))
	ada, tidak := 0, 0
	for _, r := range rows {
		br := a.batchCheckRow(r)
		if br.Status == "ada" {
			ada++
		} else {
			tidak++
		}
		results = append(results, br)
	}
	return batchResp{Total: len(rows), Ada: ada, Tidak: tidak,
		MS: time.Since(t0).Milliseconds(), Results: results}
}

func everyMatch(list []string, fn func(string) bool) bool {
	for _, v := range list {
		if !fn(v) {
			return false
		}
	}
	return true
}

func mathCeil(v float64) float64 {
	i := int(v)
	if float64(i) == v {
		return float64(i)
	}
	if v < 0 {
		return float64(i)
	}
	return float64(i + 1)
}
