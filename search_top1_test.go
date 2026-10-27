package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var reProdCode = regexp.MustCompile(`(?i)\b(FQ|HQ|CM)\s*[- ]?(\d{1,4})\b`)
var reNonAlnum2 = regexp.MustCompile(`[^A-Z0-9]+`)

func prodLabel(dir string) string {
	d := strings.ReplaceAll(dir, "\\", "/")
	parts := []string{}
	for _, p := range strings.Split(d, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return dir
	}
	return parts[len(parts)-1]
}

func prodBrand(dir string) string {
	lab := prodLabel(dir)
	up := strings.ToUpper(lab)
	i := strings.Index(up, "BUFFALO")
	if i <= 0 {
		return ""
	}
	s := lab[:i]
	s = strings.TrimRight(s, " -_\t")
	return strings.TrimSpace(s)
}

func flat(s string) string {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "BUFFALO", "")
	return reNonAlnum2.ReplaceAllString(s, "")
}

func TestTopOneAccuracy(t *testing.T) {
	app := loadRealApp(t)
	si := app.getSearchIndex()

	type pair struct{ brand, code string }
	seen := map[pair]int{}
	for _, en := range si.entries {
		lab := prodLabel(en.e.Dir)
		up := strings.ToUpper(lab)
		if !strings.Contains(up, "BUFFALO") {
			continue
		}
		if up != "" && up[0] >= '0' && up[0] <= '9' {
			continue
		}
		m := reProdCode.FindStringSubmatch(up)
		if m == nil {
			continue
		}
		b := prodBrand(en.e.Dir)
		c := strings.ToUpper(m[1] + "-" + m[2])
		if b == "" || len(b) < 3 {
			continue
		}
		seen[pair{b, c}]++
	}
	keys := make([]pair, 0, len(seen))
	for k, v := range seen {
		if v >= 2 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].brand != keys[j].brand {
			return keys[i].brand < keys[j].brand
		}
		return keys[i].code < keys[j].code
	})

	ok, bad, miss, tie := 0, 0, 0, 0
	var badList []string
	for _, k := range keys {
		q := k.brand + " " + k.code
		resp := app.search(q)
		if len(resp.Results) == 0 {
			miss++
			if len(badList) < 30 {
				badList = append(badList, fmt.Sprintf("KOSONG  %-30s (partial=%d)", q, len(resp.Partial)))
			}
			continue
		}
		nb, nc := flat(k.brand), flat(k.code)
		want := func(r int) bool {
			n := flat(resp.Results[r].Dir)
			return strings.Contains(n, nc) && strings.Contains(n, nb)
		}
		if want(0) {
			ok++
			continue
		}
		found := -1
		for r := 1; r < len(resp.Results) && r < 5; r++ {
			if want(r) {
				found = r
				break
			}
		}
		if found > 0 {
			tie++
			if len(badList) < 30 {
				badList = append(badList, fmt.Sprintf("TURUN#%d %-30s top1=%s", found+1, q, trunc(prodLabel(resp.Results[0].Dir), 60)))
			}
			continue
		}
		bad++
		if len(badList) < 30 {
			badList = append(badList, fmt.Sprintf("HILANG  %-30s top1=%s", q, trunc(prodLabel(resp.Results[0].Dir), 60)))
		}
	}
	n := len(keys)
	fmt.Printf("\npasangan brand+kode (>=2 foto): %d\n", n)
	fmt.Printf("top1 benar : %d (%.1f%%)\n", ok, 100*float64(ok)/float64(n))
	fmt.Printf("nyangkut 2-5: %d (%.1f%%)\n", tie, 100*float64(tie)/float64(n))
	fmt.Printf("hilang      : %d (%.1f%%)\n", bad, 100*float64(bad)/float64(n))
	fmt.Printf("kosong      : %d (%.1f%%)\n", miss, 100*float64(miss)/float64(n))
	for _, s := range badList {
		fmt.Println("   " + s)
	}
}
