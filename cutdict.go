package main

import (
	"regexp"
	"sort"
	"strings"
)

var reNonAlnumCut = regexp.MustCompile(`[^A-Z0-9]+`)

func cutKey(s string) string {
	return reNonAlnumCut.ReplaceAllString(strings.ToUpper(strings.TrimSpace(s)), "")
}

func cutLabel(k string) string {

	rep := []struct{ from, to string }{
		{"SHORTLOIN", "Short Loin"}, {"STRIPLOIN", "Striploin"},
		{"TENDERLOIN", "Tenderloin"}, {"TENDERCROSS", "Tender Cross"},
		{"TOPSIRLOIN", "Top Sirloin"}, {"BOTTOMROUND", "Bottom Round"},
		{"TOPROUND", "Top Round"}, {"RIBEYE", "Rib Eye"},
		{"RIBFINGER", "Rib Finger"}, {"BACKRIB", "Backrib"},
		{"SHORTRIB", "Short Rib"}, {"SPARERIB", "Spare Rib"},
		{"CUBEROLL", "Cube Roll"}, {"EYEROUND", "Eye Round"},
		{"RUMPSTEAK", "Rump Steak"}, {"PORTERHOUSE", "Porterhouse"},
		{"TOMAHAWK", "Tomahawk"}, {"HANGINGTENDER", "Hanging Tender"},
		{"MIXEDBONE", "Mixed Bone"},
	}
	for _, r := range rep {
		if strings.Contains(k, r.from) {
			return strings.Replace(k, r.from, r.to, 1)
		}
	}

	return strings.ToLower(k)
}

var cutGroups = map[string]string{

	"SHORTLOIN": "loin", "STRIPLOIN": "loin", "TENDERLOIN": "loin",
	"SIRLOIN": "loin", "TOPSIRLOIN": "loin", "TOPSIRLOINBUTT": "loin",
	"BUTTLOIN": "loin", "LONGLOIN": "loin", "RUMPSTEAK": "loin",
	"PORTERHOUSE": "loin", "TOMAHAWK": "loin", "TENDERLOINPEELED": "loin",
	"BONELESSHQLEG": "loin", "LEG": "loin", "LEGBONE": "loin", "LEGCHUMP": "loin",

	"RIBEYE": "rib", "RIBFINGER": "rib", "RIBCAP": "rib", "RIBCUT": "rib",
	"BACKRIB": "rib", "SHORTRIB": "rib", "SPARERIB": "rib", "BUTTONRIB": "rib",
	"RIBPLATE": "rib", "SHORTPLATE": "rib", "RIBPLATEBONES": "rib",
	"BACKBONE": "rib", "BACKBONELOKAL": "rib", "RIB": "rib", "RIBBLADE": "rib",
	"RIBMEAT": "rib", "RIBEND": "rib", "RIBENDMEAT": "rib",

	"TOPROUND": "round", "BOTTOMROUND": "round", "RUMP": "round",
	"CUBEROLL": "round", "EYEROUND": "round", "TOPSIDE": "round",
	"SILVERSIDE": "round", "SHANK": "round", "ROSTBIFF": "round",
	"DRUMP": "round", "RUMPCAP": "round", "RUMPHEART": "round",
	"SHIN": "round", "SHINSHANK": "round", "HINDSHANK": "round",
	"FQROLL": "round", "ROLL": "round", "THICKSIDE": "round",

	"BLADE": "chuck", "CHUCK": "chuck", "CHUCKROLL": "chuck",
	"CHUCKTENDER": "chuck", "SHOULDER": "chuck", "SCAPULA": "chuck",
	"OYSTERBLADE": "chuck", "BOLARBLADE": "chuck", "FULLCHUCK": "chuck",
	"CHUCKCREST": "chuck", "CHUCKBACKRIB": "chuck", "CHUCKVERTEBRAEBONES": "chuck",

	"BRISKET": "brisket", "BRISKETBONE": "brisket", "BRISKETPLATE": "brisket",
	"BRISKETPOINT": "brisket", "BRISKETPOINTEND": "brisket", "BRISKETNAVELLEND": "brisket",
	"BRISKETFAT": "brisket", "POINTEND": "brisket",

	"FLANK": "flank", "FLAP": "flank", "THICKFLANK": "flank", "THINFLANK": "flank",
	"SKIRT": "flank", "INSIDESKIRT": "flank", "THICKSKIRT": "flank",
	"THINSKIRT": "flank", "OUTSIDEFLAT": "flank",

	"CARCASS": "carcass", "LIVESTOCK": "carcass", "KARKAS": "carcass",
	"CHINEBONE": "carcass", "AITCHBONE": "carcass", "BACKSTRAP": "carcass",
	"ROBBEDFOREQUARTER": "carcass", "ROBBEDHINDQUARTER": "carcass",
	"FOREQUARTER": "carcass", "HINDQUARTER": "carcass",

	"TONGUE": "offal", "TONGUEROOT": "offal", "TONGUEMEAT": "offal",
	"HEART": "offal", "LIVER": "offal", "LUNGS": "offal", "KIDNEY": "offal",
	"SINEW": "offal", "SINEWS": "offal", "TENDON": "offal", "FAT": "offal",
	"SKIN": "offal", "HEADMEAT": "offal", "FEET": "offal", "OXTAIL": "offal",
	"PATELLA": "offal", "LIPS": "offal", "BONE": "offal", "BONES": "offal",
	"KNUCKLE": "offal", "CHEECK": "offal", "TRIPE": "offal", "THROAT": "offal",
	"NECK": "offal", "NECKBONE": "offal", "NECKMEAT": "offal", "NECKTRIM": "offal",
	"TRIMMING": "offal", "MEMBRANE": "offal", "CARTILAGE": "offal",
	"INDUSTRIALMEAT": "offal", "INDUSTRIALTRIMMING": "offal",
	"CHEUK": "offal", "SHORTRIBLOKAL": "rib",
}

