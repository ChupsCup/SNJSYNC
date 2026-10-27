package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type mlItem struct {
	ID          int
	UploadID    int
	NoContainer string
	Brand       string
	NamaItem    string
	ItemCode    string
	ItemDesc    string
	Gabungan    string
	Importir    string
}

type plItem struct {
	ID          int
	UploadID    int
	Sheet       string
	NoContainer string
	Brand       string
	NamaItem    string
	ETA         string
	ItemCode    string
	ItemDesc    string
	Gabungan    string
	Importir    string

	File string
}

type compareRow struct {
	No          int     `json:"no"`
	NoContainer string  `json:"noContainer"`
	Brand       string  `json:"brand"`
	NamaItem    string  `json:"namaItem"`
	ETA         string  `json:"eta"`
	ItemCode    string  `json:"itemCode"`
	ItemDesc    string  `json:"itemDesc"`
	Sheet       string  `json:"sheet"`
	Importir    string  `json:"importir"`
	Flag        string  `json:"flag"`
	NeedsReview bool    `json:"needsReview"`
	Note        string  `json:"note"`
	Score       float64 `json:"score"`
	MasterCode  string  `json:"masterCode"`
	MasterDesc  string  `json:"masterDesc"`
	CandCode    string  `json:"candCode"`
	CandDesc    string  `json:"candDesc"`

	NoContainerSnake string `json:"no_container,omitempty"`
	NamaItemSnake    string `json:"nama_item,omitempty"`
}

type reviewDecision struct {
	Key        string
	Decision   string
	MasterCode string
	MasterDesc string
}

func sliceRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

var brandAliases = map[string]string{
	"nolans": "nolan", "meramistptyltd": "meramist", "estrella": "estrela",
	"silverfern": "silverfarms", "silverfernfarms": "silverfarms", "sff": "silverfarms",
	"moonbeef": "monbeef", "jbs": "swift", "jbsswift": "swift", "cab": "swift",
	"lockyearvalley": "lockyervalley", "alaali": "alali",
	"frigon": "frigol", "amh": "swift", "tyson": "ibp", "tysonfreshmeats": "ibp",
	"gandkoconnor": "oconnor", "gkoconnor": "oconnor", "gandk": "oconnor", "g k oconnor": "oconnor",
	"hardwick": "hardwicks", "kilcoyhardwick": "hardwicks",
}

var synMap = map[string]string{
	"bovine": "beef", "bnls": "boneless", "offals": "offal", "papilae": "papillae",
	"papila": "papillae", "trmg": "trimming", "trmm": "trimming", "trimm": "trimming",
	"trimmings": "trimming", "diafragma": "diaphragm", "knukle": "knuckle",
	"flang": "flank", "striploin": "striploin", "shortloin": "shortloin",
	"cuberoll": "cuberoll", "ribeye": "ribeye", "chuckroll": "chuckroll",
}

var plurals = map[string]string{
	"bones": "bone", "roots": "root", "hearts": "heart", "necks": "neck",
	"sinews": "sinew", "tendons": "tendon", "tracheas": "trachea", "tripes": "tripe",
	"ribs": "rib", "loins": "loin", "chops": "chop", "shanks": "shank",
	"lobes": "lobe",
}

func newSetOf(keys ...string) *strSet {
	s := newSet()
	for _, k := range keys {
		s.Add(k)
	}
	return s
}

var placeholderWords = newSetOf("blm", "ada", "link")
var descriptors = newSetOf("frozen", "beef", "bovine", "boneless", "offal", "meat", "halal",
	"fresh", "chilled", "food", "industrial", "trim", "trimming", "necktrim", "trimmed", "papillae",
	"pack", "packing", "package", "packaged", "higher", "certified", "premium", "quality", "product", "export",
	"out", "carton", "cartons", "box", "poly", "polybag", "vp", "iwp",
	"lobes", "lobe", "throat", "intercostal", "shoulder")
var prepDrop = newSetOf("frozen", "boneless", "beef", "offal", "pack", "packing", "higher", "certified", "premium", "shoulder", "intercostal", "throat", "lobes")
var prefixOK = newSetOf("steak", "rib", "ribs")
var stopBrandWords = newSetOf("beef", "meat", "packing", "processing", "food", "valley", "frozen",
	"goods", "pty", "ltd", "sdn", "bhd", "inc", "co", "angus", "usda", "prime", "choice", "grade",
	"brand", "halal", "wagyu")
var tokenStopList = newSetOf("est", "au", "br", "in", "ppi", "bkr", "box", "kg", "ton")
var codePrefixSkip = newSetOf("EST", "SFF", "PPI", "BKR", "IWP", "LP", "BOX", "KG")

const (
	tAuto   = 0.55
	tReview = 0.35
)

func normImportir(s string) string {
	v := strings.ToUpper(s)
	if strings.Contains(v, "BERDIKARI") || strings.Contains(v, "BKR") || strings.Contains(v, "BDK") {
		return "BKR"
	}
	if strings.Contains(v, "PPI") {
		return "PPI"
	}
	return ""
}

func (r *plItem) importirPL(desc, gab string) string {
	return normImportir(r.Sheet + " " + r.Importir + " " + desc + " " + gab + " " + r.File)
}

func canonParts(b string) []string {
	raw := b
	seen := newSet()
	out := []string{}
	addK := func(k string) {
		nk := normBrand(k)
		if nk != "" && len(nk) >= 3 && !seen.Has(nk) {
			seen.Add(nk)
			if c, ok := brandAliases[nk]; ok {
				out = append(out, c)
			} else {
				out = append(out, nk)
			}
		}
	}
	addK(raw)
	for _, seg := range regexp.MustCompile(`[(),\[\]/&+]+`).Split(raw, -1) {
		n0 := normBrand(seg)
		if len(n0) >= 3 {
			addK(n0)
		}
		wds := splitFields(seg, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
		})
		li := 0
		for li < len(wds)-1 && len(wds[li]) <= 1 {
			li++
		}
		if li > 0 {
			rest := strings.Join(wds[li:], "")
			if len(rest) >= 3 {
				addK(rest)
			}
		}
	}
	for _, w := range splitFields(raw, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
	}) {
		t := normBrand(w)
		if len(t) >= 3 && !stopBrandWords.Has(t) {
			addK(t)
		}
	}
	return out
}

var rePlur = regexp.MustCompile(`(?i)\b(bones|roots|hearts|necks|sinews|tendons|tracheas|tripes|ribs|loins|chops|shanks)\b`)

func plur(s string) string {
	return rePlur.ReplaceAllStringFunc(s, func(w string) string {
		if v, ok := plurals[strings.ToLower(w)]; ok {
			return v
		}
		return w
	})
}

