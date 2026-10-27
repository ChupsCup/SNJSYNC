package main

import (
	"regexp"
	"strings"
)

var reSpeciesPrefix = regexp.MustCompile(`^(?:BEEF|VEAL|MUTTON|PORK|CHICKEN|DOMBA|SAPI|AYAM)\s+`)

var reCLVL = regexp.MustCompile(`(?i)(?:^|[\s-]+)(\d{1,3})\s*(CL|VL)\b`)

type descParts struct {
	Name    string `json:"name"`
	Brand   string `json:"brand"`
	Spec    string `json:"spec"`
	Est     string `json:"est"`
	Country string `json:"country"`
}

func parseDesc(desc string) descParts {
	var p descParts
	s := strings.TrimSpace(desc)

	if i := strings.Index(s, "("); i >= 0 {
		if j := strings.Index(s[i:], ")"); j >= 0 {
			p.Brand = strings.TrimSpace(s[i+1 : i+j])
			s = strings.TrimSpace(s[:i] + " " + s[i+j+1:])
		}
	}

	if i := strings.Index(s, "---"); i >= 0 {
		rest := s[i+3:]
		p.Spec = strings.TrimSpace(rest)
		s = strings.TrimSpace(s[:i])
	}
	p.Name = strings.TrimSpace(s)

	if i := strings.Index(strings.ToUpper(p.Spec), "EST."); i >= 0 {
		p.Est = strings.TrimSpace(p.Spec[i:])
		p.Spec = strings.TrimSpace(p.Spec[:i])
	}
	up := strings.ToUpper(p.Spec)
	for _, c := range []string{"AU", "US", "ID", "NZ"} {
		if strings.HasSuffix(up, " "+c) || strings.HasSuffix(up, c) {
			p.Country = c
			break
		}
	}
	return p
}

func cutSpecKeys(name string) []string {
	var out []string
	for _, m := range reCLVL.FindAllStringSubmatch(name, -1) {
		out = append(out, strings.ToUpper(strings.Join(m[1:], "")))
	}
	return out
}

func stripCLVL(name string) string {
	s := reCLVL.ReplaceAllString(name, " ")
	s = reSpeciesPrefix.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

var reTrailingGeneric = regexp.MustCompile(`(?i)[\s-]+(MEAT|LOKAL|LOCAL|FRESH|FROZEN|DAGING)$`)

func cutFamilyRaw(key string) string { return cutGroups[key] }

func normCutName(name string) string {
	s := strings.TrimSpace(name)
	s = stripTrailingGeneric(s)
	s = stripCLVL(s)
	s = reSpeciesPrefix.ReplaceAllString(s, "")
	return cutKey(s)
}

func cutKeys(name string) (base, suffix string) {
	n := normCutName(name)
	if n == "" {
		return "", ""
	}
	if cutFamilyRaw(n) != "" {
		return n, ""
	}
	for sl := 1; sl <= 2; sl++ {
		if len(n)-sl < 4 {
			break
		}
		head, suf := n[:len(n)-sl], n[len(n)-sl:]
		if cutFamilyRaw(head) != "" {
			return head, suf
		}
	}
	return n, ""
}

func cutCanon(words []string) string {
	if len(words) == 0 || len(words) > 3 {
		return ""
	}
	var b strings.Builder
	for _, w := range words {
		if w == "" {
			return ""
		}
		for _, r := range w {
			switch {
			case r >= 'a' && r <= 'z':
				b.WriteRune(r - 32)
			case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				b.WriteRune(r)
			}
		}
	}
	k := b.String()
	if len(k) < 3 || len(k) > 24 {
		return ""
	}
	if _, ok := cutGroups[k]; ok {
		return k
	}
	for sl := 1; sl <= 2; sl++ {
		if len(k)-sl < 4 {
			break
		}
		if _, ok := cutGroups[k[:len(k)-sl]]; ok {
			return k[:len(k)-sl]
		}
	}
	return ""
}

func stripTrailingGeneric(name string) string {
	prev := ""
	s := strings.TrimSpace(name)
	for i := 0; i < 3 && s != prev; i++ {
		prev = s
		s = strings.TrimSpace(reTrailingGeneric.ReplaceAllString(s, ""))
	}
	return s
}

var gradeWords = map[string]bool{
	"CL": true, "VL": true, "KG": true, "GM": true, "CM": true, "HM": true,
	"AU": true, "US": true, "ID": true, "NZ": true, "BNLS": true, "BM": true,
	"MEAT": true, "LOKAL": true, "LOCAL": true,
}

func cutFamily(key string) string {
	base, _ := cutKeys(key)
	if base == "" {
		return ""
	}
	if g, ok := cutGroups[base]; ok {
		return g
	}
	return cutGroups[key]
}

func cutKnownKey(key string) bool { return cutFamily(key) != "" }