func cutGroupOf(key string) string { return cutGroups[key] }

var cutPrefix3 = func() map[string]bool {
	m := map[string]bool{}
	for k := range cutGroups {
		if len(k) >= 3 {
			m[k[:3]] = true
		}
	}
	return m
}()

func cutKnown(key string) bool { _, ok := cutGroups[key]; return ok }

func aliasFromML(names []string) map[string]string {
	variants := map[string][]string{}
	for _, n := range names {
		k := cutKey(n)
		if k == "" {
			continue
		}
		variants[k] = append(variants[k], n)
	}
	out := map[string]string{}
	for _, list := range variants {
		if len(list) < 2 {
			continue
		}

		best := list[0]
		for _, n := range list[1:] {
			if len(n) < len(best) || (len(n) == len(best) && n < best) {
				best = n
			}
		}
		for _, n := range list {
			if n != best {
				out[n] = best
			}
		}
	}
	return out
}

func cutNameFromDesc(desc string) string {
	s := desc
	if i := strings.Index(s, "---"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

type dictDraftGroup struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	Variants []string `json:"variants"`
	Count    int      `json:"count"`
	Known    bool     `json:"known"`
	Code     string   `json:"code"`
}

type dictDraft struct {
	Groups    []dictDraftGroup `json:"groups"`
	NoGroup   []dictDraftGroup `json:"noGroup"`
	Noise     []string         `json:"noise"`
	TotalName int              `json:"totalName"`
	UniqueKey int              `json:"uniqueKey"`
	Collapsed int              `json:"collapsed"`
}

func deriveDictDraft(descs []string) dictDraft {
	names := make([]string, 0, len(descs))
	for _, d := range descs {
		if n := cutNameFromDesc(d); n != "" {
			names = append(names, n)
		}
	}
	byKey := map[string][]string{}
	for _, n := range names {
		k := cutKey(n)
		if k == "" {
			continue
		}
		byKey[k] = append(byKey[k], n)
	}

	d := dictDraft{TotalName: len(names), UniqueKey: len(byKey)}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		variants := dedupStrings(byKey[k])
		if len(variants) > 1 {
			d.Collapsed += len(variants) - 1
		}
		g := dictDraftGroup{
			Key:      k,
			Label:    strings.ToUpper(variants[0]),
			Group:    cutGroupOf(k),
			Variants: variants,
			Count:    len(byKey[k]),
			Known:    cutKnown(k),
		}
		if g.Group == "" {
			d.NoGroup = append(d.NoGroup, g)
		} else {
			d.Groups = append(d.Groups, g)
		}
	}
	return d
}

func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		k := strings.ToUpper(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