var reVL = regexp.MustCompile(`(?i)(\d{1,3})\s*v(?:l)?\b`)
var reME = regexp.MustCompile(`(?i)\bme[\s-]*(\d{2,4})\b`)
var reProdKey = regexp.MustCompile(`(?i)\b([a-z]{2,5})\s*[-]?\s*(\d{2,5})\b`)

func normVL(s string) string { return reVL.ReplaceAllString(s, "$1VL") }
func normME(s string) string { return reME.ReplaceAllString(s, "ME$1") }

func prodKeys(s string) []string {
	out := newSet()
	for _, m := range reProdKey.FindAllStringSubmatch(s, -1) {
		l := strings.ToLower(m[1])
		if l == "est" {
			continue
		}
		d := m[2]
		out.Add(l + d)
		out.Add(l + "-" + d)
		out.Add(l + " " + d)
	}
	return out.List()
}

func tokenize(s string) []string {
	var out []string
	for _, w := range splitAlphaNum(strings.ToLower(s)) {
		if w == "" || len(w) < 3 {
			continue
		}
		if reNumOnly.MatchString(w) && false {
			continue
		}
		if regexp.MustCompile(`^me\d+$`).MatchString(w) {
			continue
		}
		if tokenStopList.Has(w) {
			continue
		}
		out = append(out, w)
	}
	return out
}

func tokenize2(s string) []string {
	var out []string
	for _, w := range splitAlphaNum(strings.ToLower(s)) {
		if w == "" {
			continue
		}
		if !(len(w) >= 3 || strings.ContainsAny(w, "0123456789")) {
			continue
		}
		if reMENum.MatchString(w) {
			continue
		}
		if tokenStopList.Has(w) {
			continue
		}
		if v, ok := synMap[w]; ok {
			out = append(out, v)
		} else {
			out = append(out, w)
		}
	}
	return out
}

var reMENum = regexp.MustCompile(`^me\d+$`)

func isNumTok(t string) bool { return reNumOnly.MatchString(t) }

func extractCodes2(s string) *strSet {
	u := strings.ToUpper(s)
	out := newSet()
	n := len(u)
	i := 0
	for i < n {
		if i > 0 && isAZ0(u[i-1]) {
			i++
			continue
		}
		maxLetters := 0
		for maxLetters < 4 && i+maxLetters < n && isAZUp(u[i+maxLetters]) {
			maxLetters++
		}
		matched := false
		for L := maxLetters; L >= 2; L-- {
			j := i + L
			if j < n && (u[j] == ' ' || u[j] == '-') {
				j++
			}
			ds := 0
			for ds < 5 && j+ds < n && u[j+ds] >= '0' && u[j+ds] <= '9' {
				ds++
			}
			if ds >= 1 {
				end := j + ds
				if end == n || !isAZ0(u[end]) {
					p := u[i : i+L]
					if !codePrefixSkip.Has(p) {
						out.Add(p + u[j:end])
					}
					i = end
					matched = true
					break
				}
			}
		}
		if !matched {
			i++
		}
	}
	return out
}

