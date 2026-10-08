package main

import (
	"sort"
	"strings"
)

type cutOutcome string

const (
	outMatch    cutOutcome = "MATCH"
	outNotMatch cutOutcome = "NOT_MATCH"
	outReview   cutOutcome = "REVIEW"
)

type cutVerdict struct {
	Outcome cutOutcome `json:"outcome"`
	Reason  []string   `json:"reason"`

	PlKey    string `json:"plKey"`
	MlKey    string `json:"mlKey"`
	PlBase   string `json:"plBase"`
	MlBase   string `json:"mlBase"`
	PlSuffix string `json:"plSuffix"`
	MlSuffix string `json:"mlSuffix"`
	PlFamily string `json:"plFamily"`
	MlFamily string `json:"mlFamily"`

	SameKey      bool     `json:"sameKey"`
	FamDiff      bool     `json:"famDiff"`
	FamSame      bool     `json:"famSame"`
	SuffixDiff   bool     `json:"suffixDiff"`
	SpecDiff     []string `json:"specDiff"`
	CodeConflict bool     `json:"codeConflict"`
	OtherCode    string   `json:"otherCode,omitempty"`

	ScopeFact  bool   `json:"scopeFact"`
	MlCode     string `json:"mlCode,omitempty"`
	UnknownKey bool   `json:"unknownKey"`
}

func cutVerdictOf(plName, plSpec, mlName, mlSpec string) cutVerdict {
	return cutVerdictAdvanced(plName, plSpec, mlName, mlSpec, "", "")
}

func cutVerdictAdvanced(plName, plSpec, mlName, mlSpec, plCode, mlCode string) cutVerdict {
	return cutVerdictCore(plName, plSpec, mlName, mlSpec, plCode, mlCode)
}

func comparableKode(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	return isDigitRune(a[0]) == isDigitRune(b[0])
}

func isDigitRune(c byte) bool { return c >= '0' && c <= '9' }

func cutVerdictCore(plName, plSpec, mlName, mlSpec, plCode, mlCode string) cutVerdict {
	v := cutVerdict{
		PlKey: normCutName(plName), MlKey: normCutName(mlName),
	}
	v.PlBase, v.PlSuffix = cutKeys(plName)
	v.MlBase, v.MlSuffix = cutKeys(mlName)
	v.PlFamily = cutFamilyRaw(v.PlBase)
	if v.PlFamily == "" {
		v.PlFamily = cutFamilyRaw(v.PlKey)
	}
	v.MlFamily = cutFamilyRaw(v.MlBase)
	if v.MlFamily == "" {
		v.MlFamily = cutFamilyRaw(v.MlKey)
	}
	v.SpecDiff = cutSpecDiff(plName, plSpec, mlName, mlSpec)

	if v.PlKey != "" && v.PlKey == v.MlKey {
		v.SameKey = true
		plHas := len(cutSpecKeys(plName+" "+plSpec)) > 0
		mlHas := len(cutSpecKeys(mlName+" "+mlSpec)) > 0
		if plHas && mlHas {
			plS := joinSet(cutSpecKeys(plName + " " + plSpec))
			mlS := joinSet(cutSpecKeys(mlName + " " + mlSpec))
			if plS != mlS {

				r, _ := checkSpec(v, plName, plSpec, mlName, mlSpec)
				return r
			}
		} else if plHas != mlHas {

			v.Outcome = outReview
			side := "PL"
			got := joinSet(cutSpecKeys(plName + " " + plSpec))
			if !plHas {
				side = "ML"
				got = joinSet(cutSpecKeys(mlName + " " + mlSpec))
			}
			v.Reason = append(v.Reason, "spesifikasi lean hanya ada di "+side+
				" ("+got+"): tidak bisa dipastikan sama atau beda")
			return v
		}
	}

	if c1, c2 := normKode(plCode), normKode(mlCode); c1 != "" && c2 != "" && c1 != c2 {
		if comparableKode(c1, c2) {
			v.CodeConflict = true
			v.Outcome = outNotMatch
			v.Reason = append(v.Reason, "kode barang beda: "+c1+" vs "+c2)
			return v
		}
		v.OtherCode = c1
		v.MlCode = c2
	}

	if v.PlKey == v.MlKey {
		v.Outcome = outMatch
		return v
	}

	if cutDistinct(v.PlKey, v.MlKey) {
		v.ScopeFact = true
		v.Outcome = outNotMatch
		v.Reason = append(v.Reason, "potongan sudah dinyatakan berbeda: "+
			labelCut(v.PlKey, "")+" vs "+labelCut(v.MlKey, ""))
		v.Reason = append(v.Reason, v.catatanSpec())
		return v
	}

	if v.PlBase != "" && v.PlBase == v.MlBase && v.PlSuffix != v.MlSuffix {
		if cutGradeBaseDiketahui(v.PlBase) {
			v.SuffixDiff = true
			v.ScopeFact = true
			v.Outcome = outNotMatch
			v.Reason = append(v.Reason, "suffix grade berbeda: "+
				labelCut(v.PlBase, v.PlSuffix)+" vs "+labelCut(v.MlBase, v.MlSuffix))
			v.Reason = append(v.Reason, v.catatanSpec())
			return v
		}
	}

	v.UnknownKey = true
	v.Outcome = outReview
	v.Reason = append(v.Reason, "belum ada aturan untuk pasangan "+labelCut(v.PlKey, "")+
		" vs "+labelCut(v.MlKey, "")+": perlu dilihat manual")
	return v
}

func (v cutVerdict) catatanSpec() string {
	if len(v.SpecDiff) > 0 {
		return "spesifikasi lean beda: " + strings.Join(v.SpecDiff, " vs ")
	}
	return ""
}