func isAZ0(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
func isAZUp(c byte) bool { return c >= 'A' && c <= 'Z' }

var reSnjSlot = regexp.MustCompile(`-{3}\s*([^-]{1,80}?)\s*-{3}`)

func mlSnjCodeSet(desc string) *strSet {
	if m := reSnjSlot.FindStringSubmatch(desc); m != nil {
		return extractCodes2(m[1])
	}
	return newSet()
}

var reEstVariant = regexp.MustCompile(`(?i)EST[\s-]*(\d+)\b`)
var reEstNums = regexp.MustCompile(`(?i)EST[.\s-]*(\d+)\b`)
var reStarGrade = regexp.MustCompile(`\*\s*([A-Za-z])\s*\*`)

func estVariant(s string) string {
	if m := reEstVariant.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func mlSlotNums(s string) *strSet {
	out := newSet()
	if slot := reSnjSlot.FindStringSubmatch(s); slot != nil {
		for _, m := range regexp.MustCompile(`\d+`).FindAllString(slot[1], -1) {
			out.Add(m)
		}
	}
	return out
}

func mlEstNums(s string) *strSet {
	out := newSet()
	for _, e := range reEstNums.FindAllStringSubmatch(s, -1) {
		out.Add(e[1])
	}
	return out
}

func starGrades(s string) *strSet {
	out := newSet()
	for _, g := range reStarGrade.FindAllStringSubmatch(s, -1) {
		out.Add(strings.ToUpper(g[1]))
	}
	return out
}

var reDigitsAny = regexp.MustCompile(`\d+`)

func standAloneNums(s string) *strSet {
	out := newSet()
	for _, loc := range reDigitsAny.FindAllStringIndex(s, -1) {
		m := s[loc[0]:loc[1]]
		i, l := loc[0], len(m)
		if l > 3 {
			continue
		}
		before := ""
		if i > 0 {
			before = s[i-1 : i]
		}
		after := ""
		if i+l < len(s) {
			after = s[i+l : i+l+1]
		}
		bL := isLetterByte(before)
		aL := isLetterByte(after)
		if bL && aL {
			continue
		}
		if l == 1 && aL {
			continue
		}
		out.Add(m)
	}
	return out
}

func isLetterByte(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func specTokens(s string) *strSet {
	out := newSet()
	for _, w := range splitAlphaNum(strings.ToLower(s)) {
		if w == "" {
			continue
		}
		if len(w) <= 4 && strings.ContainsAny(w, "0123456789") && strings.ContainsAny(w, "abcdefghijklmnopqrstuvwxyz") {
			if reNumUnit.MatchString(w) {
				continue
			}
			if w == "3d" || strings.HasPrefix(w, "me") || w == "0kg" {
				continue
			}
			out.Add(w)
		}
	}
	return out
}

func gradeOf(s string) string {
	toks := splitAlphaNum(strings.ToLower(reGradePair.ReplaceAllString(s, " ")))
	for i := len(toks) - 1; i >= 0; i-- {
		if len(toks[i]) == 1 && toks[i][0] >= 'a' && toks[i][0] <= 'z' {
			return toks[i]
		}
	}
	return ""
}

type masterEntry struct {
	m             *mlItem
	brandKey      string
	bCanon        string
	parens        *strSet
	prods         *strSet
	desc          string
	name          string
	words         []string
	wordsSet      *strSet
	nums          *strSet
	numsLong      *strSet
	star          *strSet
	grade         string
	gradeAll      string
	snjCodes      *strSet
	slotNums      *strSet
	estNums       *strSet
	estList       []string
	specSet       *strSet
	bStop         *strSet
	descBrandFlat string
	importir      string
	wSumWords     float64
}

type compareEngine struct {
	entries        []*masterEntry
	brandIndex     map[string][]*masterEntry
	tokenIndex     map[string][]*masterEntry
	dfTok          map[string]int
	idfW           map[string]float64
	idfN           float64
	masterVocab    *strSet
	allBrandCanons *strSet
	decisions      map[string]*reviewDecision
	masterByCode   map[string]*masterEntry
	snjCodeIndex   map[string][]*masterEntry
	lev            *levCacheT
	aliases        *aliasRules
	ignores        *ignoreWords
}

func newCompareEngine() *compareEngine {
	return &compareEngine{
		brandIndex:     map[string][]*masterEntry{},
		tokenIndex:     map[string][]*masterEntry{},
		dfTok:          map[string]int{},
		idfW:           map[string]float64{},
		allBrandCanons: newSet(),
		decisions:      map[string]*reviewDecision{},
		masterByCode:   map[string]*masterEntry{},
		snjCodeIndex:   map[string][]*masterEntry{},
		lev:            &levCacheT{m: map[string]int{}},
		masterVocab:    newSet(),
	}
}

func (e *compareEngine) wOf(t string) float64 {
	if v, ok := e.idfW[t]; ok {
		return v
	}
	return mathLog(e.idfN)
}

func (e *compareEngine) build(approved []mlItem, decisions map[string]*reviewDecision) {
	for i := range approved {
		m := &approved[i]
		descS := ""
		if m.ItemDesc != "" && m.ItemDesc != m.NamaItem {
			descS = normME(normVL(e.aliases.apply(plur(m.ItemDesc))))
		}
		nameS := ""
		if m.NamaItem != "" && m.NamaItem != m.Gabungan {
			nameS = normME(normVL(e.aliases.apply(plur(m.NamaItem))))
		}
		full := normStr(descS) + " " + normStr(nameS)
		toks := newSet()
		for _, t := range tokenize2(full) {
			toks.Add(t)
		}
		words := []string{}
		wordsSet := newSet()
		for _, t := range toks.List() {
			if isNumTok(t) {
				continue
			}
			if e.ignores.Has(t) {
				continue
			}
			words = append(words, t)
			wordsSet.Add(t)
		}
		nb := normBrand(m.Brand)
		bCanon := nb
		if c, ok := brandAliases[nb]; ok {
			bCanon = c
		}
		parens := newSet()
		for _, mm := range regexp.MustCompile(`\(([^)]{1,40})\)`).FindAllStringSubmatch(descS+" "+nameS, -1) {
			for _, chunk := range strings.Split(mm[1], ",") {
				k := normBrand(chunk)
				if len(k) >= 3 {
					parens.Add(k)
				}
			}
		}
		specSet := specTokens(normStr(e.aliases.apply(m.ItemDesc)) + " " + normStr(e.aliases.apply(m.NamaItem)) + " " + normStr(e.aliases.apply(m.Gabungan)))
		e.ignores.filterSet(specSet)
		ent := &masterEntry{
			m:             m,
			brandKey:      nb,
			bCanon:        bCanon,
			parens:        parens,
			prods:         newSetFromSlice(prodKeys(descS + " " + nameS)),
			desc:          normStr(descS),
			name:          normStr(nameS),
			words:         words,
			wordsSet:      wordsSet,
			nums:          standAloneNums(full),
			numsLong:      newSetFromSlice(reDigitsAny4.FindAllString(full, -1)),
			star:          starGrades(e.aliases.apply(m.ItemDesc) + " " + e.aliases.apply(m.NamaItem) + " " + e.aliases.apply(m.Gabungan)),
			grade:         gradeOf(e.aliases.apply(m.ItemDesc) + " " + e.aliases.apply(m.NamaItem)),
			gradeAll:      gradeOf(e.aliases.apply(m.ItemDesc) + " " + e.aliases.apply(m.NamaItem) + " " + e.aliases.apply(m.Gabungan)),
			snjCodes:      mlSnjCodeSet(m.ItemDesc + " " + m.NamaItem),
			slotNums:      mlSlotNums(m.ItemDesc + " " + m.NamaItem + " " + m.Gabungan),
			estNums:       mlEstNums(m.ItemDesc + " " + m.NamaItem + " " + m.Gabungan),
			estList:       extractEsts(m.ItemDesc, m.NamaItem, m.Gabungan),
			specSet:       specSet,
			bStop:         newSetFromSlice(filterLen3(splitAlphaNum(strings.ToLower(m.Brand)))),
			descBrandFlat: normBrand(descS + " " + nameS),
			importir:      normImportir(m.Importir + " " + m.ItemDesc + " " + m.Gabungan),
		}
		e.entries = append(e.entries, ent)
		e.brandIndex[ent.brandKey] = append(e.brandIndex[ent.brandKey], ent)
		for _, t := range ent.parens.List() {
			e.brandIndex[t] = append(e.brandIndex[t], ent)
		}
		for _, t := range ent.wordsSet.List() {
			e.tokenIndex[t] = append(e.tokenIndex[t], ent)
			e.dfTok[t]++
		}
	}
	e.idfN = float64(len(e.entries))
	if e.idfN == 0 {
		e.idfN = 1
	}
	for t, c := range e.dfTok {
		e.idfW[t] = mathLog(e.idfN / float64(c))
	}
	for _, ent := range e.entries {
		s := 0.0
		for _, t := range ent.words {
			s += e.wOf(t)
		}
		ent.wSumWords = s
		for _, t := range ent.wordsSet.List() {
			e.masterVocab.Add(t)
		}
		e.allBrandCanons.Add(ent.bCanon)
	}
	for alias, canon := range brandAliases {
		if canonList, ok := e.brandIndex[canon]; ok && canonList != nil {
			if _, exists := e.brandIndex[alias]; !exists {
				e.brandIndex[alias] = canonList
			}
		}
	}
	for _, ent := range e.entries {
		if ent.m.ItemCode != "" {
			if _, ok := e.masterByCode[ent.m.ItemCode]; !ok {
				e.masterByCode[ent.m.ItemCode] = ent
			}
		}
		for _, c := range ent.snjCodes.List() {
			e.snjCodeIndex[c] = append(e.snjCodeIndex[c], ent)
		}
	}
	for k, v := range decisions {
		e.decisions[k] = v
	}
}

var reDigitsAny4 = regexp.MustCompile(`\d{4,}`)

func newSetFromSlice(list []string) *strSet {
	s := newSet()
	for _, v := range list {
		s.Add(v)
	}
	return s
}

func filterLen3(in []string) []string {
	out := []string{}
	for _, v := range in {
		if len(v) >= 3 {
			out = append(out, v)
		}
	}
	return out
}

func (e *compareEngine) isSpecific(t string) bool {
	c := e.dfTok[t]
	return c > 0 && c <= 12
}

func (e *compareEngine) subMatch(a, b string) bool {
	if len(a) >= 4 && len(b) >= 4 {
		if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
			var longer, shorter string
			if len(a) >= len(b) {
				longer, shorter = a, b
			} else {
				longer, shorter = b, a
			}
			rest := longer[len(shorter):]
			if len(rest) >= 3 && !prefixOK.Has(rest) && e.masterVocab.Has(rest) {
				return false
			}
			return true
		}
		if absInt(len(a)-len(b)) > 2 {
			return false
		}
		if strings.Contains(a, b) || strings.Contains(b, a) {
			return true
		}
		if minI(len(a), len(b)) >= 6 && levDistFast(e.lev, a, b) <= 1 {
			return true
		}
	}
	return false
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type overlapDetail struct {
	ov    float64
	missK []string
	missM []string
}

func (e *compareEngine) overlapDetail(keyWords []string, en *masterEntry, plProdFlat *strSet) overlapDetail {
	var ov float64
	var missK []string
	matchedM := newSet()
	for _, k := range keyWords {
		if prepDrop.Has(k) {
			continue
		}
		if en.wordsSet.Has(k) {
			ov += e.wOf(k)
			matchedM.Add(k)
			continue
		}
		f := ""
		for _, mm := range en.words {
			if e.subMatch(k, mm) {
				f = mm
				break
			}
		}
		if f != "" {
			ov += 0.5 * (e.wOf(k) + e.wOf(f)) / 2
			matchedM.Add(f)
		} else {
			missK = append(missK, k)
		}
	}
	var missM []string
	for _, mm := range en.words {
		if matchedM.Has(mm) || (plProdFlat != nil && plProdFlat.Has(mm)) {
			continue
		}
		found := false
		for _, k := range keyWords {
			if e.subMatch(mm, k) {
				found = true
				break
			}
		}
		if !found {
			missM = append(missM, mm)
		}
	}
	return overlapDetail{ov: ov, missK: missK, missM: missM}
}

type lookupHit struct {
	status string
	review bool
	sym    float64
	m      *mlItem
	cand   *mlItem
	note   string
}

var (
	rePaddy      = regexp.MustCompile(`(?i)paddy[\s-]?w[ha]c?k`)
	reNuche      = regexp.MustCompile(`(?i)nuch(?:ae|a)`)
	rePlHolder   = regexp.MustCompile(`(?i)blm\s*ada\s*link`)
	reBoneIn     = regexp.MustCompile(`(?i)\bbone[\s-]*in\b`)
	reBoneOut    = regexp.MustCompile(`(?i)\bbone\s*out\b`)
	reWithBone   = regexp.MustCompile(`(?i)\bwith\s+bone\b`)
	reBoneLess   = regexp.MustCompile(`(?i)\bbone[\s-]*less\b`)
	reBI         = regexp.MustCompile(`(?i)\bb\s*[/ -]\s*i\b`)
	reST         = regexp.MustCompile(`(?i)\bs\s*[/ -]\s*t\b`)
	reNaturafrig = regexp.MustCompile(`(?i)naturafrig`)
	reIndustrial = regexp.MustCompile(`(?i)industrial meat`)
	reNeckTrim   = regexp.MustCompile(`(?i)\bneck\s*trim\b|\bnecktrim\b`)
	reCM         = regexp.MustCompile(`(?i)\bcm\b`)
	reHM         = regexp.MustCompile(`(?i)\bhm\b`)
	reDiaphragm  = regexp.MustCompile(`(?i)diaphragm`)
	reVacPack    = regexp.MustCompile(`(?i)\bvac(?:uum)?[\s-]*pack(?:ing)?\b`)
	reGradePair  = regexp.MustCompile(`(?i)\b[a-z]\s*/\s*[a-z]\b`)
	reShortLoin  = regexp.MustCompile(`(?i)\bshort[\s-]+loins?\b`)
	reStripLoin  = regexp.MustCompile(`(?i)\bstrip[\s-]+loins?\b`)
	reOxZero     = regexp.MustCompile(`(?i)\box(\d)`)
	reNumUnit    = regexp.MustCompile(`^\d+(?:kg|g|gr|lbs?|ml|oz)$`)
	reRemoveCMHM = regexp.MustCompile(`(?i)\b(?:cm|hm)\b`)
)

func clearSyn(s string) string {
	s = rePaddy.ReplaceAllString(s, " ligamentum ")
	s = reVacPack.ReplaceAllString(s, " ")
	s = reShortLoin.ReplaceAllString(s, " SHORTLOIN ")
	s = reStripLoin.ReplaceAllString(s, " STRIPLOIN ")
	s = reOxZero.ReplaceAllString(s, "0X$1")
	s = reST.ReplaceAllString(s, " ")
	return reNuche.ReplaceAllString(s, " ligamentum ")
}

func dropBonePhrase(s string) string {
	s = reBoneIn.ReplaceAllString(s, " ")
	s = reBoneOut.ReplaceAllString(s, " ")
	s = reWithBone.ReplaceAllString(s, " ")
	s = reBI.ReplaceAllString(s, " ")
	return reBoneLess.ReplaceAllString(s, "boneless")
}

func (e *compareEngine) lookupMaster(r *plItem) lookupHit {
	rawNama := normME(normVL(e.aliases.apply(plur(r.NamaItem))))
	itemDesc := normME(normVL(e.aliases.apply(plur(r.ItemDesc))))
	gab := normME(normVL(e.aliases.apply(plur(r.Gabungan))))

	rawNamaT := clearSyn(rawNama)
	itemDescT := clearSyn(itemDesc)
	gabT := clearSyn(gab)
	isPlaceholder := rePlHolder.MatchString(rawNama)

	rawNamaB := dropBonePhrase(rawNamaT)
	itemDescB := dropBonePhrase(itemDescT)
	gabB := dropBonePhrase(gabT)

	keyTokSet := newSet()
	for _, t := range tokenize2(normStr(rawNamaB) + " " + normStr(itemDescB) + " " + normStr(gabB)) {
		keyTokSet.Add(t)
	}
	if reNaturafrig.MatchString(r.Brand) && reIndustrial.MatchString(rawNamaT+" "+itemDescT+" "+gabT) {
		rewriteIm := func(s string) string {
			out := s
			switch {
			case reNeckTrim.MatchString(out):
				out = reIndustrial.ReplaceAllString(out, "NECK TRIM")
			case reCM.MatchString(out):
				out = reIndustrial.ReplaceAllString(out, "CHEEK MEAT")
			case reHM.MatchString(out):
				out = reIndustrial.ReplaceAllString(out, "HEAD MEAT")
			case reDiaphragm.MatchString(out):
				out = reIndustrial.ReplaceAllString(out, "DIAPHRAGM TRIMM")
			}
			return reRemoveCMHM.ReplaceAllString(out, " ")
		}
		keyTokSet = newSet()
		for _, t := range tokenize2(normStr(rewriteIm(rawNamaT)) + " " + normStr(rewriteIm(itemDescT)) + " " + normStr(rewriteIm(gabT))) {
			keyTokSet.Add(t)
		}
	}
	if isPlaceholder {
		filtered := make([]string, 0, keyTokSet.Len())
		for _, t := range keyTokSet.List() {
			if !placeholderWords.Has(t) {
				filtered = append(filtered, t)
			}
		}
		keyTokSet = newSetFromSlice(filtered)
	}
	if e.ignores != nil && e.ignores.set != nil && e.ignores.set.Len() > 0 {
		filtered := make([]string, 0, keyTokSet.Len())
		for _, t := range keyTokSet.List() {
			if !e.ignores.Has(t) {
				filtered = append(filtered, t)
			}
		}
		keyTokSet = newSetFromSlice(filtered)
	}
	rawBrand := normBrand(r.Brand)
	if aliasCanon, ok := brandAliases[rawBrand]; ok {
		keyTokSet.Add(aliasCanon)
	}
	keyToks := keyTokSet.List()
	keyWords := []string{}
	for _, t := range keyToks {
		if !isNumTok(t) {
			keyWords = append(keyWords, t)
		}
	}
	keyNums := standAloneNums(rawNamaT + " " + itemDescT + " " + gabT)
	keySet := newSetFromSlice(keyWords)
	plImportir := r.importirPL(itemDesc, gab)
	nameExact := normStr(rawNamaT)
	plProds := newSetFromSlice(prodKeys(rawNamaT + " " + itemDesc + " " + r.ItemCode + " " + r.Gabungan))
	plGrade := gradeOf(rawNama + " " + itemDesc + " " + gab)
	plVarN := estVariant(rawNama + " " + itemDesc + " " + gab)
	plEsts := extractEsts(rawNama, itemDesc, gab)
	parts := canonParts(r.Brand)
	partSet := newSetFromSlice(parts)
	plSpec := specTokens(normStr(rawNamaT) + " " + normStr(itemDescT) + " " + normStr(gabT))
	e.ignores.filterSet(plSpec)
	if plGrade != "" {
		plSpec.Delete(plGrade)
	}
	for _, p := range partSet.List() {
		plSpec.Delete(p)
	}
	agree := func(en *masterEntry) bool {
		for _, p := range parts {
			if p == "" {
				continue
			}
			if en.bCanon == p || en.parens.Has(p) || (len(p) >= 4 && strings.Contains(en.descBrandFlat, p)) {
				return true
			}
		}
		return false
	}

	dKeyNorm := normStr(r.Brand) + "|" + normStr(rawNama)
	dKeyRaw := normStr(r.Brand) + "|" + normStr(r.NamaItem)
	decNew := false
	var dec *reviewDecision
	if d, ok := e.decisions[dKeyNorm]; ok {
		dec = d
	} else if d, ok := e.decisions[dKeyRaw]; ok {
		dec = d
	}
	if dec != nil {
		if dec.Decision == "new" {
			decNew = true
		}
		if dec.Decision != "new" && dec.MasterCode != "" {
			if dm, ok := e.masterByCode[dec.MasterCode]; ok {
				if plImportir == "" || dm.importir == "" || dm.importir == plImportir {
					desc := dm.m.ItemDesc
					if desc == "" {
						desc = dm.m.NamaItem
					}
					code := dm.m.ItemCode
					if code == "" {
						code = "-"
					}
					return lookupHit{status: "EXISTING", sym: 1, m: dm.m,
						note: "Sesuai konfirmasi manual: " + sliceRunes(desc, 70) + " [" + code + "]"}
				}
			}
		}
	}

	// Direct item code match from packing list
	if r.ItemCode != "" && r.ItemCode != "-" {
		normIC := normKode(r.ItemCode)
		if me, ok := e.masterByCode[r.ItemCode]; ok && me != nil {
			if plImportir == "" || me.importir == "" || me.importir == plImportir {
				desc := orName(me.m.ItemDesc, me.m.NamaItem)
				return lookupHit{status: "EXISTING", sym: 1, m: me.m,
					note: "Cocok item code PL (" + r.ItemCode + ") -> " + sliceRunes(desc, 70) + " [" + me.m.ItemCode + "]"}
			}
		}
		if me, ok := e.masterByCode[normIC]; ok && me != nil {
			if plImportir == "" || me.importir == "" || me.importir == plImportir {
				desc := orName(me.m.ItemDesc, me.m.NamaItem)
				return lookupHit{status: "EXISTING", sym: 1, m: me.m,
					note: "Cocok item code PL (" + r.ItemCode + ") -> " + sliceRunes(desc, 70) + " [" + me.m.ItemCode + "]"}
			}
		}
	}

	plCodes := extractCodes2(rawNamaT + " " + itemDesc + " " + r.ItemCode + " " + r.Gabungan)
	if plCodes.Len() > 0 {
		var codeBestBrand, codeBestBrandExact *masterEntry
		var codeBestBrandNoImp, codeBestBrandExactNoImp *masterEntry
		rupper := ""
		brandParen := func(en *masterEntry) bool {
			raw := en.m.ItemDesc + " " + en.m.NamaItem + " " + en.m.Gabungan
			u := strings.ToUpper(raw)
			if rupper == "" {
				rupper = strings.ToUpper("(" + r.Brand + ")")
			}
			return strings.Contains(u, rupper)
		}
		inCands := newSet()
		for _, c := range plCodes.List() {
			if l, ok := e.snjCodeIndex[c]; ok {
				for _, ent := range l {
					inCands.Add(strconv.Itoa(ent.m.ID))
				}
			}
		}
		for _, ent := range e.entries {
			if !inCands.Has(strconv.Itoa(ent.m.ID)) {
				continue
			}
			if plImportir != "" && ent.importir != "" && ent.importir != plImportir {
				continue
			}
			impOK := plImportir == "" || ent.importir == plImportir
			if brandParen(ent) {
				if impOK {
					if codeBestBrandExact == nil {
						codeBestBrandExact = ent
					}
				} else if codeBestBrandExactNoImp == nil {
					codeBestBrandExactNoImp = ent
				}
			} else if agree(ent) {
				if impOK {
					if codeBestBrand == nil {
						codeBestBrand = ent
					}
				} else if codeBestBrandNoImp == nil {
					codeBestBrandNoImp = ent
				}
			}
		}
		em := codeBestBrandExact
		if em == nil {
			em = codeBestBrandExactNoImp
		}
		if em == nil && !decNew {
			em = codeBestBrand
		}
		if em == nil && !decNew {
			em = codeBestBrandNoImp
		}
		if em != nil {
			desc := em.m.ItemDesc
			if desc == "" {
				desc = em.m.NamaItem
			}
			code := em.m.ItemCode
			if code == "" {
				code = "-"
			}
			note := "Kode kepemilikan SNJ (" + strings.Join(plCodes.List(), ",") + ") -> " + sliceRunes(desc, 70) + " [" + code + "]"
			if decNew {
				note = "Item sudah ada di master, keputusan manual lama diabaikan: " + sliceRunes(desc, 70) + " [" + code + "]"
			}
			return lookupHit{status: "EXISTING", sym: 1, m: em.m, note: note}
		}
	}

	if decNew {
		return lookupHit{status: "NEW", note: "Sudah divalidasi manual: item baru"}
	}

	contentWords := []string{}
	for _, t := range keyWords {
		if partSet.Has(t) {
			continue
		}
		inParts := false
		for _, pp := range parts {
			if strings.Contains(pp, t) {
				inParts = true
				break
			}
		}
		if !inParts {
			contentWords = append(contentWords, t)
		}
	}
	if len(contentWords) == 0 {
		brand := r.Brand
		if brand == "" {
			brand = "-"
		}
		note := "Nama item kosong di file PL (gabungan cuma \"" + gab + "\") — tidak bisa dicocokkan, perbaiki file PL"
		if isPlaceholder {
			note = "Data PL rusak: nama item \"" + rawNama + "\" placeholder — perbaiki file PL"
		}
		return lookupHit{status: "NEW", note: note}
	}

	importirOk := func(en *masterEntry) bool {
		if plImportir == "" || en.importir == "" {
			return true
		}
		return en.importir == plImportir
	}

	seen := newSet()
	var cands []*masterEntry
	pushCand := func(en *masterEntry) {
		k := strconv.Itoa(en.m.ID)
		if seen.Has(k) {
			return
		}
		seen.Add(k)
		if importirOk(en) {
			cands = append(cands, en)
		}
	}
	for _, t := range keyWords {
		if list, ok := e.tokenIndex[t]; ok {
			for _, en := range list {
				pushCand(en)
			}
		}
	}
	for _, p := range parts {
		if list, ok := e.brandIndex[p]; ok {
			for _, en := range list {
				pushCand(en)
			}
		}
	}
	if len(cands) == 0 {
		for _, en := range e.entries {
			if importirOk(en) {
				cands = append(cands, en)
			}
		}
	}
	if len(cands) == 0 {
		imp := plImportir
		if imp == "" {
			imp = "-"
		}
		return lookupHit{status: "NEW", note: "Tidak ada kandidat di ML (filter importir " + imp + ")"}
	}

	plBrandStop := newSetFromSlice(filterLen3(splitAlphaNum(strings.ToLower(r.Brand))))
	plContent := []string{}
	for _, t := range keyWords {
		if prepDrop.Has(t) || partSet.Has(t) || plBrandStop.Has(t) || len(t) < 4 {
			continue
		}
		plContent = append(plContent, t)
	}
	cCandCache := map[string][]string{}
	candContent := func(en *masterEntry) []string {
		if c, ok := cCandCache[strconv.Itoa(en.m.ID)]; ok {
			return c
		}
		cC := []string{}
		for _, t := range en.words {
			if prepDrop.Has(t) || partSet.Has(t) || en.bStop.Has(t) || len(t) < 4 || isNumTok(t) {
				continue
			}
			cC = append(cC, t)
		}
		cCandCache[strconv.Itoa(en.m.ID)] = cC
		return cC
	}
	compatible := func(en *masterEntry) bool {
		if len(plContent) == 0 {
			return true
		}
		cC := candContent(en)
		if len(cC) == 0 {
			return true
		}
		for _, a := range plContent {
			for _, b := range cC {
				if a == b || e.subMatch(a, b) || (len(a) >= 4 && len(b) >= 4 &&
					(strings.HasPrefix(a, b) || strings.HasPrefix(b, a))) {
					return true
				}
			}
		}
		return false
	}
	revCompat := func(en *masterEntry) bool {
		cC := candContent(en)
		if len(cC) == 0 {
			return true
		}
		for _, t := range cC {
			for _, k := range keyWords {
				if k == t || e.subMatch(k, t) || strings.HasPrefix(k, t) || strings.HasPrefix(t, k) {
					return true
				}
			}
		}
		return false
	}
	plStar := starGrades(rawNama + " " + itemDesc + " " + gab)
	gradeOk := func(en *masterEntry) bool {
		if en.star.Len() > 0 {
			if en.star.Len() != plStar.Len() {
				return false
			}
			for _, g := range plStar.List() {
				if !en.star.Has(g) {
					return false
				}
			}
			return true
		}
		if en.grade != "" && plStar.Len() == 1 {
			return en.grade == strings.ToLower(plStar.List()[0])
		}
		return false
	}
	pool := cands
	if plStar.Len() > 0 {
		kept := []*masterEntry{}
		for _, en := range pool {
			if gradeOk(en) && compatible(en) {
				kept = append(kept, en)
			}
		}
		if len(kept) == 0 {
			return lookupHit{status: "NEW",
				note: "Grade " + joinStar(plStar.List()) + " di PL tidak ada master dengan grade yang sama"}
		}
		pool = kept
	}
	if len(plContent) > 0 {
		full := []*masterEntry{}
		for _, en := range pool {
			cC := candContent(en)
			if len(cC) == 0 {
				continue
			}
			okAll := true
			for _, a := range plContent {
				hit := false
				for _, b := range cC {
					if a == b || e.subMatch(a, b) || strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
						hit = true
						break
					}
				}
				if !hit {
					okAll = false
					break
				}
			}
			if okAll {
				full = append(full, en)
			}
		}
		if len(full) > 0 {
			hasAgreeInPool := false
			for _, en := range pool {
				if agree(en) {
					hasAgreeInPool = true
					break
				}
			}
			if hasAgreeInPool {
				fullAgree := []*masterEntry{}
				for _, en := range full {
					if agree(en) {
						fullAgree = append(fullAgree, en)
					}
				}
				if len(fullAgree) > 0 {
					pool = fullAgree
				} else {
					sameBrand := []*masterEntry{}
					for _, en := range pool {
						if agree(en) {
							sameBrand = append(sameBrand, en)
						}
					}
					if len(sameBrand) > 0 {
						pool = sameBrand
					}
				}
			} else {
				pool = full
			}
		}
	}

	if nameExact != "" && !isPlaceholder {
		for _, en := range pool {
			if en.desc == nameExact || en.name == nameExact {
				desc := en.m.ItemDesc
				if desc == "" {
					desc = en.m.NamaItem
				}
				code := en.m.ItemCode
				if code == "" {
					code = "-"
				}
				return lookupHit{status: "EXISTING", sym: 1, m: en.m,
					note: "Cocok persis: " + sliceRunes(desc, 70) + " [" + code + "]"}
			}
		}
	}

	plProdFlat := newSet()
	for _, pp := range plProds.List() {
		plProdFlat.Add(normBrand(pp))
	}
	scoreKeys := []string{}
	scoreKeysW := 0.0
	for _, t := range keyWords {
		if prepDrop.Has(t) {
			continue
		}
		scoreKeys = append(scoreKeys, t)
		scoreKeysW += e.wOf(t)
	}

	type candObj struct {
		e           *masterEntry
		sym         float64
		covPL       float64
		covM        float64
		pri         int
		missK       []string
		missM       []string
		sharedNums  int
		isAgree     bool
		prodCompat  bool
		specMiss    []string
		numConflict bool
		varSlot     bool
		varEst      bool
	}
	var best, bestNon *candObj
	wins := func(b, a *candObj) bool {
		if a == nil {
			return true
		}
		if b.isAgree != a.isAgree {
			return b.isAgree
		}
		if b.pri != a.pri {
			return b.pri > a.pri
		}
		if b.sym != a.sym {
			return b.sym > a.sym
		}
		if b.varSlot != a.varSlot {
			return b.varSlot
		}
		if b.varEst != a.varEst {
			return b.varEst
		}
		return len(b.missK) < len(a.missK)
	}

	for _, cand := range pool {
		od := e.overlapDetail(keyWords, cand, plProdFlat)
		covPL := 0.0
		if len(scoreKeys) > 0 {
			covPL = od.ov / scoreKeysW
		}
		ovM := 0.0
		for _, t := range cand.words {
			if prepDrop.Has(t) {
				continue
			}
			if keySet.Has(t) || plProdFlat.Has(t) {
				ovM += e.wOf(t)
			} else {
				for _, k := range keyWords {
					if e.subMatch(k, t) {
						ovM += 0.5 * (e.wOf(t) + e.wOf(k)) / 2
						break
					}
				}
			}
		}
		covM := 0.0
		if cand.wSumWords != 0 {
			covM = ovM / cand.wSumWords
		}
		sym := maxF(covPL, covM)
		pri := 0
		if plProds.Len() > 0 {
			for _, pp := range cand.prods.List() {
				if plProds.Has(pp) {
					pri = 1
					sym = maxF(sym, 0.8)
					break
				}
			}
		}
		sharedNums := 0
		for _, kn := range keyNums.List() {
			if cand.nums.Has(kn) {
				sharedNums++
				continue
			}
			for _, cn := range cand.numsLong.List() {
				if len(cn) >= 5 && len(kn) >= 3 && (strings.Contains(cn, kn) || strings.Contains(kn, cn)) {
					sharedNums++
					break
				}
			}
		}
		isAgree := agree(cand)
		prodCompat := compatible(cand)
		var specMiss []string
		for _, t := range plSpec.Values() {
			if !cand.specSet.Has(t) {
				specMiss = append(specMiss, t)
			}
		}
		gradeAdj := 0.0
		if plGrade != "" {
			if cand.gradeAll != "" {
				if plGrade == cand.gradeAll {
					gradeAdj = 0.1
				} else {
					gradeAdj = -0.4
				}
			} else {
				gradeAdj = -0.3
			}
		}
		varAdj := 0.0
		if plVarN != "" {
			sN, eN := cand.slotNums, cand.estNums
			switch {
			case sN.Has(plVarN):
				varAdj = 0
			case eN.Has(plVarN):
				varAdj = -0.05
			case sN.Len() > 0 || eN.Len() > 0:
				varAdj = -0.6
			default:
				varAdj = -0.25
			}
		}
		candNonEstNums := newSet()
		for _, cn := range cand.nums.List() {
			if !cand.estNums.Has(cn) {
				candNonEstNums.Add(cn)
			}
		}
		plNonEstNums := newSet()
		for _, pn := range keyNums.List() {
			if !cand.estNums.Has(pn) && pn != plVarN {
				plNonEstNums.Add(pn)
			}
		}
		if keyNums.Len() > 0 {
			if cand.nums.Len() > 0 {
				if sharedNums == 0 && candNonEstNums.Len() > 0 && plNonEstNums.Len() > 0 {
					varAdj += -0.35
				}
			} else if plVarN == "" && plNonEstNums.Len() > 0 {
				varAdj += -0.15
			}
		}
		if !prodCompat {
			sharedNums = 0
			varAdj = 0
		}
		numConflict := plNonEstNums.Len() > 0 && candNonEstNums.Len() > 0 && sharedNums == 0
		adj := minF(0.2, float64(sharedNums)*0.1)
		if isAgree {
			adj += 0.08
		}
		if len(specMiss) > 0 {
			adj -= 0.45
		}
		sym = minF(1, maxF(0, sym+adj+gradeAdj+varAdj))
		candVarSlot := false
		candVarEst := false
		if prodCompat {
			for _, s := range cand.slotNums.List() {
				if keyNums.Has(s) || s == plVarN {
					candVarSlot = true
					break
				}
			}
			for _, en2 := range cand.estNums.List() {
				if keyNums.Has(en2) || en2 == plVarN {
					candVarEst = true
					break
				}
			}
		}
		co := &candObj{e: cand, sym: sym, covPL: covPL, covM: covM, pri: pri, missK: od.missK, missM: od.missM,
			sharedNums: sharedNums, isAgree: isAgree, prodCompat: prodCompat, specMiss: specMiss,
			numConflict: numConflict, varSlot: candVarSlot, varEst: candVarEst}
		if isAgree {
			if wins(co, best) {
				best = co
			}
		} else if best == nil && wins(co, bestNon) {
			bestNon = co
		}
	}
	if best == nil {
		best = bestNon
	}
	if best == nil {
		return lookupHit{status: "NEW", note: "Tidak ada kandidat di ML"}
	}
	be := best.e
	desc := be.m.ItemDesc
	if desc == "" {
		desc = be.m.NamaItem
	}
	short := sliceRunes(desc, 70)
	code := be.m.ItemCode
	if code == "" {
		code = "-"
	}
	sym := best.sym

	keyBoneless := keySet.Has("boneless")
	mBoneless := be.wordsSet.Has("boneless")
	boneWord := keySet.Has("bone") || be.wordsSet.Has("bone")
	boneConflict := boneWord && (keyBoneless != mBoneless)
	keyLean := keySet.Has("boneless") || keySet.Has("lean")
	keyFat := keySet.Has("fat")
	mLean := be.wordsSet.Has("boneless") || be.wordsSet.Has("lean")
	mFat := be.wordsSet.Has("fat")
	fatConflict := (keyFat != mFat) && (keyLean || mLean)

	brandWords := newSet()
	for _, p := range partSet.List() {
		brandWords.Add(p)
	}
	brandWords.Add(be.bCanon)
	for _, bc := range e.allBrandCanons.List() {
		brandWords.Add(bc)
	}
	contentOverlap := false
	for _, k := range keyWords {
		if partSet.Has(k) {
			continue
		}
		if be.wordsSet.Has(k) {
			contentOverlap = true
			break
		}
		for _, mm := range be.words {
			if e.subMatch(k, mm) {
				contentOverlap = true
				break
			}
		}
		if contentOverlap {
			break
		}
	}
	var specMissK, specMissM []string
	for _, t := range best.missK {
		if e.isSpecific(t) && !brandWords.Has(t) && !descriptors.Has(t) {
			specMissK = append(specMissK, t)
		}
	}
	for _, t := range best.missM {
		if e.isSpecific(t) && !brandWords.Has(t) && !isNumTok(t) {
			specMissM = append(specMissM, t)
		}
	}

	brand := r.Brand
	brandDash := brand
	if brandDash == "" {
		brandDash = "-"
	}

	if best.isAgree {
		wrongProduct := len(plContent) > 0 && (!best.prodCompat || !revCompat(be))
		specConflict := len(best.specMiss) > 0
		numConflict := best.numConflict
		if os.Getenv("MC_DEBUG_STAR") != "" && wrongProduct {
			b, _ := json.Marshal(map[string]any{
				"nama": rawNama, "brand": r.Brand, "plContent": plContent, "keyWords": keyWords,
				"cand": candContent(be), "prodCompat": best.prodCompat, "rev": revCompat(be),
			})
			fmt.Println("[WRONGPROD]", string(b))
		}
		if wrongProduct {
			cc := "-"
			if len(plContent) > 0 {
				join := []string{}
				if len(plContent) > 2 {
					join = plContent[:2]
				} else {
					join = plContent
				}
				cc = strings.Join(join, " ")
			}
			return lookupHit{status: "NEW", sym: sym,
				note: "Produk PL (\"" + cc + "\") tidak ada di master " + brandDash + "; kandidat terdekat: " + short + " [" + code + "]"}
		}

		if hit := e.cutGate(r, be, rawNama, plSpec, plEsts); hit != nil {
			return *hit
		}
		if sym >= tAuto && contentOverlap && len(specMissK) == 0 && !boneConflict && !fatConflict && !specConflict && !numConflict {
			return lookupHit{status: "EXISTING", sym: sym, m: be.m, note: "Cocok: " + short + " [" + code + "]"}
		}
		if (boneConflict || fatConflict || specConflict || numConflict) || (sym >= tReview && len(specMissK) == 0) {
			reasons := []string{}
			if len(specMissK) > 0 {
				reasons = append(reasons, "kata \""+headN(specMissK, 2, ", ")+"\" di PL tak ada di master")
			}
			if len(specMissM) > 0 {
				reasons = append(reasons, "master lebih spesifik: \""+headN(specMissM, 2, ", ")+"\"")
			}
			if boneConflict {
				reasons = append(reasons, "bone vs boneless beda")
			}
			if fatConflict {
				reasons = append(reasons, "fat vs lean beda")
			}
			if specConflict {
				reasons = append(reasons, "spesifikasi beda: \""+headN(best.specMiss, 3, ", ")+"\" di PL tak ada di master")
			}
			if numConflict {
				reasons = append(reasons, "nomor beda: PL "+strings.Join(keyNums.List(), ",")+" vs master "+strings.Join(be.nums.List(), ","))
			}
			if len(plEsts) > 0 && len(be.estList) > 0 && strings.Join(plEsts, ",") != strings.Join(be.estList, ",") {
				reasons = append(reasons, "EST beda")
			}
			rs := "kemiripan rendah"
			if len(reasons) > 0 {
				rs = headN(reasons, 2, "; ")
			}
			return lookupHit{status: "NEW", review: true, sym: sym, cand: be.m,
				note: "Mirip kandidat di master, tapi perlu dicek: " + rs}
		}
		kunci := "kunci"
		if len(specMissK) > 0 {
			join := specMissK
			if len(join) > 2 {
				join = join[:2]
			}
			kunci = strings.Join(join, "\", \"")
		}
		if sym >= tAuto && contentOverlap && short != "" && code != "" {
			return lookupHit{status: "NEW", review: true, sym: sym, cand: be.m,
				note: "Kandidat kuat di master (" + short + " [" + code + "]), tapi kata \"" + kunci + "\" di PL tak ada di master; perlu dicek"}
		}
		return lookupHit{status: "NEW", sym: sym,
			note: "Belum ada produk ber-brand " + brandDash + " yang cocok di ML (kata \"" + kunci + "\" tak ada)"}
	}

	brandKnown := false
	for _, pp := range parts {
		if _, ok := e.brandIndex[pp]; ok {
			brandKnown = true
			break
		}
	}
	why := ""
	if !brandKnown {
		why = "Brand " + brandDash + " tidak ada di ML"
	} else if short != "" && code != "" {
		why = "Belum ada item brand " + brandDash + " utk produk ini di ML (terdekat: " + short + " [" + code + "] = beda brand)"
	} else {
		why = "Belum ada item brand " + brandDash + " utk produk ini di ML"
	}
	return lookupHit{status: "NEW", sym: sym, note: why}
}

func joinStar(list []string) string {
	out := make([]string, 0, len(list))
	for _, g := range list {
		out = append(out, "*"+g+"*")
	}
	return strings.Join(out, " ")
}

func headN(list []string, n int, sep string) string {
	if len(list) > n {
		list = list[:n]
	}
	return strings.Join(list, sep)
}