func cutGradeBaseDiketahui(base string) bool {
	for _, group := range cutDistinctGroup {
		for _, k := range group {
			if k != base && strings.HasPrefix(k, base) {
				return true
			}
		}
	}
	return false
}
func checkSpec(v cutVerdict, plName, plSpec, mlName, mlSpec string) (cutVerdict, bool) {
	pl := cutSpecKeys(plName + " " + plSpec)
	ml := cutSpecKeys(mlName + " " + mlSpec)
	plS, mlS := joinSet(dedupStrings(pl)), joinSet(dedupStrings(ml))

	switch {
	case plS == "" && mlS == "":
		return v, false

	case plS == "" || mlS == "":
		v.Outcome = outReview
		side, got := "PL", plS
		if plS == "" {
			side, got = "ML", mlS
		}
		v.Reason = append(v.Reason,
			"spesifikasi lean hanya ada di "+side+" ("+got+"): tidak bisa dipastikan sama atau beda")
		return v, true

	case plS != mlS:
		if cutSpecBerbedaProduk {
			v.Outcome = outNotMatch
			v.Reason = append(v.Reason,
				"spesifikasi lean beda: PL "+plS+" vs ML "+mlS+" (CL/VL adalah pembeda produk)")
			return v, true
		}
		v.Outcome = outReview
		v.Reason = append(v.Reason,
			"spesifikasi lean beda: PL "+plS+" vs ML "+mlS+" (belum diverifikasi)")
		return v, true
	}
	return v, false
}

func cutSpecDiff(plName, plSpec, mlName, mlSpec string) []string {
	a := cutSpecKeys(plName + " " + plSpec)
	b := cutSpecKeys(mlName + " " + mlSpec)
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	sa, sb := joinSet(dedupStrings(a)), joinSet(dedupStrings(b))
	if sa == sb {
		return nil
	}
	return []string{sa, sb}
}

func joinSet(list []string) string {
	parts := make([]string, 0, len(list))
	for _, s := range list {
		if s != "" {
			parts = append(parts, s)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "/")
}

func labelCut(base, suffix string) string {
	if base == "" {
		return "-"
	}
	if suffix == "" {
		return humanizeCut(base)
	}
	return humanizeCut(base) + "-" + suffix
}

func humanizeCut(k string) string {
	if v, ok := cutReadable[k]; ok {
		return v
	}
	return k
}

var cutReadable = map[string]string{
	"SHORTLOIN": "SHORT LOIN", "STRIPLOIN": "STRIPLOIN",
	"TENDERLOIN": "TENDERLOIN", "SIRLOIN": "SIRLOIN",
	"TOPSIRLOIN": "TOP SIRLOIN", "TOPSIRLOINBUTT": "TOP SIRLOIN BUTT",
	"BOTTOMROUND": "BOTTOM ROUND", "TOPROUND": "TOP ROUND",
	"RIBEYE": "RIB EYE", "RIBFINGER": "RIB FINGER", "RIBCAP": "RIB CAP",
	"BACKRIB": "BACKRIB", "SHORTRIB": "SHORT RIB", "SPARERIB": "SPARE RIB",
	"BUTTONRIB": "BUTTON RIB", "RIBPLATE": "RIB PLATE", "SHORTPLATE": "SHORT PLATE",
	"CUBEROLL": "CUBE ROLL", "EYEROUND": "EYE ROUND", "RUMPSTEAK": "RUMP STEAK",
	"BRISKET": "BRISKET", "BRISKETBONE": "BRISKET BONE",
	"BRISKETPLATE": "BRISKET PLATE", "BRISKETPOINT": "BRISKET POINT",
	"BRISKETPOINTEND": "BRISKET POINT END", "BRISKETNAVELLEND": "BRISKET NAVEL END",
	"TONGUE": "TONGUE", "TONGUEROOT": "TONGUE ROOT",
	"HEART": "HEART", "LIVER": "LIVER", "LUNGS": "LUNGS", "KIDNEY": "KIDNEY",
	"NECKTRIM": "NECK TRIM", "NECKBONE": "NECK BONE", "NECKMEAT": "NECK MEAT",
	"HANGINGTENDER": "HANGING TENDER", "CHUCKTENDER": "CHUCK TENDER",
	"CHUCKROLL": "CHUCK ROLL", "CHUCKSHORTRIB": "CHUCK SHORT RIB",
	"BNLSBEEFHANGINGTENDER": "BNLS BEEF HANGING TENDER",
	"HEADMEAT":              "HEAD MEAT", "BEEFPASTRAMI": "BEEF PASTRAMI",
	"BEEFRIBFINGERMEAT": "BEEF RIB FINGER MEAT", "BEEFTONGUE": "BEEF TONGUE",
	"BLADE": "BLADE", "OYSTERBLADE": "OYSTER BLADE", "BOLARBLADE": "BOLAR BLADE",
	"THICKFLANK": "THICK FLANK", "THINFLANK": "THIN FLANK",
	"THICKSKIRT": "THICK SKIRT", "THINSKIRT": "THIN SKIRT",
	"INSIDESKIRT": "INSIDE SKIRT", "OUTSIDEFLAT": "OUTSIDE FLAT",
	"SHIN": "SHIN", "SHINSHANK": "SHIN SHANK", "HINDSHANK": "HIND SHANK",
	"HEARTMEAT": "HEART MEAT", "FLANKSTEAK": "FLANK STEAK",
	"LONGLOIN": "LONG LOIN", "BUTTLOIN": "BUTT LOIN",
}
